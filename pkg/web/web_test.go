package web

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/ixugo/goddd/pkg/logger"
)

func TestLogger(t *testing.T) {
	_, _ = logger.SetupSlog(logger.Config{
		Debug: true,
	})

	gin.SetMode(gin.TestMode)
	g := gin.New()
	g.Use(Logger(slog.Default(), nil))

	g.GET("/a/:id", func(c *gin.Context) {
		slog.InfoContext(c.Request.Context(), "request", "path", c.FullPath())
		c.JSON(http.StatusOK, gin.H{"message": "Hello, World!"})
	})

	req := httptest.NewRequest(http.MethodGet, "/a/123", nil)
	rec := httptest.NewRecorder()
	g.ServeHTTP(rec, req)
}
