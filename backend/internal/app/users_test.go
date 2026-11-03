//go:build integration

package app_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gatepass/internal/testutil"
)

func newUser(code, email string) map[string]any {
	return map[string]any{"employeeCode": code, "name": "New Person", "email": email, "role": "EMPLOYEE", "tempPassword": "TempPass1"}
}

func TestUsers_CreateAndRules_TC_USR_001_to_005(t *testing.T) {
	w := newWorld(t)

	r := w.admin.Post("/api/users", newUser("EMP100", "New.Person@Gatepass.local")).Expect(201)
	assert.Equal(t, true, r.Path("mustChangePassword"))
	assert.Equal(t, "new.person@gatepass.local", r.Str("email"), "emails are stored lowercase")
	assert.NotContains(t, string(r.Raw), "passwordHash")

	// duplicates by email (any case) and by employee code
	d1 := w.admin.Post("/api/users", newUser("EMP101", "NEW.PERSON@gatepass.local")).Expect(409)
	assert.Equal(t, "DUPLICATE", d1.Code())
	assert.NotEmpty(t, d1.Path("error.details.fields.email"))
	d2 := w.admin.Post("/api/users", newUser("EMP100", "other@gatepass.local")).Expect(409)
	assert.NotEmpty(t, d2.Path("error.details.fields.employeeCode"))

	// only an admin may manage users
	w.empA.Post("/api/users", newUser("EMP102", "x@gatepass.local")).Expect(403)
	w.guard.Get("/api/users").Expect(403)

	// validation
	bad := newUser("EMP103", "abc")
	bad["role"] = "SUPERUSER"
	v := w.admin.Post("/api/users", bad).Expect(400)
	assert.NotEmpty(t, v.Path("error.details.fields.role"))
	assert.NotEmpty(t, v.Path("error.details.fields.email"))

	// update; an admin cannot change their own role
	empB := w.UserID("empB")
	u := w.admin.Patch("/api/users/"+empB, map[string]any{"department": "Ops", "role": "SECURITY"}).Expect(200)
	assert.Equal(t, "Ops", u.Str("department"))
	assert.Equal(t, "SECURITY", u.Str("role"))
	w.admin.Patch("/api/users/"+w.UserID("admin"), map[string]any{"role": "EMPLOYEE"}).Expect(403)

	// clearing an optional field really sets it to NULL (rule G-6: maps, not structs)
	w.admin.Patch("/api/users/"+empB, map[string]any{"phone": "9876500000"}).Expect(200)
	w.admin.Patch("/api/users/"+empB, map[string]any{"phone": ""}).Expect(200)
	var phone *string
	require.NoError(t, w.Owner.QueryRow("SELECT phone FROM users WHERE id = $1", empB).Scan(&phone))
	assert.Nil(t, phone)

	// list filters
	l := w.admin.Get("/api/users?role=SECURITY&isActive=true").Expect(200)
	assert.GreaterOrEqual(t, int(l.Path("total").(float64)), 3)
}

func TestUsers_DeactivateEndsSessionsAtOnce_TC_USR_006_to_009(t *testing.T) {
	w := newWorld(t)
	empB := w.UserID("empB")

	w.empB.Get("/api/auth/me").Expect(200)
	assert.Equal(t, 1, w.guard.Get("/api/employees/search?q=bharat").Expect(200).Len("items"))

	w.admin.Post("/api/users/"+empB+"/deactivate", nil).Expect(200)
	assert.Equal(t, 401, w.empB.Get("/api/auth/me").Status, "the open session must stop working immediately")
	w.Anon().Post("/api/auth/login", map[string]any{"email": testutil.Email("empB"), "password": testutil.Password}).Expect(401)
	assert.Equal(t, 0, w.guard.Get("/api/employees/search?q=bharat").Expect(200).Len("items"), "inactive users cannot be chosen as host")
	var n int
	require.NoError(t, w.Owner.QueryRow("SELECT count(*) FROM user_sessions WHERE user_id = $1", empB).Scan(&n))
	assert.Zero(t, n, "session rows are removed")

	w.admin.Post("/api/users/"+w.UserID("admin")+"/deactivate", nil).Expect(403) // not yourself

	w.admin.Post("/api/users/"+w.UserID("exEmp")+"/activate", nil).Expect(200)
	w.Anon().Post("/api/auth/login", map[string]any{"email": testutil.Email("exEmp"), "password": testutil.Password}).Expect(200)

	// admin password reset forces a change and kills old sessions
	empA := w.UserID("empA")
	w.admin.Post("/api/users/"+empA+"/reset-password", map[string]any{"tempPassword": "short"}).Expect(400)
	w.admin.Post("/api/users/"+empA+"/reset-password", map[string]any{"tempPassword": "TempReset1"}).Expect(204)
	assert.Equal(t, 401, w.empA.Get("/api/auth/me").Status)
	w.Anon().Post("/api/auth/login", map[string]any{"email": testutil.Email("empA"), "password": testutil.Password}).Expect(401)
	c := w.Anon()
	c.Post("/api/auth/login", map[string]any{"email": testutil.Email("empA"), "password": "TempReset1"}).Expect(200)
	assert.Equal(t, true, c.Get("/api/auth/me").Expect(200).Path("user.mustChangePassword"))
}

