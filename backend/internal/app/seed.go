package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"

	"github.com/google/uuid"

	"gatepass/internal/modules/users"
	"gatepass/internal/platform/authctx"
)

type seedUser struct {
	Key, Code, Name, Dept string
	Role                  authctx.Role
	Active                bool
}

var seedUsers = []seedUser{
	{"admin", "EMP001", "Asha Admin", "Facilities", authctx.RoleAdmin, true},
	{"empA", "EMP002", "Anita Desai", "Engineering", authctx.RoleEmployee, true},
	{"empB", "EMP003", "Bharat Mehta", "Sales", authctx.RoleEmployee, true},
	{"guard", "SEC001", "Suresh Guard", "Security", authctx.RoleSecurity, true},
	{"guard2", "SEC002", "Vikram Guard", "Security", authctx.RoleSecurity, true},
	{"exEmp", "EMP099", "Old Employee", "Engineering", authctx.RoleEmployee, false},
}

// Seed creates the development/test fixture users (skips existing ones). Password: cfgPassword,
// no forced change. Never used in production.
func (a *App) Seed(ctx context.Context, pw string, out io.Writer) error {
	hash, err := a.Hasher.Hash(pw)
	if err != nil {
		return err
	}
	now := a.Clock.Now()
	for _, s := range seedUsers {
		email := fmt.Sprintf("%s@gatepass.local", lower(s.Key))
		if _, err := a.Users.FindByEmail(ctx, email); err == nil {
			fmt.Fprintf(out, "exists   %-24s %s\n", email, s.Role)
			continue
		}
		dept := s.Dept
		u := users.User{ID: uuid.New(), EmployeeCode: s.Code, Name: s.Name, Email: email, Department: &dept, Role: s.Role,
			PasswordHash: hash, IsActive: s.Active, CreatedAt: now, UpdatedAt: now}
		if err := a.DB.WithContext(ctx).Create(&u).Error; err != nil {
			return err
		}
		fmt.Fprintf(out, "created  %-24s %s\n", email, s.Role)
	}
	fmt.Fprintf(out, "\nAll seed users sign in with password: %s\n", pw)
	return nil
}

// CreateAdmin creates an ADMIN with a random temporary password and a forced password change.
func (a *App) CreateAdmin(ctx context.Context, email, name, code string, out io.Writer) error {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	temp := "Gp" + hex.EncodeToString(b) + "9"
	actor := authctx.Principal{Role: authctx.RoleAdmin, ID: uuid.Nil}
	u, err := a.Users.Create(ctx, actor, users.CreateInput{EmployeeCode: code, Name: name, Email: email, Role: authctx.RoleAdmin, TempPassword: temp})
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "Created admin %s (%s)\nTemporary password (change it at first sign-in): %s\n", u.Email, u.Name, temp)
	return nil
}

func lower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 32
		}
	}
	return string(b)
}
