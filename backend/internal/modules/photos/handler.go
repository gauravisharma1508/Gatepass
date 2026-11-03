package photos

import (
	"errors"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"

	"gatepass/internal/platform/apperr"
	"gatepass/internal/platform/authctx"
	"gatepass/internal/platform/httpx"
)

type Handler struct{ svc *Service }

func NewHandler(s *Service) *Handler { return &Handler{svc: s} }

func (h *Handler) Routes(r chi.Router) {
	r.With(httpx.RequireRoles(authctx.RoleSecurity)).Put("/visits/{id}/photos/{kind}", httpx.H(h.upload))
	r.Get("/visits/{id}/photos/{kind}", httpx.H(h.get))
}

func (h *Handler) upload(w http.ResponseWriter, r *http.Request) error {
	p, err := authctx.Require(r.Context())
	if err != nil {
		return err
	}
	id, err := httpx.URLUUID(r, "id")
	if err != nil {
		return err
	}
	r.Body = http.MaxBytesReader(w, r.Body, MaxUploadBytes+64<<10)
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			return apperr.ErrFileTooLarge
		}
		return apperr.Validation(map[string]string{"file": "Send the photo as multipart form data in the 'file' field"})
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		return apperr.Validation(map[string]string{"file": "Attach the photo in the 'file' field"})
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, MaxUploadBytes+1))
	if err != nil {
		return err
	}
	if len(data) > MaxUploadBytes {
		return apperr.ErrFileTooLarge
	}
	in := UploadInput{Kind: chi.URLParam(r, "kind"), Data: data, IDType: r.FormValue("idType"), IDLast4: r.FormValue("idLast4")}
	if err := h.svc.Upload(r.Context(), p, id, in); err != nil {
		return err
	}
	httpx.NoContent(w)
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
	rc, err := h.svc.Open(r.Context(), p, id, chi.URLParam(r, "kind"))
	if err != nil {
		return err
	}
	defer rc.Close()
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "private, no-store")
	_, _ = io.Copy(w, rc)
	return nil
}
