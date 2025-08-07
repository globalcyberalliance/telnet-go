package telnet

import (
	"net"
	"testing"
	"time"
)

// TODO: refactor entire client, and tests.

// testHandler stores the session for later inspection in the test.
type testHandler struct {
	lastSession *Session
	done        chan struct{}
}

func (h *testHandler) ServeTELNETSession(session *Session) {
	defer close(h.done)
	// Wait for NAWS negotiation.
	_ = session.RequestWindowSize()
	h.lastSession = session
}

// scanForNAWS scans the connection for the IAC DO NAWS sequence, up to a timeout.
func scanForNAWS(t *testing.T, conn net.Conn, timeout time.Duration) {
	buf := make([]byte, 3)
	deadline := time.Now().Add(timeout)
	for {
		if err := conn.SetReadDeadline(deadline); err != nil {
			t.Fatalf("failed to set read deadline: %v", err)
		}

		if _, err := conn.Read(buf); err != nil {
			t.Fatalf("failed to read from server: %v", err)
		}

		if buf[0] == IAC && buf[1] == DO && buf[2] == NAWS {
			return
		}
	}
}

// TestRequestWindowSize_NAWS tests server handling of NAWS negotiation and window size reporting.
func TestRequestWindowSize_NAWS(t *testing.T) {
	handler := &testHandler{done: make(chan struct{})}

	ln, err := net.Listen("tcp", "127.0.0.1:0") // free port
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	defer ln.Close()

	addr := ln.Addr().String()

	// Start server in goroutine.
	go func() {
		_ = Serve(ln, handler.ServeTELNETSession)
	}()

	// Connect as a raw client.
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("client could not connect to server: %v", err)
	}
	defer conn.Close()

	// Wait for server's IAC DO NAWS negotiation..
	scanForNAWS(t, conn, 2*time.Second)

	// Respond with IAC WILL NAWS (client agrees).
	if _, err = conn.Write([]byte{IAC, WILL, NAWS}); err != nil {
		t.Fatalf("failed to write WILL NAWS: %v", err)
	}

	// Send subnegotiation: IAC SB NAWS <cols=80> <rows=24> IAC SE.
	cols, rows := 99, 99
	nawsSB := []byte{
		IAC, SB, NAWS,
		byte(cols >> 8), byte(cols & 0xFF),
		byte(rows >> 8), byte(rows & 0xFF),
		IAC, SE,
	}

	if _, err = conn.Write(nawsSB); err != nil {
		t.Fatalf("failed to write NAWS subnegotiation: %v", err)
	}

	// Wait for the handler to finish.
	select {
	case <-handler.done:
	case <-time.After(2 * time.Second):
		t.Fatal("telnet handler did not finish in time")
	}

	// Verify that the window size was set.
	gotCols, gotRows := handler.lastSession.GetWindowSize()
	if gotCols != cols || gotRows != rows {
		t.Fatalf("server did not parse NAWS correctly, got cols=%d rows=%d (want cols=%d, rows=%d)",
			gotCols, gotRows, cols, rows)
	}
}
