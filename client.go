package telnet

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
)

type (
	// A Caller represents the client end of a TELNET (or TELNETS) connection.
	//
	// Writing data to the Writer passed as an argument to the CallTELNET method
	// will send data to the TELNET (or TELNETS) server.
	//
	// Reading data from the Reader passed as an argument to the CallTELNET method
	// will receive data from the TELNET server.
	//
	// The Writer's Write method sends "escaped" TELNET (and TELNETS) data.
	//
	// The Reader's Read method "un-escapes" TELNET (and TELNETS) data, and filters
	// out TELNET (and TELNETS) command sequences.
	Caller interface {
		CallTELNET(context.Context, io.Writer, io.Reader)
	}

	Client struct {
		Caller Caller
		Logger *slog.Logger
	}
)

func NewClient(caller Caller, logger *slog.Logger) *Client {
	if logger == nil {
		logger = slog.Default()
	}

	return &Client{Caller: caller, Logger: logger}
}

func (client *Client) Call(conn *Conn) error {
	caller := client.Caller
	if caller == nil {
		client.Logger.Debug("defaulted caller to EchoCaller")
		caller = EchoCaller
	}

	caller.CallTELNET(context.Background(), conn.writer, conn.reader)

	// TODO: should this be closed here? Seems irresponsible to not leave it up to the caller
	conn.Close()

	return nil
}

func DialAndCall(srvAddr string, caller Caller) error {
	conn, err := Dial("", srvAddr)
	if err != nil {
		return err
	}

	client := NewClient(caller, nil)

	return client.Call(conn)
}

func DialAndCallTLS(srvAddr string, caller Caller, tlsConfig *tls.Config) error {
	conn, err := DialTLS("", srvAddr, tlsConfig)
	if err != nil {
		return err
	}

	client := NewClient(caller, nil)

	return client.Call(conn)
}

// The CallerFunc type is an adapter to allow the use of ordinary functions as TELNET callers.
type CallerFunc func(context.Context, io.Writer, io.Reader)

// CallTELNET calls f(ctx, w, r).
func (f CallerFunc) CallTELNET(ctx context.Context, w io.Writer, r io.Reader) {
	f(ctx, w, r)
}

// EchoCaller is a simple TELNET client which sends to the server any data it gets from os.Stdin
// as TELNET data, and writes any TELNET data it receives from the server to os.Stdout.
var EchoCaller CallerFunc = func(ctx context.Context, w io.Writer, r io.Reader) {
	for {
		serverLine, err := ReadLine(r)
		if err != nil {
			if err == io.EOF {
				fmt.Println("Connection closed by foreign host.")
				return
			}

			fmt.Printf("Failed to read from the server: %v\n", err)
			return
		}

		if _, err = os.Stdout.WriteString(serverLine); err != nil {
			fmt.Printf("Failed to write server response to stdout: %v\n", err)
			return
		}

		clientLine, err := ReadLine(os.Stdin)
		if err != nil {
			fmt.Printf("Failed to read client input: %v\n", err)
			return
		}

		if !strings.HasSuffix(clientLine, "\r\n") {
			// The client may have supplied a new line without a carriage return.
			clientLine = strings.TrimSuffix(clientLine, "\n") + "\r\n"
		}

		if err = WriteLine(w, clientLine); err != nil {
			fmt.Printf("Failed to write to server: %v\n", err)
			return
		}
	}
}
