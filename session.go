package telnet

import (
	"context"
	"net"
)

type Session struct {
	ctx context.Context
	net.Conn
	*reader
	*writer
}

func (s *Session) Context() context.Context {
	return s.ctx
}

func (s *Session) Read(data []byte) (n int, err error) {
	return s.reader.Read(data)
}

func (s *Session) ReadLine() (string, error) {
	return ReadLine(s)
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
