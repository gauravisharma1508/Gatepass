package validation_test

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gatepass/internal/platform/clock"
	"gatepass/internal/platform/validation"
)

// The same vectors are run against the web app's Zod schemas (frontend), so both sides agree.
type vectors struct {
	Name struct {
		Valid, Invalid []string
		Normalize      [][2]string
	}
	Mobile struct {
		Valid   [][2]string
		Invalid []string
	}
	Email          struct{ Valid, Invalid []string }
	Purpose        struct{ Valid, Invalid []string }
	Duration       struct{ Valid, Invalid []int }
	Arrival        struct{ Valid, Invalid []string }
	ReasonRequired struct{ Valid, Invalid []string }
	IDLast4        struct{ Valid, Invalid []string }
	Password       struct{ Valid, Invalid []string }
	VisitDate      struct {
		Today          string
		MaxAdvanceDays int
		Valid, Invalid []string
	}
}

func load(t *testing.T) vectors {
	t.Helper()
	b, err := os.ReadFile("../../../../testdata/validation_vectors.json")
	require.NoError(t, err)
	var v vectors
	require.NoError(t, json.Unmarshal(b, &v))
	return v
}

func TestVectors_TC_VAL(t *testing.T) {
	v := load(t)
	for _, s := range v.Name.Valid {
		_, msg := validation.Name(s)
		assert.Empty(t, msg, "name %q should be valid", s)
	}
	for _, s := range v.Name.Invalid {
		_, msg := validation.Name(s)
		assert.NotEmpty(t, msg, "name %q should be invalid", s)
	}
	for _, p := range v.Name.Normalize {
		got, _ := validation.Name(p[0])
		assert.Equal(t, p[1], got)
	}
	for _, p := range v.Mobile.Valid {
		got, msg := validation.Mobile(p[0])
		assert.Empty(t, msg, "mobile %q", p[0])
		assert.Equal(t, p[1], got)
	}
	for _, s := range v.Mobile.Invalid {
		_, msg := validation.Mobile(s)
		assert.NotEmpty(t, msg, "mobile %q should be invalid", s)
	}
	for _, s := range v.Email.Valid {
		_, msg := validation.OptionalEmail(s)
		assert.Empty(t, msg, "email %q", s)
	}
	for _, s := range v.Email.Invalid {
		_, msg := validation.OptionalEmail(s)
		assert.NotEmpty(t, msg, "email %q should be invalid", s)
	}
	for _, s := range v.Purpose.Valid {
		_, msg := validation.Text(s, 3, 250)
		assert.Empty(t, msg)
	}
	for _, s := range v.Purpose.Invalid {
		_, msg := validation.Text(s, 3, 250)
		assert.NotEmpty(t, msg, "purpose %q", s)
	}
	for _, n := range v.Duration.Valid {
		assert.Empty(t, validation.Duration(n), n)
	}
	for _, n := range v.Duration.Invalid {
		assert.NotEmpty(t, validation.Duration(n), n)
	}
	for _, s := range v.Arrival.Valid {
		assert.Empty(t, validation.Arrival(s), s)
	}
	for _, s := range v.Arrival.Invalid {
		assert.NotEmpty(t, validation.Arrival(s), s)
	}
	for _, s := range v.ReasonRequired.Valid {
		_, msg := validation.Reason(s, true)
		assert.Empty(t, msg, s)
	}
	for _, s := range v.ReasonRequired.Invalid {
		_, msg := validation.Reason(s, true)
		assert.NotEmpty(t, msg, "reason %q", s)
	}
	for _, s := range v.IDLast4.Valid {
		assert.Empty(t, validation.IDLast4(s), s)
	}
	for _, s := range v.IDLast4.Invalid {
		assert.NotEmpty(t, validation.IDLast4(s), s)
	}
	for _, s := range v.Password.Valid {
		assert.Empty(t, validation.Password(s), s)
	}
	for _, s := range v.Password.Invalid {
		assert.NotEmpty(t, validation.Password(s), "password %q", s)
	}
	today := clock.Date(v.VisitDate.Today)
	for _, s := range v.VisitDate.Valid {
		_, msg := validation.VisitDate(s, today, v.VisitDate.MaxAdvanceDays)
		assert.Empty(t, msg, s)
	}
	for _, s := range v.VisitDate.Invalid {
		_, msg := validation.VisitDate(s, today, v.VisitDate.MaxAdvanceDays)
		assert.NotEmpty(t, msg, "date %q", s)
	}
}

func TestFieldsCollector(t *testing.T) {
	f := validation.New()
	f.Set("a", "")
	assert.NoError(t, f.Err())
	f.Set("a", "first")
	f.Set("a", "second")
	assert.Error(t, f.Err())
	assert.True(t, f.Has())
}
