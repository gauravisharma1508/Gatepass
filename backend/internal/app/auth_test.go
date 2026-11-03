//go:build integration

package app_test

import (
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gatepass/internal/testutil"
)

func TestSystem_HealthAndErrorFormat_TC_SYS_001_003(t *testing.T) {
	e := testutil.New(t)
	c := e.Anon()
	assert.Equal(t, "ok", c.Get("/api/health").Expect(200).Str("status"))

	r := c.Get("/api/does-not-exist").Expect(404)
	assert.Equal(t, "NOT_FOUND", r.Code())
	assert.NotNil(t, r.Path("error.details"))
	assert.NotContains(t, string(r.Raw), "goroutine", "no stack traces in error bodies")
}

func TestLogin_TC_AUTH_001_to_005(t *testing.T) {
	e := testutil.New(t)

	t.Run("valid login sets a hardened cookie, no password hash", func(t *testing.T) {
		c := e.Anon()
		r := c.Post("/api/auth/login", map[string]any{"email": testutil.Email("empA"), "password": testutil.Password}).Expect(200)
		assert.Equal(t, "EMPLOYEE", r.Str("user.role"))
		assert.NotContains(t, string(r.Raw), "password")
		cookie := strings.Join(r.Header.Values("Set-Cookie"), ";")
		assert.Contains(t, cookie, "gp.sid=")
		assert.Contains(t, cookie, "HttpOnly")
		assert.Contains(t, cookie, "SameSite=Lax")
	})
	t.Run("email is case-insensitive", func(t *testing.T) {
		e.Anon().Post("/api/auth/login", map[string]any{"email": "ANITA@x.com", "password": "x"}).Expect(401)
		e.Anon().Post("/api/auth/login", map[string]any{"email": "EMPA@GATEPASS.LOCAL", "password": testutil.Password}).Expect(200)
	})
	t.Run("wrong password, unknown email and inactive user give the same generic answer", func(t *testing.T) {
		a := e.Anon().Post("/api/auth/login", map[string]any{"email": testutil.Email("empA"), "password": "WrongPass1"}).Expect(401)
		b := e.Anon().Post("/api/auth/login", map[string]any{"email": "nobody@gatepass.local", "password": "WrongPass1"}).Expect(401)
		c := e.Anon().Post("/api/auth/login", map[string]any{"email": testutil.Email("exEmp"), "password": testutil.Password}).Expect(401)
		for _, r := range []*testutil.Resp{a, b, c} {
			assert.Equal(t, "Invalid email or password", r.Str("error.message"))
		}
	})
	t.Run("missing fields are field errors", func(t *testing.T) {
		r := e.Anon().Post("/api/auth/login", map[string]any{}).Expect(400)
		assert.Equal(t, "VALIDATION_ERROR", r.Code())
	})
}

func TestLockout_TC_AUTH_006(t *testing.T) {
	e := testutil.New(t)
	c := e.Anon()
	bad := map[string]any{"email": testutil.Email("empA"), "password": "WrongPass1"}
	for i := 0; i < 5; i++ {
		c.Post("/api/auth/login", bad).Expect(401)
	}
	good := map[string]any{"email": testutil.Email("empA"), "password": testutil.Password}
	assert.Equal(t, "ACCOUNT_LOCKED", c.Post("/api/auth/login", good).Expect(423).Code(), "even the right password is refused while locked")

	e.Clock.Advance(14 * time.Minute)
	c.Post("/api/auth/login", good).Expect(423)
	e.Clock.Advance(2 * time.Minute)
	c.Post("/api/auth/login", good).Expect(200)

	// the counter was reset: four more failures do not lock again
	for i := 0; i < 4; i++ {
		e.Anon().Post("/api/auth/login", bad).Expect(401)
	}
	e.Anon().Post("/api/auth/login", good).Expect(200)
}

func TestLogoutAndSession_TC_AUTH_007(t *testing.T) {
	e := testutil.New(t)
	c := e.Login("empA")
	c.Get("/api/auth/me").Expect(200)
	c.Post("/api/auth/logout", nil).Expect(204)
	assert.Equal(t, "UNAUTHENTICATED", c.Get("/api/auth/me").Expect(401).Code())
}

func TestCSRFHeader_TC_AUTH_009(t *testing.T) {
	e := testutil.New(t)
	c := e.Login("empA")
	r := c.PostNoCSRF("/api/visits/planned", testutil.PlannedBody(nil)).Expect(403)
	assert.Equal(t, "FORBIDDEN", r.Code())
	c.Get("/api/visits").Expect(200) // reads are unaffected
}

