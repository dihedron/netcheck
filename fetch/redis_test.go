package fetch

import (
	"context"
	"log"
	"slices"
	"testing"

	"github.com/redis/go-redis/v9"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestFromRedis(t *testing.T) {
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
		log.Fatalf("Could not start redis: %s", err)
	}
	defer func() {
		if err := redisC.Terminate(ctx); err != nil {
			log.Fatalf("Could not stop redis: %s", err)
		}
	}()

	// Redis is ready, set the key
	path := "../_test/netcheck.json"
	before, _, err := FromFile(path)
	if err != nil {
		log.Fatalf("Could not read file %s: %v", path, err)
	}

	client := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})

	err = client.Set(context.Background(), "/path/to/key", string(before), 0).Err()
	if err != nil {
		log.Fatalf("Could not write key file %s: %v", path, err)
	}

	after, _, err := FromRedis("redis://localhost:6379?db=0&key=/path/to/key")
	if err != nil {
		log.Fatalf("Could not open client: %s", err)
	}

	if slices.Compare(before, after) != 0 {
		log.Fatalf("Invalid vallue read: expected %v, got %v", before, after)
	}
}
