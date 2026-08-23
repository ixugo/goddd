package server

import (
	"context"
	"net"
	"net/http"
	"sync"
	"time"
)

const (
	defaultReadTimeout     = 10 * time.Second
	defaultWriteTimeout    = 10 * time.Second
	defaultIdleTimeout     = 120 * time.Second
	defaultAddr            = ":8080"
	defaultShutdownTimeout = 5 * time.Second
)

// Server HTTP 服务
type Server struct {
	server          *http.Server
	notify          chan error
	shutdownTimeout time.Duration
	once            sync.Once
	lis             net.Listener
}

// New 初始化并启动路由
func New(handler http.Handler, opts ...Option) *Server {
	// protocols 同时启用 HTTP/1.1 与明文 HTTP/2（h2c）
	protocols := new(http.Protocols)
	protocols.SetHTTP1(true)
	protocols.SetUnencryptedHTTP2(true)

	httpSer := http.Server{
		Addr:         defaultAddr,
		Handler:      handler,
		Protocols:    protocols,
		ReadTimeout:  defaultReadTimeout,
		WriteTimeout: defaultWriteTimeout,
		IdleTimeout:  defaultIdleTimeout,
	}

	s := &Server{
		server:          &httpSer,
		notify:          make(chan error, 1),
		shutdownTimeout: defaultShutdownTimeout,
	}

	_ = Raise(65535)
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Start 启动 HTTP 服务
func (s *Server) Start() {
	s.once.Do(func() {
		if s.lis != nil {
			s.notify <- s.server.Serve(s.lis)
		} else {
			s.notify <- s.server.ListenAndServe()
		}
		close(s.notify)
	})
}

// StartTLS 启动 HTTPS 服务
func (s *Server) StartTLS(certFile, keyFile string) {
	s.once.Do(func() {
		if s.lis != nil {
			s.notify <- s.server.ServeTLS(s.lis, certFile, keyFile)
		} else {
			s.notify <- s.server.ListenAndServeTLS(certFile, keyFile)
		}
		close(s.notify)
	})
}

// Notify .
func (s *Server) Notify() <-chan error {
	return s.notify
}

// Shutdown 关闭服务
func (s *Server) Shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), s.shutdownTimeout)
	defer cancel()
	return s.server.Shutdown(ctx)
}
