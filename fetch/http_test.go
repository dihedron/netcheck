package fetch

import (
	"log"
	"log/slog"
	"net/http"
	"path/filepath"
	"testing"
)

func TestFromHTTP(t *testing.T) {
	slog.Debug("hello")

	// func downloadHandler)() {

	// }

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Clean("../test/netcheck.yaml"))
	})

	if err := http.ListenAndServe(":3333", nil); err != nil {
		log.Fatalf("error starting web server: %v", err)
	}
}
