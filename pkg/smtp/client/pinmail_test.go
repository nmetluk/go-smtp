package client

import (
	"crypto/tls"
	"fmt"
	"io"
	"testing"

	"github.com/rest-mail/go-smtp/pkg/smtp/server"
)

func TestPublicStartTLSAfterCustomHello(t *testing.T) {
	hook := testHookStartTLS
	testHookStartTLS = nil
	t.Cleanup(func() { testHookStartTLS = hook })
	names := make(chan string, 4)
	addr, mails := rfc3207PolicyServer(t, func(s *server.Server, be *tlsPolicyBackend) {
		s.TLSConfig = rfc3207TLSConfig(t)
		be.requireTLS = func(c *server.Conn) bool { names <- c.Hostname(); return true }
	})
	c, err := Dial(addr)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if err = c.Hello("mx.pinmail.test"); err != nil {
		t.Fatal(err)
	}
	if got := <-names; got != "mx.pinmail.test" {
		t.Fatalf("имя до TLS: %q", got)
	}
	if err = c.StartTLS(&tls.Config{InsecureSkipVerify: true}); err != nil {
		t.Fatal(err)
	}
	if err = rfc3207Send(c); err != nil {
		t.Fatal(err)
	}
	if got := <-names; got != "mx.pinmail.test" {
		t.Fatalf("имя после TLS: %q", got)
	}
	if secure := <-mails; !secure {
		t.Fatal("письмо ушло без TLS")
	}
}

func TestDataResponsePreservesCode(t *testing.T) {
	for _, code := range []int{250, 251, 259} {
		t.Run(fmt.Sprint(code), func(t *testing.T) {
			text := "2.0.0 точный текст  с пробелами"
			c, _ := newScriptedClient("220 mx\r\n250 mx\r\n250 ok\r\n250 ok\r\n354 send\r\n" + fmt.Sprintf("%d %s\r\n", code, text))
			defer c.Close()
			if err := c.Mail("a@example.test", nil); err != nil {
				t.Fatal(err)
			}
			if err := c.Rcpt("b@example.test", nil); err != nil {
				t.Fatal(err)
			}
			data, err := c.Data()
			if err != nil {
				t.Fatal(err)
			}
			if _, err = io.WriteString(data, "body\r\n"); err != nil {
				t.Fatal(err)
			}
			response, err := data.CloseWithResponse()
			if err != nil {
				t.Fatal(err)
			}
			if response.Code != code || response.StatusText != text {
				t.Fatalf("ответ: %+v", response)
			}
		})
	}
}
