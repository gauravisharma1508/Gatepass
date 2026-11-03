// Package mailer sends plain-text email over SMTP (MailHog in development).
package mailer

import (
	"context"
	"log/slog"
	"strings"
	"sync"

	gomail "github.com/wneessen/go-mail"
)

type Message struct {
	To      string
	Subject string
	Body    string
}

type Mailer interface {
	Send(ctx context.Context, m Message) error
}

type SMTP struct {
	host, from, user, pass string
	port                   int
}

func NewSMTP(host string, port int, from, user, pass string) *SMTP {
	return &SMTP{host: host, port: port, from: from, user: user, pass: pass}
}

func (s *SMTP) Send(ctx context.Context, m Message) error {
	msg := gomail.NewMsg()
	if err := msg.From(s.from); err != nil {
		return err
	}
	if err := msg.To(m.To); err != nil {
		return err
	}
	msg.Subject(m.Subject)
	msg.SetBodyString(gomail.TypeTextPlain, m.Body)

	opts := []gomail.Option{gomail.WithPort(s.port)}
	if s.user != "" {
		opts = append(opts, gomail.WithSMTPAuth(gomail.SMTPAuthPlain), gomail.WithUsername(s.user), gomail.WithPassword(s.pass),
			gomail.WithTLSPolicy(gomail.TLSMandatory))
	} else {
		opts = append(opts, gomail.WithTLSPolicy(gomail.NoTLS))
	}
	c, err := gomail.NewClient(s.host, opts...)
	if err != nil {
		return err
	}
	return c.DialAndSendWithContext(ctx, msg)
}

// Log writes mail to the log instead of sending it (used when no SMTP host is configured).
type Log struct{ Log *slog.Logger }

func (l Log) Send(_ context.Context, m Message) error {
	l.Log.Info("email (not sent: SMTP not configured)", "to", m.To, "subject", m.Subject)
	return nil
}

// Fake records messages; tests assert on them.
type Fake struct {
	mu   sync.Mutex
	msgs []Message
	Err  error
}

func (f *Fake) Send(_ context.Context, m Message) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.Err != nil {
		return f.Err
	}
	f.msgs = append(f.msgs, m)
	return nil
}

func (f *Fake) Messages() []Message {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]Message(nil), f.msgs...)
}

func (f *Fake) To(addr string) []Message {
	var out []Message
	for _, m := range f.Messages() {
		if strings.EqualFold(m.To, addr) {
			out = append(out, m)
		}
	}
	return out
}

func (f *Fake) Reset() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.msgs = nil
}
