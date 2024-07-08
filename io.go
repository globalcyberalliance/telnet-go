package telnet

import (
	"errors"
	"io"
)

// LongWrite attempts to write the bytes from 'p' to the writer 'w', handling
// short writes where w.Write returns io.ErrShortWrite and n < len(p).
func LongWrite(w io.Writer, p []byte) (int64, error) {
	var numWritten int64
	for len(p) > 0 {
		n, err := w.Write(p)
		numWritten += int64(n)
		if err != nil && !errors.Is(err, io.ErrShortWrite) {
			return numWritten, err
		}

		p = p[n:]
	}
	return numWritten, nil
}
