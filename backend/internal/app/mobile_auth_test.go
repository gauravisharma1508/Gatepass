//go:build integration

package app_test

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gatepass/internal/testutil"
)

// bare client: no cookie jar, so only the bearer token can authenticate.
func mobileDo(t *testing.T, base, method, path, token, body string, extra map[string]string) (int, http.Header, []byte) {
	t.Helper()
	req, err := http.NewRequest(method, base+path, strings.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	for k, v := range extra {
		req.Header.Set(k, v)
	}
	resp, err := (&http.Client{Timeout: 20 * time.Second}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, resp.Header, raw
}

func mobileLogin(t *testing.T, e *testutil.Env, key string) string {
	t.Helper()
	st, h, raw := mobileDo(t, e.Server.URL, "POST", "/api/auth/login", "",
		`{"email":"`+testutil.Email(key)+`","password":"`+testutil.Password+`"}`, map[string]string{"X-Client": "mobile", "X-Requested-With": "gatepass"})
	require.Equal(t, 200, st, string(raw))
	assert.Empty(t, h.Values("Set-Cookie"), "mobile login sets no cookie")
	var out struct {
		User  map[string]any `json:"user"`
		Token string         `json:"token"`
	}
	require.NoError(t, json.Unmarshal(raw, &out))
	require.NotEmpty(t, out.Token)
	assert.Equal(t, "EMPLOYEE", out.User["role"])
	return out.Token
}

func TestMobileAuth(t *testing.T) {
	e := testutil.New(t)

	t.Run("web login still sets the cookie and returns no token", func(t *testing.T) {
		r := e.Anon().Post("/api/auth/login", map[string]any{"email": testutil.Email("empA"), "password": testutil.Password}).Expect(200)
		assert.NotEmpty(t, r.Header.Values("Set-Cookie"))
		assert.Nil(t, r.Path("token"))
	})

	t.Run("bearer login, access without CSRF header, no cookies, then logout revokes", func(t *testing.T) {
		token := mobileLogin(t, e, "empA")
		st, h, raw := mobileDo(t, e.Server.URL, "GET", "/api/auth/me", token, "", nil)
		assert.Equal(t, 200, st, string(raw))
		assert.Empty(t, h.Values("Set-Cookie"))
		assert.Contains(t, string(raw), testutil.Email("empA"))

		// state-changing call with only the bearer token (no X-Requested-With)
		st, _, raw = mobileDo(t, e.Server.URL, "POST", "/api/auth/change-password", token,
			`{"currentPassword":"wrong","newPassword":"Another@12345"}`, nil)
		assert.Equal(t, 400, st, "reaches the handler (validation), not the CSRF check: %s", raw)

		st, _, raw = mobileDo(t, e.Server.URL, "POST", "/api/auth/logout", token, "", nil)
		assert.Equal(t, 204, st, string(raw))
		st, _, _ = mobileDo(t, e.Server.URL, "GET", "/api/auth/me", token, "", nil)
		assert.Equal(t, 401, st, "token is dead after logout")
	})

	t.Run("bad or missing token is 401", func(t *testing.T) {
		st, _, _ := mobileDo(t, e.Server.URL, "GET", "/api/auth/me", "not-a-token", "", nil)
		assert.Equal(t, 401, st)
		st, _, _ = mobileDo(t, e.Server.URL, "GET", "/api/auth/me", "", "", nil)
		assert.Equal(t, 401, st)
	})

	t.Run("cookie sessions still need the CSRF header", func(t *testing.T) {
		c := e.Login("empA")
		assert.Equal(t, 403, c.PostNoCSRF("/api/auth/logout", map[string]any{}).Status)
	})

	t.Run("security users are blocked on mobile and get no session", func(t *testing.T) {
		st, h, raw := mobileDo(t, e.Server.URL, "POST", "/api/auth/login", "",
			`{"email":"`+testutil.Email("guard")+`","password":"`+testutil.Password+`"}`, map[string]string{"X-Client": "mobile", "X-Requested-With": "gatepass"})
		assert.Equal(t, 403, st)
		assert.Contains(t, string(raw), "MOBILE_NOT_ALLOWED")
		assert.NotContains(t, string(raw), "token")
		assert.Empty(t, h.Values("Set-Cookie"))
		// the same user still signs in on the web
		e.Login("guard")
	})

	t.Run("a second mobile login works independently", func(t *testing.T) {
		token := mobileLogin(t, e, "empB")
		st, _, _ := mobileDo(t, e.Server.URL, "GET", "/api/auth/me", token, "", nil)
		assert.Equal(t, 200, st)
	})
}

func TestMobileSSE(t *testing.T) {
	e := testutil.New(t)
	token := mobileLogin(t, e, "empA")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", e.Server.URL+"/api/events", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"))
	line, err := bufio.NewReader(resp.Body).ReadString('\n')
	require.NoError(t, err)
	assert.Contains(t, line, ": connected")

	// no token: refused
	st, _, _ := mobileDo(t, e.Server.URL, "GET", "/api/events", "", "", nil)
	assert.Equal(t, 401, st)
}
