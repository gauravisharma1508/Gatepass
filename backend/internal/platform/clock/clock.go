// Package clock makes time injectable so the 30-minute window, no-show and
// overstay rules can be tested without waiting (spec §6.3).
package clock

import (
	"database/sql/driver"
	"fmt"
	"sync"
	"time"
	_ "time/tzdata" // office time zone must work in a distroless image
)

const OfficeZone = "Asia/Kolkata"

func OfficeLocation() *time.Location {
	loc, err := time.LoadLocation(OfficeZone)
	if err != nil {
		return time.FixedZone("IST", 5*3600+1800)
	}
	return loc
}

type Clock interface {
	Now() time.Time // always UTC
	Today() Date    // calendar date in the office time zone
}

// Settable follows the real time until Set freezes it (tests, test-only endpoint).
type Settable struct {
	mu    sync.Mutex
	fixed *time.Time
	loc   *time.Location
}

func New() *Settable { return &Settable{loc: OfficeLocation()} }

func (c *Settable) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.fixed != nil {
		return *c.fixed
	}
	return time.Now().UTC()
}

func (c *Settable) Today() Date { return DateOf(c.Now().In(c.loc)) }

// Set freezes the clock at t.
func (c *Settable) Set(t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	u := t.UTC()
	c.fixed = &u
}

// Advance moves a frozen clock forward (freezes at the current time first if needed).
func (c *Settable) Advance(d time.Duration) {
	c.Set(c.Now().Add(d))
}

// Unfreeze goes back to real time.
func (c *Settable) Unfreeze() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.fixed = nil
}

// Date is a calendar date "YYYY-MM-DD" (office time zone). The string form sorts correctly.
type Date string

const dateLayout = "2006-01-02"

func DateOf(t time.Time) Date { return Date(t.Format(dateLayout)) }

func ParseDate(s string) (Date, error) {
	t, err := time.Parse(dateLayout, s)
	if err != nil {
		return "", fmt.Errorf("invalid date %q", s)
	}
	return DateOf(t), nil
}

func (d Date) AddDays(n int) Date {
	t, err := time.Parse(dateLayout, string(d))
	if err != nil {
		return d
	}
	return DateOf(t.AddDate(0, 0, n))
}

func (d Date) String() string { return string(d) }

func (d *Date) Scan(src any) error {
	switch v := src.(type) {
	case nil:
		*d = ""
	case time.Time:
		*d = DateOf(v)
	case string:
		if len(v) >= 10 {
			*d = Date(v[:10])
		}
	case []byte:
		if len(v) >= 10 {
			*d = Date(v[:10])
		}
	default:
		return fmt.Errorf("cannot scan %T into Date", src)
	}
	return nil
}

func (d Date) Value() (driver.Value, error) {
	if d == "" {
		return nil, nil
	}
	return string(d), nil
}

func (Date) GormDataType() string { return "date" }

// TimeOfDay is "HH:MM".
type TimeOfDay string

func (t *TimeOfDay) Scan(src any) error {
	switch v := src.(type) {
	case nil:
		*t = ""
	case time.Time:
		*t = TimeOfDay(v.Format("15:04"))
	case string:
		if len(v) >= 5 {
			*t = TimeOfDay(v[:5])
		}
	case []byte:
		if len(v) >= 5 {
			*t = TimeOfDay(string(v[:5]))
		}
	default:
		return fmt.Errorf("cannot scan %T into TimeOfDay", src)
	}
	return nil
}

func (t TimeOfDay) Value() (driver.Value, error) {
	if t == "" {
		return nil, nil
	}
	return string(t), nil
}

func (TimeOfDay) GormDataType() string { return "time" }
