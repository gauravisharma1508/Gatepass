// Package auth owns sign-in, sessions and password flows (spec §7).
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/alexedwards/scs/postgresstore"
	"github.com/alexedwards/scs/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"gatepass/internal/modules/users"
	"gatepass/internal/platform/apperr"
	"gatepass/internal/platform/authctx"
	"gatepass/internal/platform/clock"
	"gatepass/internal/platform/mailer"
	"gatepass/internal/platform/password"
	"gatepass/internal/platform/validation"
)

const sessionUserKey = "userID"

type Deps struct {
	DB              *gorm.DB
	Users           *users.Service
	Hasher          *password.Hasher
	Clock           clock.Clock
	Mailer          mailer.Mailer
	WebOrigin       string
	Production      bool
	CleanupInterval time.Duration // 0 disables the expired-session cleanup goroutine (tests)
	OnRevoke        func(userID uuid.UUID)
}

type Service struct {
	d  Deps
	sm *scs.SessionManager
	// a hash to compare against when the email is unknown, so timing does not reveal accounts
	dummyHash string
}

func New(d Deps) (*Service, error) {
	sqlDB, err := d.DB.DB()
	if err != nil {
		return nil, err
	}
	sm := scs.New()
	sm.Store = postgresstore.NewWithCleanupInterval(sqlDB, d.CleanupInterval)
	sm.Lifetime = 12 * time.Hour
	sm.IdleTimeout = 30 * time.Minute
	sm.Cookie.Name = "gp.sid"
	sm.Cookie.HttpOnly = true
	sm.Cookie.SameSite = http.SameSiteLaxMode
	sm.Cookie.Secure = d.Production
	sm.Cookie.Path = "/"
	dummy, err := d.Hasher.Hash("not-a-real-password-1")
	if err != nil {
		return nil, err
	}
	return &Service{d: d, sm: sm, dummyHash: dummy}, nil
}

func (s *Service) Sessions() *scs.SessionManager { return s.sm }

var errInvalidLogin = apperr.ErrUnauthenticated.With("Invalid email or password")

// Authenticate checks the credentials, applying the lockout rule (5 failures → 15 min).
func (s *Service) Authenticate(ctx context.Context, email, pw string) (*users.User, error) {
	u, err := s.d.Users.FindByEmail(ctx, email)
	if err != nil {
		s.d.Hasher.Check(s.dummyHash, pw)
		return nil, errInvalidLogin
	}
	now := s.d.Clock.Now()
	if u.LockedUntil != nil && now.Before(*u.LockedUntil) {
		return nil, apperr.ErrAccountLocked
	}
	if !s.d.Hasher.Check(u.PasswordHash, pw) {
		if err := s.d.Users.RecordLoginFailure(ctx, u.ID); err != nil {
			return nil, err
		}
		return nil, errInvalidLogin
	}
	if !u.IsActive {
		return nil, errInvalidLogin
	}
	if err := s.d.Users.RecordLoginSuccess(ctx, u.ID); err != nil {
		return nil, err
	}
	return u, nil
}

// StartSession binds the (request-scoped) session to the user and remembers it for revocation.
// It returns the session token (mobile clients carry it as a bearer token).
func (s *Service) StartSession(ctx context.Context, u *users.User) (string, error) {
	if err := s.sm.RenewToken(ctx); err != nil {
		return "", err
	}
	s.sm.Put(ctx, sessionUserKey, u.ID.String())
	token, _, err := s.sm.Commit(ctx)
	if err != nil {
		return "", err
	}
	err = s.d.DB.WithContext(ctx).Exec(
		"INSERT INTO user_sessions (token, user_id, created_at) VALUES (?, ?, ?) ON CONFLICT (token) DO NOTHING",
		token, u.ID, s.d.Clock.Now()).Error
	return token, err
}

// BearerBridge lets a mobile client send its session token as "Authorization: Bearer <token>".
// It runs before the session manager: the token is presented to it as the session cookie, the
// response never carries Set-Cookie, and the request skips the CSRF check (a bearer header is
// not attached by browsers on their own, so it cannot be forged cross-site).
func BearerBridge(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		const prefix = "Bearer "
		h := r.Header.Get("Authorization")
		mobile := r.Header.Get("X-Client") == "mobile"
		if len(h) > len(prefix) && strings.EqualFold(h[:len(prefix)], prefix) {
			r.Header.Set("Cookie", (&http.Cookie{Name: "gp.sid", Value: strings.TrimSpace(h[len(prefix):])}).String())
			r = r.WithContext(context.WithValue(r.Context(), bearerKey{}, true))
			mobile = true
		}
		if mobile {
			w = &noCookieWriter{ResponseWriter: w}
		}
		next.ServeHTTP(w, r)
	})
}

type bearerKey struct{}

// IsBearer reports whether the request was authenticated with a bearer token.
func IsBearer(r *http.Request) bool { b, _ := r.Context().Value(bearerKey{}).(bool); return b }

// noCookieWriter drops Set-Cookie so mobile responses never set a cookie.
type noCookieWriter struct{ http.ResponseWriter }

