// Generates the JSON wire contracts directly from the host's Go declarations.
// Only reachable fields of the explicitly listed public API views are emitted.
package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
)

type declaration struct {
	expr    ast.Expr
	imports map[string]string
}
type generator struct {
	root     string
	packages map[string]map[string]declaration
	done     map[string]bool
	output   []string
}

func identifier(pkg, name string) string {
	return strings.ToUpper(filepath.Base(pkg)[:1]) + filepath.Base(pkg)[1:] + strings.ToUpper(name[:1]) + name[1:]
}
func (g *generator) load(pkg string) map[string]declaration {
	if result, ok := g.packages[pkg]; ok {
		return result
	}
	result := map[string]declaration{}
	entries, err := os.ReadDir(filepath.Join(g.root, filepath.FromSlash(pkg)))
	if err != nil {
		panic(err)
	}
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(g.root, pkg, entry.Name()), nil, 0)
		if err != nil {
			panic(err)
		}
		imports := map[string]string{}
		for _, item := range file.Imports {
			value, _ := strconv.Unquote(item.Path.Value)
			name := filepath.Base(value)
			if item.Name != nil {
				name = item.Name.Name
			}
			imports[name] = strings.TrimPrefix(value, "github.com/sylvinus/agent-vm/")
		}
		for _, node := range file.Decls {
			group, ok := node.(*ast.GenDecl)
			if !ok || group.Tok != token.TYPE {
				continue
			}
			for _, spec := range group.Specs {
				item := spec.(*ast.TypeSpec)
				result[item.Name.Name] = declaration{item.Type, imports}
			}
		}
	}
	g.packages[pkg] = result
	return result
}
func (g *generator) named(pkg, name string) string {
	id := identifier(pkg, name)
	if g.done[id] {
		return id + "Schema"
	}
	g.done[id] = true
	decl, ok := g.load(pkg)[name]
	if !ok {
		panic("Missing Go contract: " + pkg + "." + name)
	}
	schema := g.schema(pkg, decl.expr, decl.imports)
	g.output = append(g.output, "export const "+id+"Schema = "+schema+";\nexport type "+id+" = z.infer<typeof "+id+"Schema>;\n")
	return id + "Schema"
}
func (g *generator) schema(pkg string, expr ast.Expr, imports map[string]string) string {
	switch node := expr.(type) {
	case *ast.Ident:
		switch node.Name {
		case "string":
			return "z.string()"
		case "bool":
			return "z.boolean()"
		case "byte", "rune", "int", "int8", "int16", "int32", "int64", "uint", "uint8", "uint16", "uint32", "uint64", "float32", "float64":
			return "z.number()"
		case "any":
			return "z.unknown()"
		default:
			return g.named(pkg, node.Name)
		}
	case *ast.SelectorExpr:
		owner := node.X.(*ast.Ident).Name
		if owner == "time" {
			return "z.string()"
		}
		if owner == "json" && node.Sel.Name == "RawMessage" {
			return "z.unknown()"
		}
		dependency := imports[owner]
		if !strings.HasPrefix(dependency, "internal/") {
			panic("Unsupported Go contract dependency: " + dependency + "." + node.Sel.Name)
		}
		return g.named(dependency, node.Sel.Name)
	case *ast.StarExpr:
		return g.schema(pkg, node.X, imports) + ".nullable()"
	case *ast.ArrayType:
		if ident, ok := node.Elt.(*ast.Ident); ok && ident.Name == "byte" {
			return "z.string()"
		}
		return "z.array(" + g.schema(pkg, node.Elt, imports) + ").nullable().transform(value => value ?? [])"
	case *ast.MapType:
		return "z.record(z.string(), " + g.schema(pkg, node.Value, imports) + ").nullable().transform(value => value ?? {})"
	case *ast.InterfaceType:
		return "z.unknown()"
	case *ast.StructType:
		fields := []string{}
		for _, field := range node.Fields.List {
			if len(field.Names) == 0 {
				fields = append(fields, "..."+g.schema(pkg, field.Type, imports)+".shape")
				continue
			}
			name := field.Names[0].Name
			if !ast.IsExported(name) {
				continue
			}
			tag := ""
			if field.Tag != nil {
				value, _ := strconv.Unquote(field.Tag.Value)
				tag = reflect.StructTag(value).Get("json")
			}
			if tag == "-" {
				continue
			}
			if tag != "" {
				name = strings.Split(tag, ",")[0]
			}
			value := g.schema(pkg, field.Type, imports)
			if strings.Contains(tag, "omitempty") {
				value += ".optional()"
			}
			fields = append(fields, strconv.Quote(name)+": "+value)
		}
		return "z.object({\n" + strings.Join(fields, ",\n") + "\n})"
	default:
		panic(fmt.Sprintf("Unsupported Go contract expression: %T", expr))
	}
}
func main() {
	root, err := filepath.Abs(".")
	if err != nil {
		panic(err)
	}
	g := generator{root: root, packages: map[string]map[string]declaration{}, done: map[string]bool{}}
	names := []string{"Settings", "Job", "ClientContext", "accountView", "Appearance", "Location", "safetyState", "projectGroup", "storageRecoveryView", "cliSessionSummary", "cliSession", "claudePanel", "claudeDesktopState", "deviceCheckReport", "initializationView", "cacheInventory", "componentVersions", "toolVersion", "Favorites", "vaultBrowserState", "vaultData", "trustedLoginReply", "browserDownloadInfo", "SavedDownload", "localBrowserProfile"}
	sort.Strings(names)
	for _, name := range names {
		g.named("internal/workbench", name)
	}
	g.named("internal/eden", "ResourceStatus")
	g.named("internal/eden", "ResourceAllocation")
	g.named("internal/version", "Build")
	g.named("internal/appupdate", "AboutCheck")
	for _, name := range []string{"ClipboardContent", "FileSelection", "FileResult", "RemoteInputResult", "InputEvent", "UpdateState", "DeviceStatus"} {
		g.named("internal/desktop", name)
	}
	text := "// Generated by ui/scripts/generate-contracts.go from the current host JSON views.\nimport { z } from \"zod\";\n\n" + strings.Join(g.output, "\n")
	if err := os.WriteFile(filepath.Join(root, "ui/src/lib/contracts.generated.ts"), []byte(text), 0644); err != nil {
		panic(err)
	}
}
