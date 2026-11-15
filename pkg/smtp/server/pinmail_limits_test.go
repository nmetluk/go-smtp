//go:debug asynctimerchan=0

package server

import (
	"fmt"
	"strings"
	"testing"
	"testing/synctest"
	"time"
)

// checklist: SMTP-SYN-02 SMTP-SYN-13
func TestPinmailCommandControls(t *testing.T) {
	for _, cmd := range []string{"HELO x%s.test", "EHLO x%s.test", "MAIL FROM:<x%s@example.test>", "RCPT TO:<x%s@example.test>"} {
		for ch := 0; ch < 33; ch++ {
			if ch == 32 {
				ch = 127
			}
			t.Run(fmt.Sprintf("%.4s/%02x", cmd, ch), func(t *testing.T) {
				_, c, br, _ := newFlushTestConn(t, func(s *Server) { s.RejectCommandControls = true })
				writeFlushBatch(t, c, "EHLO client.test\r\n")
				readFlushReply(t, br)
				writeFlushBatch(t, c, "MAIL FROM:<a@example.test>\r\n")
				readFlushReply(t, br)
				writeFlushBatch(t, c, fmt.Sprintf(cmd, string(byte(ch)))+"\r\n")
				for _, line := range readFlushReply(t, br) {
					if !strings.HasPrefix(line, "501 ") {
						t.Fatalf("ответ: %q", line)
					}
					for _, b := range []byte(line) {
						if b < 32 || b == 127 {
							t.Fatalf("управляющий байт в ответе: %q", line)
						}
					}
				}
			})
		}
	}
}

// checklist: SMTP-LIM-05
func TestPinmailSessionAndIdleLimits(t *testing.T) {
	for _, session := range []bool{false, true} {
		t.Run(fmt.Sprint(session), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				_, c, br, done := newFlushTestConn(t, func(s *Server) {
					if session {
						s.SessionTimeout = time.Minute
					} else {
						s.ReadTimeout = time.Minute
					}
				})
				c.SetDeadline(time.Now().Add(2 * time.Minute))
				if session {
					time.Sleep(30 * time.Second)
					writeFlushBatch(t, c, "NOOP\r\n")
					readFlushReply(t, br)
				}
				time.Sleep(time.Minute)
				reply := readFlushReply(t, br)
				if !strings.HasPrefix(reply[0], "421 ") {
					t.Fatal(reply)
				}
				if err := <-done; err != nil {
					t.Fatal(err)
				}
			})
		})
	}
}
