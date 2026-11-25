package main

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
)

// 可调参数：命中路由的任务 ID
const taskID = "42"

// 可调参数：不存在的路径，用于触发 404
const unknownID = "404"

// trackedBody 记录 Close 是否被调用，用来验证资源释放责任。
type trackedBody struct {
	io.Reader
	closed bool
}

func (b *trackedBody) Close() error {
	b.closed = true
	return nil
}

func health(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func getTask(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "task id=%s", r.PathValue("id"))
}

func createTask(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close() // 谁持有 Body，谁负责关掉它
	_, _ = io.ReadAll(r.Body)
	w.WriteHeader(http.StatusCreated)
}

func newHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", health)
	mux.HandleFunc("GET /tasks/{id}", getTask)
	mux.HandleFunc("POST /tasks", createTask)
	return mux
}

func statusOf(h http.Handler, method, target string) int {
	req := httptest.NewRequest(method, target, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec.Code
}

func main() {
	h := newHandler()
	cases := []struct{ method, target string }{
		{"GET", "/healthz"},
		{"GET", "/tasks/" + taskID},
		{"POST", "/tasks"},
		{"DELETE", "/tasks/" + taskID},
		{"GET", "/unknown/" + unknownID},
	}
	for _, c := range cases {
		fmt.Printf("%-6s %-20s -> %d\n", c.method, c.target, statusOf(h, c.method, c.target))
	}

	// 把 Body 换成可追踪的实现，确认 Handler 真的关了它。
	tb := &trackedBody{Reader: strings.NewReader(`{"title":"Go"}`)}
	req := httptest.NewRequest("POST", "/tasks", tb)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	fmt.Printf("POST /tasks body closed = %t, status = %d\n", tb.closed, rec.Code)
}
