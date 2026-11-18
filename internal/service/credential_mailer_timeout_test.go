package service

import (
	"errors"
	"fmt"
	"net"
	"testing"
	"time"
)

func TestSendSMTPWithTimeoutBoundsServerGreeting(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen for SMTP fixture: %v", err)
	}
	defer listener.Close()

	accepted := make(chan struct{})
	go func() {
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			return
		}
		close(accepted)
		// Deliberately never send the SMTP 220 greeting. The client must
		// abandon this connection at its absolute operation deadline.
		defer conn.Close()
		var one [1]byte
		_, _ = conn.Read(one[:])
	}()

	host, portText, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	var port int
	if _, err := fmt.Sscan(portText, &port); err != nil {
		t.Fatal(err)
	}
	timeout := 120 * time.Millisecond
	started := time.Now()
	err = sendSMTPWithTimeout(host, port, "", "", "sender@example.test", "recipient@example.test", []byte("test"), timeout)
	elapsed := time.Since(started)
	if err == nil {
		t.Fatal("SMTP send unexpectedly succeeded without a server greeting")
	}
	var networkError net.Error
	if !errors.As(err, &networkError) || !networkError.Timeout() {
		t.Fatalf("expected a network timeout, got %T: %v", err, err)
	}
	if elapsed > time.Second {
		t.Fatalf("SMTP operation exceeded bounded timeout: %s", elapsed)
	}
	select {
	case <-accepted:
	case <-time.After(time.Second):
		t.Fatal("SMTP fixture did not accept the client connection")
	}
}