func TestRoleGuard_TC_AUTH_010(t *testing.T) {
	w := newWorld(t)
	assert.Equal(t, "UNAUTHENTICATED", w.Anon().Get("/api/visits").Expect(401).Code())
	assert.Equal(t, "FORBIDDEN", w.empA.Post("/api/visits/walk-in", testutil.WalkInBody(w.UserID("empB"), nil)).Expect(403).Code())
	w.empA.Get("/api/users").Expect(403)
	w.guard.Get("/api/settings").Expect(403)
}

func TestForcedPasswordChange_TC_AUTH_011_to_013(t *testing.T) {
	w := newWorld(t)
	w.admin.Post("/api/users", map[string]any{"employeeCode": "EMP100", "name": "New Person", "email": "new.person@gatepass.local",
		"role": "EMPLOYEE", "tempPassword": "TempPass1"}).Expect(201)
	c := w.Anon()
	c.Post("/api/auth/login", map[string]any{"email": "new.person@gatepass.local", "password": "TempPass1"}).Expect(200)

	assert.Equal(t, "PASSWORD_CHANGE_REQUIRED", c.Get("/api/visits").Expect(403).Code())
	assert.Equal(t, true, c.Get("/api/auth/me").Expect(200).Path("user.mustChangePassword"))

	r := c.Post("/api/auth/change-password", map[string]any{"currentPassword": "nope", "newPassword": "short1"}).Expect(400)
	assert.NotEmpty(t, r.Path("error.details.fields.currentPassword"))
	assert.NotEmpty(t, r.Path("error.details.fields.newPassword"))
	c.Post("/api/auth/change-password", map[string]any{"currentPassword": "TempPass1", "newPassword": "allletters"}).Expect(400)

	c.Post("/api/auth/change-password", map[string]any{"currentPassword": "TempPass1", "newPassword": "NewPass123"}).Expect(204)
	c.Get("/api/visits").Expect(200)
	w.Anon().Post("/api/auth/login", map[string]any{"email": "new.person@gatepass.local", "password": "TempPass1"}).Expect(401)
	w.Anon().Post("/api/auth/login", map[string]any{"email": "new.person@gatepass.local", "password": "NewPass123"}).Expect(200)
}

var tokenRe = regexp.MustCompile(`token=([0-9a-f]{64})`)

func resetToken(t *testing.T, e *testutil.Env, email string) string {
	t.Helper()
	msgs := e.Mail.To(email)
	require.NotEmpty(t, msgs, "no reset mail for %s", email)
	m := tokenRe.FindStringSubmatch(msgs[len(msgs)-1].Body)
	require.NotNil(t, m, msgs[len(msgs)-1].Body)
	return m[1]
}

func TestForgotAndResetPassword_TC_AUTH_014_to_017(t *testing.T) {
	w := newWorld(t)
	c := w.Anon()
	email := testutil.Email("empA")

	// unknown and known emails look the same; only the known one gets mail
	assert.Equal(t, 202, c.Post("/api/auth/forgot-password", map[string]any{"email": "nobody@gatepass.local"}).Status)
	assert.Equal(t, 202, c.Post("/api/auth/forgot-password", map[string]any{"email": email}).Status)
	assert.Empty(t, w.Mail.To("nobody@gatepass.local"))
	token := resetToken(t, w.Env, email)

	c.Post("/api/auth/reset-password", map[string]any{"token": token, "newPassword": "short"}).Expect(400)
	c.Post("/api/auth/reset-password", map[string]any{"token": token, "newPassword": "BrandNew123"}).Expect(204)
	c.Post("/api/auth/reset-password", map[string]any{"token": token, "newPassword": "BrandNew456"}).Expect(400) // single use
	w.Anon().Post("/api/auth/login", map[string]any{"email": email, "password": "BrandNew123"}).Expect(200)

	// reset ends existing sessions (TC-AUTH-017)
	assert.Equal(t, 401, w.empA.Get("/api/auth/me").Status, "old session must be revoked")

	// expired token (TC-AUTH-016)
	c.Post("/api/auth/forgot-password", map[string]any{"email": email}).Expect(202)
	token2 := resetToken(t, w.Env, email)
	w.Clock.Advance(31 * time.Minute)
	c.Post("/api/auth/reset-password", map[string]any{"token": token2, "newPassword": "BrandNew789"}).Expect(400)
}

func TestLoginRateLimit_TC_AUTH_018(t *testing.T) {
	e := testutil.New(t, testutil.WithLoginRateLimit(10))
	c := e.Anon()
	for i := 0; i < 10; i++ {
		r := c.Post("/api/auth/login", map[string]any{"email": "nobody@gatepass.local", "password": "WrongPass1"})
		require.Equal(t, 401, r.Status)
	}
	r := c.Post("/api/auth/login", map[string]any{"email": "nobody@gatepass.local", "password": "WrongPass1"}).Expect(429)
	assert.Equal(t, "RATE_LIMITED", r.Code())
}
