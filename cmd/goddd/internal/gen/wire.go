package gen

import (
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// FileExists 返回相对 baseDir 下是否存在 internal/web/api/<filename> 文件。
func FileExists(baseDir, filename string) bool {
	providerPath := filepath.Join(baseDir, "internal", "web", "api", filename)
	_, err := os.Stat(providerPath)
	return err == nil
}

// AppendProviderSetArg 在 provider.go 的 wire.NewSet 调用中追加参数（自动去重）。
func AppendProviderSetArg(baseDir string, newArgExprs ...string) (bool, error) {
	providerPath := filepath.Join(baseDir, "internal", "web", "api", "provider.go")
	srcBytes, err := os.ReadFile(providerPath)
	if err != nil {
		return false, err
	}
	fset := token.NewFileSet()
	fileAST, err := parser.ParseFile(fset, providerPath, srcBytes, parser.ParseComments)
	if err != nil {
		return false, err
	}

	var (
		targetCall     *ast.CallExpr
		lastWireNewSet *ast.CallExpr
		modified       bool
	)

	ast.Inspect(fileAST, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
				if sel.Sel != nil && sel.Sel.Name == "NewSet" {
					if ident, ok := sel.X.(*ast.Ident); ok && ident.Name == "wire" {
						lastWireNewSet = call
					}
				}
			}
		}

		gen, ok := n.(*ast.GenDecl)
		if !ok || gen.Tok != token.VAR {
			return true
		}
		for _, spec := range gen.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			hasProviderSetName := false
			for _, name := range vs.Names {
				if name != nil && name.Name == "ProviderSet" {
					hasProviderSetName = true
					break
				}
			}
			if !hasProviderSetName || len(vs.Values) == 0 {
				continue
			}
			if call, ok := vs.Values[0].(*ast.CallExpr); ok {
				if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
					if sel.Sel != nil && sel.Sel.Name == "NewSet" {
						if ident, ok := sel.X.(*ast.Ident); ok && ident.Name == "wire" {
							targetCall = call
							return false
						}
					}
				}
			}
		}
		return true
	})

	callToEdit := targetCall
	if callToEdit == nil {
		callToEdit = lastWireNewSet
	}
	if callToEdit == nil {
		return false, nil
	}

	existing := make(map[string]struct{}, len(callToEdit.Args))
	for _, a := range callToEdit.Args {
		start := fset.Position(a.Pos()).Offset
		end := fset.Position(a.End()).Offset
		if start >= 0 && end <= len(srcBytes) && start < end {
			s := string(bytes.TrimSpace(srcBytes[start:end]))
			if len(s) > 0 && s[len(s)-1] == ',' {
				s = s[:len(s)-1]
			}
			existing[s] = struct{}{}
		}
	}

	uniqToInsert := make([]string, 0, len(newArgExprs))
	seen := make(map[string]struct{}, len(newArgExprs))
	for _, arg := range newArgExprs {
		arg = strings.TrimSpace(arg)
		if arg == "" {
			continue
		}
		if _, ok := seen[arg]; ok {
			continue
		}
		seen[arg] = struct{}{}
		if _, ok := existing[arg]; ok {
			continue
		}
		uniqToInsert = append(uniqToInsert, arg)
	}

	if len(uniqToInsert) == 0 {
		return false, nil
	}

	endOffset := fset.Position(callToEdit.End()).Offset
	if endOffset == 0 || endOffset > len(srcBytes) {
		return false, errors.New("invalid call position")
	}
	insertAt := endOffset - 1
	var insertion bytes.Buffer
	insertion.WriteString("\t")
	for _, arg := range uniqToInsert {
		insertion.WriteString(arg)
		insertion.WriteString(", ")
	}
	insertion.WriteString("\n")

	var out bytes.Buffer
	out.Write(srcBytes[:insertAt])
	out.Write(insertion.Bytes())
	out.Write(srcBytes[insertAt:])
	if err := os.WriteFile(providerPath, out.Bytes(), 0o600); err != nil {
		return false, err
	}
	modified = true

	if !modified {
		return false, nil
	}
	return true, nil
}

