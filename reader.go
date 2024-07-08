package telnet

import (
	"bufio"
	"bytes"
	"errors"
	"io"
)

const (
	ECHO     byte = 1
	SGA      byte = 3
	NL       byte = 10 // New line.
	CR       byte = 13 // Carriage return.
	LINEMODE byte = 34
	SE       byte = 240
	SB       byte = 250
	WILL     byte = 251
	WONT     byte = 252
	DO       byte = 253
	DONT     byte = 254
	IAC      byte = 255
)

// reader handles un-escaping data according to the TELNET protocol.
//
// In the TELNET protocol, byte value 255 (IAC, "interpret as command") is used to indicate commands.
// TELNET distinguishes between 'data' and 'commands'.
//
// When byte 255 (IAC) appears in data, it must be escaped by doubling it.
// For example:
//
//	[]byte{255} becomes []byte{255, 255}
//
// A more complete example:
//
//	Original:  []byte{1, 55, 2, 155, 3, 255, 4, 40, 255, 30, 20}
//	Escaped:   []byte{1, 55, 2, 155, 3, 255, 255, 4, 40, 255, 255, 30, 20}
//
// telnetReader un-escapes data, reversing the escaping process.
// For example:
//
//	[]byte{255, 255} becomes []byte{255}
//	Escaped:   []byte{1, 55, 2, 155, 3, 255, 255, 4, 40, 255, 255, 30, 20}
//	Unescaped: []byte{1, 55, 2, 155, 3, 255, 4, 40, 255, 30, 20}
type reader struct {
	buffered *bufio.Reader
	reader   io.Reader
}

// newReader creates a new DataReader reading from 'r'.
func newReader(r io.Reader) *reader {
	return &reader{
		buffered: bufio.NewReader(r),
		reader:   r,
	}
}

// Read reads the Telnet data stream, and parses Telnet-specific data.
func (r *reader) Read(data []byte) (n int, err error) {
	for len(data) > 0 {
		if n > 0 && r.buffered.Buffered() < 1 {
			break
		}

		b, err := r.buffered.ReadByte()
		if err != nil {
			return n, err
		}

		if b == IAC {
			var peeked []byte

			peeked, err = r.buffered.Peek(1)
			if err != nil {
				return n, err
			}

			switch peeked[0] {
			case WILL, WONT, DO, DONT:
				if _, err = r.buffered.Discard(2); err != nil {
					return n, err
				}
			case IAC:
				data[0] = IAC
				n++
				data = data[1:]

				if _, err = r.buffered.Discard(1); err != nil {
					return n, err
				}
			case SB:
				for {
					b2, err := r.buffered.ReadByte()
					if err != nil {
						return n, err
					}

					if b2 == IAC {
						peeked, err = r.buffered.Peek(1)
						if err != nil {
							return n, err
						}

						if peeked[0] == IAC || peeked[0] == SE {
							if _, err = r.buffered.Discard(1); err != nil {
								return n, err
							}

							if peeked[0] == SE {
								break
							}
						}
					}
				}
			case SE:
				if _, err = r.buffered.Discard(1); err != nil {
					return n, err
				}
			default:
				// If we're here, it's not following the telnet protocol.
				return n, errors.New("corrupted")
			}
		} else {
			data[0] = b
			n++
			data = data[1:]
		}
	}

	return n, nil
}

// ReadLine is a helper function to read a line from the Telnet client.
//
// This doesn't really work for reading from servers, as servers may not finish a line with a \r or \n (e.g. an auth
// prompt), causing reader.Read(p) to block indefinitely.
func ReadLine(reader io.Reader) (string, error) {
	var line bytes.Buffer
	var buffer [1]byte
	p := buffer[:]

	for {
		n, err := reader.Read(p)
		if n <= 0 && err == nil {
			continue
		} else if n <= 0 && err != nil {
			return "", err
		}

		line.WriteByte(p[0])

		if p[0] == NL {
			break
		}
	}

	// Remove the \r\n from the end of the string.
	lineBytes := line.Bytes()
	if len(lineBytes) >= 2 && lineBytes[len(lineBytes)-2] == '\r' && lineBytes[len(lineBytes)-1] == '\n' {
		return string(lineBytes[:len(lineBytes)-2]), nil
	}

	return line.String(), nil
}
