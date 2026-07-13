package orm

import (
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"testing"
)

// User 纯数据结构体，作为 StructMap 泛型参数
type User struct {
	Name  string `json:"name"`
	Age   int    `json:"age"`
	Email string `json:"email"`
}

// StructMap 序列化/反序列化测试

func TestStructMap_UnmarshalJSON(t *testing.T) {
	input := `{"name":"张三","age":25,"email":"zhangsan@example.com","city":"北京"}`
	var sm StructMap[User]
	if err := json.Unmarshal([]byte(input), &sm); err != nil {
		t.Fatalf("UnmarshalJSON 失败: %v", err)
	}
	// 结构体字段应正确解析
	if sm.Data.Name != "张三" {
		t.Errorf("Name = %q, want %q", sm.Data.Name, "张三")
	}
	if sm.Data.Age != 25 {
		t.Errorf("Age = %d, want %d", sm.Data.Age, 25)
	}
	if sm.Data.Email != "zhangsan@example.com" {
		t.Errorf("Email = %q, want %q", sm.Data.Email, "zhangsan@example.com")
	}
	// 未知字段应保留在 Map 中
	if sm.Map.GetString("city") != "北京" {
		t.Errorf("Map[city] = %q, want %q", sm.Map.GetString("city"), "北京")
	}
}

