package auth

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"gatepass/internal/modules/users"
	"gatepass/internal/platform/apperr"
	"gatepass/internal/platform/authctx"
	"gatepass/internal/platform/httpx"
	"gatepass/internal/platform/scheduler"
	"gatepass/internal/platform/validation"
)

type Handler struct{ svc *Service }

func NewHandler(s *Service) *Handler { return &Handler{svc: s} }

// PublicRoutes need no session. limit (rate limiter) wraps login and forgot-password.
func (h *Handler) PublicRoutes(r chi.Router, limit func(http.Handler) http.Handler) {
	r.With(limit).Post("/auth/login", httpx.H(h.login))
	r.With(limit).Post("/auth/forgot-password", httpx.H(h.forgot))
	r.Post("/auth/reset-password", httpx.H(h.reset))
}

// AuthedRoutes need a signed-in user (the password gate lets these three through).
func (h *Handler) AuthedRoutes(r chi.Router) {
	r.Post("/auth/logout", httpx.H(h.logout))
	r.Get("/auth/me", httpx.H(h.me))
	r.Post("/auth/change-password", httpx.H(h.changePassword))
}

func (s *Service) Jobs() []scheduler.Job {
	return []scheduler.Job{{Name: "purgeTokens", Spec: "0 3 * * *", Run: s.PurgeExpiredTokens}}
}

type SessionUser struct {
	ID                 uuid.UUID    `json:"id"`
	Name               string       `json:"name"`
	Email              string       `json:"email"`
	Role               authctx.Role `json:"role"`
	EmployeeCode       string       `json:"employeeCode"`
	Department         *string      `json:"department"`
	MustChangePassword bool         `json:"mustChangePassword"`
}

func toSessionUser(u *users.User) SessionUser {
	return SessionUser{u.ID, u.Name, u.Email, u.Role, u.EmployeeCode, u.Department, u.MustChangePassword}
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := httpx.DecodeJSON(w, r, &in); err != nil {
		return err
	}
	if in.Email == "" || in.Password == "" {
		return apperr.Validation(map[string]string{"email": reqIf(in.Email), "password": reqIf(in.Password)})
	}
	u, err := h.svc.Authenticate(r.Context(), in.Email, in.Password)
	if err != nil {
		return err
	}
	mobile := r.Header.Get("X-Client") == "mobile"
	if mobile && u.Role == authctx.RoleSecurity {
		return apperr.ErrMobileNotAllowed
	}
	token, err := h.svc.StartSession(r.Context(), u)
	if err != nil {
		return err
	}
	if mobile {
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"user": toSessionUser(u), "token": token})
		return nil
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"user": toSessionUser(u)})
	return nil
}

func reqIf(s string) string {
	if s == "" {
		return "Required"
	}
	return ""
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) error {
	if err := h.svc.EndSession(r.Context()); err != nil {
		return err
	}
	httpx.NoContent(w)
	return nil
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) error {
	p, err := authctx.Require(r.Context())
	if err != nil {
		return err
	}
	u, err := h.svc.d.Users.Get(r.Context(), p.ID)
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"user": toSessionUser(u)})
	return nil
}

func (h *Handler) changePassword(w http.ResponseWriter, r *http.Request) error {
	p, err := authctx.Require(r.Context())
	if err != nil {
		return err
	}
	var in struct {
		CurrentPassword string `json:"currentPassword"`
		NewPassword     string `json:"newPassword"`
	}
	if err := httpx.DecodeJSON(w, r, &in); err != nil {
		return err
	}
	if err := h.svc.ChangePassword(r.Context(), p, in.CurrentPassword, in.NewPassword); err != nil {
		return err
	}
	httpx.NoContent(w)
	return nil
}

func (h *Handler) forgot(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		Email string `json:"email"`
	}
	if err := httpx.DecodeJSON(w, r, &in); err != nil {
		return err
	}
	if _, msg := validation.Email(in.Email); msg != "" {
		return apperr.Validation(map[string]string{"email": msg})
	}
	_ = h.svc.ForgotPassword(r.Context(), in.Email) // always 202: never reveal whether the account exists
	w.WriteHeader(http.StatusAccepted)
	return nil
}

func (h *Handler) reset(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		Token       string `json:"token"`
		NewPassword string `json:"newPassword"`
	}
	if err := httpx.DecodeJSON(w, r, &in); err != nil {
		return err
	}
	if err := h.svc.ResetPassword(r.Context(), in.Token, in.NewPassword); err != nil {
		return err
	}
	httpx.NoContent(w)
	return nil
}