// AppendLineToSetupRouter 在 api.go 的 setupRouter 函数体结束前插入一行代码。
func AppendLineToSetupRouter(baseDir string, funcName, newLine string) (bool, error) {
	apiPath := filepath.Join(baseDir, "internal", "web", "api", "api.go")
	if _, err := os.Stat(apiPath); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}

	srcBytes, err := os.ReadFile(apiPath)
	if err != nil {
		return false, err
	}

	fset := token.NewFileSet()
	fileAST, err := parser.ParseFile(fset, apiPath, srcBytes, 0)
	if err != nil {
		return false, err
	}

	var (
		bodyRBrace token.Pos
		setupFn    *ast.FuncDecl
		found      bool
	)
	for _, decl := range fileAST.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok {
			if fn.Name != nil && fn.Name.Name == "setupRouter" && fn.Body != nil {
				setupFn = fn
				bodyRBrace = fn.Body.Rbrace
				found = true
				break
			}
		}
	}
	if !found {
		return false, nil
	}

	called := false
	ast.Inspect(setupFn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		switch fun := call.Fun.(type) {
		case *ast.Ident:
			if fun.Name == funcName {
				called = true
				return false
			}
		case *ast.SelectorExpr:
			if fun.Sel != nil && fun.Sel.Name == funcName {
				called = true
				return false
			}
		}
		return true
	})
	if called {
		return false, nil
	}

	rbraceOffset := fset.Position(bodyRBrace).Offset
	if rbraceOffset == 0 || rbraceOffset > len(srcBytes) {
		return false, errors.New("invalid setupRouter body position")
	}

	insertion := "\t// TODO: 待补充中间件\n\t" + newLine + "\n"

	var out bytes.Buffer
	out.Write(srcBytes[:rbraceOffset])
	out.Write([]byte(insertion))
	out.Write(srcBytes[rbraceOffset:])
	if err := os.WriteFile(apiPath, out.Bytes(), 0o600); err != nil {
		return false, err
	}
	return true, nil
}

// AppendUsecaseField 在 provider.go 的 Usecase 结构体中追加字段。
func AppendUsecaseField(baseDir string, fieldDecl string) (bool, error) {
	providerPath := filepath.Join(baseDir, "internal", "web", "api", "provider.go")
	srcBytes, err := os.ReadFile(providerPath)
	if err != nil {
		return false, err
	}

	fset := token.NewFileSet()
	fileAST, err := parser.ParseFile(fset, providerPath, srcBytes, 0)
	if err != nil {
		return false, err
	}

	var (
		rbrace token.Pos
		found  bool
	)
	for _, decl := range fileAST.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.TYPE {
			continue
		}
		for _, spec := range gen.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok || ts.Name == nil || ts.Name.Name != "Usecase" {
				continue
			}
			st, ok := ts.Type.(*ast.StructType)
			if !ok || st.Fields == nil || !st.Fields.Closing.IsValid() {
				continue
			}

			if st.Fields != nil {
				for _, f := range st.Fields.List {
					if len(f.Names) > 0 {
						existName := f.Names[0].Name
						fd := strings.TrimSpace(fieldDecl)
						for i := 0; i < len(fd); i++ {
							if fd[i] == ' ' || fd[i] == '\t' {
								fd = fd[:i]
								break
							}
						}
						if fd == existName {
							return false, nil
						}
					}
				}
			}

			rbrace = st.Fields.Closing
			found = true
			break
		}
		if found {
			break
		}
	}

	if !found {
		return false, nil
	}

	offset := fset.Position(rbrace).Offset
	if offset == 0 || offset > len(srcBytes) {
		return false, errors.New("invalid struct position")
	}
	insertion := []byte("\n\t" + fieldDecl + "\n")
	var out bytes.Buffer
	out.Write(srcBytes[:offset])
	out.Write(insertion)
	out.Write(srcBytes[offset:])
	if err := os.WriteFile(providerPath, out.Bytes(), 0o600); err != nil {
		return false, err
	}
	return true, nil
}

// MakeWire 执行 wire 代码生成，返回命令输出。
func MakeWire() ([]byte, error) {
	return CommandContext("wire", "./...")
}

// CommandContext 执行外部命令，返回标准输出与错误输出。
func CommandContext(args ...string) ([]byte, error) {
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Env = os.Environ()
	output, err := cmd.CombinedOutput()
	if err != nil {
		return output, fmt.Errorf("%s: %w\n%s", args[0], err, string(output))
	}
	return output, nil
}
