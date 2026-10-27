package loopback

import (
	"net"
	"strings"
	"testing"
)

func TestListenBindsLoopbackOnly(t *testing.T) {
	listener, err := Listen()
	if err != nil {
		t.Fatalf("Listen() failed: %v", err)
	}
	defer listener.Close()

	addr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		t.Fatalf("listener address is %T, want *net.TCPAddr", listener.Addr())
	}
	if addr.Port == 0 {
		t.Fatal("Listen() returned port 0, want an ephemeral port the server can advertise")
	}
	// The tunnel dials 127.0.0.1, so the helper server must never be
	// reachable from the local network.
	if !addr.IP.IsLoopback() {
		t.Fatalf("Listen() bound %s, want a loopback address", addr.IP)
	}
	if addr.IP.String() != listenHost {
		t.Fatalf("Listen() bound %s, want %s", addr.IP, listenHost)
	}
}

func TestListenReturnsUsableListener(t *testing.T) {
	listener, err := Listen()
	if err != nil {
		t.Fatalf("Listen() failed: %v", err)
	}
	defer listener.Close()

	// A second listener on the same port must fail, which proves the first
	// one really holds the port the caller is about to advertise.
	if extra, err := net.Listen("tcp", listener.Addr().String()); err == nil {
		extra.Close()
		t.Fatalf("port %s was still free after Listen()", listener.Addr())
	}

	done := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			done <- err
			return
		}
		_, _ = conn.Write([]byte("ok"))
		conn.Close()
		done <- nil
	}()

	conn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("dialing the listener failed: %v", err)
	}
	defer conn.Close()

	buf := make([]byte, 2)
	if _, err := conn.Read(buf); err != nil {
		t.Fatalf("reading from the listener failed: %v", err)
	}
	if strings.TrimSpace(string(buf)) != "ok" {
		t.Fatalf("read %q, want %q", buf, "ok")
	}
	if err := <-done; err != nil {
		t.Fatalf("accept failed: %v", err)
	}
}