func (w *noCookieWriter) WriteHeader(code int) {
	w.Header().Del("Set-Cookie")
	w.ResponseWriter.WriteHeader(code)
}
func (w *noCookieWriter) Write(b []byte) (int, error) {
	w.Header().Del("Set-Cookie")
	return w.ResponseWriter.Write(b)
}
func (w *noCookieWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (s *Service) EndSession(ctx context.Context) error { return s.sm.Destroy(ctx) }

// RevokeUser deletes every session of the user (deactivation, password reset, role change).
func (s *Service) RevokeUser(ctx context.Context, id uuid.UUID) error {
	err := s.d.DB.WithContext(ctx).Exec(
		"DELETE FROM sessions WHERE token IN (SELECT token FROM user_sessions WHERE user_id = ?)", id).Error
	if err == nil && s.d.OnRevoke != nil {
		s.d.OnRevoke(id)
	}
	return err
}

// Middleware: loads the session and, when it belongs to an active user, puts the principal in the context.
func (s *Service) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		if idStr := s.sm.GetString(ctx, sessionUserKey); idStr != "" {
			if id, err := uuid.Parse(idStr); err == nil {
				if u, err := s.d.Users.Get(ctx, id); err == nil && u.IsActive {
					ctx = authctx.With(ctx, u.Principal())
				} else {
					_ = s.sm.Destroy(ctx)
				}
			}
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *Service) ChangePassword(ctx context.Context, p authctx.Principal, current, next string) error {
	u, err := s.d.Users.Get(ctx, p.ID)
	if err != nil {
		return err
	}
	f := validation.New()
	if !s.d.Hasher.Check(u.PasswordHash, current) {
		f.Set("currentPassword", "Incorrect password")
	}
	f.Set("newPassword", validation.Password(next))
	if err := f.Err(); err != nil {
		return err
	}
	hash, err := s.d.Hasher.Hash(next)
	if err != nil {
		return err
	}
	return s.d.Users.SetPassword(ctx, p.ID, hash, false)
}

const resetTokenTTL = 30 * time.Minute

// ForgotPassword never reveals whether the email exists; a mail is sent only to active users.
func (s *Service) ForgotPassword(ctx context.Context, email string) error {
	u, err := s.d.Users.FindByEmail(ctx, email)
	if err != nil || !u.IsActive {
		return nil
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return err
	}
	token := hex.EncodeToString(raw)
	now := s.d.Clock.Now()
	err = s.d.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("DELETE FROM password_reset_tokens WHERE user_id = ? AND used_at IS NULL", u.ID).Error; err != nil {
			return err
		}
		return tx.Exec("INSERT INTO password_reset_tokens (id, user_id, token_hash, expires_at, created_at) VALUES (?, ?, ?, ?, ?)",
			uuid.New(), u.ID, hashToken(token), now.Add(resetTokenTTL), now).Error
	})
	if err != nil {
		return err
	}
	link := fmt.Sprintf("%s/reset-password?token=%s", strings.TrimRight(s.d.WebOrigin, "/"), token)
	return s.d.Mailer.Send(ctx, mailer.Message{To: u.Email, Subject: "Reset your Gatepass password",
		Body: fmt.Sprintf("Hello %s,\n\nUse this link to set a new password (valid for 30 minutes):\n%s\n\nIf you did not ask for this, ignore this email.\n", u.Name, link)})
}

func (s *Service) ResetPassword(ctx context.Context, token, next string) error {
	bad := apperr.Validation(map[string]string{"token": "This reset link is invalid or has expired"})
	if msg := validation.Password(next); msg != "" {
		return apperr.Validation(map[string]string{"newPassword": msg})
	}
	var row struct {
		ID     uuid.UUID
		UserID uuid.UUID
	}
	now := s.d.Clock.Now()
	res := s.d.DB.WithContext(ctx).Raw(
		"SELECT id, user_id FROM password_reset_tokens WHERE token_hash = ? AND used_at IS NULL AND expires_at > ?",
		hashToken(token), now).Scan(&row)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return bad
	}
	hash, err := s.d.Hasher.Hash(next)
	if err != nil {
		return err
	}
	// mark used first (conditional) so the token cannot be used twice concurrently
	used := s.d.DB.WithContext(ctx).Exec("UPDATE password_reset_tokens SET used_at = ? WHERE id = ? AND used_at IS NULL", now, row.ID)
	if used.Error != nil {
		return used.Error
	}
	if used.RowsAffected == 0 {
		return bad
	}
	if err := s.d.Users.SetPassword(ctx, row.UserID, hash, false); err != nil {
		return err
	}
	return s.RevokeUser(ctx, row.UserID)
}

// PurgeExpiredTokens is the purgeTokens job.
func (s *Service) PurgeExpiredTokens(ctx context.Context) error {
	return s.d.DB.WithContext(ctx).Exec("DELETE FROM password_reset_tokens WHERE expires_at < ? OR used_at IS NOT NULL",
		s.d.Clock.Now().Add(-24*time.Hour)).Error
}

func hashToken(t string) string {
	h := sha256.Sum256([]byte(t))
	return hex.EncodeToString(h[:])
}
