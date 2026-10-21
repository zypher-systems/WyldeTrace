package mail

import (
	"bytes"
	"context"
	"log"
	"net"
	"net/smtp"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/wyldetechnologies/wyldetrace/internal/db"
)

const (
	SettingEnabled         = "smtp_enabled"
	SettingHost            = "smtp_host"
	SettingPort            = "smtp_port"
	SettingUser            = "smtp_user"
	SettingPass            = "smtp_pass"
	SettingFrom            = "smtp_from"
	SettingTo              = "smtp_to"
	SettingSubjectTemplate = "smtp_subject_template"
	SettingBodyTemplate    = "smtp_body_template"

	defaultSubjectTemplate = "[WyldeTrace] {{.Title}}"
	defaultBodyTemplate    = "{{.Title}}\n\n{{.Body}}\n\nEvent: {{.Event}}\nEntity: {{.EntityKey}}\n"
)

type AlertData struct {
	Event     string
	Title     string
	Body      string
	EntityKey string
}

type Sender struct {
	Store  *db.Store
	OpenFn func(ciphertext string) (string, error) // optional decrypt for smtp_pass
}

func New(store *db.Store) *Sender {
	return &Sender{Store: store}
}

func (s *Sender) Enabled(ctx context.Context) bool {
	return strings.EqualFold(strings.TrimSpace(s.setting(ctx, SettingEnabled, "false")), "true")
}

// SendAlert renders templates and delivers to configured recipients when SMTP is enabled.
func (s *Sender) SendAlert(ctx context.Context, data AlertData) {
	if s == nil || s.Store == nil || !s.Enabled(ctx) {
		return
	}
	host := strings.TrimSpace(s.setting(ctx, SettingHost, ""))
	from := strings.TrimSpace(s.setting(ctx, SettingFrom, ""))
	toRaw := strings.TrimSpace(s.setting(ctx, SettingTo, ""))
	if host == "" || from == "" || toRaw == "" {
		return
	}
	port := strings.TrimSpace(s.setting(ctx, SettingPort, "587"))
	if _, err := strconv.Atoi(port); err != nil {
		port = "587"
	}
	recipients := splitAddrs(toRaw)
	if len(recipients) == 0 {
		return
	}

	subjTmpl := s.setting(ctx, SettingSubjectTemplate, defaultSubjectTemplate)
	bodyTmpl := s.setting(ctx, SettingBodyTemplate, defaultBodyTemplate)
	subject, err := render(subjTmpl, data)
	if err != nil {
		log.Printf("mail: subject template: %v", err)
		subject = data.Title
	}
	body, err := render(bodyTmpl, data)
	if err != nil {
		log.Printf("mail: body template: %v", err)
		body = data.Body
	}

	user := strings.TrimSpace(s.setting(ctx, SettingUser, ""))
	pass := s.setting(ctx, SettingPass, "")
	if s.OpenFn != nil {
		if plain, err := s.OpenFn(pass); err == nil {
			pass = plain
		} else {
			log.Printf("mail: decrypt smtp_pass: %v", err)
			return
		}
	}
	addr := net.JoinHostPort(host, port)

	msg := buildMessage(from, recipients, subject, body)
	var auth smtp.Auth
	if user != "" {
		auth = smtp.PlainAuth("", user, pass, host)
	}

	sendCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	errCh := make(chan error, 1)
	go func() {
		errCh <- smtp.SendMail(addr, auth, from, recipients, msg)
	}()
	select {
	case <-sendCtx.Done():
		log.Printf("mail: send timeout for event %s", data.Event)
	case err := <-errCh:
		if err != nil {
			log.Printf("mail: send %s: %v", data.Event, err)
		}
	}
}

func (s *Sender) setting(ctx context.Context, key, fallback string) string {
	row, err := s.Store.Queries.GetSetting(ctx, key)
	if err != nil {
		return fallback
	}
	return row.Value
}

func splitAddrs(raw string) []string {
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n'
	})
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func render(tmpl string, data AlertData) (string, error) {
	t, err := template.New("smtp").Parse(tmpl)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func buildMessage(from string, to []string, subject, body string) []byte {
	var b strings.Builder
	b.WriteString("From: ")
	b.WriteString(from)
	b.WriteString("\r\n")
	b.WriteString("To: ")
	b.WriteString(strings.Join(to, ", "))
	b.WriteString("\r\n")
	b.WriteString("Subject: ")
	b.WriteString(sanitizeHeader(subject))
	b.WriteString("\r\n")
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	b.WriteString("\r\n")
	b.WriteString(body)
	if !strings.HasSuffix(body, "\n") {
		b.WriteString("\r\n")
	}
	return []byte(b.String())
}

func sanitizeHeader(s string) string {
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	return s
}
