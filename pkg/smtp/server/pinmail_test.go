package server

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"github.com/rest-mail/go-smtp/pkg/smtp"
	"io"
	"strings"
	"testing"
	"time"
)

// checklist: CHUNK-EHLO-01 CHUNK-EHLO-02 BINARY-BODY-01
func TestDisableCHUNKINGCapabilitiesAndMail(t *testing.T) {
	for _, disabled := range []bool{false, true} {
		for _, binary := range []bool{false, true} {
			t.Run(fmt.Sprintf("disabled=%v/binary=%v", disabled, binary), func(t *testing.T) {
				_, c, br := newConformanceServer(t, func(s *Server) { s.DisableCHUNKING = disabled; s.EnableBINARYMIME = binary })
				c.SetDeadline(time.Now().Add(3 * time.Second))
				writeCmd(t, c, "EHLO sender.test\r\n")
				caps := make(map[string]bool)
				for {
					line := readSMTPLine(t, br)
					caps[line[4:]] = true
					if strings.HasPrefix(line, "250 ") {
						break
					}
				}
				if caps["CHUNKING"] == disabled || caps["BINARYMIME"] != (binary && !disabled) {
					t.Fatalf("capabilities: %v", caps)
				}
				if !caps["8BITMIME"] {
					t.Fatal("изменена несвязанная возможность")
				}
				writeCmd(t, c, "MAIL FROM:<sender@example.test> BODY=BINARYMIME\r\n")
				want := "504 5.5.4"
				if binary && !disabled {
					want = "250 "
				}
				if line := readSMTPLine(t, br); !strings.HasPrefix(line, want) {
					t.Fatalf("MAIL: %s", line)
				}
			})
		}
	}
}

// checklist: SMTP-CMD-07
func TestDisableCHUNKINGRejectsBDAT(t *testing.T) {
	for _, payload := range []string{"BDAT 0 LAST\r\n", "BDAT 12 LAST\r\nQUIT\r\nNOOP\r\n", "BDAT invalid\r\n"} {
		t.Run(strings.Split(payload, "\r\n")[0], func(t *testing.T) {
			_, c, br := newConformanceServer(t, func(s *Server) { s.DisableCHUNKING = true })
			c.SetDeadline(time.Now().Add(3 * time.Second))
			drainEHLO(t, c, br)
			writeCmd(t, c, "MAIL FROM:<sender@example.test>\r\nRCPT TO:<rcpt@example.test>\r\n")
			readSMTPLine(t, br)
			readSMTPLine(t, br)
			writeCmd(t, c, payload)
			if line := readSMTPLine(t, br); line != "502 5.5.1 CHUNKING is disabled" {
				t.Fatalf("BDAT: %s", line)
			}
			// Непрочитанные байты отключённого BDAT не должны стать командами SMTP.
			if _, err := br.ReadByte(); err != io.EOF {
				t.Fatalf("соединение не закрыто: %v", err)
			}
		})
	}
}

// checklist: CHUNK-DATA-01
func TestDefaultCHUNKINGStillDelivers(t *testing.T) {
	delivered := make(chan []byte, 1)
	_, c, br := newConformanceServer(t, func(s *Server) {
		s.Backend = BackendFunc(func(*Conn) (Session, error) {
			return rfc6152Session{data: delivered, mail: func(string, *smtp.MailOptions) error { return nil }}, nil
		})
	})
	c.SetDeadline(time.Now().Add(3 * time.Second))
	drainEHLO(t, c, br)
	writeCmd(t, c, "MAIL FROM:<sender@example.test>\r\nRCPT TO:<rcpt@example.test>\r\n")
	readSMTPLine(t, br)
	readSMTPLine(t, br)
	writeCmd(t, c, "BDAT 6 LAST\r\nbody\r\n")
	if line := readSMTPLine(t, br); !strings.HasPrefix(line, "250 ") {
		t.Fatalf("BDAT по умолчанию: %s", line)
	}
	if got := string(<-delivered); got != "body\r\n" {
		t.Fatalf("тело BDAT: %q", got)
	}
}

type pinmailFeatures struct{ nopBackend }

func (pinmailFeatures) Features(*Conn) []string { return []string{"binarymime", "X-PINMAIL-TEST"} }

// checklist: CHUNK-EHLO-01 CHUNK-EHLO-02
func TestDisableCHUNKINGOverridesCustomCapabilities(t *testing.T) {
	_, c, br := newConformanceServer(t, func(s *Server) {
		s.DisableCHUNKING = true
		s.ExtraCaps = []string{"chunking"}
		s.Backend = pinmailFeatures{}
	})
	c.SetDeadline(time.Now().Add(3 * time.Second))
	writeCmd(t, c, "EHLO sender.test\r\n")
	found := false
	for {
		line := readSMTPLine(t, br)
		upper := strings.ToUpper(line)
		if strings.Contains(upper, "CHUNKING") || strings.Contains(upper, "BINARYMIME") {
			t.Fatalf("возможность снова включена: %s", line)
		}
		if strings.Contains(line, "X-PINMAIL-TEST") {
			found = true
		}
		if strings.HasPrefix(line, "250 ") {
			break
		}
	}
	if !found {
		t.Fatal("потеряна несвязанная возможность backend")
	}
}

// checklist: CHUNK-EHLO-01 SMTP-CMD-07
func TestDisableCHUNKINGAfterSTARTTLS(t *testing.T) {
	_, c, br := newConformanceServer(t, func(s *Server) {
		s.DisableCHUNKING = true
		s.EnableBINARYMIME = true
		s.TLSConfig = rfc3207TLSConfig(t)
	})
	c.SetDeadline(time.Now().Add(3 * time.Second))
	drainEHLO(t, c, br)
	writeCmd(t, c, "STARTTLS\r\n")
	if line := readSMTPLine(t, br); !strings.HasPrefix(line, "220 ") {
		t.Fatal(line)
	}
	secure := tls.Client(c, &tls.Config{InsecureSkipVerify: true})
	if err := secure.Handshake(); err != nil {
		t.Fatal(err)
	}
	br = bufio.NewReader(secure)
	writeCmd(t, secure, "EHLO sender.test\r\n")
	for {
		line := readSMTPLine(t, br)
		if strings.Contains(line, "CHUNKING") || strings.Contains(line, "BINARYMIME") {
			t.Fatal(line)
		}
		if strings.HasPrefix(line, "250 ") {
			break
		}
	}
	writeCmd(t, secure, "BDAT 0 LAST\r\n")
	if line := readSMTPLine(t, br); !strings.HasPrefix(line, "502 5.5.1 ") {
		t.Fatal(line)
	}
}
