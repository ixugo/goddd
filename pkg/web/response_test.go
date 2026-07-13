package web

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// reqBind 同时带 form 和 uri tag 的请求结构体
type reqBind struct {
	ID   int    `uri:"id"  form:"id"`
	Name string `uri:"name" form:"name"`
	Page int    `form:"page"`
}

func init() {
	gin.SetMode(gin.TestMode)
}

// setupBindRouter 构建测试路由，覆盖 GET/POST/PUT/PATCH/DELETE 五种方法
func setupBindRouter() *gin.Engine {
	r := gin.New()

	handler := func(c *gin.Context, req *reqBind) (map[string]any, error) {
		return map[string]any{
			"id":   req.ID,
			"name": req.Name,
			"page": req.Page,
		}, nil
	}

	// GET: 仅查询参数，无 uri 参数
	r.GET("/items", WrapH(handler))

	// GET: 带 uri 参数 + 查询参数
	r.GET("/items/:id/:name", WrapH(handler))

	// POST: uri 参数 + body
	r.POST("/items/:id/:name", WrapH(handler))

	// PUT: uri 参数 + body
	r.PUT("/items/:id/:name", WrapH(handler))

	// PATCH: uri 参数 + body
	r.PATCH("/items/:id/:name", WrapH(handler))

	// DELETE: uri 参数 + 查询参数（无 body）
	r.DELETE("/items/:id/:name", WrapH(handler))

	// DELETE: uri 参数 + body
	r.DELETE("/items/:id/:name/body", WrapH(handler))

	return r
}

