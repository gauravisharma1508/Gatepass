// Package httpx has the JSON/error helpers and the shared middleware.
package httpx

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"gatepass/internal/platform/apperr"
	"gatepass/internal/platform/authctx"
)

// HandlerFunc is a handler that returns its error; H writes it as the standard error body.
type HandlerFunc func(w http.ResponseWriter, r *http.Request) error

func H(fn HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := fn(w, r); err != nil {
			WriteError(w, r, err)
		}
	}
}

func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if v != nil {
		_ = json.NewEncoder(w).Encode(v)
	}
}

func NoContent(w http.ResponseWriter) { w.WriteHeader(http.StatusNoContent) }

type errorBody struct {
	Error struct {
		Code    string         `json:"code"`
		Message string         `json:"message"`
		Details map[string]any `json:"details"`
	} `json:"error"`
}

func WriteError(w http.ResponseWriter, r *http.Request, err error) {
	var ae *apperr.Error
	var tooBig *http.MaxBytesError
	switch {
	case errors.As(err, &ae):
	case errors.As(err, &tooBig):
		ae = apperr.ErrFileTooLarge
	default:
		slog.Error("unexpected error", "err", err, "path", r.URL.Path, "request_id", RequestIDFrom(r.Context()))
		ae = apperr.New(http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong")
	}
	var b errorBody
	b.Error.Code, b.Error.Message = ae.Code, ae.Message
	b.Error.Details = ae.Details
	if b.Error.Details == nil {
		b.Error.Details = map[string]any{}
	}
	WriteJSON(w, ae.Status, b)
}

const maxJSONBody = 100 << 10

// DecodeJSON reads a size-limited JSON body; unknown fields are ignored.
func DecodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBody)
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			return apperr.ErrFileTooLarge
		}
		if errors.Is(err, io.EOF) {
			return apperr.Validation(map[string]string{"body": "A JSON body is required"})
		}
		return apperr.Validation(map[string]string{"body": "Invalid JSON"})
	}
	return nil
}

// DecodeOptionalJSON is like DecodeJSON but accepts an empty body.
func DecodeOptionalJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	if r.ContentLength == 0 {
		return nil
	}
	err := DecodeJSON(w, r, dst)
	if e, ok := err.(*apperr.Error); ok && e.Code == "VALIDATION_ERROR" && e.Details["fields"] != nil {
		if f, _ := e.Details["fields"].(map[string]any); f["body"] == "A JSON body is required" {
			return nil
		}
	}
	return err
}

func URLUUID(r *http.Request, name string) (uuid.UUID, error) {
	id, err := uuid.Parse(chi.URLParam(r, name))
	if err != nil {
		return uuid.Nil, apperr.ErrNotFound
	}
	return id, nil
}

// Page parses ?page=&pageSize= (defaults 1 and 20, max 100).
func Page(r *http.Request) (page, size int, err error) {
	page, size = 1, 20
	if v := r.URL.Query().Get("page"); v != "" {
		if page, err = strconv.Atoi(v); err != nil || page < 1 {
			return 0, 0, apperr.Validation(map[string]string{"page": "Must be 1 or more"})
		}
	}
	if v := r.URL.Query().Get("pageSize"); v != "" {
		if size, err = strconv.Atoi(v); err != nil || size < 1 || size > 100 {
			return 0, 0, apperr.Validation(map[string]string{"pageSize": "Must be between 1 and 100"})
		}
	}
	return page, size, nil
}

type PageOf[T any] struct {
	Items    []T `json:"items"`
	Page     int `json:"page"`
	PageSize int `json:"pageSize"`
	Total    int `json:"total"`
}

// ---- middleware ----

type ctxKey struct{}

func RequestIDFrom(ctx context.Context) string {
	s, _ := ctx.Value(ctxKey{}).(string)
	return s
}

func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b := make([]byte, 8)
		_, _ = rand.Read(b)
		id := hex.EncodeToString(b)
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, id)))
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (s *statusWriter) WriteHeader(c int) { s.status = c; s.ResponseWriter.WriteHeader(c) }

// Unwrap keeps http.ResponseController (flush, deadlines) working for the SSE stream.
func (s *statusWriter) Unwrap() http.ResponseWriter { return s.ResponseWriter }

func AccessLog(lg *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			sw := &statusWriter{ResponseWriter: w, status: 200}
			next.ServeHTTP(sw, r)
			lg.Info("request", "method", r.Method, "path", r.URL.Path, "status", sw.status,
				"ms", time.Since(start).Milliseconds(), "request_id", RequestIDFrom(r.Context()))
		})
	}
}

func Recover(lg *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					lg.Error("panic", "panic", rec, "stack", string(debug.Stack()), "request_id", RequestIDFrom(r.Context()))
					WriteError(w, r, apperr.New(http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong"))
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

func SecurityHeaders(production bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := w.Header()
			h.Set("Content-Security-Policy", "default-src 'self'; img-src 'self' blob: data:; style-src 'self' 'unsafe-inline'; script-src 'self'; connect-src 'self'; frame-ancestors 'none'; base-uri 'self'; form-action 'self'")
			h.Set("X-Content-Type-Options", "nosniff")
			h.Set("Referrer-Policy", "same-origin")
			h.Set("X-Frame-Options", "DENY")
			h.Set("Permissions-Policy", "camera=(self), microphone=(), geolocation=()")
			if production {
				h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
			}
			next.ServeHTTP(w, r)
		})
	}
}

// CSRF: every state-changing request must carry X-Requested-With: gatepass (spec §7).
// skip reports requests exempt from the check (bearer-token clients).
func CSRF(skip func(*http.Request) bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler { return csrf(skip, next) }
}

func csrf(skip func(*http.Request) bool, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if skip != nil && skip(r) {
			next.ServeHTTP(w, r)
			return
		}
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
		default:
			if r.Header.Get("X-Requested-With") != "gatepass" {
				WriteError(w, r, apperr.ErrForbidden.With("Missing X-Requested-With header"))
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// NoStore marks API responses as not cacheable.
func NoStore(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

// ---- auth guards (the principal is put in the context by the auth module) ----

func RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := authctx.From(r.Context()); !ok {
			WriteError(w, r, apperr.ErrUnauthenticated)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func RequireRoles(roles ...authctx.Role) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p, ok := authctx.From(r.Context())
			if !ok {
				WriteError(w, r, apperr.ErrUnauthenticated)
				return
			}
			for _, role := range roles {
				if p.Role == role {
					next.ServeHTTP(w, r)
					return
				}
			}
			WriteError(w, r, apperr.ErrForbidden)
		})
	}
}

// PasswordGate blocks everything except changing the password until it is changed (spec §7).
func PasswordGate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if p, ok := authctx.From(r.Context()); ok && p.MustChangePassword {
			switch r.URL.Path {
			case "/api/auth/change-password", "/api/auth/me", "/api/auth/logout":
			default:
				WriteError(w, r, apperr.ErrPasswordChangeRequired)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
