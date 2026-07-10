// Package mail saves generated daily reports as IONOS drafts.
package mail

import (
	"bytes"
	"fmt"
	"html"
	"mime"
	"mime/quotedprintable"
	"regexp"
	"strings"
	"time"
)

var markdownIssueLinkRE = regexp.MustCompile(`\[([A-Z][A-Z0-9]+-\d+)\]\((https?://[^)]+)\)`)

// Draft is an email draft to be appended through IMAP.
type Draft struct {
	From    string
	To      []string
	CC      []string
	Subject string
	Body    string
	Date    time.Time
}

// BuildMessage builds a UTF-8 HTML MIME message suitable for IMAP APPEND.
func BuildMessage(d Draft) ([]byte, error) {
	if d.From == "" {
		return nil, fmt.Errorf("draft from address is required")
	}
	if len(d.To) == 0 {
		return nil, fmt.Errorf("draft recipient is required")
	}
	if d.Date.IsZero() {
		d.Date = time.Now()
	}
	var b bytes.Buffer
	writeHeader(&b, "From", d.From)
	writeHeader(&b, "To", strings.Join(d.To, ", "))
	if len(d.CC) > 0 {
		writeHeader(&b, "Cc", strings.Join(d.CC, ", "))
	}
	writeHeader(&b, "Subject", mime.QEncoding.Encode("utf-8", d.Subject))
	writeHeader(&b, "Date", d.Date.Format(time.RFC1123Z))
	writeHeader(&b, "MIME-Version", "1.0")
	writeHeader(&b, "Content-Type", "text/html; charset=UTF-8")
	writeHeader(&b, "Content-Transfer-Encoding", "quoted-printable")
	b.WriteString("\r\n")
	body := normalizeCRLF(htmlBody(d.Body))
	qp := quotedprintable.NewWriter(&b)
	if _, err := qp.Write([]byte(body)); err != nil {
		return nil, fmt.Errorf("encode quoted-printable body: %w", err)
	}
	if err := qp.Close(); err != nil {
		return nil, fmt.Errorf("close quoted-printable body: %w", err)
	}
	return b.Bytes(), nil
}

func htmlBody(body string) string {
	body = strings.TrimLeft(body, "\r\n \t")
	lines := strings.SplitSeq(strings.ReplaceAll(body, "\r\n", "\n"), "\n")
	var b strings.Builder
	b.WriteString(`<html><head><style>html,body{margin:0!important;padding:0!important;background:transparent!important;}a{color:#0891b2!important;text-decoration:underline!important;}</style></head><body style="margin:0!important;padding:0!important;background:transparent!important;font-family:Arial,Helvetica,sans-serif;font-size:14px;line-height:1.45;color:#2f3137;"><div style="margin:0!important;padding:28px 40px!important;background:transparent!important;">`)
	inList := false
	for line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if item, ok := reportListItem(trimmed); ok {
			if !inList {
				b.WriteString(`<ul style="margin:0 0 18px 28px;padding:0;">`)
				inList = true
			}
			fmt.Fprintf(&b, `<li style="margin:0 0 6px 0;padding-left:4px;">%s</li>`, linkedHTMLEscape(normalizeListItem(item)))
			continue
		}
		if inList {
			b.WriteString(`</ul>`)
			inList = false
		}
		writeHTMLReportBlock(&b, trimmed)
	}
	if inList {
		b.WriteString(`</ul>`)
	}
	b.WriteString("</div></body></html>")
	return b.String()
}

func writeHTMLReportBlock(b *strings.Builder, line string) {
	switch {
	case strings.HasPrefix(line, "📍"):
		fmt.Fprintf(b, `<div style="margin:0 0 18px 0;padding:0;font-weight:700;">%s</div>`, linkedHTMLEscape(normalizeProjectHeading(line)))
	case isReportHeading(line):
		fmt.Fprintf(b, `<div style="margin:18px 0 8px 0;padding:0;font-weight:700;">%s</div>`, linkedHTMLEscape(normalizeReportHeading(line)))
	default:
		fmt.Fprintf(b, `<div style="margin:0 0 8px 0;padding:0;">%s</div>`, linkedHTMLEscape(line))
	}
}

func isReportHeading(line string) bool {
	return strings.HasPrefix(line, "✅") || strings.HasPrefix(line, "🚧") || strings.HasPrefix(line, "❌") || strings.HasPrefix(line, "📝") || strings.HasPrefix(line, "🚦")
}

func reportListItem(line string) (string, bool) {
	for _, marker := range []string{"- ", "• "} {
		if item, ok := strings.CutPrefix(line, marker); ok {
			return strings.TrimSpace(item), true
		}
	}
	return "", false
}

func normalizeListItem(item string) string {
	if strings.EqualFold(strings.Trim(strings.TrimSpace(item), ".!?"), "None") {
		return "None at the moment."
	}
	return item
}

func normalizeProjectHeading(line string) string {
	line = strings.TrimSpace(strings.TrimPrefix(line, "📍"))
	return strings.ReplaceAll(line, " - ", " – ")
}

func normalizeReportHeading(line string) string {
	switch line {
	case "🚧 Work In-Progress":
		return "🚧 In Progress"
	case "❌ Blockers/Pain points":
		return "❌ Blockers"
	case "📝 TODO Next":
		return "📝 Next Step"
	}
	if rest, ok := strings.CutPrefix(line, "🚦"); ok {
		return "🚦 " + strings.TrimSpace(rest)
	}
	return strings.TrimSpace(line)
}

func linkedHTMLEscape(line string) string {
	matches := markdownIssueLinkRE.FindAllStringSubmatchIndex(line, -1)
	if len(matches) == 0 {
		return html.EscapeString(line)
	}
	var b strings.Builder
	pos := 0
	for _, match := range matches {
		b.WriteString(html.EscapeString(line[pos:match[0]]))
		key := line[match[2]:match[3]]
		url := line[match[4]:match[5]]
		fmt.Fprintf(&b, `<a href="%s" style="color:#0891b2!important;text-decoration:underline!important;font-weight:600;">[%s]</a>`, html.EscapeString(url), html.EscapeString(key))
		pos = match[1]
	}
	b.WriteString(html.EscapeString(line[pos:]))
	return b.String()
}

func normalizeCRLF(body string) string {
	body = strings.ReplaceAll(body, "\r\n", "\n")
	return strings.ReplaceAll(body, "\n", "\r\n")
}

func writeHeader(b *bytes.Buffer, key, value string) {
	value = strings.ReplaceAll(value, "\r", "")
	value = strings.ReplaceAll(value, "\n", "")
	b.WriteString(key)
	b.WriteString(": ")
	b.WriteString(value)
	b.WriteString("\r\n")
}
