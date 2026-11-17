package fetch_test

import (
	"context"
	"fmt"
	"net/url"

	"github.com/dihedron/netcheck/v2/pkg/netcheck/fetch"
)

func ExampleParse() {
	bundle, err := fetch.Parse([]byte("id: example\nchecks:\n  - address: db.example.com:5432\n"), fetch.Unknown)
	if err != nil {
		fmt.Println(err)
		return
	}
	// a check read without a protocol is a tcp check
	fmt.Println(bundle.ID, bundle.Checks[0].Address, bundle.Checks[0].Protocol)
	// Output: example db.example.com:5432 tcp
}

// inline is a source that serves a bundle kept in the location itself.
type inline struct{}

func (inline) Fetch(_ context.Context, u *url.URL, _ int64) ([]byte, fetch.Format, error) {
	return []byte(`{"id": "` + u.Host + `", "checks": [{"address": "127.0.0.1:1"}]}`), fetch.Unknown, nil
}

func ExampleWithSource() {
	// a scheme of your own: a source returns the bytes and, if it knows it,
	// the format; Load parses them
	bundle, err := fetch.Load(context.Background(), "inline://my-bundle", fetch.WithSource(inline{}, "inline"))
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(bundle.ID)
	// Output: my-bundle
}
