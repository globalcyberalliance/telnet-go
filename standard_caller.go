package telnet

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// TODO: StandardCaller could do with being refactored, similarly to how the server code was refactored.

// StandardCaller is a simple TELNET client which sends to the server any data it gets from os.Stdin
// as TELNET (and TELNETS) data, and writes any TELNET (or TELNETS) data it receives from
// the server to os.Stdout, and writes any error it has to os.Stderr.
var StandardCaller Caller = internalStandardCaller{}

type internalStandardCaller struct{}

func (caller internalStandardCaller) CallTELNET(ctx context.Context, w io.Writer, r io.Reader) {
	standardCallerCallTELNET(os.Stdin, os.Stdout, os.Stderr, w, r)
}

func standardCallerCallTELNET(stdin io.ReadCloser, stdout io.WriteCloser, stderr io.WriteCloser, w io.Writer, r io.Reader) {
	received := make(chan struct{})
	out := &stopWriter{w: stdout}

	go func(writer io.Writer, reader io.Reader) {
		defer close(received)

		var buffer [1]byte // Seems like the length of the buffer needs to be small, otherwise will have to wait for buffer to fill up.
		p := buffer[:]

		for {
			// Read 1 byte.
			n, err := reader.Read(p)
			if n <= 0 && nil == err {
				continue
			} else if n <= 0 && err != nil {
				break
			}

			if _, err = LongWrite(writer, p); err != nil {
				return
			}
		}
	}(out, r)

	var buffer bytes.Buffer
	var p []byte
	crlfBuffer := [2]byte{'\r', '\n'}
	crlf := crlfBuffer[:]

	scanner := bufio.NewScanner(stdin)
	scanner.Split(scannerSplitFunc)

	for scanner.Scan() {
		buffer.Write(scanner.Bytes())
		buffer.Write(crlf)

		p = buffer.Bytes()

		n, err := LongWrite(w, p)
		if err != nil {
			break
		}
		if expected, actual := int64(len(p)), n; expected != actual {
			// TODO: improve this?
			fmt.Fprint(stderr, fmt.Errorf("Transmission problem: tried sending %d bytes, but actually only sent %d bytes.", expected, actual))
			return
		}

		buffer.Reset()
	}

	// Wait to receive data from the server (that we would send to io.Stdout). Not unconditionally:
	// against a live connection the reader only ends when the server hangs up.
	select {
	case <-received:
	case <-time.After(3 * time.Millisecond):
		out.stop()
	}
}

// stopWriter serialises writes to an underlying writer. Once stop returns, nothing else is written,
// which lets a caller give up on the reader goroutine without racing its final write.
type stopWriter struct {
	w       io.Writer
	mu      sync.Mutex
	stopped bool
}

func (sw *stopWriter) Write(p []byte) (int, error) {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	if sw.stopped {
		return len(p), nil
	}

	return sw.w.Write(p)
}

func (sw *stopWriter) stop() {
	sw.mu.Lock()
	sw.stopped = true
	sw.mu.Unlock()
}

func scannerSplitFunc(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF {
		return 0, nil, nil
	}

	return bufio.ScanLines(data, atEOF)
}
