package telnet

import (
	"bytes"
	"context"
	"io"
	"net"
	"sync"
	"testing"
	"time"
)

func TestEchoHandler(t *testing.T) {
	tests := []struct {
		Bytes    []byte
		Expected []byte
	}{
		{
			Bytes:    []byte{},
			Expected: []byte{},
		},

		{
			Bytes:    []byte("a"),
			Expected: []byte("a"),
		},
		{
			Bytes:    []byte("b"),
			Expected: []byte("b"),
		},
		{
			Bytes:    []byte("c"),
			Expected: []byte("c"),
		},

		{
			Bytes:    []byte("apple"),
			Expected: []byte("apple"),
		},
		{
			Bytes:    []byte("banana"),
			Expected: []byte("banana"),
		},
		{
			Bytes:    []byte("cherry"),
			Expected: []byte("cherry"),
		},

		{
			Bytes:    []byte("apple banana cherry"),
			Expected: []byte("apple banana cherry"),
		},

		{
			Bytes:    []byte{255, 255},
			Expected: []byte{255, 255},
		},
		{
			Bytes:    []byte{255, 255, 255, 255},
			Expected: []byte{255, 255, 255, 255},
		},
		{
			Bytes:    []byte{255, 255, 255, 255, 255, 255},
			Expected: []byte{255, 255, 255, 255, 255, 255},
		},
		{
			Bytes:    []byte{255, 255, 255, 255, 255, 255, 255, 255},
			Expected: []byte{255, 255, 255, 255, 255, 255, 255, 255},
		},
		{
			Bytes:    []byte{255, 255, 255, 255, 255, 255, 255, 255, 255, 255},
			Expected: []byte{255, 255, 255, 255, 255, 255, 255, 255, 255, 255},
		},

		{
			Bytes:    []byte("apple\xff\xffbanana\xff\xffcherry"),
			Expected: []byte("apple\xff\xffbanana\xff\xffcherry"),
		},
		{
			Bytes:    []byte("\xff\xffapple\xff\xffbanana\xff\xffcherry\xff\xff"),
			Expected: []byte("\xff\xffapple\xff\xffbanana\xff\xffcherry\xff\xff"),
		},

		{
			Bytes:    []byte("apple\xff\xff\xff\xffbanana\xff\xff\xff\xffcherry"),
			Expected: []byte("apple\xff\xff\xff\xffbanana\xff\xff\xff\xffcherry"),
		},
		{
			Bytes:    []byte("\xff\xff\xff\xffapple\xff\xff\xff\xffbanana\xff\xff\xff\xffcherry\xff\xff\xff\xff"),
			Expected: []byte("\xff\xff\xff\xffapple\xff\xff\xff\xffbanana\xff\xff\xff\xffcherry\xff\xff\xff\xff"),
		},

		{
			Bytes:    []byte{255, 251, 24}, // IAC WILL TERMINAL-TYPE
			Expected: []byte{},
		},
		{
			Bytes:    []byte{255, 252, 24}, // IAC WON'T TERMINAL-TYPE
			Expected: []byte{},
		},
		{
			Bytes:    []byte{255, 253, 24}, // IAC DO TERMINAL-TYPE
			Expected: []byte{},
		},
		{
			Bytes:    []byte{255, 254, 24}, // IAC DON'T TERMINAL-TYPE
			Expected: []byte{},
		},

		{
			Bytes:    []byte{67, 255, 251, 24}, // 'C' IAC WILL TERMINAL-TYPE
			Expected: []byte{67},
		},
		{
			Bytes:    []byte{67, 255, 252, 24}, // 'C' IAC WON'T TERMINAL-TYPE
			Expected: []byte{67},
		},
		{
			Bytes:    []byte{67, 255, 253, 24}, // 'C' IAC DO TERMINAL-TYPE
			Expected: []byte{67},
		},
		{
			Bytes:    []byte{67, 255, 254, 24}, // 'C' IAC DON'T TERMINAL-TYPE
			Expected: []byte{67},
		},

		{
			Bytes:    []byte{255, 251, 24, 68}, // IAC WILL TERMINAL-TYPE 'D'
			Expected: []byte{68},
		},
		{
			Bytes:    []byte{255, 252, 24, 68}, // IAC WON'T TERMINAL-TYPE 'D'
			Expected: []byte{68},
		},
		{
			Bytes:    []byte{255, 253, 24, 68}, // IAC DO TERMINAL-TYPE 'D'
			Expected: []byte{68},
		},
		{
			Bytes:    []byte{255, 254, 24, 68}, // IAC DON'T TERMINAL-TYPE 'D'
			Expected: []byte{68},
		},

		{
			Bytes:    []byte{67, 255, 251, 24, 68}, // 'C' IAC WILL TERMINAL-TYPE 'D'
			Expected: []byte{67, 68},
		},
		{
			Bytes:    []byte{67, 255, 252, 24, 68}, // 'C' IAC WON'T TERMINAL-TYPE 'D'
			Expected: []byte{67, 68},
		},
		{
			Bytes:    []byte{67, 255, 253, 24, 68}, // 'C' IAC DO TERMINAL-TYPE 'D'
			Expected: []byte{67, 68},
		},
		{
			Bytes:    []byte{67, 255, 254, 24, 68}, // 'C' IAC DON'T TERMINAL-TYPE 'D'
			Expected: []byte{67, 68},
		},

		{
			Bytes:    []byte{255, 250, 24, 1, 255, 240}, // IAC SB TERMINAL-TYPE SEND IAC SE
			Expected: []byte{},
		},
		{
			Bytes:    []byte{255, 250, 24, 0, 68, 69, 67, 45, 86, 84, 53, 50, 255, 240}, // IAC SB TERMINAL-TYPE IS "DEC-VT52" IAC SE
			Expected: []byte{},
		},

		{
			Bytes:    []byte{67, 255, 250, 24, 1, 255, 240}, // 'C' IAC SB TERMINAL-TYPE SEND IAC SE
			Expected: []byte{67},
		},
		{
			Bytes:    []byte{67, 255, 250, 24, 0, 68, 69, 67, 45, 86, 84, 53, 50, 255, 240}, // 'C' IAC SB TERMINAL-TYPE IS "DEC-VT52" IAC SE
			Expected: []byte{67},
		},

		{
			Bytes:    []byte{255, 250, 24, 1, 255, 240, 68}, // IAC SB TERMINAL-TYPE SEND IAC SE 'D'
			Expected: []byte{68},
		},
		{
			Bytes:    []byte{255, 250, 24, 0, 68, 69, 67, 45, 86, 84, 53, 50, 255, 240, 68}, // IAC SB TERMINAL-TYPE IS "DEC-VT52" IAC SE 'D'
			Expected: []byte{68},
		},

		{
			Bytes:    []byte{67, 255, 250, 24, 1, 255, 240, 68}, // 'C' IAC SB TERMINAL-TYPE SEND IAC SE 'D'
			Expected: []byte{67, 68},
		},
		{
			Bytes:    []byte{67, 255, 250, 24, 0, 68, 69, 67, 45, 86, 84, 53, 50, 255, 240, 68}, // 'C' IAC SB TERMINAL-TYPE IS "DEC-VT52" IAC SE 'D'
			Expected: []byte{67, 68},
		},

		{
			Bytes:    []byte{255, 250, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 255, 240}, // IAC SB 0 1 2 3 4 5 6 7 8 9 10 11 12 13 IAC SE
			Expected: []byte{},
		},
		{
			Bytes:    []byte{67, 255, 250, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 255, 240}, // 'C' IAC SB 0 1 2 3 4 5 6 7 8 9 10 11 12 13 IAC SE
			Expected: []byte{67},
		},
		{
			Bytes:    []byte{255, 250, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 255, 240, 68}, // IAC SB 0 1 2 3 4 5 6 7 8 9 10 11 12 13 IAC SE 'D'
			Expected: []byte{68},
		},
		{
			Bytes:    []byte{67, 255, 250, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 255, 240, 68}, // 'C' IAC SB 0 1 2 3 4 5 6 7 8 9 10 11 12 13 IAC SE 'D'
			Expected: []byte{67, 68},
		},

		// TODO: Is this correct? Can IAC appear between thee 'IAC SB' and ''IAC SE'?... and if "yes", do escaping rules apply?
		{
			Bytes:    []byte{255, 250, 255, 255, 240, 255, 240}, //     IAC SB 255 255 240 IAC SE = IAC SB IAC IAC SE IAC SE
			Expected: []byte{},
		},
		{
			Bytes:    []byte{67, 255, 250, 255, 255, 240, 255, 240}, // 'C' IAC SB 255 255 240 IAC SE = IAC SB IAC IAC SE IAC SE
			Expected: []byte{67},
		},
		{
			Bytes:    []byte{255, 250, 255, 255, 240, 255, 240, 68}, //     IAC SB 255 255 240 IAC SE = IAC SB IAC IAC SE IAC SE 'D'
			Expected: []byte{68},
		},
		{
			Bytes:    []byte{67, 255, 250, 255, 255, 240, 255, 240, 68}, // 'C' IAC SB 255 255 240 IAC SE = IAC SB IAC IAC SE IAC SE 'D'
			Expected: []byte{67, 68},
		},

		// TODO: Is this correct? Can IAC appear between thee 'IAC SB' and ''IAC SE'?... and if "yes", do escaping rules apply?
		{
			Bytes:    []byte{255, 250, 71, 255, 255, 240, 255, 240}, //     IAC SB 'G' 255 255 240 IAC SE = IAC SB 'G' IAC IAC SE IAC SE
			Expected: []byte{},
		},
		{
			Bytes:    []byte{67, 255, 250, 71, 255, 255, 240, 255, 240}, // 'C' IAC SB 'G' 255 255 240 IAC SE = IAC SB 'G' IAC IAC SE IAC SE
			Expected: []byte{67},
		},
		{
			Bytes:    []byte{255, 250, 71, 255, 255, 240, 255, 240, 68}, //     IAC SB 'G' 255 255 240 IAC SE = IAC SB 'G' IAC IAC SE IAC SE 'D'
			Expected: []byte{68},
		},
		{
			Bytes:    []byte{67, 255, 250, 71, 255, 255, 240, 255, 240, 68}, // 'C' IAC SB 'G' 255 255 240 IAC SE = IAC 'G' SB IAC IAC SE IAC SE 'D'
			Expected: []byte{67, 68},
		},

		// TODO: Is this correct? Can IAC appear between thee 'IAC SB' and ''IAC SE'?... and if "yes", do escaping rules apply?
		{
			Bytes:    []byte{255, 250, 255, 255, 240, 72, 255, 240}, //     IAC SB 255 255 240 'H' IAC SE = IAC SB IAC IAC SE 'H' IAC SE
			Expected: []byte{},
		},
		{
			Bytes:    []byte{67, 255, 250, 255, 255, 240, 72, 255, 240}, // 'C' IAC SB 255 255 240 'H' IAC SE = IAC SB IAC IAC SE 'H' IAC SE
			Expected: []byte{67},
		},
		{
			Bytes:    []byte{255, 250, 255, 255, 240, 72, 255, 240, 68}, //     IAC SB 255 255 240 'H' IAC SE = IAC SB IAC IAC SE 'H' IAC SE 'D'
			Expected: []byte{68},
		},
		{
			Bytes:    []byte{67, 255, 250, 255, 255, 240, 72, 255, 240, 68}, // 'C' IAC SB 255 255 240 'H' IAC SE = IAC SB IAC IAC SE 'H' IAC SE 'D'
			Expected: []byte{67, 68},
		},

		// TODO: Is this correct? Can IAC appear between thee 'IAC SB' and ''IAC SE'?... and if "yes", do escaping rules apply?
		{
			Bytes:    []byte{255, 250, 71, 255, 255, 240, 72, 255, 240}, //     IAC SB 'G' 255 255 240 'H' IAC SE = IAC SB 'G' IAC IAC SE 'H' IAC SE
			Expected: []byte{},
		},
		{
			Bytes:    []byte{67, 255, 250, 71, 255, 255, 240, 72, 255, 240}, // 'C' IAC SB 'G' 255 255 240 'H' IAC SE = IAC SB 'G' IAC IAC SE 'H' IAC SE
			Expected: []byte{67},
		},
		{
			Bytes:    []byte{255, 250, 71, 255, 255, 240, 72, 255, 240, 68}, //     IAC SB 'G' 255 255 240 'H' IAC SE = IAC SB 'G' IAC IAC SE 'H' IAC SE 'D'
			Expected: []byte{68},
		},
		{
			Bytes:    []byte{67, 255, 250, 71, 255, 255, 240, 72, 255, 240, 68}, // 'C' IAC SB 'G' 255 255 240 'H' IAC SE = IAC 'G' SB IAC IAC SE 'H' IAC SE 'D'
			Expected: []byte{67, 68},
		},
	}

	for testNumber, test := range tests {
		ctx := context.Background()

		var buffer bytes.Buffer

		EchoHandler.ServeTELNET(&Session{
			ctx:    ctx,
			reader: newReader(bytes.NewReader(test.Bytes)),
			writer: newWriter(&buffer),
		})

		if expected, actual := string(test.Expected), buffer.String(); expected != actual {
			t.Errorf("For test #%d, expected %q, but actually got %q.", testNumber, expected, actual)
			continue
		}
	}
}

