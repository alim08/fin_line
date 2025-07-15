package config

import (
    "os"
    "reflect"
    "testing"
)

func TestLoad_Valid(t *testing.T) {
    // Set up environment
    t.Setenv("REDIS_URL", "redis://localhost:6379/0")
    t.Setenv("FEED_URLS", "ws://feed1,https://feed2")

    cfg, err := Load()
    if err != nil {
        t.Fatalf("expected no error, got %v", err)
    }
    if cfg.RedisURL != "redis://localhost:6379/0" {
        t.Errorf("RedisURL = %q; want %q", cfg.RedisURL, "redis://localhost:6379/0")
    }
    wantFeeds := []string{"ws://feed1", "https://feed2"}
    if !reflect.DeepEqual(cfg.FeedURLs, wantFeeds) {
        t.Errorf("FeedURLs = %v; want %v", cfg.FeedURLs, wantFeeds)
    }
}

func TestLoad_MissingRedis(t *testing.T) {
    t.Setenv("FEED_URLS", "ws://feed1")
    os.Unsetenv("REDIS_URL")

    _, err := Load()
    if err == nil {
        t.Fatal("expected error due to missing REDIS_URL, got nil")
    }
}

func TestSplitAndTrim(t *testing.T) {
    in := " a , ,b ,c"
    got := splitAndTrim(in, ",")
    want := []string{"a", "b", "c"}
    if !reflect.DeepEqual(got, want) {
        t.Errorf("splitAndTrim = %v; want %v", got, want)
    }
}
