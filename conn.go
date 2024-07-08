package telnet

import (
	"crypto/tls"
	"net"
)

type Conn struct {
	conn   net.Conn
	reader *reader
	writer *writer
}

// TODO: implement timeout for dialing

// Dial makes an unsecured TELNET client connection to the specified address.
// If no address is supplied, it'll default to localhost.
func Dial(protocol, addr string) (*Conn, error) {
	if protocol == "" {
		protocol = "tcp"
	}
	if addr == "" {
		addr = "127.0.0.1:telnet"
	}

	conn, err := net.Dial(protocol, addr)
	if err != nil {
		return nil, err
	}

	return &Conn{
		conn:   conn,
		reader: newReader(conn),
		writer: newWriter(conn),
	}, nil
}

// DialTLS makes a secure TELNETS client connection to the specified address.
// If no address is supplied, it'll default to localhost.
func DialTLS(protocol, addr string, tlsConfig *tls.Config) (*Conn, error) {
	if protocol == "" {
		protocol = "tcp"
	}
	if addr == "" {
		addr = "127.0.0.1:telnets"
	}

	conn, err := tls.Dial(protocol, addr, tlsConfig)
	if err != nil {
		return nil, err
	}

	return &Conn{
		conn:   conn,
		reader: newReader(conn),
		writer: newWriter(conn),
	}, nil
}

// Close closes the client connection.
func (c *Conn) Close() error {
	return c.conn.Close()
}

// Read reads bytes from the server into p.
func (c *Conn) Read(p []byte) (int, error) {
	return c.reader.Read(p)
}

// Write writes bytes to the server from p.
func (c *Conn) Write(p []byte) (int, error) {
	return c.writer.Write(p)
}

// LocalAddr returns the local network address.
func (c *Conn) LocalAddr() net.Addr {
	return c.conn.LocalAddr()
}

// RemoteAddr returns the remote network address.
func (c *Conn) RemoteAddr() net.Addr {
	return c.conn.RemoteAddr()
}
