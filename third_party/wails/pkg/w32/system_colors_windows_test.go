//go:build windows

package w32

import (
	"go/ast"
	"go/parser"
	gotoken "go/token"
	"path/filepath"
	"runtime"
	"testing"
)

// Validate the Win32 ABI without allocating native objects on the test host.
// The VM acceptance entry independently compares actual system brush handles
// and reads the rendered dialog pixels through the production wrapper.
func TestGetSysColorBrushPassesOnlyTheRequestedColorIndex(t *testing.T) {
	_, testFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate system_colors_windows_test.go")
	}
	file, err := parser.ParseFile(gotoken.NewFileSet(), filepath.Join(filepath.Dir(testFile), "user32.go"), nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	function := findFunctionDeclaration(t, file, "GetSysColorBrush")
	var invocation *ast.CallExpr
	ast.Inspect(function.Body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || selector.Sel.Name != "Call" {
			return true
		}
		procedure, ok := selector.X.(*ast.Ident)
		if ok && procedure.Name == "procGetSysColorBrush" {
			invocation = call
			return false
		}
		return true
	})
	if invocation == nil || len(invocation.Args) != 1 {
		t.Fatal("GetSysColorBrush must pass exactly one native argument: the requested color index")
	}
	assertUintptrIdent(t, invocation.Args[0], "nIndex")
}
