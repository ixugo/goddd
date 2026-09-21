package gen

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

// TestCacheTombstoneDeclaration 检查同域多实体的共享声明，防止生成包出现重复常量。
func TestCacheTombstoneDeclaration(t *testing.T) {
	domain, err := ParseContent("package sample; type User struct { ID int }; type Team struct { ID int }")
	if err != nil {
		t.Fatal(err)
	}
	out := make(map[string]*bytes.Buffer)
	if err := handlerDomainCache(domain, out); err != nil {
		t.Fatal(err)
	}
	count := 0
	for path, source := range out {
		file, err := parser.ParseFile(token.NewFileSet(), path, source.Bytes(), 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(file, func(node ast.Node) bool {
			if spec, ok := node.(*ast.ValueSpec); ok {
				for _, name := range spec.Names {
					if name.Name == "tombstone" {
						count++
					}
				}
			}
			return true
		})
	}
	if count != 1 {
		t.Fatalf("tombstone declarations = %d, want 1", count)
	}
}
