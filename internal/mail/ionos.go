package mail

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/emersion/go-imap/v2/imapclient"
)

// Client appends drafts to an IMAP mailbox.
type Client struct {
	Host          string
	Port          int
	UseTLS        bool
	Username      string
	Password      string
	DraftsMailbox string
}

// SaveDraft appends d to the configured IONOS Drafts mailbox.
func (c Client) SaveDraft(ctx context.Context, d Draft) error {
	message, err := BuildMessage(d)
	if err != nil {
		return err
	}
	c = c.normalized()
	if err := c.validate(); err != nil {
		return err
	}
	var failures []string
	for _, mailbox := range c.draftMailboxCandidates() {
		for _, withDraftFlag := range []bool{true, false} {
			if err := c.appendDraft(ctx, mailbox, message, withDraftFlag); err != nil {
				failures = append(failures, err.Error())
				continue
			}
			return nil
		}
	}
	return fmt.Errorf("append draft failed for candidate mailboxes %s: %s", strings.Join(c.draftMailboxCandidates(), ", "), strings.Join(failures, "; "))
}

func (c Client) appendDraft(ctx context.Context, mailbox string, message []byte, withDraftFlag bool) error {
	conn, err := c.rawDial(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	} else {
		_ = conn.SetDeadline(time.Now().Add(45 * time.Second))
	}
	reader := bufio.NewReader(conn)
	if line, err := readIMAPLine(reader); err != nil {
		return fmt.Errorf("read imap greeting: %w", err)
	} else if !isUntaggedOK(line) {
		return fmt.Errorf("imap greeting: %s", line)
	}
	if err := writeIMAPCommand(conn, "A001 LOGIN %s %s", quoteIMAPString(c.Username), quoteIMAPString(c.Password)); err != nil {
		return fmt.Errorf("write imap login: %w", err)
	}
	if err := waitTaggedOK(reader, "A001"); err != nil {
		return fmt.Errorf("imap login for %s on %s:%d failed: %w", c.Username, c.Host, c.Port, err)
	}
	flags := ""
	if withDraftFlag {
		flags = " (\\Draft)"
	}
	date := quoteIMAPString(time.Now().Format("02-Jan-2006 15:04:05 -0700"))
	if err := writeIMAPCommand(conn, "A002 APPEND %s%s %s {%d}", mailboxName(mailbox), flags, date, len(message)); err != nil {
		return fmt.Errorf("write append command to %q: %w", mailbox, err)
	}
	if err := waitContinuation(reader, "A002"); err != nil {
		return fmt.Errorf("append draft to %q rejected before literal: %w", mailbox, err)
	}
	if _, err := conn.Write(message); err != nil {
		return fmt.Errorf("write draft to %q: %w", mailbox, err)
	}
	if _, err := conn.Write([]byte("\r\n")); err != nil {
		return fmt.Errorf("finish draft literal to %q: %w", mailbox, err)
	}
	if err := waitTaggedOK(reader, "A002"); err != nil {
		return fmt.Errorf("append draft to %q: %w", mailbox, err)
	}
	_ = writeIMAPCommand(conn, "A003 LOGOUT")
	return nil
}

// CheckLogin validates that the configured IMAP credentials can authenticate.
func (c Client) CheckLogin(ctx context.Context) error {
	c = c.normalized()
	if err := c.validate(); err != nil {
		return err
	}
	addr := net.JoinHostPort(c.Host, fmt.Sprint(c.Port))
	client, err := c.dial(ctx, addr)
	if err != nil {
		return err
	}
	defer client.Close()
	if err := c.login(client); err != nil {
		return err
	}
	defer client.Logout().Wait()
	return nil
}

func (c Client) normalized() Client {
	c.Host = strings.TrimSpace(c.Host)
	c.Username = strings.TrimSpace(c.Username)
	c.DraftsMailbox = strings.TrimSpace(c.DraftsMailbox)
	if c.Port == 0 {
		c.Port = 993
	}
	return c
}

