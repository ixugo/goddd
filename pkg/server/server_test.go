package server

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"testing"
	"time"
)

func TestServer(t *testing.T) {
	svr := New(http.NewServeMux(), Port("8081"), DefaultPrintln())
	go svr.Start()

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, syscall.SIGINT, syscall.SIGTERM)

	select {
	case s := <-interrupt:
		fmt.Printf("s(%s) := <-interrupt\n", s.String())
	case err := <-svr.Notify():
		fmt.Printf("err(%s) = <-server.Notify()\n", err)
	case <-time.After(2 * time.Second):
		fmt.Println("timeout")
	}
	if err := svr.Shutdown(); err != nil {
		fmt.Printf("err(%s) := server.Shutdown()\n", err)
	}
}

func TestListener(t *testing.T) {
	lis, err := net.ListenTCP("tcp", &net.TCPAddr{
		IP:   net.ParseIP("127.0.0.1"),
		Port: 0,
	})
	if err != nil {
		t.Fatal(err)
	}
	s := http.NewServeMux()
	s.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Hello, World!"))
	})

	svr := New(s, Port("8081"), Listener(lis))
	go svr.Start()

	resp, err := http.DefaultClient.Get("http://" + lis.Addr().String())
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}
	fmt.Println(resp.StatusCode, string(body))
}
