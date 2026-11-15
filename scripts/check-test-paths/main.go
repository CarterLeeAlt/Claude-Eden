// check-test-paths rejects statically known absolute filesystem mutations in Go
// tests. It does not execute tests, resolve arbitrary Go code, or sandbox a process.
// Unknown arguments are counted, not certified safe; run tests in the isolated
// workspace environment even after this check passes.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/ast"
	"go/build"
	"go/build/constraint"
	"go/parser"
	"go/printer"
	"go/token"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const tempRoot = "<test-temp>"

// A value is a bounded set of possible strings/integers or a small literal
// container. Missing information remains unknown instead of becoming trusted.
type value struct {
	texts   []string
	numbers []int64
	unknown bool
	tester  bool
	file    bool
	symbol  string
	items   []value
	fields  map[string]value
	fn      *function
	replace []string
}

func text(s string) value { return value{texts: []string{s}} }
func unknown() value      { return value{unknown: true} }

func union(a, b value) value {
	a.texts = append([]string(nil), a.texts...)
	a.numbers = append([]int64(nil), a.numbers...)
	a.items = append(append([]value(nil), a.items...), b.items...)
	if len(a.items) > 128 {
		a.items, a.unknown = a.items[:128], true
	}
	a.unknown = a.unknown || b.unknown
	if a.symbol != b.symbol && (a.symbol != "" || b.symbol != "") {
		a.unknown = true
	}
	a.tester = a.tester || b.tester
	a.file = a.file || b.file
	for _, s := range b.texts {
		if !contains(a.texts, s) {
			a.texts = append(a.texts, s)
		}
	}
	for _, n := range b.numbers {
		found := false
		for _, old := range a.numbers {
			found = found || old == n
		}
		if !found {
			a.numbers = append(a.numbers, n)
		}
	}
	if len(a.texts) > 128 {
		a.texts, a.unknown = nil, true
	}
	if len(a.numbers) > 128 {
		a.numbers, a.unknown = nil, true
	}
	if len(b.fields) != 0 {
		fields := map[string]value{}
		for k, v := range a.fields {
			fields[k] = v
		}
		for k, v := range b.fields {
			fields[k] = union(fields[k], v)
		}
		a.fields = fields
	}
	return a
}

func contains(values []string, wanted string) bool {
	for _, v := range values {
		if v == wanted {
			return true
		}
	}
	return false
}

type scope map[string]value

func clone(s scope) scope {
	out := scope{}
	for k, v := range s {
		out[k] = v
	}
	return out
}

type source struct {
	name      string
	ast       *ast.File
	imports   map[string]string
	linuxOnly bool
}

// A filename constraint or a build expression must require Linux. Merely
// excluding Windows (for example !windows or unix) does not establish that.
func linuxOnlySource(s *source) bool {
	name := strings.TrimSuffix(filepath.Base(s.name), "_test.go")
	for _, suffix := range []string{"_linux", "_linux_amd64", "_linux_arm64"} {
		if strings.HasSuffix(name, suffix) {
			return true
		}
	}
	for _, group := range s.ast.Comments {
		if group.Pos() > s.ast.Package {
			break
		}
		for _, comment := range group.List {
			if !constraint.IsGoBuild(comment.Text) {
				continue
			}
			expr, err := constraint.Parse(comment.Text)
			if err != nil {
				return false
			}
			canBeTrue, _ := withoutLinux(expr)
			return !canBeTrue
		}
	}
	return false
}

// Conservative truth possibilities with linux=false and all other tags unknown.
func withoutLinux(expr constraint.Expr) (canBeTrue, canBeFalse bool) {
	switch x := expr.(type) {
	case *constraint.TagExpr:
		return x.Tag != "linux", true
	case *constraint.NotExpr:
		yes, no := withoutLinux(x.X)
		return no, yes
	case *constraint.AndExpr:
		ly, ln := withoutLinux(x.X)
		ry, rn := withoutLinux(x.Y)
		return ly && ry, ln || rn
	case *constraint.OrExpr:
		ly, ln := withoutLinux(x.X)
		ry, rn := withoutLinux(x.Y)
		return ly || ry, ln && rn
	}
	return true, true
}

type function struct {
	source *source
	params *ast.FieldList
	body   *ast.BlockStmt
	env    scope
}