func (c Client) validate() error {
	if c.Host == "" {
		return fmt.Errorf("imap host is required")
	}
	if c.Username == "" || c.Password == "" {
		return fmt.Errorf("imap username and password are required")
	}
	return nil
}

func (c Client) login(client *imapclient.Client) error {
	if err := client.Login(c.Username, c.Password).Wait(); err != nil {
		return fmt.Errorf("imap login for %s on %s:%d failed: %w. Use the full IONOS mailbox email address and the mailbox password, not the IONOS account/control-panel password", c.Username, c.Host, c.Port, err)
	}
	return nil
}

func (c Client) dial(ctx context.Context, addr string) (*imapclient.Client, error) {
	conn, err := c.rawDial(ctx)
	if err != nil {
		return nil, err
	}
	return imapclient.New(conn, nil), nil
}

func (c Client) rawDial(ctx context.Context) (net.Conn, error) {
	addr := net.JoinHostPort(c.Host, fmt.Sprint(c.Port))
	dialer := &net.Dialer{}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("dial imap: %w", err)
	}
	if c.UseTLS {
		tlsConn := tls.Client(conn, &tls.Config{ServerName: c.Host, MinVersion: tls.VersionTLS12})
		if err := tlsConn.HandshakeContext(ctx); err != nil {
			conn.Close()
			return nil, fmt.Errorf("imap tls handshake: %w", err)
		}
		conn = tlsConn
	}
	return conn, nil
}

func readIMAPLine(reader *bufio.Reader) (string, error) {
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}

func writeIMAPCommand(conn net.Conn, format string, args ...any) error {
	_, err := fmt.Fprintf(conn, format+"\r\n", args...)
	return err
}

func waitContinuation(reader *bufio.Reader, tag string) error {
	for {
		line, err := readIMAPLine(reader)
		if err != nil {
			return err
		}
		if strings.HasPrefix(line, "+") {
			return nil
		}
		if strings.HasPrefix(line, tag+" ") {
			return responseError(line)
		}
	}
}

func waitTaggedOK(reader *bufio.Reader, tag string) error {
	for {
		line, err := readIMAPLine(reader)
		if err != nil {
			return err
		}
		if !strings.HasPrefix(line, tag+" ") {
			continue
		}
		if taggedOK(line) {
			return nil
		}
		return responseError(line)
	}
}

func isUntaggedOK(line string) bool {
	upper := strings.ToUpper(line)
	return strings.HasPrefix(upper, "* OK") || strings.HasPrefix(upper, "* PREAUTH")
}

func taggedOK(line string) bool {
	parts := strings.SplitN(line, " ", 3)
	return len(parts) >= 2 && strings.EqualFold(parts[1], "OK")
}

func responseError(line string) error {
	parts := strings.SplitN(line, " ", 3)
	if len(parts) == 3 {
		return fmt.Errorf("%s", parts[2])
	}
	return fmt.Errorf("%s", line)
}

func quoteIMAPString(value string) string {
	value = strings.ReplaceAll(value, "\\", "\\\\")
	value = strings.ReplaceAll(value, "\"", "\\\"")
	return "\"" + value + "\""
}

func mailboxName(value string) string {
	if strings.EqualFold(value, "INBOX") {
		return "INBOX"
	}
	return quoteIMAPString(encodeKnownMailbox(value))
}

func encodeKnownMailbox(value string) string {
	// IMAP mailbox names are modified UTF-7. The app only needs the known
	// German IONOS drafts folder here; ASCII names pass through unchanged.
	if value == "Entwürfe" {
		return "Entw&APw-rfe"
	}
	return value
}

func (c Client) draftMailboxCandidates() []string {
	candidates := []string{
		"Entwürfe",
		c.DraftsMailbox,
		"Drafts",
		"INBOX.Entwürfe",
		"INBOX/Entwürfe",
		"INBOX.Drafts",
		"INBOX/Drafts",
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		key := strings.ToLower(candidate)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, candidate)
	}
	return out
}
