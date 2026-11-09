package telnet

import (
	"bufio"
	"bytes"
	"errors"
	"io"
)

const (
	// The NVT control codes, and the options this package names. An option is negotiated with the
	// WILL/WONT/DO/DONT exchange below; everything else is sent as IAC and the code.
	NUL      byte = 0  // Pads a bare carriage return, per RFC 854.
	ECHO     byte = 1  // Option: the server echoes what the client types, which masks a password.
	SGA      byte = 3  // Option: suppress go-ahead, which puts the client in character-at-a-time mode.
	NL       byte = 10 // New line.
	CR       byte = 13 // Carriage return.
	NAWS     byte = 31 // Option: negotiate about window size, which carries the client's rows and columns.
	LINEMODE byte = 34 // Option: the client sends whole lines rather than keystrokes.

	// The commands. SE through GA are two bytes, IAC and the code; the rest take an option byte after.
	SE   byte = 240 // End of the subnegotiation begun by SB.
	NOP  byte = 241 // No operation.
	DM   byte = 242 // Data mark, the synchronising byte of a TCP urgent-data interrupt.
	BRK  byte = 243 // Break.
	IP   byte = 244 // Interrupt process, which is what a client sends for Ctrl-C.
	AO   byte = 245 // Abort output.
	AYT  byte = 246 // Are you there, a liveness check expecting any visible answer.
	EC   byte = 247 // Erase character.
	EL   byte = 248 // Erase line.
	GA   byte = 249 // Go ahead, which only a half-duplex client without SGA waits for.
	SB   byte = 250 // Begin a subnegotiation, ended by IAC SE.
	WILL byte = 251 // The sender offers to enable an option.
	WONT byte = 252 // The sender refuses, or stops, an option.
	DO   byte = 253 // The sender asks the other side to enable an option.
	DONT byte = 254 // The sender asks the other side to stop an option.
	IAC  byte = 255 // Interpret as command: introduces every command, and doubled is a literal 255.
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
			case SE, NOP, DM, BRK, IP, AO, AYT, EC, EL, GA:
				// The two-byte commands of RFC 854. A client sends IAC IP for Ctrl-C and IAC AYT for a
				// liveness check, so treating either as corruption would drop the connection mid-session.
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

// maxEmptyReads bounds how many times a read that returns nothing without failing is retried, matching what
// bufio allows before it gives up.
const maxEmptyReads = 100

// ReadLine is a helper function to read a line from the Telnet client.
//
// This doesn't really work for reading from servers, as servers may not finish a line with a \r or \n (e.g. an auth
// prompt), causing reader.Read(p) to block indefinitely.
func ReadLine(reader io.Reader) (string, error) {
	var line bytes.Buffer
	var buffer [1]byte
	p := buffer[:]

	readByte := func() (byte, error) {
		// A reader may return nothing without failing, as this one does for a run of commands with no data
		// behind it. Retrying is how the next byte is reached, but a reader that never progresses would
		// otherwise spin, so it is bounded the way bufio bounds its own.
		for range maxEmptyReads {
			n, err := reader.Read(p)
			if n > 0 {
				return p[0], nil
			}

			if err != nil {
				return 0, err
			}
		}

		return 0, io.ErrNoProgress
	}

	for {
		b, err := readByte()
		if err != nil {
			return "", err
		}

		if b == NL {
			line.WriteByte(b)

			break
		}

		if b != CR {
			line.WriteByte(b)

			continue
		}

		// RFC 854 requires a CR to be followed by LF for a new line or NUL for a carriage return alone.
		// A client in character-at-a-time mode sends ENTER as CR NUL, so that ends the line too; a CR
		// followed by anything else is data, which keeps both bytes.
		next, err := readByte()
		if err != nil {
			return "", err
		}

		line.WriteByte(CR)

		if next == NUL || next == NL {
			if next == NL {
				line.WriteByte(NL)
			}

			break
		}

		line.WriteByte(next)
	}

	// Remove the line terminator. RFC 854 mandates CRLF, but plenty of clients send a bare LF, so
	// trim whichever turned up. Leaving a lone \n attached silently breaks callers that compare
	// or pattern-match the result (e.g. auth handlers).
	return string(bytes.TrimRight(line.Bytes(), "\r\n")), nil
}

// Subnegotiation contains the option and payload of a Telnet SB...SE block.
type Subnegotiation struct {
	Option  byte
	Payload []byte // unescaped (IAC IAC → IAC)
}

// ReadSubnegotiation waits for and returns the next subnegotiation block.
func (r *reader) ReadSubnegotiation() (*Subnegotiation, error) {
	for {
		b, err := r.buffered.ReadByte()
		if err != nil {
			return nil, err
		}
		if b == IAC {
			next, err := r.buffered.ReadByte()
			if err != nil {
				return nil, err
			}
			if next == SB {
				option, err := r.buffered.ReadByte()
				if err != nil {
					return nil, err
				}
				payload := make([]byte, 0, 8)

			subloop:
				for {
					c, err := r.buffered.ReadByte()
					if err != nil {
						return nil, err
					}
					if c == IAC {
						// Peek ahead to see if this is IAC SE or an escaped IAC.
						d, err := r.buffered.ReadByte()
						if err != nil {
							return nil, err
						}

						if d == SE {
							break subloop
						} else if d == IAC {
							payload = append(payload, IAC)
						} else {
							// Protocol error, but skip.
							continue
						}
					} else {
						payload = append(payload, c)
					}
				}
				return &Subnegotiation{Option: option, Payload: payload}, nil
			}
		}
	}
}
