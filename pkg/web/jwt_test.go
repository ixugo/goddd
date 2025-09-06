package web

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestJWT(t *testing.T) {
	const secret = "test_secret_key"

	data := NewClaimsData().SetLevel(1)
	token, err := NewToken(data, secret, WithExpires(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	cli, err := ParseToken(token, secret)
	v := cli.Data[KeyLevel].(float64)
	if v != 1 {
		t.Fatal("level not equal")
	}

	if err := cli.Valid(); err != nil {
		t.Fatal(err)
	}
	time.Sleep(time.Second)
	if err := cli.Valid(); err == nil {
		t.Fatal("valid faild")
	}
}

func TestClaimsData(t *testing.T) {
	data := NewClaimsData()
	data.SetUserID(123)

	if data[KeyUserID] != 123 {
		t.Errorf("SetUserID failed")
	}

	for i := range 100000 {
		data.Set(fmt.Sprintf("key%d", i), i)
	}

	if len(data) != 100001 {
		t.Errorf("Set failed")
	}
}

func TestEtag(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	g := gin.New()
	g.Use(EtagHandler())

	g.GET("/", func(ctx *gin.Context) {
		ctx.String(200, "O1K")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("If-None-Match", `"e0aa021e21dddbd6d8cecec71e9cf564"`)
	g.ServeHTTP(w, req)

	resp := w.Result()
	fmt.Println(resp.StatusCode)
	fmt.Println(resp.Header.Get("ETag"))
	s, _ := io.ReadAll(resp.Body)
	fmt.Println(string(s))
}
