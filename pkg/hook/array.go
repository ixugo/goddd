package hook

import "slices"

// Reverse 反转数组
func Reverse[T any](s []T) []T {
	arr := slices.Clone(s)
	i, j := 0, len(s)-1
	for i < j {
		arr[i], arr[j] = arr[j], arr[i]
		i++
		j--
	}
	return arr
}

// Unique 切片中所有值都是唯一，返回 true
func Unique[T comparable](values []T) bool {
	uniqueValues := make(map[T]struct{}, len(values))
	for _, v := range values {
		uniqueValues[v] = struct{}{}
	}
	return len(values) == len(uniqueValues)
}

// Any 存在指定的值即返回 true
func Any[T comparable](items []T, callback func(T) bool) bool {
	return slices.ContainsFunc(items, callback)
}

// DeduplicationFunc 自定义条件去重
func DeduplicationFunc[T comparable](vs []T, fn func(T) string) []T {
	uniqueMap := make(map[string]struct{}, len(vs))
	uniqueSlice := make([]T, 0, len(vs))
	for _, v := range vs {
		key := fn(v)
		if _, ok := uniqueMap[key]; !ok {
			uniqueMap[key] = struct{}{}
			uniqueSlice = append(uniqueSlice, v)
		}
	}
	return uniqueSlice
}

// Deduplication 去重
func Deduplication[T comparable](vs ...T) []T {
	uniqueMap := make(map[T]struct{}, len(vs))
	uniqueSlice := make([]T, 0, len(vs))
	for _, v := range vs {
		if _, ok := uniqueMap[v]; !ok {
			uniqueMap[v] = struct{}{}
			uniqueSlice = append(uniqueSlice, v)
		}
	}
	return uniqueSlice
}
