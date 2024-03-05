package fetch

import (
	"errors"
	"fmt"
	"log/slog"
	"net/url"

	"github.com/dihedron/netcheck/format"
	"github.com/dihedron/netcheck/logging"
	capi "github.com/hashicorp/consul/api"
)

// FromConsulKV fetches a resource from a Consul key/value store; the URL must contain
// the scheme (consulkv://, "consulkvs://" for TLS API calls, or "consulkvs-://" to skip
// TLS certificate verification), optional authentication info either as basic HTTP auth
// (in the form consulkv://<username>:<password>@consul.example.com:8200) or as a token
// (in the form consulkv://:token@consul.example.com:8201), the name of the host or its IP
// address and the optional port; the query string can specify a datacenter ("dc") and must
// specify the key under which the bundle is stored ("key"); all in all, the URL looks
// something like the following:
//
//	consulkv://username:password@consul.example.com:8200?dc=myDC&key=my_key
//
// or
//
//	consulkvs-://:token@redis.example.com:8200?dc=myDC&key=/path/to/my/my_key
func FromConsulKV(path string) ([]byte, format.Format, error) {
	u, err := url.Parse(path)
	if err != nil {
		slog.Error("error parsing Consul URL", "url", path, "error", err)
		return nil, format.Format(-1), err
	}

	slog.Debug("parsed Consul URL", "value", logging.ToJSON(u))

	// prepare the consul API client configuration
	config := capi.DefaultConfig()
	config.Address = u.Host
	switch u.Scheme {
	case "consulkv":
		config.Scheme = "http"
	case "consulkvs":
		config.Scheme = "https"
	case "consulkvs-":
		config.Scheme = "https"
		config.TLSConfig = capi.TLSConfig{
			InsecureSkipVerify: true,
		}
	}

	// retrieve and populate authentication info
	username := u.User.Username()
	password, ok := u.User.Password()
	if len(username) > 0 && ok && len(password) > 0 {
		// basic HTTP auth
		slog.Debug("using HTTP basic authentication", "username", username, "password", password[0:1]+"******"+password[len(password)-1:])
		config.HttpAuth = &capi.HttpBasicAuth{
			Username: username,
			Password: password,
		}
	} else if ok && len(password) > 0 {
		// token-based (e.g. JWT)
		slog.Debug("using token authentication", "token", password[0:1]+"******"+password[len(password)-1:])
		config.Token = password
	}

	dc := u.Query().Get("dc")
	if len(dc) > 0 {
		slog.Debug("selecting Consul datacenter", "dc", dc)
		config.Datacenter = dc
	}

	// get the path to the key
	key := u.Query().Get("key")
	if len(key) == 0 {
		slog.Error("invalid Consul key")
		return nil, format.Format(-1), fmt.Errorf("invalid key")
	}

	client, err := capi.NewClient(config)
	if err != nil {
		slog.Error("error connecting to Consul KV store", "error", err)
		return nil, format.Format(-1), err
	}

	// // Get a handle to the KV API
	// kv := client.KV()

	// // PUT a new KV pair
	// /* 	p := &capi.KVPair{Key: u.RawPath, Value: []byte("1000")}
	//    	_, err = kv.Put(p, nil)
	//    	if err != nil {
	//    		panic(err)
	//    	}
	// */

	// lookup the key
	slog.Debug("looking up key", "key", key)
	pair, meta, err := client.KV().Get(key, &capi.QueryOptions{
		Datacenter: dc,
	})
	if err != nil {
		slog.Error("error looking up consul key", "key", key, "error", err)
		return nil, format.Format(-1), err
	}
	slog.Debug("retrieved value", "key", pair.Key, "value", string(pair.Value), "meta", logging.ToJSON(meta))

	// detect the format
	f, err := format.Detect(string(pair.Value))
	if err != nil {
		slog.Error("error detecting data format", "error", err)
		return nil, format.Format(-1), err
	}

	return pair.Value, f, nil
}

// FromConsulSR fetches a resource from a Consul service registry.
func FromConsulSR(path string) ([]byte, format.Format, error) {
	return nil, format.Format(-1), errors.New("not implemented")
}
