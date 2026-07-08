package mail

import (
	"io"
	"mime/quotedprintable"
	"strings"
	"testing"
	"time"
)

func TestBuildMessage(t *testing.T) {
	msg, err := BuildMessage(Draft{
		From:    "me@example.com",
		To:      []string{"team@example.com"},
		Subject: "Daily Report",
		Body:    "\n\n✅ Done\n- Fixed thing. [R2-1](https://teachinghero.atlassian.net/browse/R2-1)",
		Date:    time.Date(2026, 7, 5, 9, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	text := string(msg)
	for _, want := range []string{"From: me@example.com", "To: team@example.com", "Subject: Daily Report", "Content-Type: text/html; charset=UTF-8"} {
		if !strings.Contains(text, want) {
			t.Fatalf("message missing %q:\n%s", want, text)
		}
	}

	bodyStart := strings.Index(text, "\r\n\r\n")
	if bodyStart == -1 {
		t.Fatalf("message missing body separator:\n%s", text)
	}
	decoded, err := io.ReadAll(quotedprintable.NewReader(strings.NewReader(text[bodyStart+4:])))
	if err != nil {
		t.Fatal(err)
	}
	body := string(decoded)
	for _, want := range []string{"✅ Done", "<ul", "<li", "Fixed thing.", "https://teachinghero.atlassian.net/browse/R2-1", ">[R2-1]</a>", "padding:28px 40px", "background:transparent", "color:#0891b2"} {
		if !strings.Contains(body, want) {
			t.Fatalf("body missing %q:\n%s", want, body)
		}
	}
}