func TestBind_GetQueryOnly(t *testing.T) {
	r := setupBindRouter()

	req := httptest.NewRequest(http.MethodGet, "/items?id=1&name=alice&page=3", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("期望 200，实际 %d, body: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp["id"].(float64) != 1 {
		t.Errorf("id = %v, 期望 1", resp["id"])
	}
	if resp["name"].(string) != "alice" {
		t.Errorf("name = %v, 期望 alice", resp["name"])
	}
	if resp["page"].(float64) != 3 {
		t.Errorf("page = %v, 期望 3", resp["page"])
	}
}

func TestBind_GetWithURI(t *testing.T) {
	r := setupBindRouter()

	// URI 参数 + 查询参数 page
	req := httptest.NewRequest(http.MethodGet, "/items/42/bob?page=5", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("期望 200，实际 %d, body: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp["id"].(float64) != 42 {
		t.Errorf("id = %v, 期望 42", resp["id"])
	}
	if resp["name"].(string) != "bob" {
		t.Errorf("name = %v, 期望 bob", resp["name"])
	}
	if resp["page"].(float64) != 5 {
		t.Errorf("page = %v, 期望 5", resp["page"])
	}
}

func TestBind_PostWithURIAndBody(t *testing.T) {
	r := setupBindRouter()

	body := bytes.NewBufferString(`{"id":99,"name":"charlie","page":7}`)
	req := httptest.NewRequest(http.MethodPost, "/items/99/charlie", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("期望 200，实际 %d, body: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp["id"].(float64) != 99 {
		t.Errorf("id = %v, 期望 99", resp["id"])
	}
	if resp["name"].(string) != "charlie" {
		t.Errorf("name = %v, 期望 charlie", resp["name"])
	}
	if resp["page"].(float64) != 7 {
		t.Errorf("page = %v, 期望 7", resp["page"])
	}
}

func TestBind_PutWithURIAndBody(t *testing.T) {
	r := setupBindRouter()

	body := bytes.NewBufferString(`{"id":11,"name":"dave","page":2}`)
	req := httptest.NewRequest(http.MethodPut, "/items/11/dave", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("期望 200，实际 %d, body: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp["id"].(float64) != 11 {
		t.Errorf("id = %v, 期望 11", resp["id"])
	}
	if resp["name"].(string) != "dave" {
		t.Errorf("name = %v, 期望 dave", resp["name"])
	}
	if resp["page"].(float64) != 2 {
		t.Errorf("page = %v, 期望 2", resp["page"])
	}
}

func TestBind_PatchWithURIAndBody(t *testing.T) {
	r := setupBindRouter()

	body := bytes.NewBufferString(`{"id":7,"name":"eve","page":9}`)
	req := httptest.NewRequest(http.MethodPatch, "/items/7/eve", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("期望 200，实际 %d, body: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp["id"].(float64) != 7 {
		t.Errorf("id = %v, 期望 7", resp["id"])
	}
	if resp["name"].(string) != "eve" {
		t.Errorf("name = %v, 期望 eve", resp["name"])
	}
	if resp["page"].(float64) != 9 {
		t.Errorf("page = %v, 期望 9", resp["page"])
	}
}

func TestBind_DeleteQueryOnly(t *testing.T) {
	r := setupBindRouter()

	// DELETE 无 body，走 ShouldBindQuery
	req := httptest.NewRequest(http.MethodDelete, "/items/8/frank?page=4", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("期望 200，实际 %d, body: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp["id"].(float64) != 8 {
		t.Errorf("id = %v, 期望 8", resp["id"])
	}
	if resp["name"].(string) != "frank" {
		t.Errorf("name = %v, 期望 frank", resp["name"])
	}
	if resp["page"].(float64) != 4 {
		t.Errorf("page = %v, 期望 4", resp["page"])
	}
}

func TestBind_DeleteWithBody(t *testing.T) {
	r := setupBindRouter()

	// DELETE 有 body，走 ShouldBind
	body := bytes.NewBufferString(`{"id":20,"name":"grace","page":6}`)
	req := httptest.NewRequest(http.MethodDelete, "/items/20/grace/body", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("期望 200，实际 %d, body: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp["id"].(float64) != 20 {
		t.Errorf("id = %v, 期望 20", resp["id"])
	}
	if resp["name"].(string) != "grace" {
		t.Errorf("name = %v, 期望 grace", resp["name"])
	}
	if resp["page"].(float64) != 6 {
		t.Errorf("page = %v, 期望 6", resp["page"])
	}
}

// reqWithValidation 回归测试结构体：无 uri tag，只有 json + binding:"required"。
// 旧版 ShouldBindUri 会对此结构体提前触发 validator，在 Body 未绑定时因零值报 required 失败。
type reqWithValidation struct {
	Name   string `json:"name"   binding:"required"`
	Status int    `json:"status" binding:"required,oneof=1 2"`
}

func setupValidationRouter() *gin.Engine {
	r := gin.New()
	handler := func(c *gin.Context, req *reqWithValidation) (map[string]any, error) {
		return map[string]any{
			"code":   c.Param("code"),
			"name":   req.Name,
			"status": req.Status,
		}, nil
	}
	r.PUT("/items/:code", WrapH(handler))
	return r
}

// TestBind_PutWithRequiredValidation 回归测试：PUT /:code + JSON body + required 校验。
// 旧版 WrapH 中 ShouldBindUri 先于 ShouldBind 执行且触发 validator，
// 此时 Body 未绑定 → Name="" Status=0 → required 校验全部失败返回 400。
// 修复后 URI 参数用 binding.MapFormWithTag 映射（不触发 validator），
// validator 延迟到 ShouldBind 时统一触发，此时所有字段已填充 → 200。
func TestBind_PutWithRequiredValidation(t *testing.T) {
	r := setupValidationRouter()

	body := bytes.NewBufferString(`{"name":"test","status":1}`)
	req := httptest.NewRequest(http.MethodPut, "/items/abc123", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("期望 200，实际 %d, body: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp["name"].(string) != "test" {
		t.Errorf("name = %v, 期望 test", resp["name"])
	}
	if resp["status"].(float64) != 1 {
		t.Errorf("status = %v, 期望 1", resp["status"])
	}
}

// TestBind_PutWithRequiredValidation_MissingField 确认 required 校验仍然生效。
func TestBind_PutWithRequiredValidation_MissingField(t *testing.T) {
	r := setupValidationRouter()

	body := bytes.NewBufferString(`{"status":1}`)
	req := httptest.NewRequest(http.MethodPut, "/items/abc123", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code == http.StatusOK {
		t.Fatal("缺少 required 字段 name 应返回 400")
	}
}

// reqMixedURIAndQuery URI 参数 + Query 参数混合，且 uri 字段带 required 校验。
// 验证 bindURIOnly 先填充 uri 字段后，ShouldBindQuery 的 validator 不会误报。
type reqMixedURIAndQuery struct {
	Code string `uri:"code" binding:"required"`
	Page int    `form:"page"`
	Name string `form:"name" binding:"required"`
}

// TestBind_GetWithURIRequiredAndQuery 验证 GET + URI(required) + Query(required)：
// bindURIOnly 先填 Code → ShouldBindQuery 填 Page/Name 并触发 validator → Code 已有值不报错。
func TestBind_GetWithURIRequiredAndQuery(t *testing.T) {
	r := gin.New()
	handler := func(c *gin.Context, req *reqMixedURIAndQuery) (map[string]any, error) {
		return map[string]any{
			"code": req.Code,
			"page": req.Page,
			"name": req.Name,
		}, nil
	}
	r.GET("/items/:code", WrapH(handler))

	req := httptest.NewRequest(http.MethodGet, "/items/xyz?page=3&name=alice", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("期望 200，实际 %d, body: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp["code"].(string) != "xyz" {
		t.Errorf("code = %v, 期望 xyz", resp["code"])
	}
	if resp["page"].(float64) != 3 {
		t.Errorf("page = %v, 期望 3", resp["page"])
	}
	if resp["name"].(string) != "alice" {
		t.Errorf("name = %v, 期望 alice", resp["name"])
	}
}

// TestBind_GetWithURIRequired_MissingQuery 确认 Query 中缺少 required 字段仍正确返回 400。
func TestBind_GetWithURIRequired_MissingQuery(t *testing.T) {
	r := gin.New()
	handler := func(c *gin.Context, req *reqMixedURIAndQuery) (map[string]any, error) {
		return map[string]any{"code": req.Code}, nil
	}
	r.GET("/items/:code", WrapH(handler))

	req := httptest.NewRequest(http.MethodGet, "/items/xyz?page=3", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code == http.StatusOK {
		t.Fatal("缺少 required 字段 name 应返回 400")
	}
}
