// Package config reads process configuration from the environment.
package config

import (
	"os"
	"strconv"
	"time"
)

// Config is the immutable process configuration. Runtime-tunable options
// (link mode, hostname overrides, probes) live in the SQLite settings store.
type Config struct {
	Listen             string
	DockerHost         string
	UnraidAPIURL       string
	UnraidAPIKey       string
	TemplatesDir       string
	DataDir            string
	ReconcileInterval  time.Duration
	EventDebounce      time.Duration
	ProbeConnectTO     time.Duration
	ProbeTotalTO       time.Duration
	ProbeConcurrency   int
	LogLevel           string
	Version            string
}

// Version is stamped at build time via -ldflags.
var Version = "dev"

// FromEnv builds a Config from environment variables with safe defaults.
func FromEnv() Config {
	return Config{
		Listen:            getenv("ARRAYDECK_LISTEN", ":8417"),
		DockerHost:        getenv("DOCKER_HOST", "unix:///var/run/docker.sock"),
		UnraidAPIURL:      os.Getenv("UNRAID_API_URL"),
		UnraidAPIKey:      os.Getenv("UNRAID_API_KEY"),
		TemplatesDir:      getenv("TEMPLATES_DIR", "/unraid/templates"),
		DataDir:           getenv("DATA_DIR", "/data"),
		ReconcileInterval: getDuration("RECONCILE_INTERVAL_SECONDS", 30*time.Second),
		EventDebounce:     400 * time.Millisecond,
		ProbeConnectTO:    750 * time.Millisecond,
		ProbeTotalTO:      1500 * time.Millisecond,
		ProbeConcurrency:  8,
		LogLevel:          getenv("LOG_LEVEL", "info"),
		Version:           Version,
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return time.Duration(n) * time.Second
		}
	}
	return fallback
}
