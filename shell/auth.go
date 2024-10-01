package shell

import (
	"context"
	"io"
	"strconv"
	"time"

	"github.com/GlobalCyberAlliance/telnet-go"
)

type AuthHandler func(ctx context.Context, w io.Writer, r io.Reader) bool

// NewAuthHandler returns an AuthHandler with the given configuration.
func NewAuthHandler(username string, password string, maxAttempts int) AuthHandler {
	return func(ctx context.Context, w io.Writer, r io.Reader) bool {
		for attempts := 0; attempts < maxAttempts; attempts++ {
			if err := telnet.WriteLine(w, "Login: "); err != nil {
				return false
			}

			userUsername, err := telnet.ReadLine(r)
			if err != nil {
				return false
			}

			if err = telnet.WriteLine(w, "Password: "); err != nil {
				return false
			}

			// Enable ECHO to hide the user password.
			if _, err = telnet.WriteCommand(w, telnet.IAC, telnet.WILL, telnet.ECHO); err != nil {
				return false
			}

			userPassword, err := telnet.ReadLine(r)
			if err != nil {
				return false
			}

			// Disable ECHO.
			if _, err = telnet.WriteCommand(w, telnet.IAC, telnet.WONT, telnet.ECHO); err != nil {
				return false
			}

			if err = telnet.WriteLine(w, "\n"); err != nil {
				return false
			}

			if userPassword == password && userUsername == username {
				return true
			}

			// Shell logins usually have a default 3 second wait between attempts.
			time.Sleep(3 * time.Second)

			if err = telnet.WriteLine(w, "\nLogin incorrect\n"); err != nil {
				return false
			}
		}

		if err := telnet.WriteLine(w, "Maximum number of tries exceeded ("+strconv.Itoa(maxAttempts)+")\n"); err != nil {
			return false
		}

		return false
	}
}
