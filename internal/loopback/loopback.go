// Package loopback binds the helpers Exposely runs on the user's behalf —
// currently the static file server used when sharing an HTML folder — to
// the loopback interface only.
//
// The tunnel always dials 127.0.0.1, so a loopback bind is all that is
// needed, and it keeps the served folder off the local network: binding
// 0.0.0.0 would let anyone on the same Wi-Fi or LAN read the project
// directory without any authentication.
package loopback

import (
	"fmt"
	"net"
)

// listenAttempts is how many times a fresh ephemeral port is requested
// before giving up, and listenHost is the only interface helpers bind to.
const (
	listenAttempts = 5
	listenHost     = "127.0.0.1"
)

// Listen binds an ephemeral port on 127.0.0.1 and returns the listener.
//
// A single bind can fail even on a healthy machine because Windows keeps
// "excluded port ranges" for Hyper-V, WSL and Docker: when the ephemeral
// port picked by the OS falls inside one of those ranges the bind is
// refused. Retrying asks for a different port instead of falling back to
// a wildcard bind, which would expose the shared folder to the network.
func Listen() (net.Listener, error) {
	var lastErr error
	for attempt := 0; attempt < listenAttempts; attempt++ {
		listener, err := net.Listen("tcp", listenHost+":0")
		if err == nil {
			return listener, nil
		}
		lastErr = err
	}
	return nil, fmt.Errorf("no free loopback port after %d attempts: %w", listenAttempts, lastErr)
}
