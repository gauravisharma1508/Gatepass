// Package testutil builds a full Gatepass instance for integration tests: real PostgreSQL
// (connected as the application role, so permission rules are real), a controllable clock,
// a recording mailer and in-memory photo storage.
package testutil

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"gatepass/internal/app"
	"gatepass/internal/platform/clock"
	"gatepass/internal/platform/config"
	"gatepass/internal/platform/db"
	"gatepass/internal/platform/mailer"
	"gatepass/internal/platform/storage"
)

const (
	defaultAppURL   = "postgres://gatepass_app:gatepass_app@localhost:5433/gatepass_test?sslmode=disable"
	defaultOwnerURL = "postgres://gatepass_owner:gatepass_owner@localhost:5433/gatepass_test?sslmode=disable"
	Password        = "Gatepass@123"
)

// Start is the default "now" of every test: 2026-11-03 10:00 IST.
var Start = time.Date(2026, 11, 3, 4, 30, 0, 0, time.UTC)

func AppURL() string   { return getenv("TEST_DATABASE_URL", defaultAppURL) }
func OwnerURL() string { return getenv("TEST_MIGRATE_DATABASE_URL", defaultOwnerURL) }

func getenv(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

var (
	migrateOnce sync.Once
	migrateErr  error
)

// Env is one test's world.
type Env struct {
	T      *testing.T
	App    *app.App
	Server *httptest.Server
	Clock  *clock.Settable
	Mail   *mailer.Fake
	Store  *storage.Memory
	Owner  *sql.DB // owner connection: for assertions and table reset
}

// Option customises the app under test.
type Option func(*app.Options, *config.Config)

func WithLoginRateLimit(n int) Option {
	return func(o *app.Options, _ *config.Config) { o.LoginRateLimit = n }
}

func WithAppEnv(env string) Option { return func(_ *app.Options, c *config.Config) { c.AppEnv = env } }

func WithStatic(fsys fs.FS) Option { return func(o *app.Options, _ *config.Config) { o.Static = fsys } }

func New(t *testing.T, opts ...Option) *Env {
	t.Helper()
	migrateOnce.Do(func() { migrateErr = db.Migrate(t.Context(), OwnerURL(), "up", io.Discard) })
	require.NoError(t, migrateErr, "cannot migrate the test database (run `make db-up` first)")

	owner, err := sql.Open("pgx", OwnerURL())
	require.NoError(t, err)
	t.Cleanup(func() { owner.Close() })
	_, err = owner.Exec(`TRUNCATE users, visitors, visits, visit_photos, notifications, audit_logs, password_reset_tokens,
		user_sessions, sessions, visit_code_counters RESTART IDENTITY CASCADE;
		UPDATE settings SET approval_window_min = 30, approval_reminder_min = 10, max_advance_days = 30,
		photo_retention_days = 90, record_retention_days = 365`)
	require.NoError(t, err)

	clk := clock.New()
	clk.Set(Start)
	mail := &mailer.Fake{}
	store := storage.NewMemory()
	o := app.Options{Log: slog.New(slog.NewTextHandler(io.Discard, nil)), Clock: clk, Store: store, Mailer: mail,
		LoginRateLimit: 100000, SyncEmail: true, SessionCleanup: -1}
	cfg := config.Config{AppEnv: "test", DatabaseURL: AppURL(), BcryptCost: 4, WebOrigin: "http://localhost:5173"}
	for _, f := range opts {
		f(&o, &cfg)
	}
	a, err := app.New(cfg, o)
	require.NoError(t, err)
	srv := httptest.NewServer(a.Handler)
	t.Cleanup(func() { a.Hub.CloseAll(); srv.Close(); a.Close() })
	require.NoError(t, a.Seed(t.Context(), Password, io.Discard))
	return &Env{T: t, App: a, Server: srv, Clock: clk, Mail: mail, Store: store, Owner: owner}
}

// Email of a seed user key: admin, empA, empB, guard, guard2, exEmp.
func Email(key string) string { return strings.ToLower(key) + "@gatepass.local" }

// UserID looks up a seed user's id.
func (e *Env) UserID(key string) string {
	var id string
	require.NoError(e.T, e.Owner.QueryRow("SELECT id FROM users WHERE email = $1", Email(key)).Scan(&id))
	return id
}

// ---- HTTP client with a cookie jar ----

type Client struct {
	T    *testing.T
	Base string
	HC   *http.Client
}

func (e *Env) Anon() *Client {
	jar, _ := cookiejar.New(nil)
	return &Client{T: e.T, Base: e.Server.URL, HC: &http.Client{Jar: jar, Timeout: 20 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}

// Login signs in a seed user and fails the test if that does not work.
func (e *Env) Login(key string) *Client {
	c := e.Anon()
	r := c.Post("/api/auth/login", map[string]any{"email": Email(key), "password": Password})
	require.Equal(e.T, 200, r.Status, "login %s: %s", key, r.Raw)
	return c
}

type Resp struct {
	T      *testing.T
	Status int
	Header http.Header
	Raw    []byte
}

func (c *Client) req(method, path string, body io.Reader, ctype string, headers map[string]string) *Resp {
	c.T.Helper()
	r, err := http.NewRequest(method, c.Base+path, body)
	require.NoError(c.T, err)
	if ctype != "" {
		r.Header.Set("Content-Type", ctype)
	}
	r.Header.Set("X-Requested-With", "gatepass")
	for k, v := range headers {
		if v == "" {
			r.Header.Del(k)
		} else {
			r.Header.Set(k, v)
		}
	}
	resp, err := c.HC.Do(r)
	require.NoError(c.T, err)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	return &Resp{T: c.T, Status: resp.StatusCode, Header: resp.Header, Raw: raw}
}

func (c *Client) Do(method, path string, body any) *Resp {
	c.T.Helper()
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		require.NoError(c.T, err)
		rd = bytes.NewReader(b)
	}
	return c.req(method, path, rd, "application/json", nil)
}

func (c *Client) Get(path string) *Resp          { c.T.Helper(); return c.Do("GET", path, nil) }
func (c *Client) Post(path string, b any) *Resp  { c.T.Helper(); return c.Do("POST", path, b) }
func (c *Client) Patch(path string, b any) *Resp { c.T.Helper(); return c.Do("PATCH", path, b) }

// PostNoCSRF sends a POST without the X-Requested-With header.
func (c *Client) PostNoCSRF(path string, b any) *Resp {
	c.T.Helper()
	raw, _ := json.Marshal(b)
	return c.req("POST", path, bytes.NewReader(raw), "application/json", map[string]string{"X-Requested-With": ""})
}

// Upload sends a multipart PUT with a "file" part and extra form fields.
func (c *Client) Upload(path string, file []byte, fields map[string]string) *Resp {
	c.T.Helper()
	return c.Multipart("PUT", path, file, fields)
}

// Raw is a non-fatal request for use inside goroutines (returns the status or the error).
func (c *Client) Raw(method, path string, body any) (int, []byte, error) {
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	r, err := http.NewRequest(method, c.Base+path, rd)
	if err != nil {
		return 0, nil, err
	}
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-Requested-With", "gatepass")
	resp, err := c.HC.Do(r)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, raw, nil
}

// Multipart sends a multipart request with a "file" part and extra form fields.
func (c *Client) Multipart(method, path string, file []byte, fields map[string]string) *Resp {
	c.T.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for k, v := range fields {
		require.NoError(c.T, mw.WriteField(k, v))
	}
	fw, err := mw.CreateFormFile("file", "photo.jpg")
	require.NoError(c.T, err)
	_, _ = fw.Write(file)
	require.NoError(c.T, mw.Close())
	return c.req(method, path, &buf, mw.FormDataContentType(), nil)
}

func (r *Resp) Expect(status int) *Resp {
	r.T.Helper()
	require.Equal(r.T, status, r.Status, "unexpected status; body: %s", r.Raw)
	return r
}

func (r *Resp) Map() map[string]any {
	r.T.Helper()
	var m map[string]any
	require.NoError(r.T, json.Unmarshal(r.Raw, &m), "body: %s", r.Raw)
	return m
}

// Path reads a value like "visitor.name" or "items.0.id" from the JSON body.
func (r *Resp) Path(p string) any {
	r.T.Helper()
	var cur any = r.Map()
	for _, part := range strings.Split(p, ".") {
		switch v := cur.(type) {
		case map[string]any:
			cur = v[part]
		case []any:
			var i int
			_, err := fmt.Sscanf(part, "%d", &i)
			require.NoError(r.T, err)
			require.Less(r.T, i, len(v), "path %s", p)
			cur = v[i]
		default:
			return nil
		}
	}
	return cur
}

func (r *Resp) Str(p string) string {
	r.T.Helper()
	s, _ := r.Path(p).(string)
	return s
}

func (r *Resp) Code() string { r.T.Helper(); return r.Str("error.code") }

func (r *Resp) Strings(p string) []string {
	r.T.Helper()
	arr, _ := r.Path(p).([]any)
	out := make([]string, 0, len(arr))
	for _, v := range arr {
		out = append(out, fmt.Sprint(v))
	}
	return out
}

func (r *Resp) Len(p string) int {
	r.T.Helper()
	arr, _ := r.Path(p).([]any)
	return len(arr)
}
