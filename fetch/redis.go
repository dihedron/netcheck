package fetch

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
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

	address = fmt.Sprintf("%s://%s:%s@%s", u.Scheme, username, password, address)
	slog.Debug("retrieving from Redis server", "address", address)

	// // the URL is like redis://<user>:<password>@<host>:<port>/<db_number>/<path/to/key>
	// // see regex101.com to check how I came up with the following regular expression:
	// pattern := regexp.MustCompile(`redis[s]{0,1}://(?:(?:(?:(.*):(.*)))@)*((?:(?:[a-zA-Z]|[a-zA-Z][a-zA-Z0-9\-]*[a-zA-Z0-9])\.)*(?:[A-Za-z]|[A-Za-z][A-Za-z0-9\-]*[A-Za-z0-9]))(?::(\d+))*/(?:(\d*)/)*(.*)`)
	// matches := pattern.FindAllStringSubmatch(path, -1)
	// var key string
	// if len(matches) > 0 {
	// 	username := matches[0][0]
	// 	password := matches[0][1]
	// 	hostname := matches[0][2]
	// 	port := matches[0][3]
	// 	db := matches[0][4]
	// 	key = matches[0][5]
	// 	slog.Debug("address parsed", "username", username, "password", password, "hostname", hostname, "port", port, "db", db, "key", key)
	// }

	opts, err := redis.ParseURL(path)
	if err != nil {
		slog.Error("error reading package from redis", "url", path, "error", err)
		return nil, format.Format(-1), err
	}

	key := u.Path

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