func TestEmployeeSearch_TC_USR_010_011(t *testing.T) {
	w := newWorld(t)
	r := w.guard.Get("/api/employees/search?q=an").Expect(200)
	require.Equal(t, 1, r.Len("items"))
	assert.Equal(t, "Anita Desai", r.Str("items.0.name"))
	item := r.Path("items.0").(map[string]any)
	assert.ElementsMatch(t, []string{"id", "name", "employeeCode", "department"}, keys(item), "only these fields are exposed")
	assert.Equal(t, 0, w.guard.Get("/api/employees/search?q=suresh").Expect(200).Len("items"), "security staff are not hosts")
	assert.Equal(t, 0, w.guard.Get("/api/employees/search?q=old+employee").Expect(200).Len("items"), "inactive users are hidden")

	w.guard.Get("/api/employees/search?q=a").Expect(400)
	w.empA.Get("/api/employees/search?q=an").Expect(403)
	assert.Equal(t, 0, w.guard.Get("/api/employees/search?q=%25%25").Expect(200).Len("items"), "LIKE wildcards are escaped")
}

func keys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func TestUsers_CSVImport_TC_USR_012_013(t *testing.T) {
	w := newWorld(t)
	csv := "employeeCode,name,email,role,department\n" +
		"EMP200,Good One,good.one@gatepass.local,EMPLOYEE,Sales\n" +
		"EMP201,Dup Email,empa@gatepass.local,EMPLOYEE,Sales\n" +
		"EMP202,Bad Role,bad.role@gatepass.local,WIZARD,Sales\n"
	r := w.admin.Multipart("POST", "/api/users/import", []byte(csv), nil).Expect(200)
	assert.Equal(t, "created", r.Str("results.0.status"))
	assert.NotEmpty(t, r.Str("results.0.tempPassword"), "a generated password is returned once")
	assert.Equal(t, "failed", r.Str("results.1.status"))
	assert.Equal(t, "failed", r.Str("results.2.status"))

	var n int
	require.NoError(t, w.Owner.QueryRow("SELECT count(*) FROM users WHERE employee_code IN ('EMP201','EMP202')").Scan(&n))
	assert.Zero(t, n, "failed rows leave nothing behind")

	// the generated password really works, and must be changed
	c := w.Anon()
	c.Post("/api/auth/login", map[string]any{"email": "good.one@gatepass.local", "password": r.Str("results.0.tempPassword")}).Expect(200)

	w.admin.Multipart("POST", "/api/users/import", []byte("name\nx\n"), nil).Expect(400)
	w.empA.Multipart("POST", "/api/users/import", []byte(csv), nil).Expect(403)
}

func TestUserActionsAreAudited_TC_AUD_004(t *testing.T) {
	w := newWorld(t)
	created := w.admin.Post("/api/users", newUser("EMP300", "audited@gatepass.local")).Expect(201)
	id := created.Str("id")
	w.admin.Patch("/api/users/"+id, map[string]any{"role": "SECURITY"}).Expect(200)
	w.admin.Post("/api/users/"+id+"/deactivate", nil).Expect(200)

	rows, err := w.Owner.Query(`SELECT action, actor_role FROM audit_logs WHERE visit_id IS NULL AND metadata->>'userId' = $1 ORDER BY seq`, id)
	require.NoError(t, err)
	defer rows.Close()
	var actions []string
	for rows.Next() {
		var a, role string
		require.NoError(t, rows.Scan(&a, &role))
		assert.Equal(t, "ADMIN", role)
		actions = append(actions, a)
	}
	assert.Equal(t, []string{"USER_CREATED", "USER_UPDATED", "USER_DEACTIVATED"}, actions)
}
