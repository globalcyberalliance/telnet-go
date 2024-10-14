package shell

import (
	"strconv"
	"time"

	"github.com/GlobalCyberAlliance/telnet-go"
)

type AuthHandler func(session *telnet.Session) bool

// NewAuthHandler returns an AuthHandler with the given configuration.
func NewAuthHandler(username string, password string, maxAttempts int) AuthHandler {
	return func(session *telnet.Session) bool {
		for attempts := 0; attempts < maxAttempts; attempts++ {
			if err := session.WriteLine("Login: "); err != nil {
				return false
			}

			userUsername, err := session.ReadLine()
			if err != nil {
				return false
			}

			if err = session.WriteLine("Password: "); err != nil {
				return false
			}

			// Enable ECHO to hide the user password.
			if _, err = session.WriteCommand(telnet.IAC, telnet.WILL, telnet.ECHO); err != nil {
				return false
			}

			userPassword, err := session.ReadLine()
			if err != nil {
				return false
			}

			// Disable ECHO.
			if _, err = session.WriteCommand(telnet.IAC, telnet.WONT, telnet.ECHO); err != nil {
				return false
			}

			if err = session.WriteLine("\n"); err != nil {
				return false
			}

			if userPassword == password && userUsername == username {
				return true
			}

			// Shell logins usually have a default 3 second wait between attempts.
			time.Sleep(3 * time.Second)

			if err = session.WriteLine("\nLogin incorrect\n"); err != nil {
				return false
			}
		}

		if err := session.WriteLine("Maximum number of tries exceeded (" + strconv.Itoa(maxAttempts) + ")\n"); err != nil {
			return false
		}

		return false
	}
}
