package telnet

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"runtime/debug"
	"sync"
	"time"
)

// ListenAndServe listens on the TCP network address 'addr' and then spawns a call to ServeTELNET
// method on 'handler' to serve each incoming connection.
func ListenAndServe(addr string, handler HandlerFunc) error {
	server := &Server{Addr: addr, Handler: handler, logger: slog.New(slog.DiscardHandler)}
	return server.ListenAndServe()
}

// Serve accepts an incoming TELNET or TELNETS client connection on the net.Listener 'listener'.
func Serve(listener net.Listener, handler HandlerFunc) error {
	server := &Server{Handler: handler, logger: slog.New(slog.DiscardHandler)}
	return server.Serve(listener)
}

type (
	// Server defines parameters of a running TELNET server.
	Server struct {
		closed       bool // set by Shutdown; guarded by handlesMu
		listener     net.Listener
		ConnCallback func(ctx context.Context, conn net.Conn) net.Conn        // optional callback for wrapping net.Conn before handling
		ConnContext  func(ctx context.Context, conn net.Conn) context.Context // optional callback for wrapping context.Context before handling
		Handler      HandlerFunc                                              // handler to invoke; default is telnet.EchoHandler if nil
		TLSConfig    *tls.Config                                              // optional TLS configuration; used by ListenAndServeTLS
		logger       *slog.Logger                                             // optional logger
		handles      map[string]context.CancelFunc
		Addr         string // TCP address to listen on; ":23" or ":992" if empty (used with ListenAndServe or ListenAndServeTLS respectively).
		Timeout      time.Duration
		handlesMu    sync.Mutex
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
		addr = ":23"
	}

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	return server.Serve(listener)
}

// Serve accepts an incoming TELNET client connection on the net.Listener 'listener'.
func (server *Server) Serve(listener net.Listener) error {
	server.handlesMu.Lock()

	if server.closed {
		server.handlesMu.Unlock()
		listener.Close()

		return errors.New("server closed")
	}

	if server.listener != nil {
		server.handlesMu.Unlock()
		listener.Close()

		return errors.New("server already listening")
	}

	if server.logger == nil {
		server.logger = slog.New(slog.DiscardHandler)
	}

	server.listener = listener
	server.handles = make(map[string]context.CancelFunc)
	server.handlesMu.Unlock()

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
			if rawConn == nil {
				panic("ConnCallback returned nil")
			}
		}

		if server.ConnContext != nil {
			ctx = server.ConnContext(ctx, rawConn)
			if ctx == nil {
				panic("ConnContext returned nil")
			}
		}

		conn := serverConn{
			Conn:   rawConn,
			cancel: cancel,
			ctx:    ctx,
		}

		server.logger.Debug("received new connection", "FROM", conn.RemoteAddr().String())

		// Spawn a new goroutine to handle the new client connection.
		go server.handle(conn, handler)
	}
}

func (server *Server) SetLogger(logger *slog.Logger) {
	server.logger = logger
}

func (server *Server) Shutdown() error {
	// Copy the cancel funcs and release the lock before calling them: a cancelled handle reacquires
	// this mutex to remove itself from the map.
	server.handlesMu.Lock()

	listener := server.listener
	server.listener = nil
	server.closed = true

	cancels := make([]context.CancelFunc, 0, len(server.handles))

	for _, cancel := range server.handles {
		cancels = append(cancels, cancel)
	}

	server.handlesMu.Unlock()

	var err error

	// Serve closes the listener on its way out, so an already closed listener is not a failure.
	if listener != nil {
		if closeErr := listener.Close(); closeErr != nil && !errors.Is(closeErr, net.ErrClosed) {
			err = fmt.Errorf("failed to close listener: %w", closeErr)
		}
	}

	for _, cancel := range cancels {
		cancel()
	}

	return err
}

// handle manages the lifecycle of a TELNET client connection.
func (server *Server) handle(conn serverConn, handler HandlerFunc) {
	defer conn.Close()

	// Leave a slight delay to close the context (needed to allow the connection to gracefully close).
	defer func() {
		if recovery := recover(); recovery != nil {
			server.logger.Error("recovered from handle panic", "recovered", recovery, "stack", string(debug.Stack()))
		}
	}()

	addr := conn.RemoteAddr().String()

	// Register before doing any work: a connection that is only registered by the goroutine below is
	// invisible to a Shutdown that runs before that goroutine is scheduled.
	server.handlesMu.Lock()
	closed := server.closed

	if !closed {
		server.handles[addr] = conn.cancel
	}

	server.handlesMu.Unlock()

	// Shutdown already walked the map, so this connection has to cancel itself.
	if closed {
		conn.cancel()
		return
	}

	// Close the handle if context is canceled.
	go func() {
		<-conn.ctx.Done()
		server.logger.Debug("received context completion, closing telnet connection", "from", addr)

		if err := conn.Conn.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			server.logger.Error("failed to close telnet connection", "from", addr, "err", err)
		}

		server.handlesMu.Lock()
		delete(server.handles, addr)
		server.handlesMu.Unlock()
	}()

	defer func() {
		conn.cancel()
	}()

	r := newReader(conn)
	w := newWriter(conn)

	// TODO: handle real protocol negotiation
	// Disable SGA by default. Clients connecting without defining a host port negotiate SGA, which causes ENTER to be
	// handled incorrectly if the server enables and disables echoing (e.g. to mask the user's password during auth).
	if _, err := WriteCommand(w, IAC, WONT, SGA); err != nil {
		return
	}

	handler.ServeTELNET(&Session{
		ctx:    conn.ctx,
		Conn:   conn,
		reader: r,
		writer: w,
	})
}

// The HandlerFunc type is an adapter to allow the use of ordinary functions as TELNET handlers.
type HandlerFunc func(server *Session)

// ServeTELNET calls f(ctx, w, r).
func (f HandlerFunc) ServeTELNET(session *Session) {
	f(session)
}

// EchoHandler is a simple TELNET server which "echos" back to the client any (non-command)
// data back to the TELNET client, it received from the TELNET client.
var EchoHandler HandlerFunc = func(session *Session) {
	// Buffer needs to be small to avoid waiting for it to fill up.
	var buffer [1]byte
	p := buffer[:]

	for {
		n, err := session.Read(p)
		if n > 0 {
			if _, err := session.Write(p[:n]); err != nil {
				return
			}
		}

		if err != nil {
			break
		}
	}
}
