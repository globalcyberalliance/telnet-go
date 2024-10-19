package telnet

import (
	"crypto/tls"
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
		addr = ":telnets"
	}

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	tlsConfig := &tls.Config{}

	if server.TLSConfig != nil {
		tlsConfig = &tls.Config{
			Rand:                   server.TLSConfig.Rand,
			Time:                   server.TLSConfig.Time,
			Certificates:           server.TLSConfig.Certificates,
			GetCertificate:         server.TLSConfig.GetCertificate,
			RootCAs:                server.TLSConfig.RootCAs,
			NextProtos:             server.TLSConfig.NextProtos,
			ServerName:             server.TLSConfig.ServerName,
			ClientAuth:             server.TLSConfig.ClientAuth,
			ClientCAs:              server.TLSConfig.ClientCAs,
			InsecureSkipVerify:     server.TLSConfig.InsecureSkipVerify,
			CipherSuites:           server.TLSConfig.CipherSuites,
			SessionTicketsDisabled: server.TLSConfig.SessionTicketsDisabled,
			ClientSessionCache:     server.TLSConfig.ClientSessionCache,
			MinVersion:             server.TLSConfig.MinVersion,
			MaxVersion:             server.TLSConfig.MaxVersion,
			CurvePreferences:       server.TLSConfig.CurvePreferences,
		}
	}

	tlsConfigHasCertificate := len(tlsConfig.Certificates) > 0 || nil != tlsConfig.GetCertificate
	if certFile == "" || keyFile == "" || !tlsConfigHasCertificate {
		tlsConfig.Certificates = make([]tls.Certificate, 1)

		tlsConfig.Certificates[0], err = tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			return err
		}
	}

	tlsListener := tls.NewListener(listener, tlsConfig)

	return server.Serve(tlsListener)
}
