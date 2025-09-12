// Package config provides a go-simpler.org/env configuration table and helpers
// for working with the list of key/value lists stored in .env files.
package config

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/adrg/xdg"
	"go-simpler.org/env"
	lol "lol.mleku.dev"
	"lol.mleku.dev/chk"
	"next.orly.dev/pkg/version"
)

// C holds application configuration settings loaded from environment variables
// and default values. It defines parameters for app behaviour, storage
// locations, logging, and network settings used across the relay service.
type C struct {
	AppName     string   `env:"ORLY_APP_NAME" usage:"set a name to display on information about the relay" default:"ORLY"`
	DataDir     string   `env:"ORLY_DATA_DIR" usage:"storage location for the event store" default:"~/.local/share/ORLY"`
	Listen      string   `env:"ORLY_LISTEN" default:"0.0.0.0" usage:"network listen address"`
	Port        int      `env:"ORLY_PORT" default:"3334" usage:"port to listen on"`
	HealthPort  int      `env:"ORLY_HEALTH_PORT" default:"0" usage:"optional health check HTTP port; 0 disables"`
	LogLevel    string   `env:"ORLY_LOG_LEVEL" default:"info" usage:"relay log level: fatal error warn info debug trace"`
	DBLogLevel  string   `env:"ORLY_DB_LOG_LEVEL" default:"info" usage:"database log level: fatal error warn info debug trace"`
	LogToStdout bool     `env:"ORLY_LOG_TO_STDOUT" default:"false" usage:"log to stdout instead of stderr"`
	Pprof       string   `env:"ORLY_PPROF" usage:"enable pprof in modes: cpu,memory,allocation"`
	IPWhitelist []string `env:"ORLY_IP_WHITELIST" usage:"comma-separated list of IP addresses to allow access from, matches on prefixes to allow private subnets, eg 10.0.0 = 10.0.0.0/8"`
	Admins      []string `env:"ORLY_ADMINS" usage:"comma-separated list of admin npubs"`
	Owners      []string `env:"ORLY_OWNERS" usage:"comma-separated list of owner npubs, who have full control of the relay for wipe and restart and other functions"`
	ACLMode     string   `env:"ORLY_ACL_MODE" usage:"ACL mode: follows,none" default:"none"`
}

// New creates and initializes a new configuration object for the relay
// application
//
// # Return Values
//
//   - cfg: A pointer to the initialized configuration struct containing default
//     or environment-provided values
//
//   - err: An error object that is non-nil if any operation during
//     initialization fails
//
// # Expected Behaviour:
//
// Initializes a new configuration instance by loading environment variables and
// checking for a .env file in the default configuration directory. Sets logging
// levels based on configuration values and returns the populated configuration
// or an error if any step fails
func New() (cfg *C, err error) {
	cfg = &C{}
	if err = env.Load(cfg, &env.Options{SliceSep: ","}); chk.T(err) {
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: %s\n\n", err)
		}
		PrintHelp(cfg, os.Stderr)
		os.Exit(0)
	}
	if cfg.DataDir == "" || strings.Contains(cfg.DataDir, "~") {
		cfg.DataDir = filepath.Join(xdg.DataHome, cfg.AppName)
	}
	if GetEnv() {
		PrintEnv(cfg, os.Stdout)
		os.Exit(0)
	}
	if HelpRequested() {
		PrintHelp(cfg, os.Stderr)
		os.Exit(0)
	}
	if cfg.LogToStdout {
		lol.Writer = os.Stdout
	}
	lol.SetLogLevel(cfg.LogLevel)
	return
}

// HelpRequested determines if the command line arguments indicate a request for help
//
// # Return Values
//
//   - help: A boolean value indicating true if a help flag was detected in the
//     command line arguments, false otherwise
//
// # Expected Behaviour
//
// The function checks the first command line argument for common help flags and
// returns true if any of them are present. Returns false if no help flag is found
func HelpRequested() (help bool) {
	if len(os.Args) > 1 {
		switch strings.ToLower(os.Args[1]) {
		case "help", "-h", "--h", "-help", "--help", "?":
			help = true
		}
	}
	return
}

// GetEnv checks if the first command line argument is "env" and returns
// whether the environment configuration should be printed.
//
// # Return Values
//
//   - requested: A boolean indicating true if the 'env' argument was
//     provided, false otherwise.
//
// # Expected Behaviour
//
// The function returns true when the first command line argument is "env"
// (case-insensitive), signalling that the environment configuration should be
// printed. Otherwise, it returns false.
func GetEnv() (requested bool) {
	if len(os.Args) > 1 {
		switch strings.ToLower(os.Args[1]) {
		case "env":
			requested = true
		}
	}
	return
}

