package fetch

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"testing"

	"github.com/dihedron/netcheck/v2/internal/format"
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

func TestFromRedisFake(t *testing.T) {
	addr, db := fakeRedis(t, map[string]string{"bundle": yamlBundle, "json": `{"id":"j"}`, "plain": "just text"})
	data, f, err := FromRedis("redis://" + addr + "?key=bundle&db=5")
	if err != nil || string(data) != yamlBundle || f != format.YAML {
		t.Fatalf("got %q, %v, %v", data, f, err)
	}
	if db() != "5" {
		t.Errorf("database %s selected, want 5", db())
	}
	if _, f, err := FromRedis("redis://" + addr + "?key=json"); err != nil || f != format.JSON {
		t.Errorf("json: %v, %v", f, err)
	}
	if _, _, err := FromRedis("redis://" + addr + "?key=plain"); err == nil {
		t.Error("a value of unknown format must be an error")
	}
	if _, _, err := FromRedis("redis://" + addr + "?key=absent"); err == nil {
		t.Error("a missing key must be an error")
	}
	// credentials in the URL are accepted by the URL parser and sent with AUTH
	// (the fake refuses the command: the error is the client's, not a panic)
	_, _, _ = FromRedis("redis://user:pw@" + addr + "?key=bundle")
	// rediss-:// speaks TLS: a plain fake cannot answer
	if _, _, err := FromRedis("rediss-://" + addr + "?key=bundle"); err == nil {
		t.Error("rediss-:// against a plain server must fail")
	}
}
