package gen

import (
	"bytes"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	gofumpt "mvdan.cc/gofumpt/format"
)

//go:embed *.go.tmpl
var files embed.FS

// Data 模板渲染的顶层数据结构。
type Data struct {
	Name        []Name
	PackageName string
}

// Name 结构体名和包名的映射。
type Name struct {
	TableName   string
	PackageName string
}

var funcMap = template.FuncMap{
	"ToUpperCamelCase":                  UnderscoreToUpperCamelCase,
	"ToLowerCamelCase":                  UnderscoreToLowerCamelCase,
	"ToUnderscore":                      CamelCaseToUnderscore,
	"Plural":                            Plural,
	"ToComment":                         ToComment,
	"IfUpperUnderscoreToUpperCamelCase": IfUpperUnderscoreToUpperCamelCase,
	"ToUpper":                           ToUpper,
	"FirstLetter":                       FirstLetter,
}

// Start 从文件路径解析领域模型并生成 DDD 代码。
func Start(path, module string) error {
	domain, err := ParseFile(path)
	if err != nil {
		return err
	}
	_, err = startGenerate(domain, module)
	return err
}

// StartFromContent 从 Go 源代码内容生成 DDD 代码，返回生成的文件列表。
func StartFromContent(content, module string) ([]string, error) {
	domain, err := ParseContent(content)
	if err != nil {
		return nil, fmt.Errorf("解析源代码失败: %w", err)
	}
	return startGenerate(domain, module)
}

// StartFromContentWithProgress 从 Go 源代码内容生成 DDD 代码，带进度回调。
// onStep 在每个子步骤完成时调用。
func StartFromContentWithProgress(content, module string, onStep func(label string)) ([]string, error) {
	domain, err := ParseContent(content)
	if err != nil {
		return nil, fmt.Errorf("解析源代码失败: %w", err)
	}
	return startGenerateWithProgress(domain, module, onStep)
}

// startGenerateWithProgress 核心代码生成逻辑，带进度回调。
// onStep 在每个子步骤完成时调用；为 nil 时忽略。
func startGenerateWithProgress(domain *Domain, module string, onStep func(label string)) ([]string, error) {
	if onStep == nil {
		onStep = func(string) {}
	}
	domain.ModuleName = module
	out := make(map[string]*bytes.Buffer)

	onStep("生成领域模型")
	if err := handlerDomainModel(domain, out); err != nil {
		return nil, err
	}
	onStep("生成业务代码")
	if err := handlerDomainCore(domain, out); err != nil {
		return nil, err
	}
	onStep("生成数据存储")
	if err := handlerDomainDB(domain, out); err != nil {
		return nil, err
	}
	onStep("生成缓存代码")
	if err := handlerDomainCache(domain, out); err != nil {
		return nil, err
	}

	onStep("生成 REST 路由")
	{
		tp, err := generateModelCode(domain)
		if err != nil {
			return nil, err
		}
		apiFile := bytes.NewBuffer(nil)
		out[fmt.Sprintf("internal/web/api/%s.go", domain.PackageName)] = apiFile

		tpl := template.Must(template.New("abc").Funcs(funcMap).
			ParseFS(files, "api.go.tmpl", "db.go.tmpl"))

		if err := tpl.ExecuteTemplate(apiFile, "api.go.tmpl", tp); err != nil {
			panic(err)
		}

		for k, v := range out {
			_ = os.MkdirAll(filepath.Dir(k), os.ModePerm)
			data := v.Bytes()
			if strings.HasSuffix(k, ".go") {
				data = formatGoSource(k, data)
			}
			if err := os.WriteFile(k, data, os.ModePerm); err != nil {
				fmt.Println("⚠️ WriteFile err:", err)
			}
		}
	}

	generatedFiles := make([]string, 0, len(out))
	for k := range out {
		generatedFiles = append(generatedFiles, k)
	}

	onStep("更新依赖注入")
	if FileExists("", "provider.go") {
		const uniqueidName = "NewUniqueID"
		if domain.ExistsUniqueID {
			if _, err := AppendProviderSetArg("", uniqueidName); err != nil {
				return generatedFiles, fmt.Errorf("缺少 NewUniqueID, 请手动更新 provider.go 依赖注入, %w", err)
			}
		}

		apiName := fmt.Sprintf("New%sAPI", UnderscoreToUpperCamelCase(domain.PackageName))
		coreName := fmt.Sprintf("New%sCore", UnderscoreToUpperCamelCase(domain.PackageName))
		if _, err := AppendProviderSetArg("", coreName, apiName); err != nil {
			return generatedFiles, fmt.Errorf("请手动更新 provider.go 依赖注入, %w", err)
		}

		fieldName := fmt.Sprintf("%sAPI", UnderscoreToUpperCamelCase(domain.PackageName))
		if _, err := AppendUsecaseField("", fmt.Sprintf("%s %s", fieldName, fieldName)); err != nil {
			return generatedFiles, fmt.Errorf("请手动更新 provider.go 依赖注入, %w", err)
		}

		if FileExists("", "api.go") {
			funcName := fmt.Sprintf("Register%s", UnderscoreToUpperCamelCase(domain.PackageName))
			line := fmt.Sprintf("%s(r, uc.%s)", funcName, fieldName)
			if _, err := AppendLineToSetupRouter("", funcName, line); err != nil {
				return generatedFiles, fmt.Errorf("请手动更新 api.go 路由, %w", err)
			}
		}
	}

	return generatedFiles, nil
}

