package clock

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TC-SYS-002: "today" follows the office time zone, not UTC.
func TestTodayUsesOfficeZone_TC_SYS_002(t *testing.T) {
	c := New()
	c.Set(time.Date(2026, 11, 3, 18, 31, 0, 0, time.UTC)) // 00:01 on 4 Nov in IST
	assert.Equal(t, Date("2026-11-04"), c.Today())
	c.Set(time.Date(2026, 11, 3, 18, 29, 0, 0, time.UTC)) // 23:59 on 3 Nov in IST
	assert.Equal(t, Date("2026-11-03"), c.Today())
}

func TestFrozenAndAdvance(t *testing.T) {
	c := New()
	start := time.Date(2026, 11, 3, 4, 30, 0, 0, time.UTC)
	c.Set(start)
	c.Advance(31 * time.Minute)
	assert.Equal(t, start.Add(31*time.Minute), c.Now())
	c.Unfreeze()
	assert.WithinDuration(t, time.Now(), c.Now(), time.Second)
}

func TestDateHelpers(t *testing.T) {
	d, err := ParseDate("2026-02-28")
	require.NoError(t, err)
	assert.Equal(t, Date("2026-03-01"), d.AddDays(1))
	assert.Equal(t, Date("2026-02-27"), d.AddDays(-1))
	_, err = ParseDate("2026-13-01")
	assert.Error(t, err)
	var s Date
	require.NoError(t, s.Scan(time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC)))
	assert.Equal(t, Date("2026-11-03"), s)
	require.NoError(t, s.Scan("2026-11-04"))
	assert.Equal(t, Date("2026-11-04"), s)
	var tod TimeOfDay
	require.NoError(t, tod.Scan("09:30:00"))
	assert.Equal(t, TimeOfDay("09:30"), tod)
}