// TestShutdownDuringConnectionChurn covers Shutdown racing the handle map. Run with -race.
func TestShutdownDuringConnectionChurn(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	server := &Server{Handler: func(session *Session) {
		// Hold the connection open until the client hangs up or shutdown cancels the context.
		var buffer [1]byte
		for {
			if _, err := session.Read(buffer[:]); err != nil {
				return
			}
		}
	}}

	served := make(chan struct{})

	go func() {
		defer close(served)

		_ = server.Serve(listener)
	}()

	addr := listener.Addr().String()

	// Populate the map before shutdown walks it.
	var held []net.Conn

	for range 5 {
		conn, err := net.Dial("tcp", addr)
		if err != nil {
			t.Fatalf("dial: %v", err)
		}

		held = append(held, conn)
	}

	time.Sleep(100 * time.Millisecond)

	// Keep handles registering and deregistering while shutdown runs.
	var clients sync.WaitGroup

	for range 25 {
		clients.Add(1)

		go func() {
			defer clients.Done()

			// A refused dial is expected once the listener closes.
			if conn, err := net.Dial("tcp", addr); err == nil {
				conn.Close()
			}
		}()
	}

	if err = server.Shutdown(); err != nil {
		t.Errorf("shutdown: %v", err)
	}

	clients.Wait()

	// Shutdown must close the connections it was holding, not just stop accepting new ones. The copy
	// drains the negotiation bytes and returns a nil error on EOF, or the deadline error otherwise.
	for i, conn := range held {
		_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))

		if _, err = io.Copy(io.Discard, conn); err != nil {
			t.Errorf("held conn %d: want EOF after shutdown, got %v", i, err)
		}

		conn.Close()
	}

	select {
	case <-served:
	case <-time.After(5 * time.Second):
		t.Fatal("Serve did not return after Shutdown")
	}

	// Shutdown is idempotent, including after Serve closed the listener itself.
	if err = server.Shutdown(); err != nil {
		t.Errorf("second shutdown: %v", err)
	}
}

// TestShutdownBeforeServe covers shutting down a server that never served, which must not panic on the
// nil handle map, and must stop a later Serve from accepting connections.
func TestShutdownBeforeServe(t *testing.T) {
	server := &Server{}

	if err := server.Shutdown(); err != nil {
		t.Fatalf("shutdown: %v", err)
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	if err = server.Serve(listener); err == nil {
		t.Error("serve on a shutdown server: want error, got nil")
	}
}