func TestStructMap_MarshalJSON(t *testing.T) {
	sm := StructMap[User]{
		Data: User{Name: "李四", Age: 30, Email: "lisi@example.com"},
		Map:  Map{"city": "上海", "score": 99},
	}
	b, err := json.Marshal(sm)
	if err != nil {
		t.Fatalf("MarshalJSON 失败: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("反向验证失败: %v", err)
	}
	// 结构体字段与 Map 字段应全部存在
	if m["name"] != "李四" {
		t.Errorf("name = %v, want %v", m["name"], "李四")
	}
	if m["city"] != "上海" {
		t.Errorf("city = %v, want %v", m["city"], "上海")
	}
	// Map 中的 score 应保留
	if m["score"] != float64(99) {
		t.Errorf("score = %v, want %v", m["score"], float64(99))
	}
}

func TestStructMap_MarshalJSON_DataOverridesMap(t *testing.T) {
	// 当 Data 和 Map 存在相同 key 时，Data 应覆盖 Map
	sm := StructMap[User]{
		Data: User{Name: "王五"},
		Map:  Map{"name": "旧名字"},
	}
	b, err := json.Marshal(sm)
	if err != nil {
		t.Fatalf("MarshalJSON 失败: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("反向验证失败: %v", err)
	}
	if m["name"] != "王五" {
		t.Errorf("name = %v, want %v (Data 应覆盖 Map)", m["name"], "王五")
	}
}

func TestStructMap_Scan(t *testing.T) {
	input := `{"name":"赵六","age":28,"extra":"data"}`
	var sm StructMap[User]
	if err := sm.Scan([]byte(input)); err != nil {
		t.Fatalf("Scan 失败: %v", err)
	}
	if sm.Data.Name != "赵六" {
		t.Errorf("Name = %q, want %q", sm.Data.Name, "赵六")
	}
	if sm.Map.GetString("extra") != "data" {
		t.Errorf("Map[extra] = %q, want %q", sm.Map.GetString("extra"), "data")
	}
}

func TestStructMap_Value(t *testing.T) {
	sm := StructMap[User]{
		Data: User{Name: "钱七", Age: 35},
		Map:  Map{"role": "admin"},
	}
	v, err := sm.Value()
	if err != nil {
		t.Fatalf("Value 失败: %v", err)
	}
	b, ok := v.([]byte)
	if !ok {
		t.Fatalf("Value 返回类型 %T, want []byte", v)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("反序列化失败: %v", err)
	}
	if m["name"] != "钱七" {
		t.Errorf("name = %v, want %v", m["name"], "钱七")
	}
	if m["role"] != "admin" {
		t.Errorf("role = %v, want %v", m["role"], "admin")
	}
}

// Map 测试

func TestMap_Get(t *testing.T) {
	var nilMap Map
	if nilMap.Get("a") != nil {
		t.Error("nil Map Get 应返回 nil")
	}
	m := Map{"x": 1, "y": "hello"}
	if m.Get("x") != 1 {
		t.Errorf("Get(x) = %v, want 1", m.Get("x"))
	}
	if m.Get("y") != "hello" {
		t.Errorf("Get(y) = %v, want hello", m.Get("y"))
	}
	if m.Get("z") != nil {
		t.Errorf("Get(z) = %v, want nil", m.Get("z"))
	}
}

func TestMap_GetString(t *testing.T) {
	var nilMap Map
	if nilMap.Get("a") != nil {
		t.Error("nil Map GetString 应返回空字符串")
	}
	m := Map{"s": "abc", "n": 123}
	if v := m.GetString("s"); v != "abc" {
		t.Errorf("GetString(s) = %q, want %q", v, "abc")
	}
	if v := m.GetString("n"); v != "" {
		t.Errorf("GetString(n) = %q, want %q", v, "")
	}
	if v := m.GetString("missing"); v != "" {
		t.Errorf("GetString(missing) = %q, want %q", v, "")
	}
}

func TestMap_GetInt(t *testing.T) {
	var nilMap Map
	if nilMap.GetInt("a") != 0 {
		t.Error("nil Map GetInt 应返回 0")
	}
	m := Map{"i": 42, "i64": int64(64), "f": 3.14, "s": "str"}
	if v := m.GetInt("i"); v != 42 {
		t.Errorf("GetInt(i) = %d, want 42", v)
	}
	if v := m.GetInt("i64"); v != 64 {
		t.Errorf("GetInt(i64) = %d, want 64", v)
	}
	if v := m.GetInt("f"); v != 3 {
		t.Errorf("GetInt(f) = %d, want 3", v)
	}
	if v := m.GetInt("s"); v != 0 {
		t.Errorf("GetInt(s) = %d, want 0", v)
	}
}

func TestMap_GetBool(t *testing.T) {
	var nilMap Map
	if nilMap.GetBool("a") != false {
		t.Error("nil Map GetBool 应返回 false")
	}
	m := Map{"t": true, "f": false, "s": "str"}
	if v := m.GetBool("t"); v != true {
		t.Errorf("GetBool(t) = %v, want true", v)
	}
	if v := m.GetBool("f"); v != false {
		t.Errorf("GetBool(f) = %v, want false", v)
	}
	if v := m.GetBool("s"); v != false {
		t.Errorf("GetBool(s) = %v, want false", v)
	}
}

func TestMap_MarshalUnmarshalJSON(t *testing.T) {
	m := Map{"a": 1, "b": "two", "c": true}
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("Marshal 失败: %v", err)
	}
	var m2 Map
	if err := json.Unmarshal(b, &m2); err != nil {
		t.Fatalf("Unmarshal 失败: %v", err)
	}
	if m2.GetInt("a") != 1 {
		t.Errorf("a = %v, want 1", m2["a"])
	}
	if m2.GetString("b") != "two" {
		t.Errorf("b = %v, want two", m2["b"])
	}
	if m2.GetBool("c") != true {
		t.Errorf("c = %v, want true", m2["c"])
	}
}

func TestMap_UnmarshalJSON_TooLarge(t *testing.T) {
	big := make([]byte, 5000)
	for i := range big {
		big[i] = 'x'
	}
	input := `{"data":"` + string(big) + `"}`
	var m Map
	if err := json.Unmarshal([]byte(input), &m); err == nil {
		t.Error("超过 4096 字节应报错")
	}
}

func TestMap_Scan(t *testing.T) {
	input := `{"key":"value"}`
	var m Map
	if err := m.Scan([]byte(input)); err != nil {
		t.Fatalf("Scan 失败: %v", err)
	}
	if m.GetString("key") != "value" {
		t.Errorf("key = %q, want %q", m.GetString("key"), "value")
	}
}

func TestMap_Value(t *testing.T) {
	m := Map{"k": "v"}
	v, err := m.Value()
	if err != nil {
		t.Fatalf("Value 失败: %v", err)
	}
	if _, ok := v.([]byte); !ok {
		t.Fatalf("Value 返回类型 %T, want []byte", v)
	}
}

func TestMap_Has(t *testing.T) {
	var nilMap Map
	if nilMap.Has("a") {
		t.Error("nil Map Has 应返回 false")
	}
	m := Map{"exists": 1}
	if !m.Has("exists") {
		t.Error("Has(exists) 应返回 true")
	}
	if m.Has("missing") {
		t.Error("Has(missing) 应返回 false")
	}
}

func TestMap_Set(t *testing.T) {
	m := make(Map)
	m.Set("a", 1)
	if m.GetInt("a") != 1 {
		t.Errorf("Set 后 Get(a) = %v, want 1", m["a"])
	}
	// 链式调用
	m.Set("b", 2).Set("c", 3)
	if m.GetInt("b") != 2 || m.GetInt("c") != 3 {
		t.Error("链式 Set 失败")
	}
}

func TestMap_Delete(t *testing.T) {
	var nilMap Map
	nilMap.Delete("a") // 不应 panic
	m := Map{"a": 1, "b": 2}
	m.Delete("a")
	if m.Has("a") {
		t.Error("Delete 后 Has(a) 应返回 false")
	}
	if !m.Has("b") {
		t.Error("Has(b) 应返回 true")
	}
}

func TestMap_Merge(t *testing.T) {
	// 注意：nil Map 调用 Merge 存在已知限制——
	// Go 中 map 是值类型，方法内 make 对调用方不可见
	m := Map{"a": 1, "b": 2}
	m.Merge(Map{"b": 99, "c": 3})
	if m.GetInt("a") != 1 {
		t.Errorf("Merge 后 a = %v, want 1", m["a"])
	}
	if m.GetInt("b") != 99 {
		t.Errorf("Merge 后 b = %v, want 99 (应被覆盖)", m["b"])
	}
	if m.GetInt("c") != 3 {
		t.Errorf("Merge 后 c = %v, want 3", m["c"])
	}
}

// 辅助函数，确保接口实现
var (
	_ json.Unmarshaler = (*Map)(nil)
	_ driver.Valuer    = Map(nil)
	_ sql.Scanner      = (*Map)(nil)

	_ json.Unmarshaler = (*StructMap[User])(nil)
	_ json.Marshaler   = StructMap[User]{}
	_ driver.Valuer    = StructMap[User]{}
	_ sql.Scanner      = (*StructMap[User])(nil)
)

// BenchmarkJSON 比较 Map 与 StructMap 的序列化/反序列化性能
func BenchmarkJSON(b *testing.B) {
	raw := []byte(`{"name":"张三","age":25,"email":"zhangsan@example.com","city":"北京","score":99,"active":true}`)
	m := Map{"name": "张三", "age": 25, "email": "zhangsan@example.com", "city": "北京", "score": 99, "active": true}
	sm := StructMap[User]{
		Data: User{Name: "张三", Age: 25, Email: "zhangsan@example.com"},
		Map:  Map{"city": "北京", "score": 99, "active": true},
	}

	b.Run("Map/Unmarshal", func(b *testing.B) {
		for b.Loop() {
			var dst Map
			_ = json.Unmarshal(raw, &dst)
		}
	})
	b.Run("StructMap/Unmarshal", func(b *testing.B) {
		for b.Loop() {
			var dst StructMap[User]
			_ = json.Unmarshal(raw, &dst)
		}
	})
	b.Run("Map/Marshal", func(b *testing.B) {
		for b.Loop() {
			_, _ = json.Marshal(m)
		}
	})
	b.Run("StructMap/Marshal", func(b *testing.B) {
		for b.Loop() {
			_, _ = json.Marshal(sm)
		}
	})
}
