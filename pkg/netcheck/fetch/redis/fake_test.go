package redis

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"testing"
)

// fakeRedis is a minimal RESP server: GET returns the value of a key from
// values, SELECT and CLIENT are accepted, anything else is an error (HELLO
// included, so the client falls back to RESP2). It records the SELECTed database.
func fakeRedis(t *testing.T, values map[string]string) (addr string, db func() string) {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = l.Close() })
	selected := "0"
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			go func() {
				defer func() { _ = c.Close() }()
				r := bufio.NewReader(c)
				for {
					args, err := readCommand(r)
					if err != nil {
						return
					}
					switch strings.ToUpper(args[0]) {
					case "CLIENT":
						_, _ = io.WriteString(c, "+OK\r\n")
					case "SELECT":
						selected = args[1]
						_, _ = io.WriteString(c, "+OK\r\n")
					case "GET":
						if v, ok := values[args[1]]; ok {
							_, _ = fmt.Fprintf(c, "$%d\r\n%s\r\n", len(v), v)
						} else {
							_, _ = io.WriteString(c, "$-1\r\n")
						}
					default:
						_, _ = io.WriteString(c, "-ERR unknown command '"+args[0]+"'\r\n")
					}
				}
			}()
		}
	}()
	return l.Addr().String(), func() string { return selected }
}

// readCommand reads one RESP array of bulk strings.
func readCommand(r *bufio.Reader) ([]string, error) {
	line, err := r.ReadString('\n')
	if err != nil {
		return nil, err
	}
	n, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(line, "*")))
	if err != nil {
		return nil, err
	}
	args := make([]string, 0, n)
	for range n {
		header, err := r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		size, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(header, "$")))
		if err != nil {
			return nil, err
		}
		buf := make([]byte, size+2)
		if _, err := io.ReadFull(r, buf); err != nil {
			return nil, err
		}
		args = append(args, string(buf[:size]))
	}
	return args, nil
}

// silentListener accepts connections and never answers.
func silentListener(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	t.Cleanup(func() { close(done); _ = l.Close() })
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			go func() { <-done; _ = c.Close() }()
		}
	}()
	return l.Addr().String()
}
