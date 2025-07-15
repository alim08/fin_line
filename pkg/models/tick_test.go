package models

import (
    //"fmt"
    "testing"
    "time"
)

func mustParseTime(t *testing.T, s string) time.Time {
    ts, err := time.Parse(time.RFC3339Nano, s)
    if err != nil {
        t.Fatalf("time.Parse: %v", err)
    }
    return ts
}

func TestRawTickFromMap_Success(t *testing.T) {
    now := mustParseTime(t, "2025-07-10T12:34:56.789Z")
    m := map[string]interface{}{
        "source":    "feedA",
        "symbol":    "BTCUSD",
        "price":     "123.45",
        "timestamp": now.Format(time.RFC3339Nano),
    }

    rt, err := RawTickFromMap(m)
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if rt.Source != "feedA" {
        t.Errorf("Source = %q; want %q", rt.Source, "feedA")
    }
    if rt.Symbol != "BTCUSD" {
        t.Errorf("Symbol = %q; want %q", rt.Symbol, "BTCUSD")
    }
    if rt.Price != 123.45 {
        t.Errorf("Price = %v; want %v", rt.Price, 123.45)
    }
    if !rt.Timestamp.Equal(now) {
        t.Errorf("Timestamp = %v; want %v", rt.Timestamp, now)
    }
}

func TestRawTickFromMap_InvalidCases(t *testing.T) {
    cases := []struct {
        name    string
        input   map[string]interface{}
        wantErr bool
    }{
        {
            name:    "missing source",
            input:   map[string]interface{}{"symbol": "X", "price": 1.0, "timestamp": "2025-01-01T00:00:00Z"},
            wantErr: true,
        },
        {
            name:    "bad price",
            input:   map[string]interface{}{"source": "A", "symbol": "X", "price": "not-a-number", "timestamp": "2025-01-01T00:00:00Z"},
            wantErr: true,
        },
        {
            name:    "bad timestamp",
            input:   map[string]interface{}{"source": "A", "symbol": "X", "price": 1.0, "timestamp": "garbage"},
            wantErr: true,
        },
    }
    for _, c := range cases {
        t.Run(c.name, func(t *testing.T) {
            _, err := RawTickFromMap(c.input)
            if (err != nil) != c.wantErr {
                t.Errorf("err = %v; wantErr %v", err, c.wantErr)
            }
        })
    }
}
