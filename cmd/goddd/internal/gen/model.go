package gen

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"strings"
	"unicode"
)

// ModelTmpl 模板渲染数据。
type ModelTmpl struct {
	PackageName string
	ModuleName  string
	Models      []Table

	ExistsUniqueID bool

	Consts []string
	Vars   []string
	Funcs  []string
}

// Table 单个结构体的模板数据。
type Table struct {
	PackageName    string
	Name           string
	Lines          []Line
	IsNotDB        bool
	IsStringID     bool
	ExistsUniqueID bool
}

// Line 结构体字段。
type Line struct {
	Name    string
	Type    string
	Tag     string
	Comment string
}

// Domain 从 Go 文件中提取的领域信息。
type Domain struct {
	ModuleName  string
	PackageName string
	Models      []*Models

	ExistsUniqueID bool

	Consts []string
	Vars   []string
	Funcs  []string
}

// Models 单个结构体解析结果。
type Models struct {
	Name       string
	Ident      []Attribute
	IsNotDB    bool
	IsStringID bool

	ExistsUniqueID bool
}

// Attribute 结构体字段解析结果。
type Attribute struct {
	Name    string
	Type    ast.Expr
	Model   *Models
	Comment string
}

// ParseFile 从文件路径解析领域模型。
func ParseFile(path string) (*Domain, error) {
	fileSet := token.NewFileSet()
	node, err := parser.ParseFile(fileSet, path, nil, parser.ParseComments|parser.AllErrors)
	if err != nil {
		return nil, err
	}
	return parseAST(fileSet, node)
}

// ParseContent 从 Go 源代码内容解析领域模型。
func ParseContent(content string) (*Domain, error) {
	fileSet := token.NewFileSet()
	node, err := parser.ParseFile(fileSet, "model.go", content, parser.ParseComments|parser.AllErrors)
	if err != nil {
		return nil, err
	}
	return parseAST(fileSet, node)
}

// parseAST 从 AST 节点解析领域模型。
func parseAST(fileSet *token.FileSet, node *ast.File) (*Domain, error) {
	var out Domain
	out.PackageName = node.Name.Name

	ast.Inspect(node, func(n ast.Node) bool {
		genDecl, ok := n.(*ast.GenDecl)
		if ok && genDecl.Tok == token.TYPE {
			for _, spec := range genDecl.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				if structType, ok := typeSpec.Type.(*ast.StructType); ok {
					var model Models
					model.Name = typeSpec.Name.Name

					if len(model.Name) > 0 && unicode.IsLower(rune(model.Name[0])) {
						model.IsNotDB = true
					}

					for _, field := range structType.Fields.List {
						for _, name := range field.Names {
							comment := strings.TrimSpace(field.Comment.Text())
							model.Ident = append(model.Ident, Attribute{
								Name:    name.Name,
								Type:    field.Type,
								Comment: comment,
							})
						}
					}
					out.Models = append(out.Models, &model)
				}
			}
		}

		if ok && (genDecl.Tok == token.CONST || genDecl.Tok == token.VAR) {
			var buf bytes.Buffer
			_ = printer.Fprint(&buf, fileSet, genDecl)
			code := buf.String()
			if genDecl.Tok == token.CONST {
				out.Consts = append(out.Consts, code)
			} else {
				out.Vars = append(out.Vars, code)
			}
		}

		if fn, ok := n.(*ast.FuncDecl); ok {
			var buf bytes.Buffer
			_ = printer.Fprint(&buf, fileSet, fn)
			out.Funcs = append(out.Funcs, buf.String())
		}
		return true
	})
	return &out, nil
}

func getStructName(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.SelectorExpr:
		return e.Sel.Name
	case *ast.ArrayType:
		return getStructName(e.Elt)
	}
	return ""
}

func getFullStructName(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.SelectorExpr:
		pkgIdent, ok := e.X.(*ast.Ident)
		if ok {
			return pkgIdent.Name + "." + e.Sel.Name
		}
		return e.Sel.Name
	case *ast.ArrayType:
		return getFullStructName(e.Elt)
	}
	return ""
}

