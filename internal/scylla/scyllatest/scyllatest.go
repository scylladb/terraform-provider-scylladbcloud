// Package scyllatest is an in-memory stand-in for the ScyllaDB Cloud public
// API (siren-frontend in public API mode) for unit tests.
package scyllatest

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/eapache/go-resiliency/retrier"

	"github.com/scylladb/terraform-provider-scylladbcloud/internal/scylla"
)

// AccountID is the account every Client made by Server.Client uses.
const AccountID int64 = 1

// Response is what a Handler answers: Data goes under "data", a non-empty
// Error under "error", as the public API does.
type Response struct {
	Status int
	Data   any
	Error  string
}

// Handler answers one request; body is the request body as sent.
type Handler func(r *http.Request, body []byte) Response

// Call is a request the server received.
type Call struct {
	Method string
	Path   string
	Query  string
	Body   []byte
}

// Server routes "METHOD /path" to handlers and records every call. A request
// with no handler fails the test.
type Server struct {
	*httptest.Server

	t      testing.TB
	mu     sync.Mutex
	routes map[string]Handler
	calls  []Call
}

// New starts a server that is closed when the test ends.
func New(t testing.TB) *Server {
	t.Helper()
	s := &Server{t: t, routes: map[string]Handler{}}
	s.Server = httptest.NewServer(http.HandlerFunc(s.serve))
	t.Cleanup(s.Close)
	return s
}

// Handle routes method and path (no query) to h, replacing any earlier one.
func (s *Server) Handle(method, path string, h Handler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.routes[method+" "+path] = h
}

// Reply answers method and path with data and status 200.
func (s *Server) Reply(method, path string, data any) {
	s.Handle(method, path, func(*http.Request, []byte) Response {
		return Response{Status: http.StatusOK, Data: data}
	})
}

// Fail answers method and path with an API error code, such as "040105".
func (s *Server) Fail(method, path string, status int, code string) {
	s.Handle(method, path, func(*http.Request, []byte) Response {
		return Response{Status: status, Error: code}
	})
}

// Sequence answers successive calls with each response in turn, repeating
// the last one.
func (s *Server) Sequence(method, path string, responses ...Response) {
	var (
		mu sync.Mutex
		n  int
	)
	s.Handle(method, path, func(*http.Request, []byte) Response {
		mu.Lock()
		defer mu.Unlock()
		r := responses[min(n, len(responses)-1)]
		n++
		return r
	})
}

// Calls returns the calls received so far.
func (s *Server) Calls() []Call {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Call(nil), s.calls...)
}

// CallsTo returns the calls received for method and path.
func (s *Server) CallsTo(method, path string) []Call {
	var out []Call
	for _, c := range s.Calls() {
		if c.Method == method && c.Path == path {
			out = append(out, c)
		}
	}
	return out
}

// Client returns a client for this server that skips metadata loading, does
// not retry and polls cluster requests every few milliseconds.
func (s *Server) Client(t testing.TB) *scylla.Client {
	t.Helper()
	c, err := scylla.NewClient(s.URL, "test-token", "scyllatest", "", false)
	if err != nil {
		t.Fatalf("scylla.NewClient: %v", err)
	}
	c.AccountID = AccountID
	c.Retry = retrier.New(nil, scylla.DefaultClassifier)
	c.PollInterval = 5 * time.Millisecond
	return c
}

func (s *Server) serve(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)

	s.mu.Lock()
	s.calls = append(s.calls, Call{Method: r.Method, Path: r.URL.Path, Query: r.URL.RawQuery, Body: body})
	h, ok := s.routes[r.Method+" "+r.URL.Path]
	s.mu.Unlock()

	res := Response{Status: http.StatusNotFound, Error: "NOT_FOUND"}
	if ok {
		res = h(r, body)
	} else {
		s.t.Errorf("scyllatest: unexpected request %s %s", r.Method, r.URL.Path)
	}
	if res.Status == 0 {
		res.Status = http.StatusOK
	}

	out := map[string]any{}
	if res.Error != "" {
		out["error"] = res.Error
	} else {
		out["data"] = res.Data
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(res.Status)
	_ = json.NewEncoder(w).Encode(out)
}

// JSON decodes a recorded request body into v, failing the test on error.
func JSON(t testing.TB, body []byte, v any) {
	t.Helper()
	if err := json.Unmarshal(body, v); err != nil {
		t.Fatalf("decoding request body %q: %v", body, err)
	}
}
