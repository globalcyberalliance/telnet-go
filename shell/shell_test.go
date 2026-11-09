package shell

import (
	"bytes"
	"net"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/globalcyberalliance/telnet-go"
)

// serve starts the example shell on a loopback listener and returns its address.
func serve(t *testing.T) string {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	server := &telnet.Server{}
	shell := &Server{
		AuthHandler: NewAuthHandler("root", "root", 3),
		Commands:    []Command{{Regex: "^whoami$", Response: "root\r\n"}},
	}
	server.Handler = shell.HandlerFunc

	go func() { _ = server.Serve(listener) }()

	t.Cleanup(func() { _ = server.Shutdown() })

	return listener.Addr().String()
}

// readTimeout bounds how long a read waits for what it is expecting, generously enough for a loaded machine.
const readTimeout = 10 * time.Second

// readUntil reads until every wanted string has arrived, or fails once the timeout is up. Waiting on the
// content rather than on a pause keeps the test from racing a slow server.
func readUntil(t *testing.T, conn net.Conn, want ...string) string {
	t.Helper()

	var out bytes.Buffer

	buffer := make([]byte, 4096)
	deadline := time.Now().Add(readTimeout)

	for {
		if got := out.String(); !slices.ContainsFunc(want, func(w string) bool { return !strings.Contains(got, w) }) {
			return got
		}

		if err := conn.SetReadDeadline(deadline); err != nil {
			t.Fatalf("deadline: %v", err)
		}

		n, err := conn.Read(buffer)
		if n > 0 {
			out.Write(buffer[:n])
		}

		if err != nil {
			t.Fatalf("waiting for %q: %v (got %q)", want, err, out.String())
		}
	}
}

// TestHandlerFuncLineEndings drives the example with each of the line endings a client may send. CR NUL is
// what a client in character-at-a-time mode sends for ENTER, which the server offering WILL ECHO to mask the
// password asks it to be in; before ReadLine understood it, the login never completed.
func TestHandlerFuncLineEndings(t *testing.T) {
	t.Parallel()

	for name, ending := range map[string]string{
		"CR LF":  "\r\n",
		"CR NUL": "\r\x00",
		"LF":     "\n",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			conn, err := net.Dial("tcp", serve(t))
			if err != nil {
				t.Fatalf("dial: %v", err)
			}

			defer func() { _ = conn.Close() }()

			readUntil(t, conn, "Login: ")

			for _, line := range []string{"root", "root", "whoami"} {
				if _, err = conn.Write([]byte(line + ending)); err != nil {
					t.Fatalf("write %q: %v", line, err)
				}
			}

			// Reaching the welcome means both the username and the password were read as whole lines, and
			// the answer after it means the command was too.
			readUntil(t, conn, "Welcome!", "root\r\n")
		})
	}
}

// TestHandlerFuncSurvivesInterrupt checks that the two-byte commands a real client sends mid-session, such
// as the IAC IP it sends for Ctrl-C, do not end the connection.
func TestHandlerFuncSurvivesInterrupt(t *testing.T) {
	t.Parallel()

	conn, err := net.Dial("tcp", serve(t))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}

	defer func() { _ = conn.Close() }()

	readUntil(t, conn, "Login: ")

	for _, line := range []string{"root", "root"} {
		if _, err = conn.Write([]byte(line + "\r\n")); err != nil {
			t.Fatalf("write: %v", err)
		}
	}

	readUntil(t, conn, "Welcome!")

	// Ctrl-C, then a liveness check, then a command that must still be answered.
	if _, err = conn.Write([]byte{telnet.IAC, telnet.IP, telnet.IAC, telnet.AYT}); err != nil {
		t.Fatalf("write commands: %v", err)
	}

	if _, err = conn.Write([]byte("whoami\r\n")); err != nil {
		t.Fatalf("write whoami: %v", err)
	}

	readUntil(t, conn, "root\r\n")
}
