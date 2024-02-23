package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strings"

	"github.com/dihedron/netcheck/checks"
	"github.com/dihedron/netcheck/version"
	"github.com/fatih/color"
	capi "github.com/hashicorp/consul/api"
	"github.com/jessevdk/go-flags"
	"github.com/mattn/go-isatty"
	"github.com/pelletier/go-toml/v2"
	"gopkg.in/yaml.v3"
)

func init() {
	options := &slog.HandlerOptions{
		Level:     slog.LevelWarn,
		AddSource: true,
	}

	level, ok := os.LookupEnv("NETCHECK_LOG_LEVEL")
	if ok {
		switch strings.ToLower(level) {
		case "debug", "dbg", "d", "trace", "trc", "t":
			options.Level = slog.LevelDebug
		case "informational", "info", "inf", "i":
			options.Level = slog.LevelInfo
		case "warning", "warn", "wrn", "w":
			options.Level = slog.LevelWarn
		case "error", "err", "e", "fatal", "ftl", "f":
			options.Level = slog.LevelError
		}
	}
	handler := slog.NewTextHandler(os.Stderr, options)
	slog.SetDefault(slog.New(handler))
}

var (
	red    = color.New(color.FgRed).FprintfFunc()
	green  = color.New(color.FgGreen).FprintfFunc()
	yellow = color.New(color.FgYellow).FprintfFunc()
	blue   = color.New(color.FgBlue).FprintfFunc()
)

func consul() {
	// Get a new client
	u, _ := url.Parse("consul://username:password@myconsul.example.com:8501/path/to/bucket/then/this/is/the/key")
	cfg := capi.DefaultConfig()
	cfg.Address = u.Host
	password, ok := u.User.Password()
	if len(u.User.Username()) > 0 && ok {
		cfg.HttpAuth = &capi.HttpBasicAuth{
			Username: u.User.Username(),
			Password: password,
		}
	}
	client, err := capi.NewClient(cfg)
	if err != nil {
		panic(err)
	}

	// Get a handle to the KV API
	kv := client.KV()

	// PUT a new KV pair
	/* 	p := &capi.KVPair{Key: u.RawPath, Value: []byte("1000")}
	   	_, err = kv.Put(p, nil)
	   	if err != nil {
	   		panic(err)
	   	}
	*/
	// Lookup the pair
	pair, _, err := kv.Get(u.RawPath, nil)
	if err != nil {
		panic(err)
	}
	fmt.Printf("KV: %v %s\n", pair.Key, pair.Value)
}

func main() {

	var options struct {
		Version      bool   `short:"v" long:"version" description:"Show vesion information"`
		Format       string `short:"f" long:"format" choice:"json" choice:"yaml" choice:"toml" choice:"text" optional:"true" default:"text"`
		WithTriggers bool   `short:"t" long:"with-triggers" optional:"true" hidden:"true"`
		WithSpinner  bool   `short:"s" long:"with-spinner" optional:"true" hidden:"true"`
	}

	args, err := flags.Parse(&options)
	if err != nil {
		slog.Error("error parsing command line", "error", err)
		os.Exit(1)
	}
	/*
		if options.WithSpinner {
			s := spinner.New(
				spinner.WithSpeed(200*time.Millisecond),
				//spinner.WithSequence(spinner.RotatingArcsSequence),
				//spinner.WithSequence(spinner.RotatingTrianglesSequence),
				//spinner.WithSequence(spinner.P6),
				spinner.WithSequence(spinner.PulsatingLozengeSequence),
				//spinner.WithSequence(spinner.PulsatingCirclesSequence),
			)
			s.Start()

			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				<-time.After(10 * time.Second)
				s.Stop()
				wg.Done()
			}()

			wg.Wait()
			os.Exit(0)
		}
	*/
	if options.Version && options.Format == "text" {
		fmt.Printf("%s v%s.%s.%s (%s/%s built with %s on %s)\n", version.Name, version.VersionMajor, version.VersionMinor, version.VersionPatch, version.GoOS, version.GoArch, version.GoVersion, version.BuildTime)
	}

	bundles := map[string][]checks.Result{}

	for _, arg := range args {
		bundle, err := checks.New(arg)
		if err != nil {
			slog.Error("error loading package", "path", arg, "error", err)
			os.Exit(1)
		}

		switch options.Format {
		case "text":
			if isatty.IsTerminal(os.Stdout.Fd()) {
				yellow(os.Stdout, "► %s\n", bundle.ID)
				for _, result := range bundle.Check(options.WithTriggers) {
					if result.Success {
						green(os.Stdout, "▲ %-4s → %s\n", result.Protocol, result.Endpoint) // was ✔
					} else {
						red(os.Stdout, "▼ %-4s → %s\n", result.Protocol, result.Endpoint) // was ✖
					}
					for _, action := range result.Actions {
						blue(os.Stdout, "----------------------------------------------------------------\n")
						blue(os.Stdout, "%s (exit code %d):\n", strings.Join(action.Command, " "), action.ExitCode)
						blue(os.Stdout, "stdout:\n%s\n", action.Stdout)
						blue(os.Stdout, "stderr:\n%s\n", action.Stderr)
					}
					if len(result.Actions) > 0 {
						blue(os.Stdout, "----------------------------------------------------------------\n")
					}
				}
			} else {
				fmt.Printf("package: %s\n", bundle.ID)
				for _, result := range bundle.Check(options.WithTriggers) {
					if result.Success {
						fmt.Printf(" - %s/%s: ok\n", result.Protocol, result.Endpoint)
					} else {
						fmt.Printf(" - %s/%s: ko\n", result.Protocol, result.Endpoint)
					}
				}
			}
		default:
			bundles[bundle.ID] = bundle.Check(options.WithTriggers)
		}
	}

	switch options.Format {
	case "json":
		data, err := json.MarshalIndent(bundles, "", "  ")
		if err != nil {
			slog.Error("error marshalling results to JSON", "error", err)
			os.Exit(1)
		}
		fmt.Printf("%s\n", string(data))
	case "yaml":
		data, err := yaml.Marshal(bundles)
		if err != nil {
			slog.Error("error marshalling results to YAML", "error", err)
			os.Exit(1)
		}
		fmt.Printf("%s\n", string(data))
	case "toml":
		data, err := toml.Marshal(bundles)
		if err != nil {
			slog.Error("error marshalling results to TOML", "error", err)
			os.Exit(1)
		}
		fmt.Printf("%s\n", string(data))
	}
}
