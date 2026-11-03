package users

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"gatepass/internal/platform/apperr"
	"gatepass/internal/platform/authctx"
	"gatepass/internal/platform/httpx"
)

type DTO struct {
	ID                 uuid.UUID    `json:"id"`
	EmployeeCode       string       `json:"employeeCode"`
	Name               string       `json:"name"`
	Email              string       `json:"email"`
	Phone              *string      `json:"phone"`
	Department         *string      `json:"department"`
	Role               authctx.Role `json:"role"`
	IsActive           bool         `json:"isActive"`
	MustChangePassword bool         `json:"mustChangePassword"`
	CreatedAt          time.Time    `json:"createdAt"`
}

func ToDTO(u *User) DTO {
	return DTO{u.ID, u.EmployeeCode, u.Name, u.Email, u.Phone, u.Department, u.Role, u.IsActive, u.MustChangePassword, u.CreatedAt}
}

type Handler struct{ svc *Service }

func NewHandler(s *Service) *Handler { return &Handler{svc: s} }

func (h *Handler) Routes(r chi.Router) {
	admin := r.With(httpx.RequireRoles(authctx.RoleAdmin))
	admin.Get("/users", httpx.H(h.list))
	admin.Post("/users", httpx.H(h.create))
	admin.Post("/users/import", httpx.H(h.importCSV))
	admin.Patch("/users/{id}", httpx.H(h.update))
	admin.Post("/users/{id}/deactivate", httpx.H(h.setActive(false)))
	admin.Post("/users/{id}/activate", httpx.H(h.setActive(true)))
	admin.Post("/users/{id}/reset-password", httpx.H(h.resetPassword))
	r.With(httpx.RequireRoles(authctx.RoleSecurity)).Get("/employees/search", httpx.H(h.search))
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) error {
	page, size, err := httpx.Page(r)
	if err != nil {
		return err
	}
	f := ListFilter{Q: r.URL.Query().Get("q"), Role: authctx.Role(r.URL.Query().Get("role")), Page: page, PageSize: size}
	if v := r.URL.Query().Get("isActive"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return apperr.Validation(map[string]string{"isActive": "Must be true or false"})
		}
		f.IsActive = &b
	}
	us, total, err := h.svc.List(r.Context(), f)
	if err != nil {
		return err
	}
	items := make([]DTO, 0, len(us))
	for i := range us {
		items = append(items, ToDTO(&us[i]))
	}
	httpx.WriteJSON(w, http.StatusOK, httpx.PageOf[DTO]{Items: items, Page: page, PageSize: size, Total: total})
	return nil
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) error {
	p, err := authctx.Require(r.Context())
	if err != nil {
		return err
	}
	var in CreateInput
	if err := httpx.DecodeJSON(w, r, &in); err != nil {
		return err
	}
	u, err := h.svc.Create(r.Context(), p, in)
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusCreated, ToDTO(u))
	return nil
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) error {
	p, err := authctx.Require(r.Context())
	if err != nil {
		return err
	}
	id, err := httpx.URLUUID(r, "id")
	if err != nil {
		return err
	}
	var in Patch
	if err := httpx.DecodeJSON(w, r, &in); err != nil {
		return err
	}
	u, err := h.svc.Update(r.Context(), p, id, in)
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, ToDTO(u))
	return nil
}

func (h *Handler) setActive(active bool) httpx.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		p, err := authctx.Require(r.Context())
		if err != nil {
			return err
		}
		id, err := httpx.URLUUID(r, "id")
		if err != nil {
			return err
		}
		u, err := h.svc.SetActive(r.Context(), p, id, active)
		if err != nil {
			return err
		}
		httpx.WriteJSON(w, http.StatusOK, ToDTO(u))
		return nil
	}
}

func (h *Handler) resetPassword(w http.ResponseWriter, r *http.Request) error {
	p, err := authctx.Require(r.Context())
	if err != nil {
		return err
	}
	id, err := httpx.URLUUID(r, "id")
	if err != nil {
		return err
	}
	var in struct {
		TempPassword string `json:"tempPassword"`
	}
	if err := httpx.DecodeJSON(w, r, &in); err != nil {
		return err
	}
	if err := h.svc.ResetPassword(r.Context(), p, id, in.TempPassword); err != nil {
		return err
	}
	httpx.NoContent(w)
	return nil
}

func (h *Handler) importCSV(w http.ResponseWriter, r *http.Request) error {
	p, err := authctx.Require(r.Context())
	if err != nil {
		return err
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	file, _, err := r.FormFile("file")
	if err != nil {
		return apperr.Validation(map[string]string{"file": "Attach a CSV file in the 'file' field"})
	}
	defer file.Close()
	res, err := h.svc.Import(r.Context(), p, file)
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"results": res})
	return nil
}

func (h *Handler) search(w http.ResponseWriter, r *http.Request) error {
	res, err := h.svc.SearchHosts(r.Context(), r.URL.Query().Get("q"))
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": res})
	return nil
}
