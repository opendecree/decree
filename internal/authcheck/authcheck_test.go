// Package authcheck enforces the convention that every non-public gRPC handler
// must call auth.MustHaveClaims(ctx) as a defense-in-depth guard.
//
// Public handlers (RPCs explicitly documented as unauthenticated in the proto)
// are listed in the allowlist below.  The standard gRPC health service
// (/grpc.health.v1.Health/*) and GetServerInfo are the only two such RPCs in
// this codebase; they are handled separately and do not appear in the files
// scanned here.
package authcheck_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"runtime"
	"testing"
)

// allowlist contains exported method names that are intentionally exempt from
// the MustHaveClaims requirement.  Every entry here must have a justification.
var allowlist = map[string]string{
	// GetServerInfo is documented as unauthenticated in server_service.proto and
	// is handled by internal/server/server_info.go, which is a separate service
	// type (ServerService, not Service) and is not scanned below.
}

// serviceFiles lists the Go source files whose exported Service methods must
// all call auth.MustHaveClaims(ctx).  Only files containing RPCs that require
// authentication belong here.  The ServerService (server/server_info.go) is
// intentionally absent — GetServerInfo is the sole public RPC and the
// ServerService type is kept separate for exactly that reason.
var serviceFiles = []string{
	"../schema/service.go",
	"../config/service.go",
	"../audit/service.go",
}

func TestAllHandlersHaveMustHaveClaims(t *testing.T) {
	t.Helper()

	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	dir := filepath.Dir(thisFile)

	for _, rel := range serviceFiles {
		path := filepath.Join(dir, rel)
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}

		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			if !isServiceHandlerMethod(fn) {
				continue
			}
			name := fn.Name.Name
			if reason, exempt := allowlist[name]; exempt {
				t.Logf("skip %s (allowlisted: %s)", name, reason)
				continue
			}
			if !bodyCallsMustHaveClaims(fn.Body) {
				t.Errorf("%s: %s missing auth.MustHaveClaims(ctx) — add it or add to the allowlist with justification",
					filepath.Base(path), name)
			}
		}
	}
}

// isServiceHandlerMethod returns true for exported methods on a *Service
// receiver that match a gRPC unary or server-streaming handler signature:
//
//	func (s *Service) Foo(ctx context.Context, req *pb.FooRequest) (*pb.FooResponse, error)
//	func (s *Service) Bar(req *pb.BarRequest, stream ...) error
func isServiceHandlerMethod(fn *ast.FuncDecl) bool {
	// Must be exported.
	if !fn.Name.IsExported() {
		return false
	}
	// Must have a pointer receiver named *Service.
	if fn.Recv == nil || len(fn.Recv.List) != 1 {
		return false
	}
	star, ok := fn.Recv.List[0].Type.(*ast.StarExpr)
	if !ok {
		return false
	}
	ident, ok := star.X.(*ast.Ident)
	if !ok || ident.Name != "Service" {
		return false
	}
	// Must return an error (last result).
	if fn.Type.Results == nil || len(fn.Type.Results.List) == 0 {
		return false
	}
	last := fn.Type.Results.List[len(fn.Type.Results.List)-1]
	errIdent, ok := last.Type.(*ast.Ident)
	if !ok || errIdent.Name != "error" {
		return false
	}
	// Must take at least one parameter.
	if fn.Type.Params == nil || len(fn.Type.Params.List) == 0 {
		return false
	}
	return true
}

// bodyCallsMustHaveClaims reports whether the function body contains a call to
// auth.MustHaveClaims at any depth.
func bodyCallsMustHaveClaims(body *ast.BlockStmt) bool {
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		pkg, ok := sel.X.(*ast.Ident)
		if !ok {
			return true
		}
		if pkg.Name == "auth" && sel.Sel.Name == "MustHaveClaims" {
			found = true
			return false
		}
		return true
	})
	return found
}
