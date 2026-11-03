// Package reports exports the visit report as CSV (admin only, FR-8.4).
package reports

import (
	"encoding/csv"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"gatepass/internal/modules/visits"
	"gatepass/internal/platform/authctx"
	"gatepass/internal/platform/clock"
	"gatepass/internal/platform/httpx"
)

type Handler struct {
	visits *visits.Service
	loc    *time.Location
}

func NewHandler(v *visits.Service) *Handler { return &Handler{visits: v, loc: clock.OfficeLocation()} }

func (h *Handler) Routes(r chi.Router) {
	r.With(httpx.RequireRoles(authctx.RoleAdmin)).Get("/reports/visits.csv", httpx.H(h.csv))
}

// safe stops spreadsheet formula injection: cells starting with = + - @ get a leading quote.
func safe(s string) string {
	if s != "" && (s[0] == '=' || s[0] == '+' || s[0] == '-' || s[0] == '@' || s[0] == '\t' || s[0] == '\r') {
		return "'" + s
	}
	return s
}

func (h *Handler) ts(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.In(h.loc).Format("2006-01-02 15:04")
}

func (h *Handler) csv(w http.ResponseWriter, r *http.Request) error {
	p, err := authctx.Require(r.Context())
	if err != nil {
		return err
	}
	f, err := visits.ParseFilter(r)
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="gatepass-visits.csv"`)
	cw := csv.NewWriter(w)
	_ = cw.Write([]string{"Visit ID", "Type", "Status", "Visitor", "Mobile", "Company", "Host", "Department", "Purpose",
		"Visit date", "Expected duration (min)", "Created (IST)", "Checked in (IST)", "Meeting over (IST)", "Checked out (IST)", "Closed reason"})
	err = h.visits.Export(r.Context(), p, f, func(x visits.ExportRow) error {
		created := x.CreatedAt
		return cw.Write([]string{safe(x.Code), x.Type, x.Status, safe(x.Visitor), safe(x.Mobile), safe(x.Company), safe(x.Host), safe(x.Department),
			safe(x.Purpose), string(x.VisitDate), itoa(x.DurationMin), h.ts(&created), h.ts(x.CheckedInAt), h.ts(x.MeetingOverAt),
			h.ts(x.CheckedOutAt), safe(x.ClosedReason)})
	})
	cw.Flush()
	return err
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
