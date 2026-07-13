package hook

import (
	"strconv"
)

// StringsToInts 将字符串数组转换为整数数组
func StringsToInts(s ...string) []int {
	out := make([]int, 0, len(s))
	for _, v := range s {
		if v == "" {
			continue
		}
		j, _ := strconv.Atoi(v)
		out = append(out, j)
	}
	return out
}

// StringsToMap 将字符串数组转换为字符串映射
func StringsToMap(s ...string) map[string]struct{} {
	out := make(map[string]struct{}, len(s))
	for _, v := range s {
		if v == "" {
			continue
		}
		out[v] = struct{}{}
	}
	return out
}

// IntsToMap 将整数数组转换为整数映射
func IntsToMap(s ...int) map[int]struct{} {
	out := make(map[int]struct{}, len(s))
	for _, v := range s {
		out[v] = struct{}{}
	}
	return out
}

// IntsToStrings 将整数数组转换为字符串数组
func IntsToStrings(s ...int) []string {
	out := make([]string, 0, len(s))
	for _, v := range s {
		out = append(out, strconv.Itoa(v))
	}
	return out
}
