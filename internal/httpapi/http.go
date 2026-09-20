// Package httpapi implements the level 1–4 REST API. TLS is configured by main.
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"time"

	"example.com/replica-control/internal/model"
	"example.com/replica-control/internal/service"
)

const maxBodyBytes int64 = 4096

func New(s *service.Service, level int, log *slog.Logger) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/namespaces/{namespace}/deployments/{name}/replicas", func(w http.ResponseWriter, r *http.Request) {
		out, err := s.Get(r.Context(), target(r))
		respond(w, out, err, http.StatusOK)
	})
	if level >= 2 {
		mux.HandleFunc("PUT /v1/namespaces/{namespace}/deployments/{name}/replicas", func(w http.ResponseWriter, r *http.Request) {
			contentType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
			if err != nil || contentType != "application/json" {
				write(w, http.StatusUnsupportedMediaType, map[string]string{"code": "invalid_argument", "error": "Content-Type must be application/json"})
				return
			}
			r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
			defer r.Body.Close()
			dec := json.NewDecoder(r.Body)
			dec.DisallowUnknownFields()
			var req model.SetRequest
			if err := dec.Decode(&req); err != nil {
				badJSON(w, err)
				return
			}
			if err := dec.Decode(&struct{}{}); err != io.EOF {
				badJSON(w, err)
				return
			}
			out, err := s.Set(r.Context(), target(r), req)
			respond(w, out, err, http.StatusOK)
		})
	}
	if level >= 3 {
		mux.HandleFunc("GET /v1/deployments", func(w http.ResponseWriter, r *http.Request) {
			out, err := s.List(r.Context(), r.URL.Query().Get("namespace"))
			if out == nil && err == nil {
				out = []model.Deployment{}
			}
			respond(w, map[string]any{"deployments": out}, err, http.StatusOK)
		})
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "no-store")
		defer func() {
			if p := recover(); p != nil {
				log.Error("HTTP handler panic", "panic", p)
				write(w, 500, map[string]string{"code": "internal", "error": "internal server error"})
			}
		}()
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		mux.ServeHTTP(w, r.WithContext(ctx))
	})
}
func target(r *http.Request) model.Target {
	return model.Target{Namespace: r.PathValue("namespace"), Name: r.PathValue("name")}
}
func badJSON(w http.ResponseWriter, err error) {
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		write(w, 413, map[string]string{"code": "invalid_argument", "error": "request body exceeds 4096 bytes"})
		return
	}
	write(w, 400, map[string]string{"code": "invalid_argument", "error": "expected one JSON object with replicas and optional expectedVersion"})
}
func respond(w http.ResponseWriter, out any, err error, success int) {
	if err == nil {
		write(w, success, out)
		return
	}
	code := model.ErrorCode(err)
	status := map[model.Code]int{
		model.InvalidArgument: 400, model.NotFound: 404, model.Conflict: 409,
		model.Forbidden: 403, model.Unavailable: 503, model.FailedPrecondition: 412,
		model.Internal: 500,
	}[code]
	if status == 0 {
		status = 500
	}
	write(w, status, map[string]any{"code": code, "error": model.PublicError(err)})
}
func write(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v) // The response cannot be recovered after the peer disconnects.
}
