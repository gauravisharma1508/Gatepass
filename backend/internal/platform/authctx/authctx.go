// Package authctx carries the signed-in user through the request context.
package authctx

import (
	"context"

	"github.com/google/uuid"

	"gatepass/internal/platform/apperr"
)

type Role string

const (
	RoleEmployee Role = "EMPLOYEE"
	RoleSecurity Role = "SECURITY"
	RoleAdmin    Role = "ADMIN"
)

func (r Role) Valid() bool { return r == RoleEmployee || r == RoleSecurity || r == RoleAdmin }

type Principal struct {
	ID                 uuid.UUID
	Role               Role
	Name               string
	Email              string
	MustChangePassword bool
}

// CanHost reports whether the user can be the host of a visit (spec §4).
func (p Principal) CanHost() bool { return p.Role == RoleEmployee || p.Role == RoleAdmin }

type ctxKey struct{}

func With(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, ctxKey{}, p)
}

func From(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(ctxKey{}).(Principal)
	return p, ok
}

func Require(ctx context.Context) (Principal, error) {
	p, ok := From(ctx)
	if !ok {
		return Principal{}, apperr.ErrUnauthenticated
	}
	return p, nil
}
