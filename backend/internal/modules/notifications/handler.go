package notifications

import (
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"gatepass/internal/platform/authctx"
	"gatepass/internal/platform/httpx"
)

type DTO struct {
	ID        uuid.UUID  `json:"id"`
	VisitID   *uuid.UUID `json:"visitId"`
	Type      string     `json:"type"`
	Title     string     `json:"title"`
	Body      string     `json:"body"`
	ReadAt    *time.Time `json:"readAt"`
	CreatedAt time.Time  `json:"createdAt"`
}

func toDTO(n Notification) DTO {
	return DTO{n.ID, n.VisitID, n.Type, n.Title, n.Body, n.ReadAt, n.CreatedAt}
}

type Handler struct{ svc *Service }

func NewHandler(s *Service) *Handler { return &Handler{svc: s} }

func (h *Handler) Routes(r chi.Router) {
	r.Get("/notifications", httpx.H(h.list))
	r.Post("/notifications/read-all", httpx.H(h.readAll))
	r.Post("/notifications/{id}/read", httpx.H(h.read))
	r.Get("/events", httpx.H(h.events))
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
	rows, total, err := h.svc.List(r.Context(), p.ID, page, size, r.URL.Query().Get("unread") == "true")
	if err != nil {
		return err
	}
	unread, err := h.svc.UnreadCount(r.Context(), p.ID)
	if err != nil {
		return err
	}
	items := make([]DTO, 0, len(rows))
	for _, n := range rows {
		items = append(items, toDTO(n))
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items, "page": page, "pageSize": size, "total": total, "unreadCount": unread})
	return nil
}

func (h *Handler) read(w http.ResponseWriter, r *http.Request) error {
	p, err := authctx.Require(r.Context())
	if err != nil {
		return err
	}
	id, err := httpx.URLUUID(r, "id")
	if err != nil {
		return err
	}
	if err := h.svc.MarkRead(r.Context(), p.ID, id); err != nil {
		return err
	}
	httpx.NoContent(w)
	return nil
}

func (h *Handler) readAll(w http.ResponseWriter, r *http.Request) error {
	p, err := authctx.Require(r.Context())
	if err != nil {
		return err
	}
	if err := h.svc.MarkAllRead(r.Context(), p.ID); err != nil {
		return err
	}
	httpx.NoContent(w)
	return nil
}

// events is the Server-Sent Events stream (spec §9.2): the session cookie authenticates it.
func (h *Handler) events(w http.ResponseWriter, r *http.Request) error {
	p, err := authctx.Require(r.Context())
	if err != nil {
		return err
	}
	rc := http.NewResponseController(w)
	_ = rc.SetWriteDeadline(time.Time{}) // the server write timeout must not cut a long-lived stream
	hd := w.Header()
	hd.Set("Content-Type", "text/event-stream")
	hd.Set("Cache-Control", "no-store")
	hd.Set("Connection", "keep-alive")
	hd.Set("X-Accel-Buffering", "no")
	// subscribe BEFORE announcing the connection, so no event can fall between the two
	sub := h.svc.d.Hub.Subscribe(p.ID, p.Role)
	defer h.svc.d.Hub.Unsubscribe(sub)
	w.WriteHeader(http.StatusOK)
	if _, err := fmt.Fprint(w, ": connected\n\n"); err != nil {
		return nil
	}
	if err := rc.Flush(); err != nil {
		return nil
	}
	ping := time.NewTicker(25 * time.Second)
	defer ping.Stop()
	for {
		select {
		case <-r.Context().Done():
			return nil
		case <-sub.done:
			return nil
		case m := <-sub.ch:
			if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", m.Event, m.Data); err != nil {
				return nil
			}
		case <-ping.C:
			if _, err := fmt.Fprint(w, ": ping\n\n"); err != nil {
				return nil
			}
		}
		if err := rc.Flush(); err != nil {
			return nil
		}
	}
}
