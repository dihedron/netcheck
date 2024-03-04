package fetch

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"strconv"

	"github.com/dihedron/netcheck/format"
	"github.com/dihedron/netcheck/logging"
	"github.com/redis/go-redis/v9"
)

// FromRedis fetches a resource from a Redis service; the URL must contain the
// scheme (one of redis:// and rediss://), optional authentication info (in the
// form redis://<username>:<password>@host:6379), the name of the host or its IP
// address and the optional port; the query string can specify a db (if not the
// default, which is assumed to be 0) and the key under which the bundle is stored;
// all in all, the URL looks something like the following:
//
//	redis://username:password@redis.example.com:6379?db=5&key=my_key
//
// or
//
//	rediss://redis.example.com:6379?db=3&key=/path/to/my/my_key
func FromRedis(path string) ([]byte, format.Format, error) {
	u, err := url.Parse(path)
	if err != nil {
		slog.Error("error parsing Redis URL", "url", path, "error", err)
		return nil, format.Format(-1), err
	}

	slog.Debug("parsed URL", "value", logging.ToJSON(u))

	username := u.User.Username()
	password, _ := u.User.Password()

	var address string
	if len(username) > 0 || len(password) > 0 {
		address = fmt.Sprintf("%s://%s:%s@%s", u.Scheme, username, password, address)
	} else {
		address = fmt.Sprintf("%s://%s", u.Scheme, address)
	}

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

	slog.Debug("data read from Redis", "key", key, "value", value)

	f, err := format.Detect(value)
	if err != nil {
		slog.Error("error detecting data format", "error", err)
		return nil, format.Format(-1), err
	}

	return []byte(value), f, nil
}
