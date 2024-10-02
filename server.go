package telnet

import (
	"context"
	"crypto/tls"
	"io"
	"log/slog"
	"net"
	"runtime/debug"
	"time"
)

// ListenAndServe listens on the TCP network address 'addr' and then spawns a call to ServeTELNET
// method on 'handler' to serve each incoming connection.
func ListenAndServe(addr string, handler HandlerFunc) error {
	server := &Server{Addr: addr, Handler: handler, logger: slog.Default()}
	return server.ListenAndServe()
}

// Serve accepts an incoming TELNET or TELNETS client connection on the net.Listener 'listener'.
func Serve(listener net.Listener, handler HandlerFunc) error {
	server := &Server{Handler: handler, logger: slog.Default()}
	return server.Serve(listener)
}

type (
	// Server defines parameters of a running TELNET server.
	Server struct {
		ConnCallback func(ctx context.Context, conn net.Conn) net.Conn // optional callback for wrapping net.Conn before handling
		Handler      HandlerFunc                                       // handler to invoke; default is telnet.EchoHandler if nil
		TLSConfig    *tls.Config                                       // optional TLS configuration; used by ListenAndServeTLS
		logger       *slog.Logger                                      // optional logger
		Addr         string                                            // TCP address to listen on; ":telnet" or ":telnets" if empty (used with ListenAndServe or ListenAndServeTLS respectively).
		Timeout      time.Duration
	}

	// serverConn is used to wrap a handle with context.
	serverConn struct {
		net.Conn

		ctx    context.Context
		cancel context.CancelFunc
	}
)

// ListenAndServe listens on the TCP network address 'server.Addr' and then spawns a call to Serve
// method on 'server.Handler' to serve each incoming connection.
func (server *Server) ListenAndServe() error {
	addr := server.Addr
	if addr == "" {
		addr = ":telnet"
	}

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	return server.Serve(listener)
}

// Serve accepts an incoming TELNET client connection on the net.Listener 'listener'.
func (server *Server) Serve(listener net.Listener) error {
	defer listener.Close()

	handler := server.Handler
	if handler == nil {
		server.logger.Debug("no handler set, using EchoHandler")
		handler = EchoHandler
	}

	for {
		rawConn, err := listener.Accept()
		if err != nil {
			return err
		}

		var ctx context.Context
		var cancel context.CancelFunc

		if server.Timeout > 0 {
			ctx, cancel = context.WithDeadline(context.Background(), time.Now().Add(server.Timeout))
		} else {
			ctx, cancel = context.WithCancel(context.Background())
		}

		if server.ConnCallback != nil {
			rawConn = server.ConnCallback(ctx, rawConn)
		}

		conn := serverConn{
			Conn:   rawConn,
			cancel: cancel,
		}

		server.logger.Debug("received new connection", "FROM", conn.RemoteAddr().String())

		// Spawn a new goroutine to handle the new client connection.
		go server.handle(conn, handler)
	}
}

func (server *Server) SetLogger(logger *slog.Logger) {
	server.logger = logger
}

// handle manages the lifecycle of a TELNET client connection.
func (server *Server) handle(conn serverConn, handler HandlerFunc) {
	defer conn.Close()

	// Leave a slight delay to close the context (needed to allow the connection to gracefully close).
	defer func() {
		time.Sleep(250 * time.Millisecond)
		conn.cancel()
	}()

	defer func() {
		if recovery := recover(); recovery != nil {
			server.logger.Error("recovered from handle panic", "recovered", recovery, "stack", string(debug.Stack()))
		}
	}()

	r := newReader(conn)
	w := newWriter(conn)

	// TODO: handle real protocol negotiation
	// Disable SGA by default. Clients connecting without defining a host port negotiate SGA, which causes ENTER to be
	// handled incorrectly if the server enables and disables echoing (e.g. to mask the user's password during auth).
	if _, err := WriteCommand(w, IAC, WONT, SGA); err != nil {
		return
	}

	handler.ServeTELNET(conn.ctx, w, r)
}

// The HandlerFunc type is an adapter to allow the use of ordinary functions as TELNET handlers.
type HandlerFunc func(context.Context, io.Writer, io.Reader)

// ServeTELNET calls f(ctx, w, r).
func (f HandlerFunc) ServeTELNET(ctx context.Context, w io.Writer, r io.Reader) {
	f(ctx, w, r)
}

// EchoHandler is a simple TELNET server which "echos" back to the client any (non-command)
// data back to the TELNET client, it received from the TELNET client.
var EchoHandler HandlerFunc = func(ctx context.Context, w io.Writer, r io.Reader) {
	// Buffer needs to be small to avoid waiting for it to fill up.
	var buffer [1]byte
	p := buffer[:]

	for {
		n, err := r.Read(p)
		if n > 0 {
			if _, err := w.Write(p[:n]); err != nil {
				return
			}
		}

		if err != nil {
			break
		}
	}
}
