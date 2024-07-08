package telnet

import (
	"bytes"
	"testing"
)

func TestWriter_Write(t *testing.T) {
	tests := []struct {
		Bytes    []byte
		Expected []byte
	}{
		{
			Bytes:    []byte{},
			Expected: []byte{},
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
			Bytes:    []byte{IAC},
			Expected: []byte{IAC, IAC},
		},
		{
			Bytes:    []byte{IAC, IAC},
			Expected: []byte{IAC, IAC, IAC, IAC},
		},
		{
			Bytes:    []byte{IAC, IAC, IAC},
			Expected: []byte{IAC, IAC, IAC, IAC, IAC, IAC},
		},
		{
			Bytes:    []byte{IAC, IAC, IAC, IAC},
			Expected: []byte{IAC, IAC, IAC, IAC, IAC, IAC, IAC, IAC},
		},
		{
			Bytes:    []byte{IAC, IAC, IAC, IAC, IAC},
			Expected: []byte{IAC, IAC, IAC, IAC, IAC, IAC, IAC, IAC, IAC, IAC},
		},

		{
			Bytes:    []byte("apple\xffbanana\xffcherry"),
			Expected: []byte("apple\xff\xffbanana\xff\xffcherry"),
		},
		{
			Bytes:    []byte("\xffapple\xffbanana\xffcherry\xff"),
			Expected: []byte("\xff\xffapple\xff\xffbanana\xff\xffcherry\xff\xff"),
		},

		{
			Bytes:    []byte("apple\xff\xffbanana\xff\xffcherry"),
			Expected: []byte("apple\xff\xff\xff\xffbanana\xff\xff\xff\xffcherry"),
		},
		{
			Bytes:    []byte("\xff\xffapple\xff\xffbanana\xff\xffcherry\xff\xff"),
			Expected: []byte("\xff\xff\xff\xffapple\xff\xff\xff\xffbanana\xff\xff\xff\xffcherry\xff\xff\xff\xff"),
		},
	}

	// TODO: Add random tests.

	for testNumber, test := range tests {
		subWriter := new(bytes.Buffer)
		telnetWriter := newWriter(subWriter)

		n, err := telnetWriter.Write(test.Bytes)
		if err != nil {
			t.Errorf("For test #%d, did not expect an error, but actually got one: (%T) %v; for %q -> %q.", testNumber, err, err, string(test.Bytes), string(test.Expected))
			continue
		}

		if expected, actual := len(test.Bytes), n; expected != actual {
			t.Errorf("For test #%d, expected %d, but actually got %d; for %q -> %q.", testNumber, expected, actual, string(test.Bytes), string(test.Expected))
			continue
		}

		if expected, actual := string(test.Expected), subWriter.String(); expected != actual {
			t.Errorf("For test #%d, expected %q, but actually got %q; for %q -> %q.", testNumber, expected, actual, string(test.Bytes), string(test.Expected))
			continue
		}
	}
}

func TestWriter_WriteCommand(t *testing.T) {
	tests := []struct {
		Bytes    []byte
		Expected []byte
	}{
		{
			Bytes:    []byte{IAC, WILL, ECHO},
			Expected: []byte{IAC, WILL, ECHO},
		},
	}

	// TODO: Add random tests.

	for testNumber, test := range tests {
		subWriter := new(bytes.Buffer)
		telnetWriter := newWriter(subWriter)

		n, err := WriteCommand(telnetWriter, test.Bytes[0], test.Bytes[1], test.Bytes[2])
		if err != nil {
			t.Errorf("For test #%d, did not expect an error, but actually got one: (%T) %v; for %q -> %q.", testNumber, err, err, string(test.Bytes), string(test.Expected))
			continue
		}

		if expected, actual := len(test.Bytes), n; expected != actual {
			t.Errorf("For test #%d, expected %d, but actually got %d; for %q -> %q.", testNumber, expected, actual, string(test.Bytes), string(test.Expected))
			continue
		}

		if expected, actual := string(test.Expected), subWriter.String(); expected != actual {
			t.Errorf("For test #%d, expected %q, but actually got %q; for %q -> %q.", testNumber, expected, actual, string(test.Bytes), string(test.Expected))
			continue
		}
	}
}
