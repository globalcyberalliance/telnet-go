package telnet

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"time"
)

type Session struct {
	ctx   context.Context
	isPTY bool // Used to normalize line formatting.
	net.Conn
	*reader
	*writer

	// Store client window size.
	termCols int
	termRows int
}

func (s *Session) Context() context.Context {
	return s.ctx
}

func (s *Session) GetWindowSize() (int, int) {
	return s.termCols, s.termRows
}

func (s *Session) HasWindowSize() bool {
	return s.termCols > 0 && s.termRows > 0
}

func (s *Session) Read(data []byte) (n int, err error) {
	return s.reader.Read(data)
}

func (s *Session) ReadLine() (string, error) {
	return ReadLine(s)
}

// RequestWindowSize sends IAC DO NAWS to the client, and stores the response for retrieval via GetWindowSize.
func (s *Session) RequestWindowSize() error {
	if _, err := s.WriteCommand(IAC, DO, NAWS); err != nil {
		return fmt.Errorf("failed to send DO NAWS: %w", err)
	}

	// Set up a timeout so we don't block forever if the client doesn't support NAWS.
	timeout := time.After(2 * time.Second)

	for {
		select {
		case <-timeout:
			// Timeout: client didn't respond to NAWS, treat as not supported (not an error).
			return nil
		default:
			peeked, err := s.reader.buffered.Peek(1)
			if err != nil {
				if err == io.EOF {
					// Connection closed.
					return nil
				}
				return fmt.Errorf("peek failed: %w", err)
			}
			if peeked[0] == IAC {
				// Read IAC.
				if _, err := s.reader.buffered.ReadByte(); err != nil {
					return fmt.Errorf("read IAC failed: %w", err)
				}

				cmd, err := s.reader.buffered.ReadByte()
				if err != nil {
					return fmt.Errorf("read command after IAC failed: %w", err)
				}

				switch cmd {
				case WILL, WONT:
					opt, err := s.reader.buffered.ReadByte()
					if err != nil {
						return fmt.Errorf("read option failed: %w", err)
					}
					if opt == NAWS && cmd == WONT {
						// Client refuses NAWS, treat as not supported, not an error
						return nil
					}
				case SB:
					opt, err := s.reader.buffered.ReadByte()
					if err != nil {
						return fmt.Errorf("read SB option failed: %w", err)
					}
					if opt == NAWS {
						// NAWS SB <width hi> <width lo> <height hi> <height lo> IAC SE.
						payload := make([]byte, 4)
						if _, err = io.ReadFull(s.reader.buffered, payload); err != nil {
							return fmt.Errorf("failed to read NAWS SB payload: %w", err)
						}

						// Expect IAC SE.
						seHdr := make([]byte, 2)
						if _, err := io.ReadFull(s.reader.buffered, seHdr); err != nil {
							return fmt.Errorf("failed to read NAWS SB terminator: %w", err)
						}

						if seHdr[0] != IAC || seHdr[1] != SE {
							return fmt.Errorf("invalid NAWS SB terminator after payload")
						}

						s.termCols = int(payload[0])<<8 | int(payload[1])
						s.termRows = int(payload[2])<<8 | int(payload[3])

						return nil
					} else {
						// Skip until IAC SE for unrelated SB.
						for {
							b, err := s.reader.buffered.ReadByte()
							if err != nil {
								return fmt.Errorf("skip SB error: %w", err)
							}
							if b == IAC {
								nextB, err := s.reader.buffered.ReadByte()
								if err != nil {
									return fmt.Errorf("skip SB error: %w", err)
								}

								if nextB == SE {
									break
								}
							}
						}
					}
				default:
					// Not related to NAWS, ignore and continue.
				}
			} else {
				// Not part of a negotiation, so NAWS not supported; gracefully return.
				return nil
			}
		}
	}
}

// SetIsPTY is only used for line formatting for the Write function since we don't support terminal modes.
func (s *Session) SetIsPTY(isPTY bool) {
	s.isPTY = isPTY
}

func (s *Session) Write(data []byte) (int, error) {
	if s.isPTY {
		originalLength := len(data)

		// Normalize \n to \r\n when pty is accepted.
		// This is a hardcoded shortcut since we don't support terminal modes.
		data = bytes.Replace(data, []byte{'\n'}, []byte{'\r', '\n'}, -1)
		data = bytes.Replace(data, []byte{'\r', '\r', '\n'}, []byte{'\r', '\n'}, -1)

		bytesWritten, err := s.writer.Write(data)
		if bytesWritten > originalLength {
			bytesWritten = originalLength
		}

		return bytesWritten, err
	}

	return s.writer.Write(data)
}

func (s *Session) WriteCommand(command byte, option byte, action byte) (n int, err error) {
	return WriteCommand(s, command, option, action)
}

func (s *Session) WriteLine(text ...string) error {
	return WriteLine(s, text...)
}
