# telnet-go
[![GoDoc](https://godoc.org/github.com/globalcyberalliance/telnet-go?status.svg)](https://godoc.org/github.com/globalcyberalliance/telnet-go)

A Telnet server and client library implementation written in Go. Heavily inspired by the standard library's `net/http` 
implementation.

_Forked from https://github.com/reiver/go-telnet._

## Get Started

The `telnet-go` library provides low level interfaces for interacting with Telnet data streams, either as a server or 
client.

There's also the `shell` package, which provides a simple Telnet shell server example.

## Setup a Telnet Server
### Basic Server

Before we can host a server, we need to specify a handler. We provide a sample handler as `telnet.EchoHandler` to
illustrate how a handler should be written. This handler echoes whatever the client sends, back to the client.

The following code will serve this handler on your system's primary IP, using port 23 (Telnet's standard port).

```go
package main

import (
	"github.com/globalcyberalliance/telnet-go"
)

func main() {
	if err := telnet.ListenAndServe("0.0.0.0:23", telnet.EchoHandler); err != nil {
		panic(err)
	}
}
```

The echo handler simply echoes client text back to the client upon submission. This is what the client would see:

```shell
~/Projects/globalcyberalliance/telnet-go
❯ telnet localhost
Trying ::1...
Connected to localhost.
Escape character is '^]'.
root
root
exit
exit
^]
telnet> quit
Connection closed.
```

### Shell Server

A common use for Telnet is to act as a shell server (similar to SSH). We provide a simple package that showcases how to 
handle user auth, and how you might want to handle commands.

```go
package main

import (
	"log"

	"github.com/globalcyberalliance/telnet-go"
	"github.com/globalcyberalliance/telnet-go/shell"
)

func main() {
	authHandler := shell.NewAuthHandler("root", "password", 3)
	commands := []shell.Command{
		{
			Regex:    "^docker$",
			Response: "\nUsage:  docker [OPTIONS] COMMAND\r\n",
		},
		{
			Regex:    "^docker .*$",
			Response: "Error response from daemon: dial unix docker.raw.sock: connect: connection refused\r\n",
		},
		{
			Regex:    "^uname$",
			Response: "Linux\r\n",
		},
	}

	srv := shell.Server{AuthHandler: authHandler, Commands: commands}}

	if err := telnet.ListenAndServe("0.0.0.0:23", srv.HandlerFunc); err != nil {
		log.Fatal(err)
	}
}
```

This will serve a simple shell server that accepts `root` and `password` as the username and password respectively, and 
will permit 3 auth attempts.

By default, the `Command` object exposed here accepts regex, and a single string response. This interface is sufficient 
for a simple shell interface; however, you can instead use the `GenericHandler` to manually handle this process yourself.
Here's what that might look like:

```go
package main

import (
	"log"
	"strings"

	"github.com/globalcyberalliance/telnet-go"
	"github.com/globalcyberalliance/telnet-go/shell"
)

func main() {
	authHandler := shell.NewAuthHandler("root", "password", 2)

	srv := shell.Server{AuthHandler: authHandler, GenericHandler: func(command string) string {
		fields := strings.Fields(command)
		if len(fields) == 0 {
			return "missing command\r\n"
		}

		switch fields[0] {
		case "docker":
			return "\nUsage:  docker [OPTIONS] COMMAND\n"
		}

		return fields[0] + ": command not found\r\n"
	}}

	if err := telnet.ListenAndServe("0.0.0.0:23", srv.HandlerFunc); err != nil {
		log.Fatal(err)
	}
}
```

### Creating a Handler

Here's a simple handler. You can write directly to the `io.Writer` and read from the `io.Reader`; however, we provide a
few functions to handle this for you (`telnet.WriteLine` and `telnet.ReadLine` respectively).

This handler will write `Welcome!` to the user, and will await their input. If they send a blank response, the server 
will close the connection after writing `Goodbye!`. If the user enters anything else, it'll simply echo back `You wrote:
whatever the user entered`.

```go
package main

import (
	"context"
	"io"

	"github.com/globalcyberalliance/telnet-go"
)

func main() {
	if err := telnet.ListenAndServe("0.0.0.0:23", YourHandlerFunc); err != nil {
		panic(err)
	}
}

func YourHandlerFunc(session *telnet.Session) {
	if err := session.WriteLine("Welcome!\n"); err != nil {
		return
	}

	for {
		line, err := session.ReadLine()
		if err != nil {
			return
		}

		if len(line) == 0 {
			if err = session.WriteLine("Goodbye!\n"); err != nil {
				return
			}
			return
		}

		if err = session.WriteLine("You wrote: "+line+"\n"); err != nil {
			return
		}
	}
}
```

### Issuing a Telnet Command

You may need to issue a Telnet command to your client. By default, the `telnet.writer` interprets `IAC` (255) as `IAC 
IAC` (255, 255), as Telnet requires this for data streams. To workaround this, we expose a `telnet.WriteCommand` function.
This function prepends `telnet.commandSignature()` to the beginning of the byte slice, to signal to the internal 
`telnet.writer.Write()` function to not escape the upcoming `IAC` (255) byte.

We make use of this in the `shell.AuthHandler` to mask the user's password during the login process:
```go
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
```

## Setup a Telnet Client

Similarly to setting up a server, before we open a client connection we need to specify a caller. We provide a sample
caller as `telnet.EchoCaller` to illustrate how a caller should be written. This caller sends data from `os.Stdin` to 
the given Telnet server, and echoes back the server's response to `os.Stdout`.

The following code will use this caller to handle calling a Telnet server listening on the localhost. The Telnet server 
in our example is a simple handler that repeats the client's submission back to them, with `You wrote: ` prepended.

```go
package main

import (
	"github.com/globalcyberalliance/telnet-go"
)

func main() {
	if err := telnet.DialAndCall("localhost:23", telnet.EchoCaller); err != nil {
		panic(err)
	}
}
```

This is what the client would see:

```shell
Welcome!
hello
You wrote: hello
this is a test
You wrote: this is a test
```

_Note: The stock `telnet.EchoCaller` uses the `telnet.ReadLine` function, which isn't ideal for interacting with a 
server's response. It relies on the server ending its data stream with a newline; however, the server may not do this
(for example, if it's sending an auth prompt)._

## Notes

This fork refactored a lot of the original author's codebase to have a cleaner and easier to use API. We required the 
server implementation internally, so the client implementation still needs some work (as we didn't really need it).