package notifications

import (
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"gatepass/internal/platform/authctx"
)

// TC-NT-014: a client that never reads is dropped; everybody else still gets every event.
func TestHub_SlowClientDoesNotBlockOthers_TC_NT_014(t *testing.T) {
	h := NewHub()
	stalled := h.Subscribe(uuid.New(), authctx.RoleSecurity)
	healthy := h.Subscribe(uuid.New(), authctx.RoleSecurity)

	var got int
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for range healthy.ch {
			got++
			if got == 100 {
				return
			}
		}
	}()
	for i := 0; i < 100; i++ {
		h.ToUsersAndRoles(nil, []authctx.Role{authctx.RoleSecurity}, Message{Event: "visit-updated", Data: []byte("{}")})
		time.Sleep(time.Millisecond) // the healthy client keeps up
	}
	wg.Wait()
	assert.Equal(t, 100, got)
	select {
	case <-stalled.done:
	default:
		t.Fatal("the stalled subscriber should have been dropped")
	}
	select {
	case <-healthy.done:
		t.Fatal("the healthy subscriber must stay connected")
	default:
	}
}

func TestHub_TargetingAndDisconnect(t *testing.T) {
	h := NewHub()
	host, other := uuid.New(), uuid.New()
	sh := h.Subscribe(host, authctx.RoleEmployee)
	so := h.Subscribe(other, authctx.RoleEmployee)
	sg := h.Subscribe(uuid.New(), authctx.RoleSecurity)

	h.ToUsersAndRoles([]uuid.UUID{host}, []authctx.Role{authctx.RoleSecurity, authctx.RoleAdmin}, Message{Event: "x"})
	assert.Len(t, sh.ch, 1, "the host")
	assert.Len(t, sg.ch, 1, "security")
	assert.Len(t, so.ch, 0, "an unrelated employee gets nothing")

	h.DisconnectUser(host)
	select {
	case <-sh.done:
	default:
		t.Fatal("DisconnectUser must end the user's streams")
	}
	h.Unsubscribe(sg)
	h.CloseAll()
	select {
	case <-so.done:
	default:
		t.Fatal("CloseAll must end every stream")
	}
}
