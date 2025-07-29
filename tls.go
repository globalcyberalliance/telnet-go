package telnet

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net"
)

// ListenAndServeTLS functions similarly to ListenAndServe, but supports the TELNET protocol over TLS.
//
// This enables 'secured telnet' (TELNETS), typically on port 992 by default,
// though this can be overridden using the 'addr' argument.
func ListenAndServeTLS(addr string, certFile string, keyFile string, handler HandlerFunc) error {
	server := &Server{Addr: addr, Handler: handler}
	return server.ListenAndServeTLS(certFile, keyFile)
}

// ListenAndServeTLS behaves similarly to ListenAndServe, but operates over the TELNET protocol with TLS encryption.
//
// In the context of the TELNET protocol, it enables 'secured telnet' (TELNETS), typically on port 992.
func (server *Server) ListenAndServeTLS(certFile string, keyFile string) error {
	addr := server.Addr
	if addr == "" {
		addr = ":992"
	}

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	if server.TLSConfig == nil {
		server.TLSConfig = &tls.Config{}
	}

	if len(server.TLSConfig.Certificates) == 0 {
		if certFile == "" && keyFile == "" {
			return errors.New("missing certificate file and key file")
		}

		tlsCert, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			return fmt.Errorf("failed to load key pair: %w", err)
		}

		server.TLSConfig.Certificates = []tls.Certificate{tlsCert}
	}

	tlsListener := tls.NewListener(listener, server.TLSConfig)

	return server.Serve(tlsListener)
}