// startGenerate 核心代码生成逻辑，供 Start 和 StartFromContent 共用。
func startGenerate(domain *Domain, module string) ([]string, error) {
	files, err := startGenerateWithProgress(domain, module, nil)
	if err != nil {
		return files, err
	}
	if output, err := MakeWire(); err != nil {
		fmt.Fprintf(os.Stderr, "⚠️ 请手动执行 make wire, err: %s\n", string(output))
	}
	return files, nil
}

func handlerDomainModel(out *Domain, bufMap map[string]*bytes.Buffer) error {
	tp, err := generateModelCode(out)
	if err != nil {
		return err
	}
	buf := bytes.NewBuffer(nil)

	tpl := template.Must(template.New("abc").Funcs(funcMap).ParseFS(files, "model.go.tmpl", "engine.model.go.tmpl"))

	if err := tpl.ExecuteTemplate(buf, "model.go.tmpl", tp); err != nil {
		panic(err)
	}
	bufMap[fmt.Sprintf("internal/core/%s/model.go", out.PackageName)] = buf

	for _, v := range tp.Models {
		if v.IsNotDB {
			continue
		}
		v.PackageName = out.PackageName
		buf := bytes.NewBuffer(nil)
		if err := tpl.ExecuteTemplate(buf, "engine.model.go.tmpl", v); err != nil {
			panic(err)
		}
		bufMap[fmt.Sprintf("internal/core/%s/%s.model.go", out.PackageName, CamelCaseToUnderscore(v.Name))] = buf
	}

	return nil
}

func handlerDomainCore(out *Domain, bufMap map[string]*bytes.Buffer) error {
	tp, err := generateModelCode(out)
	if err != nil {
		return err
	}
	buf := bytes.NewBuffer(nil)

	tpl := template.Must(template.New("abc").Funcs(funcMap).ParseFS(files, "core.go.tmpl", "engine.core.go.tmpl", "engine.param.go.tmpl"))

	if err := tpl.ExecuteTemplate(buf, "core.go.tmpl", tp); err != nil {
		panic(err)
	}
	bufMap[fmt.Sprintf("internal/core/%s/core.go", out.PackageName)] = buf

	for _, v := range tp.Models {
		if v.IsNotDB {
			continue
		}
		v.PackageName = out.PackageName
		buf := bytes.NewBuffer(nil)
		if err := tpl.ExecuteTemplate(buf, "engine.core.go.tmpl", v); err != nil {
			panic(err)
		}
		bufMap[fmt.Sprintf("internal/core/%s/%s.go", out.PackageName, CamelCaseToUnderscore(v.Name))] = buf
	}

	for _, v := range tp.Models {
		if v.IsNotDB {
			continue
		}
		v.PackageName = out.PackageName
		buf := bytes.NewBuffer(nil)
		if err := tpl.ExecuteTemplate(buf, "engine.param.go.tmpl", v); err != nil {
			panic(err)
		}
		bufMap[fmt.Sprintf("internal/core/%s/%s.param.go", out.PackageName, CamelCaseToUnderscore(v.Name))] = buf
	}

	return nil
}

