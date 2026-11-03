package notifications

import (
	"sync"

	"github.com/google/uuid"

	"gatepass/internal/platform/authctx"
)

// Message is one server-sent event.
type Message struct {
	Event string
	Data  []byte
}

type subscriber struct {
	userID uuid.UUID
	role   authctx.Role
	ch     chan Message
	done   chan struct{}
	once   sync.Once
}

func (s *subscriber) kick() { s.once.Do(func() { close(s.done) }) }

// Hub keeps the open event streams (in-process; one API instance in Release 1, spec §9.2).
// A subscriber whose buffer is full is dropped instead of blocking everyone else; the
// browser reconnects on its own.
type Hub struct {
	mu   sync.RWMutex
	subs map[*subscriber]struct{}
}

func NewHub() *Hub { return &Hub{subs: map[*subscriber]struct{}{}} }

func (h *Hub) Subscribe(userID uuid.UUID, role authctx.Role) *subscriber {
	s := &subscriber{userID: userID, role: role, ch: make(chan Message, 32), done: make(chan struct{})}
	h.mu.Lock()
	h.subs[s] = struct{}{}
	h.mu.Unlock()
	return s
}

func (h *Hub) Unsubscribe(s *subscriber) {
	h.mu.Lock()
	delete(h.subs, s)
	h.mu.Unlock()
	s.kick()
}

func (h *Hub) send(match func(*subscriber) bool, m Message) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for s := range h.subs {
		if !match(s) {
			continue
		}
		select {
		case s.ch <- m:
		default:
			s.kick() // too slow: drop the stream
		}
	}
}

func (h *Hub) ToUser(id uuid.UUID, m Message) {
	h.send(func(s *subscriber) bool { return s.userID == id }, m)
}

// ToUsersAndRoles delivers to the given users and to every subscriber with one of the roles (each once).
func (h *Hub) ToUsersAndRoles(users []uuid.UUID, roles []authctx.Role, m Message) {
	h.send(func(s *subscriber) bool {
		for _, u := range users {
			if s.userID == u {
				return true
			}
		}
		for _, r := range roles {
			if s.role == r {
				return true
			}
		}
		return false
	}, m)
}

// DisconnectUser closes the streams of a user (their sessions were revoked).
func (h *Hub) DisconnectUser(id uuid.UUID) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for s := range h.subs {
		if s.userID == id {
			s.kick()
		}
	}
}

// CloseAll ends every stream (graceful shutdown).
func (h *Hub) CloseAll() {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for s := range h.subs {
		s.kick()
	}
}
