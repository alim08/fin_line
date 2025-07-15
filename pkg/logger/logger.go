package logger

import (
  "go.uber.org/zap"
  "go.uber.org/zap/zapcore"
  "os"
  "strings"
)

var Log *zap.Logger

// Init sets up a global logger. Call once in main().
func Init() error {
  // Use production config for JSON+level filtering; swap to zap.NewDevelopment() if you prefer console
  cfg := zap.NewProductionConfig()
  cfg.EncoderConfig.TimeKey = "ts"
  cfg.EncoderConfig.MessageKey = "msg"
  // e.g. override level via env
  if level := os.Getenv("LOG_LEVEL"); level != "" {
    cfg.Level.SetLevel(parseLevel(level))
  }
  var err error
  Log, err = cfg.Build()
  return err
}

// parseLevel is a helper mapping strings to zapcore.Level
func parseLevel(s string) zapcore.Level {
  switch strings.ToLower(s) {
  case "debug":
    return zapcore.DebugLevel
  case "warn":
    return zapcore.WarnLevel
  case "error":
    return zapcore.ErrorLevel
  default:
    return zapcore.InfoLevel
  }
}
