//go:build integration

// Redis runs in a container: this test needs Docker, so it only builds with
// -tags integration (CI does; "go test ./..." does not need it).

package redis

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	goredis "github.com/redis/go-redis/v9"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestAgainstARealRedis(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "test", "fixtures", "netcheck.json"))
	if err != nil {
		t.Fatal(err)
	}
	ctx := t.Context()
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image: "redis:latest", ExposedPorts: []string{"6379/tcp"}, WaitingFor: wait.ForLog("Ready to accept connections"),
		},
		Started: true,
	})
	if err != nil {
		t.Fatalf("could not start redis: %v", err)
	}
	defer func() { _ = c.Terminate(ctx) }()
	port, err := c.MappedPort(ctx, "6379/tcp")
	if err != nil {
		t.Fatal(err)
	}
	// a database other than the default, to prove it is selected
	client := goredis.NewClient(&goredis.Options{Addr: fmt.Sprintf("localhost:%d", int(port.Num())), DB: 7})
	defer func() { _ = client.Close() }()
	if err := client.Set(ctx, "/path/to/key", string(data), 0).Err(); err != nil {
		t.Fatal(err)
	}
	id, err := load(t, fmt.Sprintf("redis://localhost:%d?db=7&key=/path/to/key", int(port.Num())))
	if err != nil || id == "" {
		t.Fatalf("%q %v", id, err)
	}
}
