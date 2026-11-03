// Package users owns user accounts: admin management, employee search, and the
// account operations the auth module needs.
package users

import (
	"context"
	"crypto/rand"
	"encoding/csv"
	"encoding/hex"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"gatepass/internal/modules/audit"
	"gatepass/internal/platform/apperr"
	"gatepass/internal/platform/authctx"
	"gatepass/internal/platform/clock"
	"gatepass/internal/platform/db"
	"gatepass/internal/platform/password"
	"gatepass/internal/platform/validation"
)

type User struct {
	ID                 uuid.UUID `gorm:"primaryKey"`
	EmployeeCode       string
	Name               string
	Email              string
	Phone              *string
	Department         *string
	Role               authctx.Role
	PasswordHash       string
	MustChangePassword bool
	IsActive           bool
	FailedLoginCount   int
	LockedUntil        *time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

func (User) TableName() string { return "users" }

func (u *User) Principal() authctx.Principal {
	return authctx.Principal{ID: u.ID, Role: u.Role, Name: u.Name, Email: u.Email, MustChangePassword: u.MustChangePassword}
}

// SessionRevoker ends all sessions of a user. It is implemented by the auth module
// (declared here so users does not import auth; injected by internal/app).
type SessionRevoker interface {
	RevokeUser(ctx context.Context, id uuid.UUID) error
}

type Service struct {
	db      *gorm.DB
	audit   *audit.Service
	hasher  *password.Hasher
	clock   clock.Clock
	revoker SessionRevoker
}

func New(g *gorm.DB, a *audit.Service, h *password.Hasher, clk clock.Clock) *Service {
	return &Service{db: g, audit: a, hasher: h, clock: clk}
}

func (s *Service) SetRevoker(r SessionRevoker) { s.revoker = r }

// ---- lookups used by other modules ----

func (s *Service) Get(ctx context.Context, id uuid.UUID) (*User, error) {
	var u User
	if err := s.db.WithContext(ctx).First(&u, "id = ?", id).Error; err != nil {
		return nil, db.MapError(err)
	}
	return &u, nil
}

func (s *Service) FindByEmail(ctx context.Context, email string) (*User, error) {
	var u User
	if err := s.db.WithContext(ctx).Where("lower(email) = ?", strings.ToLower(strings.TrimSpace(email))).First(&u).Error; err != nil {
		return nil, db.MapError(err)
	}
	return &u, nil
}

// IDsByRole returns the ids of active users with one of the roles.
func (s *Service) IDsByRole(ctx context.Context, roles ...authctx.Role) ([]uuid.UUID, error) {
	var ids []uuid.UUID
	err := s.db.WithContext(ctx).Model(&User{}).Where("is_active AND role IN ?", roles).Pluck("id", &ids).Error
	return ids, err
}

// ValidateHost returns the user when they are an active EMPLOYEE or ADMIN (valid host).
func (s *Service) ValidateHost(ctx context.Context, id uuid.UUID) (*User, error) {
	u, err := s.Get(ctx, id)
	if err != nil || !u.IsActive || !(u.Role == authctx.RoleEmployee || u.Role == authctx.RoleAdmin) {
		return nil, apperr.Validation(map[string]string{"hostId": "Choose an active employee"})
	}
	return u, nil
}

const (
	maxFailures = 5
	lockFor     = 15 * time.Minute
)

// RecordLoginFailure counts a failed login; the 5th failure locks the account for 15 minutes.
func (s *Service) RecordLoginFailure(ctx context.Context, id uuid.UUID) error {
	now := s.clock.Now()
	return s.db.WithContext(ctx).Exec(`UPDATE users SET
		failed_login_count = CASE WHEN failed_login_count + 1 >= ? THEN 0 ELSE failed_login_count + 1 END,
		locked_until = CASE WHEN failed_login_count + 1 >= ? THEN ?::timestamptz ELSE locked_until END,
		updated_at = ? WHERE id = ?`, maxFailures, maxFailures, now.Add(lockFor), now, id).Error
}

func (s *Service) RecordLoginSuccess(ctx context.Context, id uuid.UUID) error {
	return s.db.WithContext(ctx).Model(&User{}).Where("id = ?", id).
		Updates(map[string]any{"failed_login_count": 0, "locked_until": nil}).Error
}

// SetPassword stores a new hash and clears any lock.
func (s *Service) SetPassword(ctx context.Context, id uuid.UUID, hash string, mustChange bool) error {
	return s.db.WithContext(ctx).Model(&User{}).Where("id = ?", id).Updates(map[string]any{
		"password_hash": hash, "must_change_password": mustChange,
		"failed_login_count": 0, "locked_until": nil, "updated_at": s.clock.Now()}).Error
}

// ---- admin operations ----

type CreateInput struct {
	EmployeeCode string       `json:"employeeCode"`
	Name         string       `json:"name"`
	Email        string       `json:"email"`
	Phone        string       `json:"phone"`
	Department   string       `json:"department"`
	Role         authctx.Role `json:"role"`
	TempPassword string       `json:"tempPassword"`
}

func (s *Service) Create(ctx context.Context, actor authctx.Principal, in CreateInput) (*User, error) {
	f := validation.New()
	code, msg := validation.EmployeeCode(in.EmployeeCode)
	f.Set("employeeCode", msg)
	name, msg := validation.Name(in.Name)
	f.Set("name", msg)
	email, msg := validation.Email(in.Email)
	f.Set("email", msg)
	phone, msg := validation.Text(in.Phone, 0, 15)
	f.Set("phone", msg)
	dept, msg := validation.Text(in.Department, 0, 100)
	f.Set("department", msg)
	if !in.Role.Valid() {
		f.Set("role", "Choose EMPLOYEE, SECURITY or ADMIN")
	}
	f.Set("tempPassword", validation.Password(in.TempPassword))
	if err := f.Err(); err != nil {
		return nil, err
	}
	if err := s.checkUnique(ctx, uuid.Nil, code, email); err != nil {
		return nil, err
	}
	hash, err := s.hasher.Hash(in.TempPassword)
	if err != nil {
		return nil, err
	}
	now := s.clock.Now()
	u := &User{ID: uuid.New(), EmployeeCode: code, Name: name, Email: email, Phone: nilIfEmpty(phone), Department: nilIfEmpty(dept),
		Role: in.Role, PasswordHash: hash, MustChangePassword: true, IsActive: true, CreatedAt: now, UpdatedAt: now}
	err = db.WithTx(ctx, s.db, func(tx *db.Tx) error {
		if err := tx.Create(u).Error; err != nil {
			return db.MapError(err)
		}
		return s.audit.Record(ctx, tx.DB, audit.Entry{ActorID: &actor.ID, ActorRole: string(actor.Role), Action: "USER_CREATED",
			Metadata: map[string]any{"userId": u.ID, "role": u.Role}})
	})
	if err != nil {
		return nil, err
	}
	return u, nil
}

func (s *Service) checkUnique(ctx context.Context, except uuid.UUID, code, email string) error {
	var n int64
	if err := s.db.WithContext(ctx).Model(&User{}).Where("lower(email) = ? AND id <> ?", email, except).Count(&n).Error; err != nil {
		return err
	}
	if n > 0 {
		return apperr.DuplicateField("email", "A user with this email already exists")
	}
	if err := s.db.WithContext(ctx).Model(&User{}).Where("employee_code = ? AND id <> ?", code, except).Count(&n).Error; err != nil {
		return err
	}
	if n > 0 {
		return apperr.DuplicateField("employeeCode", "This employee code is already used")
	}
	return nil
}

type Patch struct {
	EmployeeCode *string       `json:"employeeCode"`
	Name         *string       `json:"name"`
	Email        *string       `json:"email"`
	Phone        *string       `json:"phone"`
	Department   *string       `json:"department"`
	Role         *authctx.Role `json:"role"`
}

func (s *Service) Update(ctx context.Context, actor authctx.Principal, id uuid.UUID, p Patch) (*User, error) {
	u, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	f := validation.New()
	changed := map[string]any{}
	updates := map[string]any{}
	if p.Name != nil {
		v, msg := validation.Name(*p.Name)
		f.Set("name", msg)
		updates["name"], changed["name"] = v, v
	}
	code, email := u.EmployeeCode, u.Email
	if p.EmployeeCode != nil {
		v, msg := validation.EmployeeCode(*p.EmployeeCode)
		f.Set("employeeCode", msg)
		code = v
		updates["employee_code"], changed["employeeCode"] = v, v
	}
	if p.Email != nil {
		v, msg := validation.Email(*p.Email)
		f.Set("email", msg)
		email = v
		updates["email"], changed["email"] = v, v
	}
	if p.Phone != nil {
		v, msg := validation.Text(*p.Phone, 0, 15)
		f.Set("phone", msg)
		updates["phone"], changed["phone"] = nilIfEmpty(v), v
	}
	if p.Department != nil {
		v, msg := validation.Text(*p.Department, 0, 100)
		f.Set("department", msg)
		updates["department"], changed["department"] = nilIfEmpty(v), v
	}
	if p.Role != nil {
		if !p.Role.Valid() {
			f.Set("role", "Choose EMPLOYEE, SECURITY or ADMIN")
		} else if *p.Role != u.Role {
			if id == actor.ID {
				return nil, apperr.ErrForbidden.With("You cannot change your own role")
			}
			updates["role"], changed["role"] = *p.Role, *p.Role
		}
	}
	if err := f.Err(); err != nil {
		return nil, err
	}
	if len(updates) == 0 {
		return u, nil
	}
	if err := s.checkUnique(ctx, id, code, email); err != nil {
		return nil, err
	}
	updates["updated_at"] = s.clock.Now()
	err = db.WithTx(ctx, s.db, func(tx *db.Tx) error {
		if err := tx.Model(&User{}).Where("id = ?", id).Updates(updates).Error; err != nil {
			return db.MapError(err)
		}
		return s.audit.Record(ctx, tx.DB, audit.Entry{ActorID: &actor.ID, ActorRole: string(actor.Role), Action: "USER_UPDATED",
			Metadata: map[string]any{"userId": id, "changed": changed}})
	})
	if err != nil {
		return nil, err
	}
	if p.Role != nil && *p.Role != u.Role && s.revoker != nil {
		_ = s.revoker.RevokeUser(ctx, id) // the role changed: force a fresh sign-in
	}
	return s.Get(ctx, id)
}

func (s *Service) SetActive(ctx context.Context, actor authctx.Principal, id uuid.UUID, active bool) (*User, error) {
	if id == actor.ID && !active {
		return nil, apperr.ErrForbidden.With("You cannot deactivate your own account")
	}
	if _, err := s.Get(ctx, id); err != nil {
		return nil, err
	}
	action := "USER_DEACTIVATED"
	if active {
		action = "USER_ACTIVATED"
	}
	err := db.WithTx(ctx, s.db, func(tx *db.Tx) error {
		if err := tx.Model(&User{}).Where("id = ?", id).Updates(map[string]any{"is_active": active, "updated_at": s.clock.Now()}).Error; err != nil {
			return err
		}
		return s.audit.Record(ctx, tx.DB, audit.Entry{ActorID: &actor.ID, ActorRole: string(actor.Role), Action: action,
			Metadata: map[string]any{"userId": id}})
	})
	if err != nil {
		return nil, err
	}
	if !active && s.revoker != nil {
		if err := s.revoker.RevokeUser(ctx, id); err != nil {
			return nil, err
		}
	}
	return s.Get(ctx, id)
}

func (s *Service) ResetPassword(ctx context.Context, actor authctx.Principal, id uuid.UUID, temp string) error {
	if msg := validation.Password(temp); msg != "" {
		return apperr.Validation(map[string]string{"tempPassword": msg})
	}
	if _, err := s.Get(ctx, id); err != nil {
		return err
	}
	hash, err := s.hasher.Hash(temp)
	if err != nil {
		return err
	}
	err = db.WithTx(ctx, s.db, func(tx *db.Tx) error {
		if err := tx.Model(&User{}).Where("id = ?", id).Updates(map[string]any{
			"password_hash": hash, "must_change_password": true, "failed_login_count": 0, "locked_until": nil,
			"updated_at": s.clock.Now()}).Error; err != nil {
			return err
		}
		return s.audit.Record(ctx, tx.DB, audit.Entry{ActorID: &actor.ID, ActorRole: string(actor.Role), Action: "USER_PASSWORD_RESET",
			Metadata: map[string]any{"userId": id}})
	})
	if err != nil {
		return err
	}
	if s.revoker != nil {
		return s.revoker.RevokeUser(ctx, id)
	}
	return nil
}

type ListFilter struct {
	Q        string
	Role     authctx.Role
	IsActive *bool
	Page     int
	PageSize int
}

func (s *Service) List(ctx context.Context, f ListFilter) ([]User, int, error) {
	q := s.db.WithContext(ctx).Model(&User{})
	if t := strings.TrimSpace(f.Q); t != "" {
		like := "%" + escapeLike(strings.ToLower(t)) + "%"
		q = q.Where(`lower(name) LIKE ? ESCAPE '\' OR lower(email) LIKE ? ESCAPE '\' OR lower(employee_code) LIKE ? ESCAPE '\'`, like, like, like)
	}
	if f.Role != "" {
		q = q.Where("role = ?", f.Role)
	}
	if f.IsActive != nil {
		q = q.Where("is_active = ?", *f.IsActive)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var out []User
	err := q.Order("name, id").Limit(f.PageSize).Offset((f.Page - 1) * f.PageSize).Find(&out).Error
	return out, int(total), err
}

type HostSummary struct {
	ID           uuid.UUID `json:"id"`
	Name         string    `json:"name"`
	EmployeeCode string    `json:"employeeCode"`
	Department   *string   `json:"department"`
}

// SearchHosts finds active employees for the "Whom to meet" field (min 2 chars, max 10 results).
func (s *Service) SearchHosts(ctx context.Context, q string) ([]HostSummary, error) {
	q = strings.TrimSpace(q)
	if len([]rune(q)) < 2 {
		return nil, apperr.Validation(map[string]string{"q": "Type at least 2 characters"})
	}
	like := "%" + escapeLike(strings.ToLower(q)) + "%"
	var us []User
	err := s.db.WithContext(ctx).Where(`is_active AND role IN ? AND (lower(name) LIKE ? ESCAPE '\' OR lower(email) LIKE ? ESCAPE '\' OR lower(employee_code) LIKE ? ESCAPE '\')`,
		[]authctx.Role{authctx.RoleEmployee, authctx.RoleAdmin}, like, like, like).Order("name").Limit(10).Find(&us).Error
	out := make([]HostSummary, 0, len(us))
	for _, u := range us {
		out = append(out, HostSummary{u.ID, u.Name, u.EmployeeCode, u.Department})
	}
	return out, err
}

type ImportResult struct {
	Row          int    `json:"row"`
	Email        string `json:"email"`
	Status       string `json:"status"` // created | failed
	Error        string `json:"error,omitempty"`
	TempPassword string `json:"tempPassword,omitempty"`
}

// Import creates users from CSV with header: employeeCode,name,email,role[,department,phone,tempPassword].
// Each row is independent; rows without tempPassword get a generated one (returned once).
func (s *Service) Import(ctx context.Context, actor authctx.Principal, r io.Reader) ([]ImportResult, error) {
	cr := csv.NewReader(r)
	cr.TrimLeadingSpace = true
	head, err := cr.Read()
	if err != nil {
		return nil, apperr.Validation(map[string]string{"file": "The CSV file is empty or unreadable"})
	}
	col := map[string]int{}
	for i, h := range head {
		col[strings.ToLower(strings.TrimSpace(h))] = i
	}
	for _, need := range []string{"employeecode", "name", "email", "role"} {
		if _, ok := col[need]; !ok {
			return nil, apperr.Validation(map[string]string{"file": "Missing column: " + need})
		}
	}
	get := func(rec []string, name string) string {
		if i, ok := col[name]; ok && i < len(rec) {
			return strings.TrimSpace(rec[i])
		}
		return ""
	}
	var out []ImportResult
	for row := 2; ; row++ {
		rec, err := cr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			out = append(out, ImportResult{Row: row, Status: "failed", Error: "Unreadable row"})
			continue
		}
		in := CreateInput{EmployeeCode: get(rec, "employeecode"), Name: get(rec, "name"), Email: get(rec, "email"),
			Role: authctx.Role(strings.ToUpper(get(rec, "role"))), Department: get(rec, "department"), Phone: get(rec, "phone"),
			TempPassword: get(rec, "temppassword")}
		generated := false
		if in.TempPassword == "" {
			in.TempPassword, generated = randomPassword(), true
		}
		res := ImportResult{Row: row, Email: in.Email}
		if _, err := s.Create(ctx, actor, in); err != nil {
			res.Status, res.Error = "failed", describe(err)
		} else {
			res.Status = "created"
			if generated {
				res.TempPassword = in.TempPassword
			}
		}
		out = append(out, res)
	}
	return out, nil
}

func describe(err error) string {
	if ae, ok := err.(*apperr.Error); ok {
		if f, ok := ae.Details["fields"].(map[string]any); ok {
			parts := make([]string, 0, len(f))
			for k, v := range f {
				parts = append(parts, k+": "+v.(string))
			}
			return strings.Join(parts, "; ")
		}
		return ae.Message
	}
	return "Could not create user"
}

func randomPassword() string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return "Gp" + hex.EncodeToString(b) + "9"
}

func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
