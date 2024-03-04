package fetch

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"strconv"
	"strings"

	"github.com/dihedron/netcheck/format"
	"github.com/dihedron/netcheck/logging"
	"github.com/redis/go-redis/v9"
)

func FromRedis(path string) ([]byte, format.Format, error) {
	u, err := url.Parse(path)
	if err != nil {
		slog.Error("error parsing Redis URL", "url", path, "error", err)
		return nil, format.Format(-1), err
	}

	slog.Debug("parsed URL", "value", logging.ToJSON(u))

	username := u.User.Username()
	password, _ := u.User.Password()
	address := u.Host

	if len(username) > 0 || len(password) > 0 {
		address = fmt.Sprintf("%s://%s:%s@%s", u.Scheme, username, password, address)
	} else {
		address = fmt.Sprintf("%s://%s", u.Scheme, address)
	}

	//address = fmt.Sprintf("%s://%s:%s@%s", u.Scheme, username, password, address)
	slog.Debug("retrieving from Redis server", "address", address)

	opts, err := redis.ParseURL(address)
	if err != nil {
		slog.Error("error parsing Redis URL", "url", address, "error", err)
		return nil, format.Format(-1), err
	}

	db := int64(0)
	if len(u.Query().Get("db")) != 0 {
		db, err = strconv.ParseInt(u.Query().Get("db"), 10, 16)
		if err != nil {
			slog.Error("error parsing DB", "value", u.Query().Get("db"), "error", err)
			return nil, format.Format(-1), err
		}
		slog.Debug("connecting to Redis with non default DB", "db", db)
		opts.DB = int(db)
	}
	key := u.Query().Get("key")
	if len(key) == 0 {
		slog.Error("invalid Redis key")
		return nil, format.Format(-1), fmt.Errorf("invalid key")
	}

	client := redis.NewClient(opts)
	value, err := client.Get(context.Background(), key).Result()
	if err != nil {
		slog.Error("error getting key from Redis", "key", key, "error", err)
		return nil, format.Format(-1), err
	}

	var f format.Format

	slog.Debug("data read from Redis", "key", key, "value", value)
	trimmed := strings.TrimLeft(value, "\n\r\t")
	if strings.HasPrefix(trimmed, "---") {
		f = format.YAML
	} else if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
		f = format.JSON
	} else {
		f = format.TOML
	}
	data := []byte(value)

	return data, f, nil
}
