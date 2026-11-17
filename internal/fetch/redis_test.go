//go:build integration

// Redis runs in a container: these tests need Docker, so they only build with
// -tags integration (CI does; "go test ./..." does not need it).

package fetch

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"testing"

	"github.com/redis/go-redis/v9"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestFromRedis(t *testing.T) {
	// read the file
	path := "../../test/fixtures/netcheck.json"
	before, _, err := FromFile(path)
	if err != nil {
		t.Fatalf("Could not read file %s: %v", path, err)
	}

	// start the test container; the container's Redis port is
	// randomly remapped to a host port which must be retrieved
	ctx := context.Background()
	req := testcontainers.ContainerRequest{
		Image:        "redis:latest",
		ExposedPorts: []string{"6379/tcp"},
		WaitingFor:   wait.ForLog("Ready to accept connections"),
	}
	redisC, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("Could not start redis: %s", err)
	}
	defer func() {
		if err := redisC.Terminate(ctx); err != nil {
			t.Fatalf("Could not stop redis: %s", err)
		}
	}()

	// Redis is ready, retrieve the remapped port
	port, err := redisC.MappedPort(ctx, "6379/tcp")
	if err != nil {
		t.Fatalf("Could not retrieved exposed port: %v", err)
	}

	// set a DB different from the default, to make sure it
	// is properly handled
	db := 7

	slog.Debug("test container ready", "port", port.Int(), "db", db)

	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("localhost:%d", port.Int()),
		Password: "",
		DB:       db,
	})

	err = client.Set(context.Background(), "/path/to/key", string(before), 0).Err()
	if err != nil {
		t.Fatalf("Could not write key file %s: %v", path, err)
	}

	after, _, err := FromRedis(fmt.Sprintf("redis://localhost:%d?db=%d&key=/path/to/key", port.Int(), db))
	if err != nil {
		t.Fatalf("Could not open client: %s", err)
	}

	if slices.Compare(before, after) != 0 {
		t.Fatalf("Invalid value read: expected %v, got %v", before, after)
	}
}
