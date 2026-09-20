package httpapi

import (
	"context"
	"example.com/replica-control/internal/model"
	"example.com/replica-control/internal/service"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type fake struct {
	err      error
	calls    int
	replicas int32
}

func (f *fake) Get(_ context.Context, t model.Target) (model.Deployment, error) {
	f.calls++
	return model.Deployment{Target: t, Replicas: 2}, f.err
}
func (f *fake) List(context.Context, string) ([]model.Deployment, error) {
	f.calls++
	return nil, f.err
}
func (f *fake) Set(_ context.Context, t model.Target, n int32, v string) (model.SetResult, error) {
	f.calls++
	f.replicas = n
	return model.SetResult{Target: t, Replicas: n}, f.err
}
// TestHTTP exercises routing and decoding without a network listener.
// Transport certificate checks are covered separately by the security package.
func TestHTTP(t *testing.T) {
	const path = "/v1/namespaces/default/deployments/demo/replicas"
	for _, tc := range []struct {
		name                            string
		level                           int
		method, path, body, contentType string
		code                            int
		err                             error
	}{
		{"get", 1, "GET", path, "", "", 200, nil},
		{"not found", 1, "GET", path, "", "", 404, model.E(model.NotFound, "missing", nil)},
		{"l1 write blocked", 1, "PUT", path, `{"replicas":2}`, "application/json", 405, nil},
		{"set", 2, "PUT", path, `{"replicas":2}`, "application/json", 200, nil},
		{"zero", 2, "PUT", path, `{"replicas":0}`, "application/json", 200, nil},
		{"missing", 2, "PUT", path, `{}`, "application/json", 400, nil},
		{"null", 2, "PUT", path, `{"replicas":null}`, "application/json", 400, nil},
		{"negative", 2, "PUT", path, `{"replicas":-1}`, "application/json", 400, nil},
		{"too large count", 2, "PUT", path, `{"replicas":1001}`, "application/json", 400, nil},
		{"unknown field", 2, "PUT", path, `{"replicas":2,"secret":"x"}`, "application/json", 400, nil},
		{"two objects", 2, "PUT", path, `{"replicas":2}{"replicas":4}`, "application/json", 400, nil},
		{"fraction", 2, "PUT", path, `{"replicas":2.5}`, "application/json", 400, nil},
		{"bad content type", 2, "PUT", path, `{"replicas":2}`, "text/plain", 415, nil},
		{"oversized", 2, "PUT", path, `{"replicas":2,"expectedVersion":"` + strings.Repeat("a", 5000) + `"}`, "application/json", 413, nil},
		{"conflict", 2, "PUT", path, `{"replicas":2}`, "application/json", 409, model.E(model.Conflict, "stale", nil)},
		{"HPA", 2, "PUT", path, `{"replicas":2}`, "application/json", 412, model.E(model.FailedPrecondition, "HPA conflict", nil)},
		{"forbidden", 2, "GET", path, "", "", 403, model.E(model.Forbidden, "denied", nil)},
		{"dependency down", 4, "GET", path, "", "", 503, model.E(model.Unavailable, "unavailable", nil)},
		{"list l2", 2, "GET", "/v1/deployments", "", "", 404, nil},
		{"list l3", 3, "GET", "/v1/deployments", "", "", 200, nil},
		{"invalid namespace", 4, "GET", "/v1/namespaces/UPPER/deployments/demo/replicas", "", "", 400, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := &fake{err: tc.err}
			h := New(&service.Service{Backend: b}, tc.level, slog.New(slog.NewTextHandler(io.Discard, nil)))
			r := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			if tc.contentType != "" {
				r.Header.Set("Content-Type", tc.contentType)
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != tc.code {
				t.Fatalf("got %d, want %d: %s", w.Code, tc.code, w.Body.String())
			}
			if tc.name == "zero" && b.replicas != 0 {
				t.Fatal("zero not honored")
			}
			if tc.name == "list l3" && !strings.Contains(w.Body.String(), `"deployments":[]`) {
				t.Fatal("empty list must be []")
			}
		})
	}
}
func TestHTTPMethod(t *testing.T) {
	h := New(&service.Service{Backend: &fake{}}, 4, slog.Default())
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/v1/namespaces/default/deployments/demo/replicas", nil))
	if w.Code != 405 {
		t.Fatal(w.Code)
	}
}
