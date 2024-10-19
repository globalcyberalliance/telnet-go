package shell

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/GlobalCyberAlliance/telnet-go"
)

const (
	DefaultCommandNotFound = ": command not found\n"
	DefaultExitCommand     = "exit"
	DefaultExitMessage     = "Goodbye!\r\n"
	DefaultPrompt          = "$ "
	DefaultWelcomeMessage  = "\r\nWelcome!\r\n"
)

type (
	Command struct {
		Regex    string
		Response string
	}

	Handler func(command string) string

	Server struct {
		// AuthHandler handles authentication attempts against the server.
		AuthHandler AuthHandler

		// GenericHandler can be used as a fallback if no matching command is found within Commands.
		GenericHandler Handler

		// Version is the server version sent to the client after the initial connection.
		Version string

		// Commands contains the available regex matching commands.
		Commands []Command
	}
)

func (s *Server) HandlerFunc(session *telnet.Session) {
	// If the AuthHandler is configured and the user fails login, return.
	if s.AuthHandler != nil && !s.AuthHandler(session) {
		return
	}

	if err := session.WriteLine(DefaultWelcomeMessage); err != nil {
		return
	}

	for {
		if err := session.WriteLine(DefaultPrompt); err != nil {
			return
		}

		line, err := session.ReadLine()
		if err != nil {
			return
		}

		fields := strings.Split(line, " ")
		if len(fields) == 0 {
			if err = session.WriteLine(DefaultExitMessage); err != nil {
				return
			}
			return
		}

		if fields[0] == DefaultExitCommand {
			if err = session.WriteLine(DefaultExitMessage); err != nil {
				return
			}
			return
		}

		var matched bool

		for _, command := range s.Commands {
			matched, err = regexp.MatchString(command.Regex, line)
			if err != nil {
				fmt.Println(err.Error())
				continue
			}

			if matched {
				if err = session.WriteLine(command.Response); err != nil {
					return
				}
				break
			}
		}

		if !matched {
			if s.GenericHandler != nil {
				if err = session.WriteLine(s.GenericHandler(line)); err != nil {
					return
				}
			} else {
				if err = session.WriteLine(fields[0], DefaultCommandNotFound); err != nil {
					return
				}
			}
		}
	}
}