func generateModelCode(domain *Domain) (*ModelTmpl, error) {
	structs := make(map[string]*Models)
	for _, model := range domain.Models {
		structs[model.Name] = model
	}

	for _, model := range domain.Models {
		for _, filed := range model.Ident {
			_, ok := filed.Type.(*ast.StructType)
			if ok {
				continue
			}

			aname := getStructName(filed.Type)
			if f, ok := structs[aname]; ok {
				f.IsNotDB = true
			}

			if filed.Name == "ID" {
				if getStructName(filed.Type) == "string" {
					model.IsStringID = true
				}
				if getFullStructName(filed.Type) == "uniqueid.Core" {
					model.ExistsUniqueID = true
					model.IsStringID = true
					domain.ExistsUniqueID = true
				}
			}
		}
	}

	otmpl := ModelTmpl{
		PackageName:    domain.PackageName,
		ModuleName:     domain.ModuleName,
		ExistsUniqueID: domain.ExistsUniqueID,
		Consts:         domain.Consts,
		Vars:           domain.Vars,
		Funcs:          domain.Funcs,
	}

	for _, model := range domain.Models {
		lines := make([]Line, 0, 8)
		for _, ident := range model.Ident {
			var tag strings.Builder
			tag.WriteString(fmt.Sprintf(`gorm:"column:%s`, CamelCaseToUnderscore(ident.Name)))

			if _, ok := ident.Type.(*ast.StarExpr); !ok {
				defaultValue := generateTagGormDefaultValue(ident.Type)
				if defaultValue != "" {
					tag.WriteString(";notNull")
					tag.WriteString(";default:" + defaultValue)
				}
				if defaultValue == "'{}'" {
					tag.WriteString(";type:jsonb")
				}
			}

			if ident.Comment != "" {
				tag.WriteString(";comment:" + ident.Comment)
			}
			tag.WriteString(`"`)

			line := Line{
				Name:    ident.Name,
				Type:    fieldTypeToString(ident.Type),
				Tag:     tag.String(),
				Comment: ident.Comment,
			}
			if ident.Name == "ID" {
				line.Tag = `gorm:"primaryKey"`
			}

			lines = append(lines, line)
		}
		otmpl.Models = append(otmpl.Models, Table{
			Name:           model.Name,
			Lines:          lines,
			IsNotDB:        model.IsNotDB,
			IsStringID:     model.IsStringID,
			ExistsUniqueID: model.ExistsUniqueID,
		})
	}

	return &otmpl, nil
}

func generateTagGormDefaultValue(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.StarExpr:
		return ""
	case *ast.Ident:
		switch e.Name {
		case "int", "int8", "int16", "int32", "int64", "float32", "float64":
			return "0"
		case "string":
			return "''"
		case "bool":
			return "FALSE"
		case "time":
			return "CURRENT_TIMESTAMP"
		default:
			return "'{}'"
		}
	case *ast.BasicLit:
		switch e.Kind {
		case token.INT:
			return "0"
		case token.STRING:
			return ""
		}
	case *ast.StructType:
		return "'{}'"
	case *ast.SelectorExpr:
		pkgIdent, ok := e.X.(*ast.Ident)
		if ok {
			if e.Sel.Name == "Time" {
				return "CURRENT_TIMESTAMP"
			}
			fullName := pkgIdent.Name + "." + e.Sel.Name
			switch fullName {
			case "uniqueid.Core":
				return "string"
			case "time.Duration":
				return "0"
			case "orm.DeletedAt", "gorm.DeletedAt":
				return ""
			}
		}
		return "'{}'"
	}
	return ""
}

func fieldTypeToString(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.SelectorExpr:
		pkgIdent, ok := e.X.(*ast.Ident)
		if ok && pkgIdent.Name+"."+e.Sel.Name == "uniqueid.Core" {
			return "string"
		}
		return fmt.Sprintf("%s.%s", fieldTypeToString(e.X), e.Sel.Name)
	case *ast.ArrayType:
		return "[]" + fieldTypeToString(e.Elt)
	case *ast.StarExpr:
		return "*" + fieldTypeToString(e.X)
	case *ast.MapType:
		return fmt.Sprintf("map[%s]%s", fieldTypeToString(e.Key), fieldTypeToString(e.Value))
	case *ast.FuncType:
		return "func(...)"
	default:
		return "unknown"
	}
}

func formatGoCode(sourceCode string) (string, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, "", sourceCode, parser.ParseComments)
	if err != nil {
		return "", fmt.Errorf("error parsing code: %w", err)
	}
	var buf bytes.Buffer
	if err := printer.Fprint(&buf, fset, node); err != nil {
		return "", fmt.Errorf("error formatting code: %w", err)
	}
	return buf.String(), nil
}
