package telnet

import (
	"context"
	"fmt"
	"io"
	"net"
	"time"
)

type Session struct {
	ctx context.Context
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

func (s *Session) Write(data []byte) (n int, err error) {
	return s.writer.Write(data)
}

func (s *Session) WriteCommand(command byte, option byte, action byte) (n int, err error) {
	return WriteCommand(s, command, option, action)
}

func (s *Session) WriteLine(text ...string) error {
	return WriteLine(s, text...)
}
