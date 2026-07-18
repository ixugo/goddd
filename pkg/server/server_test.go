package server

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"io"
	"log"
	"math"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/ixugo/goddd/pkg/assert"
)

// ==== server.go ====

// TestServer 验证服务能启动并被优雅关闭。
// 端口用 127.0.0.1:0 让系统随机分配，避免端口冲突。
// Shutdown 之后 Serve 必然返回 http.ErrServerClosed，
// 从 Notify 读到这个错误就证明启动和关闭链路都走通了。
func TestServer(t *testing.T) {
	svr := New(http.NewServeMux(), Port("127.0.0.1:0"), DefaultPrintln())
	go svr.Start()

	assert.NoError(t, svr.Shutdown())
	assert.ErrorIs(t, <-svr.Notify(), http.ErrServerClosed)
}

// TestListener 验证传入自定义监听器时，服务直接用这个监听器对外服务。
// 请求能打通且响应内容正确，说明 Listener 选项确实生效了。
func TestListener(t *testing.T) {
	lis, err := net.ListenTCP("tcp", &net.TCPAddr{
		IP:   net.ParseIP("127.0.0.1"),
		Port: 0,
	})
	assert.NoError(t, err)
	if err != nil {
		return
	}

	s := http.NewServeMux()
	s.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("Hello, World!"))
	})

	svr := New(s, Listener(lis))
	go svr.Start()
	t.Cleanup(func() {
		_ = svr.Shutdown()
	})

	resp, err := http.DefaultClient.Get("http://" + lis.Addr().String())
	assert.NoError(t, err)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)
	assert.Equal(t, "Hello, World!", string(body))
}

// ==== options.go ====

// TestPortWithColon 验证传入带冒号的地址时直接用原值，
// 因为用户已经给了完整地址，再拼端口反而会搞坏。
func TestPortWithColon(t *testing.T) {
	s := New(nil, Port("127.0.0.1:9090"))
	assert.Equal(t, "127.0.0.1:9090", s.server.Addr)
}

// TestPortWithoutColon 验证只传端口号时拼成 ":端口" 形式，
// 这样 http.Server 才能监听所有网卡。
func TestPortWithoutColon(t *testing.T) {
	s := New(nil, Port("9090"))
	assert.Equal(t, ":9090", s.server.Addr)
}

// TestShutdownTimeout 验证关闭超时配置确实写进了 Server 字段，
// 防止 Option 写了个寂寞。
func TestShutdownTimeout(t *testing.T) {
	s := New(nil, ShutdownTimeout(7*time.Second))
	assert.Equal(t, 7*time.Second, s.shutdownTimeout)
}

// TestReadTimeout 验证读超时透传到内部 http.Server，
// 超时是打在底层 http.Server 上的，所以要断言内层字段。
func TestReadTimeout(t *testing.T) {
	s := New(nil, ReadTimeout(5*time.Second))
	assert.Equal(t, 5*time.Second, s.server.ReadTimeout)
}

// TestWriteTimeout 验证写超时透传到内部 http.Server，理由同上。
func TestWriteTimeout(t *testing.T) {
	s := New(nil, WriteTimeout(6*time.Second))
	assert.Equal(t, 6*time.Second, s.server.WriteTimeout)
}

// TestErrorLog 验证自定义错误日志器被挂到 http.Server 上，
// 用同一个指针比较，最可靠。
func TestErrorLog(t *testing.T) {
	var buf bytes.Buffer
	logger := log.New(&buf, "test: ", 0)
	s := New(nil, ErrorLog(logger))
	assert.Equal(t, logger, s.server.ErrorLog)
}

// ==== rlimit_unix.go ====

// TestRaiseTooHighLimit 验证请求的 fd 上限超过系统硬限制时返回错误。
// Cur 不能超过硬限制 Max，Setrlimit 必然失败，所以能稳定覆盖错误分支，
// 且设置失败后进程原有的 rlimit 不受影响，不会干扰其他测试。
// Windows 上 Raise 是空实现恒返回 nil，没有硬限制语义，直接跳过。
func TestRaiseTooHighLimit(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows 上 Raise 为空实现，无硬限制语义")
	}
	assert.Error(t, Raise(math.MaxUint64))
}

// ==== starttls.go (StartTLS) ====

// writeSelfSignedCert 生成自签名证书和私钥写到临时目录，
// 因为 StartTLS 只认文件路径，没法直接塞内存里的证书。
// 任何一步失败都直接返回空路径，调用方后续的断言会随之失败，
// 不会带着脏状态继续跑。
func writeSelfSignedCert(t *testing.T) (certFile, keyFile string) {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	assert.NoError(t, err, "生成私钥失败")
	if err != nil {
		return "", ""
	}

	tmpl := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "localhost"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{"localhost"},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}
	der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &key.PublicKey, key)
	assert.NoError(t, err, "生成证书失败")
	if err != nil {
		return "", ""
	}

	dir := t.TempDir()
	certFile = filepath.Join(dir, "cert.pem")
	keyFile = filepath.Join(dir, "key.pem")

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	err = os.WriteFile(certFile, certPEM, 0o644)
	assert.NoError(t, err, "写证书文件失败")
	if err != nil {
		return "", ""
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	err = os.WriteFile(keyFile, keyPEM, 0o600)
	assert.NoError(t, err, "写私钥文件失败")
	if err != nil {
		return "", ""
	}
	return certFile, keyFile
}

// TestStartTLS 验证 HTTPS 服务能正常启动并响应请求。
// 用 127.0.0.1:0 随机端口避免端口冲突；服务启动是异步的，
// 所以客户端要重试几次等它就绪，测完立刻 Shutdown 释放资源。
func TestStartTLS(t *testing.T) {
	certFile, keyFile := writeSelfSignedCert(t)

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	assert.NoError(t, err, "监听端口失败")
	if err != nil {
		return
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("hello tls"))
	})

	svr := New(mux, Listener(lis))
	go svr.StartTLS(certFile, keyFile)
	t.Cleanup(func() {
		_ = svr.Shutdown()
	})

	// 自签名证书过不了系统 CA 校验，测试客户端跳过验证即可。
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // 测试环境专用
		},
		Timeout: 2 * time.Second,
	}

	url := "https://" + lis.Addr().String() + "/"
	deadline := time.Now().Add(5 * time.Second)
	var resp *http.Response
	for {
		resp, err = client.Get(url)
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			assert.NoError(t, err, "等待 HTTPS 服务就绪超时")
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	assert.NoError(t, err, "读取响应体失败")
	assert.Equal(t, "hello tls", string(body))
}
