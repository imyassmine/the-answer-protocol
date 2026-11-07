// Command client is a minimal line-oriented CLI client for the TAP MUD server.
//
// It reads protocol lines from stdin and prints server replies as they arrive.
// The socket reader and the stdin writer run in separate goroutines and the
// process exits cleanly (closing the connection, draining pending replies)
// instead of calling os.Exit from a goroutine.
package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	serverAddr  = "127.0.0.1:4242"
	dialTimeout = 5 * time.Second
	// drainTimeout bounds how long we wait for the last server replies after
	// stdin reaches EOF or the user sends QUIT.
	drainTimeout = 2 * time.Second
)

func main() {
	os.Exit(run())
}

func run() int {
	conn, err := net.DialTimeout("tcp", serverAddr, dialTimeout)
	if err != nil {
		fmt.Fprintln(os.Stderr, "connection error:", err)
		return 1
	}
	defer func() { _ = conn.Close() }()

	// Serialize stdout so the reader and the prompt writer cannot interleave.
	var outMu sync.Mutex
	printOutput := func(format string, args ...any) {
		outMu.Lock()
		defer outMu.Unlock()
		fmt.Printf(format, args...)
	}

	sess := &session{}
	readDone := make(chan error, 1)
	go func() { readDone <- readLoop(conn, printOutput, sess) }()

	writeDone := make(chan error, 1)
	go func() { writeDone <- writeLoop(conn, printOutput, sess) }()

	select {
	case err := <-readDone:
		if err != nil {
			fmt.Fprintln(os.Stderr, "\n[connection lost]", err)
			return 1
		}
		return 0
	case err := <-writeDone:
		if err != nil {
			fmt.Fprintln(os.Stderr, "\n[input error]", err)
		}
		// Half-close the write side so the server sees EOF, then give the
		// reader a moment to print the server's final replies.
		closeWrite(conn)
		select {
		case <-readDone:
		case <-time.After(drainTimeout):
		}
		if err != nil {
			return 1
		}
		return 0
	}
}

// session records why the client stopped, so a clean shutdown can be told
// apart from a connection lost mid-session.
type session struct {
	quitSent    atomic.Bool
	stdinClosed atomic.Bool
}

// readLoop prints every line received from the server until the connection is
// closed or fails. A closed connection is only a clean shutdown when the user
// asked for it (QUIT) or when stdin is already exhausted.
func readLoop(conn net.Conn, print func(string, ...any), sess *session) error {
	reader := bufio.NewReader(conn)
	for {
		line, err := reader.ReadString('\n')
		if line = strings.TrimSpace(line); line != "" {
			print("\n%s\n> ", line)
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				if sess.quitSent.Load() || sess.stdinClosed.Load() {
					return nil
				}
				return errors.New("server closed the connection")
			}
			return err
		}
	}
}

// writeLoop sends each line read from stdin verbatim. It returns when stdin
// ends, on a write error, or after sending QUIT.
func writeLoop(conn net.Conn, print func(string, ...any), sess *session) error {
	defer sess.stdinClosed.Store(true)

	scanner := bufio.NewScanner(os.Stdin)
	print("> ")
	for scanner.Scan() {
		line := scanner.Text()
		quit := strings.EqualFold(strings.TrimSpace(line), "QUIT")
		if quit {
			sess.quitSent.Store(true)
		}
		if _, err := fmt.Fprintf(conn, "%s\n", line); err != nil {
			return fmt.Errorf("send %q: %w", line, err)
		}
		if quit {
			return nil
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read stdin: %w", err)
	}
	return nil
}

// closeWrite half-closes the connection when supported, leaving the read side
// open to collect the server's remaining output.
func closeWrite(conn net.Conn) {
	type closeWriter interface {
		CloseWrite() error
	}
	if cw, ok := conn.(closeWriter); ok {
		_ = cw.CloseWrite()
	}
}