func handlerDomainDB(out *Domain, bufMap map[string]*bytes.Buffer) error {
	tp, err := generateModelCode(out)
	if err != nil {
		return err
	}
	buf := bytes.NewBuffer(nil)

	tpl := template.Must(template.New("abc").Funcs(funcMap).ParseFS(files, "engine.db.go.tmpl", "db.go.tmpl", "db_test.go.tmpl", "engine.db_test.go.tmpl"))

	if err := tpl.ExecuteTemplate(buf, "db.go.tmpl", tp); err != nil {
		panic(err)
	}
	bufMap[fmt.Sprintf("internal/core/%s/stores/%sdb/db.go", out.PackageName, out.PackageName)] = buf

	{
		buf := bytes.NewBuffer(nil)
		if err := tpl.ExecuteTemplate(buf, "db_test.go.tmpl", tp); err != nil {
			panic(err)
		}
		bufMap[fmt.Sprintf("internal/core/%s/stores/%sdb/db_test.go", out.PackageName, out.PackageName)] = buf
	}

	for _, v := range tp.Models {
		if v.IsNotDB {
			continue
		}
		v.PackageName = out.PackageName
		v.ModuleName = out.ModuleName
		buf := bytes.NewBuffer(nil)
		if err := tpl.ExecuteTemplate(buf, "engine.db.go.tmpl", v); err != nil {
			panic(err)
		}
		bufMap[fmt.Sprintf("internal/core/%s/stores/%sdb/%s.db.go", out.PackageName, out.PackageName, CamelCaseToUnderscore(v.Name))] = buf

		tbuf := bytes.NewBuffer(nil)
		if err := tpl.ExecuteTemplate(tbuf, "engine.db_test.go.tmpl", v); err != nil {
			panic(err)
		}
		bufMap[fmt.Sprintf("internal/core/%s/stores/%sdb/%s.db_test.go", out.PackageName, out.PackageName, CamelCaseToUnderscore(v.Name))] = tbuf
	}

	return nil
}

func handlerDomainCache(out *Domain, bufMap map[string]*bytes.Buffer) error {
	tp, err := generateModelCode(out)
	if err != nil {
		return err
	}
	buf := bytes.NewBuffer(nil)

	tpl := template.Must(template.New("abc").Funcs(funcMap).ParseFS(files, "cache.go.tmpl", "engine.cache.go.tmpl"))

	if err := tpl.ExecuteTemplate(buf, "cache.go.tmpl", tp); err != nil {
		panic(err)
	}
	bufMap[fmt.Sprintf("internal/core/%s/stores/%scache/cache.go", out.PackageName, out.PackageName)] = buf

	for _, v := range tp.Models {
		if v.IsNotDB {
			continue
		}
		v.PackageName = out.PackageName
		buf := bytes.NewBuffer(nil)
		if err := tpl.ExecuteTemplate(buf, "engine.cache.go.tmpl", v); err != nil {
			panic(err)
		}
		bufMap[fmt.Sprintf("internal/core/%s/stores/%scache/%s.cache.go", out.PackageName, out.PackageName, CamelCaseToUnderscore(v.Name))] = buf
	}

	return nil
}

// formatGoSource 对生成的 Go 源码执行 gofumpt 格式化（包含 gofmt 能力）。
// 格式化失败时返回原始内容，不阻断生成流程。
func formatGoSource(_ string, src []byte) []byte {
	formatted, err := gofumpt.Source(src, gofumpt.Options{})
	if err != nil {
		return src
	}
	return formatted
}
