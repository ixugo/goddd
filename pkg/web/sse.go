package web

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// SSE 发送事件
/*
	使用案例

	http.HandleFunc("/stream", func(w http.ResponseWriter, r *http.Request) {
		sse := web.NewSSE(1024, time.Minute)

		go func(){
			defer sse.Close()
			for range 3 {
				sse.Publish(web.Event{
					ID:    uuid.New().String(),
					Event: "ping",
					Data: []byte("pong"),
				})
				time.Sleep(time.Second)
			}
		}()
		sse.ServeHTTP(w, r)
	})


*/
type SSE struct {
	Headers map[string]string
	stream  chan Event
	timeout time.Duration
	cancel  context.CancelFunc

	m      sync.Mutex
	closed bool
}

type Event struct {
	ID    string
	Event string
	Data  []byte
}

func NewSSE(length int, timeout time.Duration) *SSE {
	if length <= 0 {
		length = 1024
	}
	return &SSE{
		stream:  make(chan Event, length),
		timeout: timeout,
		closed:  false,
	}
}

func (s *SSE) Publish(v Event) {
	s.m.Lock()
	defer s.m.Unlock()
	if s.closed {
		return
	}
	s.stream <- v
}

// Stop 会立即停止发送事件，stop 后应该调用 Close()
func (s *SSE) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
}

// Close 会确保所有事件被发送完毕
func (s *SSE) Close() {
	s.m.Lock()
	defer s.m.Unlock()
	if s.closed {
		return
	}
	s.closed = true
	close(s.stream)
}

func (s *SSE) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	rc := http.NewResponseController(w) // nolint
	_ = rc.SetWriteDeadline(time.Now().Add(s.timeout))
	_ = rc.SetReadDeadline(time.Now().Add(s.timeout))

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	for k, v := range s.Headers {
		w.Header().Set(k, v)
	}

	ctx, cancel := context.WithCancel(req.Context())
	s.cancel = cancel

	for {
		select {
		case <-ctx.Done():
			return
		case ev := <-s.stream:
			if ev.ID == "" && ev.Event == "" && len(ev.Data) == 0 {
				return
			}
			if len(ev.ID) > 0 {
				_, _ = fmt.Fprintf(w, "id: %s\n", ev.ID)
			}
			if len(ev.Event) > 0 {
				_, _ = fmt.Fprintf(w, "event: %s\n", ev.Event)
			}
			if len(ev.Data) > 0 {
				_, _ = fmt.Fprintf(w, "data: %s\n", ev.Data)
			}
			_, _ = fmt.Fprint(w, "\n")
			if err := rc.Flush(); err != nil {
				slog.ErrorContext(req.Context(), "flush", "err", err)
				return
			}
		}
	}
}

type EventMessage struct {
	id    string
	event string
	data  string
}

func NewEventMessage(event string, data map[string]any) *EventMessage {
	b, _ := json.Marshal(data)
	return &EventMessage{
		event: event,
		data:  string(b),
	}
}

func SendSSE(ch <-chan EventMessage, c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.Header("Content-Type", "text/event-stream")
	tick := time.NewTicker(40 * time.Millisecond)
	defer tick.Stop()
	var last *EventMessage
	var zero EventMessage
	for {
		select {
		case <-tick.C:
			if last != nil {
				_, _ = io.WriteString(c.Writer, fmt.Sprintf("%v\n", *last))
				c.Writer.Flush()
				last = nil
			}
		case v := <-ch:
			if v != zero {
				last = &v
				continue
			}
			if last != nil {
				_, _ = io.WriteString(c.Writer, fmt.Sprintf("%v\n", *last))
				c.Writer.Flush()
			}
			return
		}
	}
}

type Chunk struct {
	Total   int    `json:"total"`
	Current int    `json:"current"`
	Success int    `json:"success"`
	Failure int    `json:"failure"`
	Err     string `json:"err,omitempty"`
}

// SendChunkPro 高性能版
func SendChunkPro(ch <-chan Chunk, c *gin.Context) {
	if c == nil || c.Writer == nil {
		return
	}
	tick := time.NewTicker(40 * time.Millisecond)
	defer tick.Stop()
	var last *Chunk
	var zero Chunk
	var i int
	for {
		if i == 1 {
			c.Header("Cache-Control", "no-store")
			c.Header("Transfer-Encoding", "chunked")
			c.Header("Content-Type", "text/plain")
		}
		select {
		case <-tick.C:
			if last != nil {
				b, _ := json.Marshal(last)
				_, err := c.Writer.Write(append(b, '\n'))
				if err != nil {
					return
				}
				c.Writer.Flush()
				last = nil
			}
		case v := <-ch:
			i++
			if v != zero {
				last = &v
				continue
			}
			if last != nil {
				b, _ := json.Marshal(last)
				_, err := c.Writer.Write(append(b, '\n'))
				if err != nil {
					return
				}
				c.Writer.Flush()
			}
			return
		}
	}
}

// SendChunk 发送分块数据
func SendChunk(ch <-chan Chunk, c *gin.Context) {
	if c == nil || c.Writer == nil {
		return
	}
	c.Header("Cache-Control", "no-store")
	c.Header("Transfer-Encoding", "chunked")
	c.Header("Content-Type", "text/plain")
	var zero Chunk
	var i int
	for {
		i++
		v := <-ch
		if v == zero {
			return
		}
		b, _ := json.Marshal(v)
		_, err := c.Writer.Write(append(b, '\n'))
		if err != nil {
			return
		}
		c.Writer.Flush()
	}
}
