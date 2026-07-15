package gen

import (
	"fmt"
	"strings"
	"unicode"
)

// UnderscoreToUpperCamelCase 首字母大写驼峰。
func UnderscoreToUpperCamelCase(s string) string {
	s = strings.Replace(s, "_", " ", -1)
	s = strings.Title(s) //nolint:staticcheck
	return strings.Replace(s, " ", "", -1)
}

// IfUpperUnderscoreToUpperCamelCase 如果首字母是大写，则首字母大写驼峰。
func IfUpperUnderscoreToUpperCamelCase(s string) string {
	if len(s) > 0 && unicode.IsLower(rune(s[0])) {
		return fmt.Sprintf("%c%s", rune(s[0]), UnderscoreToUpperCamelCase(s)[1:])
	}
	s = strings.Replace(s, "_", " ", -1)
	s = strings.Title(s) //nolint:staticcheck
	return strings.Replace(s, " ", "", -1)
}

// UnderscoreToLowerCamelCase 首字母小写驼峰。
func UnderscoreToLowerCamelCase(s string) string {
	s = UnderscoreToUpperCamelCase(s)
	return string(unicode.ToLower(rune(s[0]))) + s[1:]
}

// CamelCaseToUnderscore 驼峰转下划线。
func CamelCaseToUnderscore(s string) string {
	output := make([]rune, 0, len(s))
	var lastIsLower bool
	for _, r := range s {
		if lastIsLower && unicode.IsUpper(r) {
			output = append(output, '_')
		}
		output = append(output, unicode.ToLower(r))
		if !unicode.IsDigit(r) {
			lastIsLower = unicode.IsLower(r)
		}
	}
	return string(output)
}

// Plural 简单复数化（追加 s）。
func Plural(s string) string {
	s = CamelCaseToUnderscore(s)
	if strings.HasSuffix(s, "s") {
		return s
	}
	return s + "s"
}

// ToComment 为非空字符串添加注释前缀。
func ToComment(s string) string {
	if s == "" {
		return ""
	}
	return "// " + s
}

// ToUpper 转大写。
func ToUpper(s string) string {
	return strings.ToUpper(s)
}

// FirstLetter 首字母小写。
func FirstLetter(s string) string {
	if len(s) == 0 {
		return ""
	}
	return strings.ToLower(s[:1])
}