type finding struct {
	file              string
	line              int
	operation, target string
}
type report struct {
	files      int
	findings   []finding
	unresolved map[string]unresolvedSite
}

type unresolvedSite struct {
	parent                  string
	expressions, candidates []string
}

func unresolvedParent(v value) string {
	if len(v.texts) == 0 {
		return "parent-unknown"
	}
	category := ""
	for _, candidate := range v.texts {
		p := strings.ReplaceAll(candidate, `\`, "/")
		index := strings.Index(p, "<unknown>")
		// An unknown alternative without a represented suffix may replace the root.
		if index < 0 && v.unknown {
			return "parent-unknown"
		}
		if index == 0 {
			return "parent-unknown"
		}
		if index > 0 {
			p = p[:index]
		}
		p = path.Clean(p)
		next := "relative-prefix-partial"
		if p == tempRoot || strings.HasPrefix(p, tempRoot+"/") {
			next = "temp-prefix-partial"
		} else if strings.HasPrefix(p, "/") || len(p) >= 2 && p[1] == ':' {
			next = "absolute-prefix-partial"
		}
		if category != "" && category != next {
			return "parent-unknown"
		}
		category = next
	}
	return category
}

type analyzer struct {
	fset    *token.FileSet
	goos    string
	goarch  string
	funcs   map[string]*function
	globals scope
	result  *report
	seen    map[string]bool
}

func (a *analyzer) check(call *ast.CallExpr, operation string, v value, expression ast.Expr) {
	pos := a.fset.Position(call.Pos())
	if v.unknown || len(v.texts) == 0 {
		key := fmt.Sprintf("%s:%d:%s", pos.Filename, pos.Line, operation)
		site := a.result.unresolved[key]
		parent := unresolvedParent(v)
		if site.parent != "" && site.parent != parent {
			parent = "parent-unknown"
		}
		site.parent = parent
		var rendered bytes.Buffer
		if expression != nil {
			_ = printer.Fprint(&rendered, a.fset, expression)
		}
		if !contains(site.expressions, rendered.String()) {
			site.expressions = append(site.expressions, rendered.String())
		}
		for _, candidate := range v.texts {
			if !contains(site.candidates, candidate) {
				site.candidates = append(site.candidates, candidate)
			}
		}
		a.result.unresolved[key] = site
	}
	for _, raw := range v.texts {
		p := raw
		if a.goos == "windows" {
			p = strings.ReplaceAll(p, `\`, "/")
		}
		clean := path.Clean(p)
		temporary := clean == tempRoot || strings.HasPrefix(clean, tempRoot+"/")
		escaped := strings.Contains(p, tempRoot) && !temporary
		absolute := strings.HasPrefix(p, "/") || a.goos == "windows" && len(p) >= 2 && p[1] == ':' && ((p[0] >= 'A' && p[0] <= 'Z') || (p[0] >= 'a' && p[0] <= 'z'))
		if !escaped && (!absolute || temporary) {
			continue
		}
		key := fmt.Sprintf("%s:%d:%s:%s", pos.Filename, pos.Line, operation, raw)
		if !a.seen[key] {
			a.seen[key] = true
			a.result.findings = append(a.result.findings, finding{pos.Filename, pos.Line, operation, raw})
		}
	}
}

func combinations(values []value, operation func([]string) string) value {
	out := value{}
	parts := []string{}
	var visit func(int)
	visit = func(index int) {
		if len(out.texts) >= 128 {
			out.unknown = true
			return
		}
		if index == len(values) {
			out = union(out, text(operation(parts)))
			return
		}
		out.unknown = out.unknown || values[index].unknown
		candidates := values[index].texts
		if values[index].unknown {
			candidates = append(append([]string(nil), candidates...), "<unknown>")
		}
		for _, s := range candidates {
			parts = append(parts, s)
			visit(index + 1)
			parts = parts[:len(parts)-1]
		}
	}
	visit(0)
	return out
}

func (a *analyzer) eval(s *source, e ast.Expr, env scope, depth int) value {
	if e == nil || depth > 24 {
		return unknown()
	}
	switch x := e.(type) {
	case *ast.BasicLit:
		if x.Kind == token.STRING {
			v, err := strconv.Unquote(x.Value)
			if err == nil {
				return text(v)
			}
		}
		if x.Kind == token.CHAR {
			v, err := strconv.Unquote(x.Value)
			if err == nil && len([]rune(v)) == 1 {
				return value{numbers: []int64{int64([]rune(v)[0])}}
			}
		}
		if x.Kind == token.INT {
			n, err := strconv.ParseInt(x.Value, 0, 64)
			if err == nil {
				return value{numbers: []int64{n}}
			}
		}
	case *ast.Ident:
		if v, ok := env[x.Name]; ok {
			return v
		}
		if x.Name == "true" {
			return value{numbers: []int64{1}}
		}
		if x.Name == "false" {
			return value{numbers: []int64{0}}
		}
		if pkg, ok := s.imports[x.Name]; ok {
			return value{symbol: pkg}
		}
		if f, ok := a.funcs[x.Name]; ok {
			return value{fn: f}
		}
		if pkg, ok := s.imports["."+x.Name]; ok {
			return value{symbol: pkg + "." + x.Name}
		}
	case *ast.ParenExpr:
		return a.eval(s, x.X, env, depth)
	case *ast.SelectorExpr:
		v := a.eval(s, x.X, env, depth)
		if field, ok := v.fields[x.Sel.Name]; ok {
			return field
		}
		if v.symbol != "" {
			name := v.symbol + "." + x.Sel.Name
			if name == "runtime.GOOS" {
				return text(a.goos)
			}
			if name == "runtime.GOARCH" {
				return text(a.goarch)
			}
			if name == "os.PathSeparator" || name == "path/filepath.Separator" {
				if a.goos == "windows" {
					return value{numbers: []int64{'\\'}}
				}
				return value{numbers: []int64{'/'}}
			}
			for k, n := range map[string]int64{"O_RDONLY": 0, "O_WRONLY": 1, "O_RDWR": 2, "O_APPEND": 1024, "O_CREATE": 64, "O_CREAT": 64, "O_TRUNC": 512, "O_EXCL": 128} {
				if (v.symbol == "os" || v.symbol == "syscall" || v.symbol == "golang.org/x/sys/unix") && x.Sel.Name == k {
					return value{numbers: []int64{n}}
				}
			}
			if a.goos == "linux" && (v.symbol == "syscall" || v.symbol == "golang.org/x/sys/unix") && x.Sel.Name == "O_NOCTTY" {
				return value{numbers: []int64{256}}
			}
			return value{symbol: name}
		}
		if v.tester {
			return value{symbol: "testing." + x.Sel.Name}
		}
		if v.file {
			v.symbol = "file." + x.Sel.Name
			return v
		}
		if v.replace != nil && x.Sel.Name == "Replace" {
			v.symbol = "replacer.Replace"
			return v
		}
	case *ast.BinaryExpr:
		left, right := a.eval(s, x.X, env, depth), a.eval(s, x.Y, env, depth)
		if x.Op == token.ADD && (len(left.texts) > 0 || len(right.texts) > 0) {
			return combinations([]value{left, right}, func(p []string) string { return p[0] + p[1] })
		}
		out := value{unknown: left.unknown || right.unknown}
		for _, l := range left.numbers {
			for _, r := range right.numbers {
				switch x.Op {
				case token.OR:
					out.numbers = append(out.numbers, l|r)
				case token.AND:
					out.numbers = append(out.numbers, l&r)
				case token.ADD:
					out.numbers = append(out.numbers, l+r)
				case token.EQL:
					out.numbers = append(out.numbers, boolNumber(l == r))
				case token.NEQ:
					out.numbers = append(out.numbers, boolNumber(l != r))
				case token.LAND:
					out.numbers = append(out.numbers, boolNumber(l != 0 && r != 0))
				case token.LOR:
					out.numbers = append(out.numbers, boolNumber(l != 0 || r != 0))
				}
			}
		}
		for _, l := range left.texts {
			for _, r := range right.texts {
				if x.Op == token.EQL {
					out.numbers = append(out.numbers, boolNumber(l == r))
				}
				if x.Op == token.NEQ {
					out.numbers = append(out.numbers, boolNumber(l != r))
				}
			}
		}
		if len(out.numbers) > 0 {
			return out
		}
	case *ast.UnaryExpr:
		v := a.eval(s, x.X, env, depth)
		if x.Op == token.NOT {
			v.numbers = append([]int64(nil), v.numbers...)
			for i, n := range v.numbers {
				v.numbers[i] = boolNumber(n == 0)
			}
			return v
		}
		if x.Op == token.AND {
			return v
		}
	case *ast.CompositeLit:
		v := value{fields: map[string]value{}}
		var fields []string
		if t, ok := x.Type.(*ast.StructType); ok {
			for _, field := range t.Fields.List {
				for _, name := range field.Names {
					fields = append(fields, name.Name)
				}
			}
		}
		for i, element := range x.Elts {
			if pair, ok := element.(*ast.KeyValueExpr); ok {
				entry := a.eval(s, pair.Value, env, depth)
				if key, ok := pair.Key.(*ast.Ident); ok {
					v.fields[key.Name] = entry
				}
				v.items = append(v.items, entry)
			} else {
				if inner, ok := element.(*ast.CompositeLit); ok && inner.Type == nil {
					if array, ok := x.Type.(*ast.ArrayType); ok {
						copy := *inner
						copy.Type = array.Elt
						element = &copy
					}
				}
				entry := a.eval(s, element, env, depth)
				v.items = append(v.items, entry)
				if i < len(fields) {
					v.fields[fields[i]] = entry
				}
			}
		}
		return v
	case *ast.IndexExpr:
		v := a.eval(s, x.X, env, depth)
		index := a.eval(s, x.Index, env, depth)
		out := value{}
		for _, n := range index.numbers {
			if n >= 0 && int(n) < len(v.items) {
				out = union(out, v.items[n])
			}
		}
		if len(index.numbers) == 0 {
			for _, item := range v.items {
				out = union(out, item)
			}
		}
		return out
	case *ast.SliceExpr:
		return a.eval(s, x.X, env, depth)
	case *ast.FuncLit:
		return value{fn: &function{s, x.Type.Params, x.Body, clone(env)}}
	case *ast.CallExpr:
		callee := a.eval(s, x.Fun, env, depth)
		args := make([]value, len(x.Args))
		for i, arg := range x.Args {
			args[i] = a.eval(s, arg, env, depth)
		}
		if x.Ellipsis.IsValid() && len(args) > 0 {
			last := args[len(args)-1]
			args = append(args[:len(args)-1], last.items...)
		}
		return a.call(s, x, callee, args, depth)
	}
	return unknown()
}

func boolNumber(v bool) int64 {
	if v {
		return 1
	}
	return 0
}

func (a *analyzer) call(s *source, call *ast.CallExpr, callee value, args []value, depth int) value {
	name := callee.symbol
	if callee.fn != nil {
		return a.invoke(callee.fn, args, depth+1)
	}
	get := func(i int) value {
		if i < len(args) {
			return args[i]
		}
		return unknown()
	}
	// Both link endpoints are checked: a test must not install a link to an
	// external absolute target which a later write can follow.
	mutations := map[string][]int{
		"os.Create": {0}, "os.WriteFile": {0}, "io/ioutil.WriteFile": {0},
		"os.Mkdir": {0}, "os.MkdirAll": {0}, "os.Remove": {0}, "os.RemoveAll": {0},
		"os.Chmod": {0}, "os.Chown": {0}, "os.Lchown": {0}, "os.Chtimes": {0}, "os.Truncate": {0},
		"os.Rename": {0, 1}, "os.Link": {0, 1}, "os.Symlink": {0, 1},
		"syscall.Mkdir": {0}, "syscall.Unlink": {0}, "syscall.Rmdir": {0}, "syscall.Chmod": {0}, "syscall.Chown": {0}, "syscall.Rename": {0, 1}, "syscall.Link": {0, 1}, "syscall.Symlink": {0, 1},
	}
	if strings.HasPrefix(name, "golang.org/x/sys/unix.") {
		name = "syscall." + strings.TrimPrefix(name, "golang.org/x/sys/unix.")
	}
	for _, index := range mutations[name] {
		if index < len(call.Args) {
			a.check(call, name, get(index), call.Args[index])
		}
	}
	if name == "os.OpenFile" || name == "syscall.Open" {
		flags := get(1)
		write := flags.unknown || len(flags.numbers) == 0
		for _, n := range flags.numbers {
			write = write || n&(1|2|64|512|1024) != 0
		}
		// This is solely a temporary kernel PTY allocation exception. Keep the
		// returned path intact so chmod, truncate, removal and rename still fail.
		device := get(0)
		pty := name == "os.OpenFile" && a.goos == "linux" && s.linuxOnly && !device.unknown && len(device.texts) == 1 && device.texts[0] == "/dev/ptmx" && !flags.unknown && len(flags.numbers) > 0
		for _, n := range flags.numbers {
			// Only access mode plus optional O_NOCTTY; unknown/extra flags cannot
			// widen the exception (in particular O_CREATE and O_TRUNC).
			pty = pty && n & ^int64(3|256) == 0 && n&3 != 3
		}
		if (write || contains(device.texts, "/dev/ptmx")) && !pty && len(call.Args) > 0 {
			a.check(call, name, device, call.Args[0])
		}
	}
	if name == "os.Open" || name == "os.OpenFile" || name == "os.Create" {
		v := get(0)
		v.file = true
		return v
	}
	if strings.HasPrefix(name, "file.") {
		if contains([]string{"file.Write", "file.WriteAt", "file.WriteString", "file.Truncate", "file.Chmod", "file.Chown"}, name) {
			a.check(call, name, callee, call.Fun)
		}
		if name == "file.Name" {
			callee.file = false
			callee.symbol = ""
			return callee
		}
	}
	if name == "testing.TempDir" || name == "os.TempDir" {
		return text(tempRoot)
	}
	if name == "os.MkdirTemp" || name == "os.CreateTemp" || name == "io/ioutil.TempDir" || name == "io/ioutil.TempFile" {
		if len(call.Args) > 0 {
			a.check(call, name, get(0), call.Args[0])
		}
		v := combinations([]value{get(0)}, func(p []string) string {
			if p[0] == "" {
				return tempRoot + "/generated"
			}
			return strings.TrimRight(p[0], "/\\") + "/generated"
		})
		v.file = name == "os.CreateTemp" || name == "io/ioutil.TempFile"
		return v
	}
	if name == "path/filepath.Join" || name == "path.Join" || name == "strings.Join" {
		sep := "/"
		if name == "strings.Join" {
			if len(get(1).texts) != 1 {
				return unknown()
			}
			sep = get(1).texts[0]
			args = get(0).items
		}
		return combinations(args, func(p []string) string {
			if name == "strings.Join" {
				return strings.Join(p, sep)
			}
			joined := strings.Join(p, "/")
			if a.goos == "windows" && name == "path/filepath.Join" {
				joined = strings.ReplaceAll(joined, `\`, "/")
			}
			clean := path.Clean(joined)
			if strings.Contains(joined, tempRoot) && clean != tempRoot && !strings.HasPrefix(clean, tempRoot+"/") {
				return joined
			}
			return clean
		})
	}
	if contains([]string{"path/filepath.Clean", "path.Clean", "path/filepath.FromSlash", "path/filepath.ToSlash", "path/filepath.EvalSymlinks", "github.com/sylvinus/agent-vm/internal/paths.Host", "github.com/sylvinus/agent-vm/internal/paths.Guest"}, name) {
		if len(args) == 0 {
			return unknown()
		}
		return combinations(args[:min(1, len(args))], func(p []string) string { return strings.ReplaceAll(p[0], `\`, "/") })
	}
	if name == "strings.ReplaceAll" || name == "strings.Replace" {
		if len(args) < 3 {
			return unknown()
		}
		return combinations(args[:3], func(p []string) string { return strings.ReplaceAll(p[0], p[1], p[2]) })
	}
	if name == "strings.NewReplacer" {
		pairs := []string{}
		for _, arg := range args {
			if arg.unknown || len(arg.texts) != 1 {
				return unknown()
			}
			pairs = append(pairs, arg.texts[0])
		}
		if len(pairs)%2 == 0 {
			return value{replace: pairs}
		}
	}
	if name == "replacer.Replace" {
		if len(args) == 0 {
			return unknown()
		}
		return combinations(args, func(p []string) string { return strings.NewReplacer(callee.replace...).Replace(p[0]) })
	}
	if name == "fmt.Sprintf" && len(args) > 0 && len(args[0].texts) == 1 {
		return combinations(args[1:], func(parts []string) string {
			values := make([]any, len(parts))
			for i, p := range parts {
				values[i] = p
			}
			return fmt.Sprintf(args[0].texts[0], values...)
		})
	}
	// Inspect callbacks without executing their owner (t.Run, t.Cleanup,
	// handlers, goroutines). Their captured paths still participate in checks.
	for _, arg := range args {
		if arg.fn != nil {
			a.invoke(arg.fn, nil, depth+1)
		}
	}
	return unknown()
}

func (a *analyzer) invoke(f *function, args []value, depth int) value {
	if depth > 24 {
		return unknown()
	}
	env := clone(a.globals)
	for k, v := range f.env {
		env[k] = v
	}
	i := 0
	if f.params != nil {
		for _, field := range f.params.List {
			for _, name := range field.Names {
				v := unknown()
				if i < len(args) {
					v = args[i]
				}
				typeExpr := field.Type
				if ptr, ok := typeExpr.(*ast.StarExpr); ok {
					typeExpr = ptr.X
				}
				if selector, ok := typeExpr.(*ast.SelectorExpr); ok {
					if pkg, ok := selector.X.(*ast.Ident); ok && f.source.imports[pkg.Name] == "testing" {
						v.tester = true
					}
				}
				env[name.Name] = v
				i++
			}
		}
	}
	ret, _ := a.block(f.source, f.body.List, env, depth)
	return ret
}

func (a *analyzer) declaration(s *source, d *ast.GenDecl, env scope, depth int) {
	var previous []ast.Expr
	for _, spec := range d.Specs {
		if v, ok := spec.(*ast.ValueSpec); ok {
			exprs := v.Values
			if len(exprs) == 0 && d.Tok == token.CONST {
				exprs = previous
			}
			previous = exprs
			for i, name := range v.Names {
				next := unknown()
				if i < len(exprs) {
					next = a.eval(s, exprs[i], env, depth)
				}
				env[name.Name] = next
			}
		}
	}
}

// block returns possible return values and whether execution can fall through.
func (a *analyzer) block(s *source, statements []ast.Stmt, env scope, depth int) (value, bool) {
	ret := value{}
	for _, statement := range statements {
		switch x := statement.(type) {
		case *ast.DeclStmt:
			if d, ok := x.Decl.(*ast.GenDecl); ok {
				a.declaration(s, d, env, depth)
			}
		case *ast.AssignStmt:
			values := make([]value, len(x.Rhs))
			for i, e := range x.Rhs {
				values[i] = a.eval(s, e, env, depth)
			}
			for i, e := range x.Lhs {
				if selector, ok := e.(*ast.SelectorExpr); ok && i < len(values) {
					if id, ok := selector.X.(*ast.Ident); ok {
						owner := a.eval(s, selector.X, env, depth)
						fields := map[string]value{}
						for key, v := range owner.fields {
							fields[key] = v
						}
						fields[selector.Sel.Name] = values[i]
						owner.fields = fields
						env[id.Name] = owner
					}
				}
				if id, ok := e.(*ast.Ident); ok {
					v := unknown()
					if i < len(values) {
						v = values[i]
					}
					if x.Tok == token.ADD_ASSIGN {
						old := env[id.Name]
						v = combinations([]value{old, v}, func(p []string) string { return p[0] + p[1] })
					}
					env[id.Name] = v
				}
			}
		case *ast.ExprStmt:
			a.eval(s, x.X, env, depth)
		case *ast.DeferStmt:
			a.eval(s, x.Call, env, depth)
		case *ast.GoStmt:
			a.eval(s, x.Call, env, depth)
		case *ast.ReturnStmt:
			if len(x.Results) > 0 {
				ret = union(ret, a.eval(s, x.Results[0], env, depth))
			}
			return ret, false
		case *ast.BlockStmt:
			v, falls := a.block(s, x.List, env, depth)
			ret = union(ret, v)
			if !falls {
				return ret, false
			}
		case *ast.IfStmt:
			local := clone(env)
			if x.Init != nil {
				a.block(s, []ast.Stmt{x.Init}, local, depth)
			}
			condition := a.eval(s, x.Cond, local, depth)
			yes, no := condition.unknown || len(condition.numbers) == 0, condition.unknown || len(condition.numbers) == 0
			for _, n := range condition.numbers {
				yes = yes || n != 0
				no = no || n == 0
			}
			states := []scope{}
			if yes {
				branch := clone(local)
				v, falls := a.block(s, x.Body.List, branch, depth)
				ret = union(ret, v)
				if falls {
					states = append(states, branch)
				}
			}
			if no {
				branch := clone(local)
				falls := true
				if x.Else != nil {
					v, f := a.block(s, []ast.Stmt{x.Else}, branch, depth)
					ret = union(ret, v)
					falls = f
				}
				if falls {
					states = append(states, branch)
				}
			}
			if len(states) == 0 {
				return ret, false
			}
			for key := range env {
				env[key] = states[0][key]
				for _, state := range states[1:] {
					env[key] = union(env[key], state[key])
				}
			}
		case *ast.RangeStmt:
			collection := a.eval(s, x.X, env, depth)
			item := value{}
			for _, v := range collection.items {
				item = union(item, v)
			}
			item.unknown = item.unknown || collection.unknown || len(collection.items) == 0
			local := clone(env)
			if id, ok := x.Key.(*ast.Ident); ok {
				local[id.Name] = unknown()
			}
			if id, ok := x.Value.(*ast.Ident); ok {
				local[id.Name] = item
			}
			v, _ := a.block(s, x.Body.List, local, depth)
			ret = union(ret, v)
			for key := range env {
				env[key] = union(env[key], local[key])
			}
		case *ast.ForStmt:
			local := clone(env)
			if x.Init != nil {
				a.block(s, []ast.Stmt{x.Init}, local, depth)
			}
			a.eval(s, x.Cond, local, depth)
			v, _ := a.block(s, x.Body.List, local, depth)
			ret = union(ret, v)
			if x.Post != nil {
				a.block(s, []ast.Stmt{x.Post}, local, depth)
			}
			for key := range env {
				env[key] = union(env[key], local[key])
			}
		case *ast.SwitchStmt:
			if x.Init != nil {
				a.block(s, []ast.Stmt{x.Init}, env, depth)
			}
			a.eval(s, x.Tag, env, depth)
			for _, item := range x.Body.List {
				clause := item.(*ast.CaseClause)
				local := clone(env)
				v, _ := a.block(s, clause.Body, local, depth)
				ret = union(ret, v)
				for key := range env {
					env[key] = union(env[key], local[key])
				}
			}
		case *ast.TypeSwitchStmt:
			if x.Init != nil {
				a.block(s, []ast.Stmt{x.Init}, env, depth)
			}
			a.block(s, []ast.Stmt{x.Assign}, env, depth)
			for _, item := range x.Body.List {
				clause := item.(*ast.CaseClause)
				local := clone(env)
				v, _ := a.block(s, clause.Body, local, depth)
				ret = union(ret, v)
				for key := range env {
					env[key] = union(env[key], local[key])
				}
			}
		case *ast.LabeledStmt:
			v, falls := a.block(s, []ast.Stmt{x.Stmt}, env, depth)
			ret = union(ret, v)
			if !falls {
				return ret, false
			}
		case *ast.SelectStmt:
			for _, item := range x.Body.List {
				clause := item.(*ast.CommClause)
				local := clone(env)
				if clause.Comm != nil {
					a.block(s, []ast.Stmt{clause.Comm}, local, depth)
				}
				v, _ := a.block(s, clause.Body, local, depth)
				ret = union(ret, v)
			}
		}
	}
	return ret, true
}

var ignored = map[string]bool{".git": true, ".tmp": true, "_output": true, "node_modules": true, "bin": true, "obj": true, "dist": true, "build": true, "target": true}

func scan(roots []string) (report, error) {
	result := report{unresolved: map[string]unresolvedSite{}}
	fset := token.NewFileSet()
	groups := map[string][]*source{}
	files := map[string]bool{}
	for _, root := range roots {
		err := filepath.WalkDir(root, func(name string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				if ignored[entry.Name()] {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(name, "_test.go") || entry.Type()&os.ModeSymlink != 0 {
				return nil
			}
			name, err = filepath.Abs(name)
			if err != nil {
				return err
			}
			if files[name] {
				return nil
			}
			files[name] = true
			parsed, err := parser.ParseFile(fset, name, nil, parser.ParseComments|parser.SkipObjectResolution)
			if err != nil {
				return err
			}
			s := &source{name: name, ast: parsed, imports: map[string]string{}}
			s.linuxOnly = linuxOnlySource(s)
			for _, item := range parsed.Imports {
				pkg, err := strconv.Unquote(item.Path.Value)
				if err != nil {
					return err
				}
				alias := path.Base(pkg)
				if item.Name != nil {
					alias = item.Name.Name
				}
				s.imports[alias] = pkg
				if alias == "." {
					for _, name := range []string{"Create", "Open", "OpenFile", "WriteFile", "Mkdir", "MkdirAll", "Remove", "RemoveAll", "Chmod", "Chown", "Lchown", "Chtimes", "Truncate", "Rename", "Link", "Symlink", "MkdirTemp", "CreateTemp", "TempDir", "TempFile"} {
						s.imports["."+name] = pkg
					}
				}
			}
			key := filepath.Dir(name) + "/" + parsed.Name.Name
			groups[key] = append(groups[key], s)
			result.files++
			return nil
		})
		if err != nil {
			return result, err
		}
	}
	seen := map[string]bool{}
	for _, sources := range groups {
		for _, target := range []struct{ goos, goarch string }{{"windows", "amd64"}, {"linux", "amd64"}, {"linux", "arm64"}} {
			a := analyzer{fset: fset, goos: target.goos, goarch: target.goarch, funcs: map[string]*function{}, globals: scope{}, result: &result, seen: seen}
			active := []*source{}
			for _, s := range sources {
				context := build.Default
				context.GOOS = target.goos
				context.GOARCH = target.goarch
				matched, err := context.MatchFile(filepath.Dir(s.name), filepath.Base(s.name))
				if err != nil {
					return result, err
				}
				if !matched {
					continue
				}
				active = append(active, s)
				for _, decl := range s.ast.Decls {
					if fn, ok := decl.(*ast.FuncDecl); ok && fn.Body != nil {
						name := fn.Name.Name
						if fn.Recv != nil {
							name = fmt.Sprintf("method:%d", fn.Pos())
						}
						a.funcs[name] = &function{s, fn.Type.Params, fn.Body, nil}
					}
				}
			}
			for _, s := range active {
				for _, decl := range s.ast.Decls {
					if d, ok := decl.(*ast.GenDecl); ok {
						a.declaration(s, d, a.globals, 0)
					}
				}
			}
			for _, fn := range a.funcs {
				a.invoke(fn, nil, 0)
			}
		}
	}
	sort.Slice(result.findings, func(i, j int) bool {
		x, y := result.findings[i], result.findings[j]
		if x.file != y.file {
			return x.file < y.file
		}
		if x.line != y.line {
			return x.line < y.line
		}
		return x.operation+x.target < y.operation+y.target
	})
	return result, nil
}

func run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("check-test-paths", flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := flags.String("root", ".", "repository root; relative input paths are resolved beneath it")
	details := flags.Bool("unresolved", false, "list unresolved expressions and partial parent-path evidence")
	flags.Usage = func() {
		fmt.Fprintln(stderr, "Usage: check-test-paths [-root repository] [-unresolved] [directory or *_test.go ...]\nScans Windows/Linux Go tests, including vendored sources; default: repository root.\nUnknown paths are counted, not certified safe. Use the isolated test runner.")
	}
	if err := flags.Parse(args); err != nil {
		return 2
	}
	roots := flags.Args()
	if len(roots) == 0 {
		roots = []string{*root}
	} else {
		for i, name := range roots {
			if !filepath.IsAbs(name) {
				roots[i] = filepath.Join(*root, name)
			}
		}
	}
	result, err := scan(roots)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	for _, item := range result.findings {
		fmt.Fprintf(stderr, "%s:%d: %s: unsafe path %q\n", item.file, item.line, item.operation, item.target)
	}
	fmt.Fprintf(stdout, "Scanned %d Go test files: %d unsafe paths, %d unresolved mutation sites (not an OS sandbox).\n", result.files, len(result.findings), len(result.unresolved))
	if *details {
		keys := make([]string, 0, len(result.unresolved))
		counts := map[string]int{}
		for key, site := range result.unresolved {
			keys = append(keys, key)
			counts[site.parent]++
		}
		sort.Strings(keys)
		for _, category := range []string{"parent-unknown", "temp-prefix-partial", "relative-prefix-partial", "absolute-prefix-partial"} {
			fmt.Fprintf(stdout, "%s: %d unresolved sites\n", category, counts[category])
		}
		for _, key := range keys {
			site := result.unresolved[key]
			sort.Strings(site.expressions)
			sort.Strings(site.candidates)
			fmt.Fprintf(stdout, "%s: %s; expressions=%q; candidates=%q\n", key, site.parent, site.expressions, site.candidates)
		}
	}
	if len(result.findings) > 0 {
		return 1
	}
	return 0
}

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }
