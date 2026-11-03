// Package validation holds the field rules of spec §8.4. The same inputs are
// checked in the web app with Zod; both read testdata/validation_vectors.json.
package validation

import (
	"net/mail"
	"regexp"
	"strings"
	"unicode/utf8"

	"gatepass/internal/platform/apperr"
	"gatepass/internal/platform/clock"
)

// Fields collects per-field messages and turns them into one 400 error.
type Fields struct{ m map[string]string }

func New() *Fields { return &Fields{m: map[string]string{}} }

// Set records msg for field when msg is not empty (first message wins).
func (f *Fields) Set(field, msg string) {
	if msg == "" {
		return
	}
	if _, ok := f.m[field]; !ok {
		f.m[field] = msg
	}
}

func (f *Fields) Has() bool { return len(f.m) > 0 }

func (f *Fields) Err() error {
	if len(f.m) == 0 {
		return nil
	}
	return apperr.Validation(f.m)
}

var (
	nameRe   = regexp.MustCompile(`^\p{L}[\p{L} .'\-]*$`)
	mobileRe = regexp.MustCompile(`^(?:\+91|0)?([6-9][0-9]{9})$`)
	last4Re  = regexp.MustCompile(`^[A-Za-z0-9]{4}$`)
	timeRe   = regexp.MustCompile(`^([01][0-9]|2[0-3]):[0-5][0-9]$`)
	codeRe   = regexp.MustCompile(`^[A-Za-z0-9\-]{3,20}$`)
)

// Name: trimmed, 2–100 chars, letters, spaces and . ' - only.
func Name(s string) (string, string) {
	s = strings.TrimSpace(s)
	n := utf8.RuneCountInString(s)
	switch {
	case s == "":
		return s, "Required"
	case n < 2 || n > 100:
		return s, "Must be 2–100 characters"
	case !nameRe.MatchString(s):
		return s, "Only letters, spaces and . ' - are allowed"
	}
	return s, ""
}

// Mobile: Indian mobile number, normalised to +91XXXXXXXXXX.
func Mobile(s string) (string, string) {
	s = strings.NewReplacer(" ", "", "-", "").Replace(strings.TrimSpace(s))
	if s == "" {
		return s, "Required"
	}
	m := mobileRe.FindStringSubmatch(s)
	if m == nil {
		return s, "Enter a valid 10-digit Indian mobile number"
	}
	return "+91" + m[1], ""
}

// OptionalEmail returns the trimmed email ("" is fine).
func OptionalEmail(s string) (string, string) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", ""
	}
	if len(s) > 254 {
		return s, "Too long"
	}
	a, err := mail.ParseAddress(s)
	if err != nil || a.Address != s || !strings.Contains(s[strings.LastIndex(s, "@"):], ".") {
		return s, "Enter a valid email address"
	}
	return s, ""
}

func Email(s string) (string, string) {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return s, "Required"
	}
	return s, mustMsg(OptionalEmail(s))
}

func mustMsg(_ string, msg string) string { return msg }

// Text: trimmed, min..max characters; min 0 means optional.
func Text(s string, min, max int) (string, string) {
	s = strings.TrimSpace(s)
	n := utf8.RuneCountInString(s)
	switch {
	case n == 0 && min > 0:
		return s, "Required"
	case n > 0 && n < min:
		return s, "Too short"
	case n > max:
		return s, "Too long"
	}
	return s, ""
}

// Duration: 30–480 minutes, multiple of 15.
func Duration(n int) string {
	if n < 30 || n > 480 || n%15 != 0 {
		return "Must be between 30 and 480 minutes, in steps of 15"
	}
	return ""
}

// Arrival: optional HH:MM.
func Arrival(s string) string {
	if s == "" || timeRe.MatchString(s) {
		return ""
	}
	return "Use HH:MM (24-hour)"
}

// VisitDate: today ≤ date ≤ today + maxAdvanceDays.
func VisitDate(s string, today clock.Date, maxAdvanceDays int) (clock.Date, string) {
	d, err := clock.ParseDate(s)
	if err != nil {
		return "", "Enter a valid date"
	}
	if d < today {
		return d, "Visit date cannot be in the past"
	}
	if d > today.AddDays(maxAdvanceDays) {
		return d, "Visit date is too far ahead"
	}
	return d, ""
}

// Reason: trimmed; required reasons need 5–250 characters, optional ones ≤ 250.
func Reason(s string, required bool) (string, string) {
	if required {
		return Text(s, 5, 250)
	}
	return Text(s, 0, 250)
}

func IDLast4(s string) string {
	if s == "" || last4Re.MatchString(s) {
		return ""
	}
	return "Enter exactly 4 letters or digits"
}

func EmployeeCode(s string) (string, string) {
	s = strings.TrimSpace(s)
	if s == "" {
		return s, "Required"
	}
	if !codeRe.MatchString(s) {
		return s, "3–20 letters, digits or -"
	}
	return s, ""
}

// Password: 8–72 bytes (bcrypt limit), at least one letter and one digit.
func Password(s string) string {
	if len(s) < 8 || len(s) > 72 {
		return "Must be 8–72 characters"
	}
	letter, digit := false, false
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9':
			digit = true
		case (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r > 127:
			letter = true
		}
	}
	if !letter || !digit {
		return "Must contain at least one letter and one digit"
	}
	return ""
}