// KV is a key/value pair.
type KV struct{ Key, Value string }

// KVSlice is a sortable slice of key/value pairs, designed for managing
// configuration data and enabling operations like merging and sorting based on
// keys.
type KVSlice []KV

func (kv KVSlice) Len() int           { return len(kv) }
func (kv KVSlice) Less(i, j int) bool { return kv[i].Key < kv[j].Key }
func (kv KVSlice) Swap(i, j int)      { kv[i], kv[j] = kv[j], kv[i] }

// Compose merges two KVSlice instances into a new slice where key-value pairs
// from the second slice override any duplicate keys from the first slice.
//
// # Parameters
//
//   - kv2: The second KVSlice whose entries will be merged with the receiver.
//
// # Return Values
//
//   - out: A new KVSlice containing all entries from both slices, with keys
//     from kv2 taking precedence over keys from the receiver.
//
// # Expected Behaviour
//
// The method returns a new KVSlice that combines the contents of the receiver
// and kv2. If any key exists in both slices, the value from kv2 is used. The
// resulting slice remains sorted by keys as per the KVSlice implementation.
func (kv KVSlice) Compose(kv2 KVSlice) (out KVSlice) {
	// duplicate the initial KVSlice
	for _, p := range kv {
		out = append(out, p)
	}
out:
	for i, p := range kv2 {
		for j, q := range out {
			// if the key is repeated, replace the value
			if p.Key == q.Key {
				out[j].Value = kv2[i].Value
				continue out
			}
		}
		out = append(out, p)
	}
	return
}

// EnvKV generates key/value pairs from a configuration object's struct tags
//
// # Parameters
//
//   - cfg: A configuration object whose struct fields are processed for env tags
//
// # Return Values
//
//   - m: A KVSlice containing key/value pairs derived from the config's env tags
//
// # Expected Behaviour
//
// Processes each field of the config object, extracting values tagged with
// "env" and converting them to strings. Skips fields without an "env" tag.
// Handles various value types including strings, integers, booleans, durations,
// and string slices by joining elements with commas.
func EnvKV(cfg any) (m KVSlice) {
	t := reflect.TypeOf(cfg)
	for i := 0; i < t.NumField(); i++ {
		k := t.Field(i).Tag.Get("env")
		v := reflect.ValueOf(cfg).Field(i).Interface()
		var val string
		switch v.(type) {
		case string:
			val = v.(string)
		case int, bool, time.Duration:
			val = fmt.Sprint(v)
		case []string:
			arr := v.([]string)
			if len(arr) > 0 {
				val = strings.Join(arr, ",")
			}
		}
		// this can happen with embedded structs
		if k == "" {
			continue
		}
		m = append(m, KV{k, val})
	}
	return
}

// PrintEnv outputs sorted environment key/value pairs from a configuration object
// to the provided writer
//
// # Parameters
//
//   - cfg: Pointer to the configuration object containing env tags
//
//   - printer: Destination for the output, typically an io.Writer implementation
//
// # Expected Behaviour
//
// Outputs each environment variable derived from the config's struct tags in
// sorted order, formatted as "key=value\n" to the specified writer
func PrintEnv(cfg *C, printer io.Writer) {
	kvs := EnvKV(*cfg)
	sort.Sort(kvs)
	for _, v := range kvs {
		_, _ = fmt.Fprintf(printer, "%s=%s\n", v.Key, v.Value)
	}
}

// PrintHelp prints help information including application version, environment
// variable configuration, and details about .env file handling to the provided
// writer
//
// # Parameters
//
//   - cfg: Configuration object containing app name and config directory path
//
//   - printer: Output destination for the help text
//
// # Expected Behaviour
//
// Prints application name and version followed by environment variable
// configuration details, explains .env file behaviour including automatic
// loading and custom path options, and displays current configuration values
// using PrintEnv. Outputs all information to the specified writer
func PrintHelp(cfg *C, printer io.Writer) {
	_, _ = fmt.Fprintf(
		printer,
		"%s %s\n\n", cfg.AppName, version.V,
	)
	_, _ = fmt.Fprintf(
		printer,
		`Usage: %s [env|help]

- env: print environment variables configuring %s
- help: print this help text

`,
		cfg.AppName, cfg.AppName,
	)
	_, _ = fmt.Fprintf(
		printer,
		"Environment variables that configure %s:\n\n", cfg.AppName,
	)
	env.Usage(cfg, printer, &env.Options{SliceSep: ","})
	fmt.Fprintf(printer, "\ncurrent configuration:\n\n")
	PrintEnv(cfg, printer)
	fmt.Fprintln(printer)
	return
}
