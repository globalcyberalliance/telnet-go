package telnet

import (
	"bytes"
	"errors"
	"io"
	"strings"
)

// writer handles escaping data according to the TELNET and TELNETS protocols.
//
// In these protocols, byte value 255 (IAC, "interpret as command") is used for commands.
// TELNET and TELNETS distinguish between 'data' and 'commands'.
//
// writer focuses on escaping 'data', not 'commands'.
// If byte 255 (IAC) appears in the data, it must be escaped by doubling it.
//
// Examples:
//
//	Original:  []byte{255}
//	Escaped:   []byte{255, 255}
//
// A more complete example:
//
//	Original:  []byte{1, 55, 2, 155, 3, 255, 4, 40, 255, 30, 20}
//	Escaped:   []byte{1, 55, 2, 155, 3, 255, 255, 4, 40, 255, 255, 30, 20}
//
// writer automatically handles this escaping process for you.
type writer struct {
	writer io.Writer
}

// newWriter creates a new writer that writes to 'w'.
//
// 'w' will receive the data written to the writer, but escaped according to
// the TELNET and TELNETS protocols. Specifically, byte 255 (IAC) is encoded as 255, 255.
//
// For example, if the following data is written to the writer's Write method:
//
//	[]byte{1, 55, 2, 155, 3, 255, 4, 40, 255, 30, 20}
//
// then the following data is written to 'w's Write method:
//
//	[]byte{1, 55, 2, 155, 3, 255, 255, 4, 40, 255, 255, 30, 20}
//
// (Notice that each byte 255 in the original data is doubled.)
//
// The writer handles this escaping process automatically.
func newWriter(w io.Writer) *writer {
	return &writer{
		writer: w,
	}
}

// Write writes the TELNET (and TELNETS) escaped data for of the data in 'data' to the writer io.Writer.
func (w *writer) Write(data []byte) (n int, err error) {
	var buffer bytes.Buffer

	// Workaround for commands.
	if len(data) > 5 && bytes.Equal(data[0:4], commandSignature()) {
		numWritten, err := LongWrite(w.writer, data[4:])
		return int(numWritten), err
	}

	for _, value := range data {
		if value != IAC {
			buffer.WriteByte(value)
			continue
		}

		// Write buffered data first if there's any.
		if buffer.Len() > 0 {
			numWritten, err := LongWrite(w.writer, buffer.Bytes())
			n += int(numWritten)
			if err != nil {
				return n, err
			}
			buffer.Reset()
		}

		// Write escape IAC sequence.
		numWritten, err := LongWrite(w.writer, w.EscapeIAC())
		if err != nil {
			return n, err
		}

		if int(numWritten) != len(w.EscapeIAC()) {
			return n, errors.New("partial IAC IAC write")
		}

		n++
	}

	// Write any remaining buffered data
	if buffer.Len() > 0 {
		numWritten, err := LongWrite(w.writer, buffer.Bytes())
		n += int(numWritten)
		if err != nil {
			return n, err
		}
	}

	return n, nil
}

func (w *writer) EscapeIAC() []byte {
	return []byte{IAC, IAC}
}

func WriteLine(writer io.Writer, text ...string) error {
	_, err := writer.Write([]byte(strings.Join(text, "")))
	return err
}

// WriteCommand is a dirty workaround to write Telnet commands directly to the client. The internal wrapper satisfies
// io.Write, preventing us from including custom logic to handle commands (without risking bodging real data). Instead,
// this submits a signature (IAC x4) the underlying Write function knows to look for, and to treat as a command.
func WriteCommand(writer io.Writer, command byte, option byte, action byte) (n int, err error) {
	return writer.Write(append(commandSignature(), command, option, action))
}

func commandSignature() []byte {
	return []byte{IAC, IAC, IAC, IAC}
}
