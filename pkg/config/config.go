package config

import (
    "flag"
    "fmt"
    "os"
    "strings"
    "strconv"
    "time"
)

type Feed struct {
    URL          string
    Type         string // "websocket" or "http"
    PollInterval time.Duration
    APIKey       string
}

type Config struct {
    RedisURL string
    HTTPPort int
    Feeds    []Feed
    AnomalyWindowSize int
    AnomalyThreshold  float64
    MaxWorkers        int
    BatchSize         int
    MetricsPort       int
}

// Load reads environment variables and application flags (via a local FlagSet),
// strips out any -test.* flags, and validates required fields.
func Load() (*Config, error) {
    // 1. Build a fresh FlagSet so we'd don't collide with `go test` flags
    fs := flag.NewFlagSet("config", flag.ContinueOnError)

    // 2. Define only the flags this package cares about
    var redisURL string
    var httpPort int
    var metricsPort int
    fs.StringVar(&redisURL, "redis", os.Getenv("REDIS_URL"), "Redis connection URL")
    fs.IntVar(&httpPort, "port", 8080, "HTTP listen port")
    fs.IntVar(&metricsPort, "metrics-port", 8082, "Metrics server port")

    // 3. Filter out any -test.* args before parsing
    var appArgs []string
    for _, arg := range os.Args[1:] {
        if strings.HasPrefix(arg, "-test.") {
            continue
        }
        appArgs = append(appArgs, arg)
    }
    if err := fs.Parse(appArgs); err != nil {
        return nil, err
    }

    // 4. Populate our Config struct
    cfg := &Config{
        RedisURL: redisURL,
        HTTPPort: httpPort,
        MetricsPort: metricsPort,
        AnomalyWindowSize: 20,  // Default window size
        AnomalyThreshold:  3.0, // Default threshold (3 standard deviations)
        MaxWorkers:        50,  // Default max concurrent workers
        BatchSize:         100, // Default batch size for processing
    }

    // Check for PORT env var (overrides flag/default if set)
    if portEnv := os.Getenv("PORT"); portEnv != "" {
        if portVal, err := strconv.Atoi(portEnv); err == nil {
            cfg.HTTPPort = portVal
        } else {
            return nil, fmt.Errorf("invalid PORT env var: %v", err)
        }
    }

    // Check for anomaly configuration
    if windowSize := os.Getenv("ANOMALY_WINDOW_SIZE"); windowSize != "" {
        if size, err := strconv.Atoi(windowSize); err == nil {
            cfg.AnomalyWindowSize = size
        }
    }
    
    if threshold := os.Getenv("ANOMALY_THRESHOLD"); threshold != "" {
        if thresh, err := strconv.ParseFloat(threshold, 64); err == nil {
            cfg.AnomalyThreshold = thresh
        }
    }

    // Check for worker configuration
    if maxWorkers := os.Getenv("MAX_WORKERS"); maxWorkers != "" {
        if workers, err := strconv.Atoi(maxWorkers); err == nil {
            cfg.MaxWorkers = workers
        }
    }

    if batchSize := os.Getenv("BATCH_SIZE"); batchSize != "" {
        if size, err := strconv.Atoi(batchSize); err == nil {
            cfg.BatchSize = size
        }
    }

    // 5. Load feed configuration
    if err := cfg.loadFeeds(); err != nil {
        return nil, err
    }

    // 6. Validate required fields
    if cfg.RedisURL == "" {
        return nil, fmt.Errorf("missing required config: REDIS_URL or -redis")
    }
    if len(cfg.Feeds) == 0 {
        return nil, fmt.Errorf("no feeds configured")
    }

    return cfg, nil
}

// loadFeeds loads feed configuration from environment variables
func (c *Config) loadFeeds() error {
    // Legacy support for FEED_URLS
    if env := os.Getenv("FEED_URLS"); env != "" {
        urls := splitAndTrim(env, ",")
        for _, url := range urls {
            feed := Feed{
                URL:          url,
                Type:         "http", // default to HTTP
                PollInterval: 30 * time.Second,
            }
            c.Feeds = append(c.Feeds, feed)
        }
        return nil
    }

    // New feed configuration format
    feedCount := 0
    for {
        feedPrefix := fmt.Sprintf("FEED_%d", feedCount)
        url := os.Getenv(feedPrefix + "_URL")
        if url == "" {
            break
        }

        feed := Feed{
            URL:          url,
            Type:         getEnvOrDefault(feedPrefix+"_TYPE", "http"),
            PollInterval: getDurationEnvOrDefault(feedPrefix+"_POLL_INTERVAL", 30*time.Second),
            APIKey:       os.Getenv(feedPrefix + "_API_KEY"),
        }

        c.Feeds = append(c.Feeds, feed)
        feedCount++
    }

    return nil
}

// splitAndTrim splits s on sep, trims spaces, and drops empty entries.
func splitAndTrim(s, sep string) []string {
    parts := []string{}
    for _, p := range strings.Split(s, sep) {
        if t := strings.TrimSpace(p); t != "" {
            parts = append(parts, t)
        }
    }
    return parts
}

// getEnvOrDefault returns environment variable value or default
func getEnvOrDefault(key, defaultValue string) string {
    if value := os.Getenv(key); value != "" {
        return value
    }
    return defaultValue
}

// getDurationEnvOrDefault returns environment variable as duration or default
func getDurationEnvOrDefault(key string, defaultValue time.Duration) time.Duration {
    if value := os.Getenv(key); value != "" {
        if duration, err := time.ParseDuration(value); err == nil {
            return duration
        }
    }
    return defaultValue
}
