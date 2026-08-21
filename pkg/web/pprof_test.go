package web

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// setupPProfEngine 构建挂载了 pprof 白名单中间件的测试引擎
func setupPProfEngine(ips []string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	SetupPProf(r, &ips)
	return r
}

// doPProfRequest 以指定直连地址与 X-Forwarded-For 头发起 pprof 请求
// xff 为空串时不设置该头部
func doPProfRequest(r *gin.Engine, remoteAddr, xff string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/debug/pprof/cmdline", nil)
	req.RemoteAddr = remoteAddr
	if xff != "" {
		req.Header.Set("X-Forwarded-For", xff)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// TestDebugAccessEmptyXFF 验证 XFF 头部存在但值为空串时按直连处理
func TestDebugAccessEmptyXFF(t *testing.T) {
	ips := []string{"5.6.7.8"}
	r := setupPProfEngine(ips)

	req := httptest.NewRequest(http.MethodGet, "/debug/pprof/cmdline", nil)
	req.RemoteAddr = "5.6.7.8:1234"
	req.Header["X-Forwarded-For"] = []string{""}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("白名单内直连 IP 携空串 XFF 应放行, 得到状态码 %d", w.Code)
	}
}

// TestDebugAccessSpoofXFF 验证伪造 X-Forwarded-For 无法绕过 IP 白名单
func TestDebugAccessSpoofXFF(t *testing.T) {
	ips := []string{"1.2.3.4"}
	r := setupPProfEngine(ips)

	w := doPProfRequest(r, "5.6.7.8:1234", "1.2.3.4")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("伪造 X-Forwarded-For 应被拒绝, 得到状态码 %d", w.Code)
	}
}

// TestDebugAccessRealIP 验证白名单按真实直连 IP 放行与拦截
func TestDebugAccessRealIP(t *testing.T) {
	ips := []string{"5.6.7.8"}
	r := setupPProfEngine(ips)

	w := doPProfRequest(r, "5.6.7.8:1234", "")
	if w.Code != http.StatusOK {
		t.Fatalf("白名单内的真实 IP 应放行, 得到状态码 %d", w.Code)
	}

	w = doPProfRequest(r, "9.9.9.9:1234", "")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("白名单外的真实 IP 应拒绝, 得到状态码 %d", w.Code)
	}
}

// TestDebugAccessViaProxy 验证经可信代理时按 XFF 最右段(代理追加的真实上游)鉴权
func TestDebugAccessViaProxy(t *testing.T) {
	// 10.0.0.1 为 nginx 出口, 9.9.9.9 为允许的客户端
	ips := []string{"10.0.0.1", "9.9.9.9"}
	r := setupPProfEngine(ips)

	// nginx 追加真实客户端 IP, 两端皆在白名单, 应放行
	w := doPProfRequest(r, "10.0.0.1:80", "9.9.9.9")
	if w.Code != http.StatusOK {
		t.Fatalf("代理追加的真实 IP 在白名单内应放行, 得到状态码 %d", w.Code)
	}

	// 客户端真实 IP 不在白名单, 应拒绝
	w = doPProfRequest(r, "10.0.0.1:80", "8.8.8.8")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("代理链中的非白名单客户端应拒绝, 得到状态码 %d", w.Code)
	}

	// 攻击者伪造首段 1.2.3.4(在白名单内), nginx 追加真实 IP 后最右段为 9.9.9.9 之外的值, 应拒绝
	w = doPProfRequest(r, "10.0.0.1:80", "1.2.3.4, 8.8.8.8")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("伪造 XFF 首段不应绕过, 得到状态码 %d", w.Code)
	}

	// 代理 IP 本身不在白名单, 即使 XFF 可信也应拒绝
	w = doPProfRequest(r, "10.0.0.2:80", "9.9.9.9")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("非白名单代理应拒绝, 得到状态码 %d", w.Code)
	}

	// XFF 最右段非法, 宁可错杀
	w = doPProfRequest(r, "10.0.0.1:80", "9.9.9.9, not-an-ip")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("XFF 最右段非法应拒绝, 得到状态码 %d", w.Code)
	}
}
