package visits

import (
	"context"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"gatepass/internal/platform/apperr"
	"gatepass/internal/platform/authctx"
	"gatepass/internal/platform/clock"
	"gatepass/internal/platform/httpx"
)

type Handler struct{ svc *Service }

func NewHandler(s *Service) *Handler { return &Handler{svc: s} }

func (h *Handler) Routes(r chi.Router) {
	host := httpx.RequireRoles(authctx.RoleEmployee, authctx.RoleAdmin)
	sec := httpx.RequireRoles(authctx.RoleSecurity)
	admin := httpx.RequireRoles(authctx.RoleAdmin)

	r.With(host).Post("/visits/planned", httpx.H(h.createPlanned))
	r.With(sec).Post("/visits/walk-in", httpx.H(h.createWalkIn))
	r.Get("/visits", httpx.H(h.list))
	r.Get("/visits/{id}", httpx.H(h.get))
	r.Patch("/visits/{id}", httpx.H(h.update))
	r.Post("/visits/{id}/approve", httpx.H(h.simple((*Service).Approve)))
	r.Post("/visits/{id}/reject", httpx.H(h.withReason((*Service).Reject)))
	r.Post("/visits/{id}/cancel", httpx.H(h.withReason((*Service).Cancel)))
	r.With(sec).Post("/visits/{id}/check-in", httpx.H(h.simple((*Service).CheckIn)))
	r.With(sec).Post("/visits/{id}/deny", httpx.H(h.withReason((*Service).Deny)))
	r.Post("/visits/{id}/meeting-over", httpx.H(h.simple((*Service).MeetingOver)))
	r.With(sec).Post("/visits/{id}/check-out", httpx.H(h.simple((*Service).CheckOut)))
	r.With(admin).Post("/visits/{id}/force-close", httpx.H(h.withReason((*Service).ForceClose)))

	r.With(host).Get("/dashboard/employee", httpx.H(h.employeeDashboard))
	r.With(httpx.RequireRoles(authctx.RoleSecurity, authctx.RoleAdmin)).Get("/dashboard/security", httpx.H(h.securityDashboard))
}

func (h *Handler) createPlanned(w http.ResponseWriter, r *http.Request) error {
	p, err := authctx.Require(r.Context())
	if err != nil {
		return err
	}
	var in PlannedInput
	if err := httpx.DecodeJSON(w, r, &in); err != nil {
		return err
	}
	d, err := h.svc.CreatePlanned(r.Context(), p, in)
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusCreated, d)
	return nil
}

func (h *Handler) createWalkIn(w http.ResponseWriter, r *http.Request) error {
	p, err := authctx.Require(r.Context())
	if err != nil {
		return err
	}
	var in WalkInInput
	if err := httpx.DecodeJSON(w, r, &in); err != nil {
		return err
	}
	d, err := h.svc.CreateWalkIn(r.Context(), p, in)
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusCreated, d)
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
	var in PlannedPatch
	if err := httpx.DecodeJSON(w, r, &in); err != nil {
		return err
	}
	d, err := h.svc.UpdatePlanned(r.Context(), p, id, in)
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, d)
	return nil
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) error {
	p, err := authctx.Require(r.Context())
	if err != nil {
		return err
	}
	id, err := httpx.URLUUID(r, "id")
	if err != nil {
		return err
	}
	d, err := h.svc.Get(r.Context(), p, id)
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, d)
	return nil
}

func (h *Handler) simple(fn func(*Service, context.Context, authctx.Principal, uuid.UUID) (*VisitDTO, error)) httpx.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		p, err := authctx.Require(r.Context())
		if err != nil {
			return err
		}
		id, err := httpx.URLUUID(r, "id")
		if err != nil {
			return err
		}
		d, err := fn(h.svc, r.Context(), p, id)
		if err != nil {
			return err
		}
		httpx.WriteJSON(w, http.StatusOK, d)
		return nil
	}
}

func (h *Handler) withReason(fn func(*Service, context.Context, authctx.Principal, uuid.UUID, string) (*VisitDTO, error)) httpx.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		p, err := authctx.Require(r.Context())
		if err != nil {
			return err
		}
		id, err := httpx.URLUUID(r, "id")
		if err != nil {
			return err
		}
		var in struct {
			Reason string `json:"reason"`
		}
		if err := httpx.DecodeOptionalJSON(w, r, &in); err != nil {
			return err
		}
		d, err := fn(h.svc, r.Context(), p, id, in.Reason)
		if err != nil {
			return err
		}
		httpx.WriteJSON(w, http.StatusOK, d)
		return nil
	}
}

// ParseFilter reads the list/report query parameters (shared with the reports module).
func ParseFilter(r *http.Request) (ListFilter, error) {
	q := r.URL.Query()
	f := ListFilter{Q: q.Get("q"), Type: Type(q.Get("type")), OnPremises: q.Get("onPremises") == "true",
		Overstay: q.Get("overstay") == "true", Mine: q.Get("mine") == "true"}
	fields := map[string]string{}
	for _, raw := range q["status"] {
		for _, part := range strings.Split(raw, ",") {
			if part = strings.TrimSpace(part); part == "" {
				continue
			}
			if st := Status(part); st.Valid() {
				f.Statuses = append(f.Statuses, st)
			} else {
				fields["status"] = "Unknown status " + part
			}
		}
	}
	if f.Type != "" && f.Type != TypePlanned && f.Type != TypeUnplanned {
		fields["type"] = "Must be PLANNED or UNPLANNED"
	}
	if v := q.Get("hostId"); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			fields["hostId"] = "Invalid id"
		} else {
			f.HostID = &id
		}
	}
	for name, dst := range map[string]*clock.Date{"from": &f.From, "to": &f.To} {
		if v := q.Get(name); v != "" {
			d, err := clock.ParseDate(v)
			if err != nil {
				fields[name] = "Use YYYY-MM-DD"
			}
			*dst = d
		}
	}
	if len(fields) > 0 {
		return f, apperr.Validation(fields)
	}
	return f, nil
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) error {
	p, err := authctx.Require(r.Context())
	if err != nil {
		return err
	}
	page, size, err := httpx.Page(r)
	if err != nil {
		return err
	}
	f, err := ParseFilter(r)
	if err != nil {
		return err
	}
	f.Page, f.PageSize = page, size
	items, total, err := h.svc.List(r.Context(), p, f)
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, httpx.PageOf[VisitDTO]{Items: items, Page: page, PageSize: size, Total: total})
	return nil
}

func (h *Handler) employeeDashboard(w http.ResponseWriter, r *http.Request) error {
	p, err := authctx.Require(r.Context())
	if err != nil {
		return err
	}
	d, err := h.svc.EmployeeDashboard(r.Context(), p)
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, d)
	return nil
}

func (h *Handler) securityDashboard(w http.ResponseWriter, r *http.Request) error {
	p, err := authctx.Require(r.Context())
	if err != nil {
		return err
	}
	d, err := h.svc.SecurityDashboard(r.Context(), p)
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, d)
	return nil
}
