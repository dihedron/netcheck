package checks

import (
	"log/slog"

	"github.com/dihedron/netcheck/logging"
	"github.com/dihedron/netcheck/pointer"
)

func init() {

	for _, path := range []string{
		"./netcheck.conf",
		"~/netcheck.conf",
	} {
		err := loadDefaultsFrom(path)
		if err == nil {
			slog.Info("defaults loaded", "values", logging.ToJSON(Default))
			return
		}
	}
	if Default == nil {
		Default = &Defaults{
			Timeout:     pointer.To(DefaultTimeout),
			Retries:     pointer.To(DefaultRetries),
			Wait:        pointer.To(DefaultWait),
			Concurrency: pointer.To(DefaultConcurrency),
			Ping: &struct {
				Count    *int     `yaml:"count"`
				Interval *Timeout `yaml:"interval"`
				Size     *int     `yaml:"size"`
			}{
				pointer.To(DefaultPingCount),
				pointer.To(DefaultPingInterval),
				pointer.To(DefaultPingSize),
			},
		}
	}
}
