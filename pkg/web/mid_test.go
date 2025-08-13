package web

import (
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

func TestLimiter(t *testing.T) {
	r := gin.New()
	r.Use(IPRateLimiterForGin(2, 4))
	r.GET("/", func(ctx *gin.Context) {
		ctx.String(200, "OK")
	})

	for i := 0; i < 10; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		r.ServeHTTP(w, req)
		_, _ = io.Copy(os.Stdout, w.Body)
		if i == 5 {
			time.Sleep(2 * time.Second)
		}
	}
}

func BenchmarkResponse(b *testing.B) {
	r := gin.New()
	r.Use(RecordResponse())
	r.GET("/", func(ctx *gin.Context) {
		ctx.String(200, "OK")
	})
	b.Run("test", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			r.ServeHTTP(w, req)
		}
	})
}

func BenchmarkMap(b *testing.B) {
	b.Run("ip limiter", func(b *testing.B) {
		r := rand.New(rand.NewSource(1))
		ipRateLimiter := IPRateLimiter(10, 10)
		b.ResetTimer()
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				ipRateLimiter(fmt.Sprintf("127.0.0.%v", r.Intn(100)+1))
			}
		})
	})

	b.Run("load or store", func(b *testing.B) {
		r := rand.New(rand.NewSource(1))
		var a sync.Map
		for i := 0; i < b.N; i++ {
			a.LoadOrStore(r.Intn(100), rate.NewLimiter(10, 10))
		}
	})

	b.Run("load", func(b *testing.B) {
		r := rand.New(rand.NewSource(1))
		var a sync.Map
		for i := 0; i < b.N; i++ {
			k := r.Intn(100)
			_, ok := a.Load(k)
			if !ok {
				a.Store(k, rate.NewLimiter(10, 10))
			}
		}
	})
}
