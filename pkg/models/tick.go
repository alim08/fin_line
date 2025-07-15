package models

import (
    "fmt"
    "strconv"
    "time"
    "encoding/json"

    "github.com/alim08/fin_line/pkg/validation"
)

// RawTick represents the untyped data coming from ingest.
type RawTick struct {
    Source    string    `json:"source" validate:"required,source"`
    Symbol    string    `json:"symbol" validate:"required,ticker"`
    Price     float64   `json:"price" validate:"required,price"`
    Timestamp time.Time `json:"timestamp" validate:"required"`
}

// Validate validates the RawTick struct
func (rt RawTick) Validate() error {
    if errors := validation.ValidateStruct(rt); len(errors) > 0 {
        return errors
    }
    return nil
}

// Sanitize cleans and validates the RawTick data
func (rt *RawTick) Sanitize() {
    rt.Source = validation.SanitizeString(rt.Source)
    rt.Symbol = validation.SanitizeString(rt.Symbol)
    rt.Price = validation.SanitizePrice(rt.Price)
    
    // Sanitize timestamp
    if rt.Timestamp.IsZero() {
        rt.Timestamp = time.Now()
    } else if rt.Timestamp.After(time.Now()) {
        rt.Timestamp = time.Now()
    }
}

// ToMap converts RawTick to a map for Redis stream storage
func (rt RawTick) ToMap() map[string]interface{} {
    return map[string]interface{}{
        "source":    rt.Source,
        "symbol":    rt.Symbol,
        "price":     fmt.Sprintf("%.8f", rt.Price),
        "timestamp": rt.Timestamp.Format(time.RFC3339Nano),
    }
}

// FromMap attempts to parse a Redis XMessage .Values into a RawTick.
// Returns an error if required fields are missing or malformed.
func RawTickFromMap(m map[string]interface{}) (RawTick, error) {
    var rt RawTick
    
    // Validate required schema
    schema := map[string]string{
        "source":    "string",
        "symbol":    "string",
        "price":     "float64",
        "timestamp": "string",
    }
    
    if errors := validation.ValidateMap(m, schema); len(errors) > 0 {
        return rt, errors
    }
    
    // Source
    if s, ok := m["source"].(string); ok {
        rt.Source = validation.SanitizeString(s)
    } else {
        return rt, fmt.Errorf("missing or invalid 'source'")
    }
    
    // Symbol
    if s, ok := m["symbol"].(string); ok {
        rt.Symbol = validation.SanitizeString(s)
    } else {
        return rt, fmt.Errorf("missing or invalid 'symbol'")
    }
    
    // Price (could be float64 or string)
    switch v := m["price"].(type) {
    case float64:
        rt.Price = validation.SanitizePrice(v)
    case string:
        p, err := strconv.ParseFloat(v, 64)
        if err != nil {
            return rt, fmt.Errorf("price parse error: %w", err)
        }
        rt.Price = validation.SanitizePrice(p)
    default:
        return rt, fmt.Errorf("missing or invalid 'price'")
    }
    
    // Timestamp (RFC3339 or ms since epoch)
    switch v := m["timestamp"].(type) {
    case string:
        if ts, err := time.Parse(time.RFC3339Nano, v); err == nil {
            rt.Timestamp = ts
        } else if ms, err := strconv.ParseInt(v, 10, 64); err == nil {
            rt.Timestamp = time.UnixMilli(validation.SanitizeTimestamp(ms))
        } else {
            return rt, fmt.Errorf("timestamp parse error: %w", err)
        }
    case float64:
        rt.Timestamp = time.UnixMilli(validation.SanitizeTimestamp(int64(v)))
    default:
        return rt, fmt.Errorf("missing or invalid 'timestamp'")
    }
    
    // Validate the parsed data
    if err := rt.Validate(); err != nil {
        return rt, fmt.Errorf("validation failed: %w", err)
    }
    
    return rt, nil
}

// NormalizedTick is the cleaned, canonicalized form we write out.
type NormalizedTick struct {
    Ticker    string `json:"ticker" validate:"required,ticker"`
    Price     float64 `json:"price" validate:"required,price"`
    Timestamp int64  `json:"timestamp" validate:"required,timestamp"` // milliseconds since epoch (UTC)
    Sector    string `json:"sector" validate:"required,sector"` // from metadata lookup
}

// Validate validates the NormalizedTick struct
func (nt NormalizedTick) Validate() error {
    if errors := validation.ValidateStruct(nt); len(errors) > 0 {
        return errors
    }
    return nil
}

// Sanitize cleans and validates the NormalizedTick data
func (nt *NormalizedTick) Sanitize() {
    nt.Ticker = validation.SanitizeString(nt.Ticker)
    nt.Price = validation.SanitizePrice(nt.Price)
    nt.Timestamp = validation.SanitizeTimestamp(nt.Timestamp)
    nt.Sector = validation.SanitizeString(nt.Sector)
}

// ToMap converts it back to a map for XAdd.
func (nt NormalizedTick) ToMap() map[string]interface{} {
    return map[string]interface{}{
        "ticker":    nt.Ticker,
        "price":     fmt.Sprintf("%.8f", nt.Price),        // string for consistency
        "ts_ms":     nt.Timestamp,
        "sector":    nt.Sector,
    }
}

// ToJSON converts to JSON string for pub/sub
func (nt NormalizedTick) ToJSON() (string, error) {
    data, err := json.Marshal(nt)
    if err != nil {
        return "", fmt.Errorf("json marshal error: %w", err)
    }
    return string(data), nil
}

// FromJSON creates NormalizedTick from JSON string
func NormalizedTickFromJSON(data string) (NormalizedTick, error) {
    var tick NormalizedTick
    if err := json.Unmarshal([]byte(data), &tick); err != nil {
        return tick, fmt.Errorf("json unmarshal error: %w", err)
    }
    
    // Sanitize and validate
    tick.Sanitize()
    if err := tick.Validate(); err != nil {
        return tick, fmt.Errorf("validation failed: %w", err)
    }
    
    return tick, nil
}

// FromMap creates NormalizedTick from Redis stream message
func NormalizedTickFromMap(m map[string]interface{}) (NormalizedTick, error) {
    var nt NormalizedTick
    
    // Validate required schema
    schema := map[string]string{
        "ticker": "string",
        "price":  "float64",
        "ts_ms":  "int64",
        "sector": "string",
    }
    
    if errors := validation.ValidateMap(m, schema); len(errors) > 0 {
        return nt, errors
    }
    
    // Ticker
    if ticker, ok := m["ticker"].(string); ok {
        nt.Ticker = validation.SanitizeString(ticker)
    } else {
        return nt, fmt.Errorf("missing or invalid 'ticker'")
    }
    
    // Price
    switch v := m["price"].(type) {
    case float64:
        nt.Price = validation.SanitizePrice(v)
    case string:
        if price, err := strconv.ParseFloat(v, 64); err == nil {
            nt.Price = validation.SanitizePrice(price)
        } else {
            return nt, fmt.Errorf("price parse error: %w", err)
        }
    default:
        return nt, fmt.Errorf("missing or invalid 'price'")
    }
    
    // Timestamp
    switch v := m["ts_ms"].(type) {
    case int64:
        nt.Timestamp = validation.SanitizeTimestamp(v)
    case string:
        if ts, err := strconv.ParseInt(v, 10, 64); err == nil {
            nt.Timestamp = validation.SanitizeTimestamp(ts)
        } else {
            return nt, fmt.Errorf("timestamp parse error: %w", err)
        }
    case float64:
        nt.Timestamp = validation.SanitizeTimestamp(int64(v))
    default:
        return nt, fmt.Errorf("missing or invalid 'ts_ms'")
    }
    
    // Sector (optional)
    if sector, ok := m["sector"].(string); ok {
        nt.Sector = validation.SanitizeString(sector)
    } else {
        nt.Sector = "unknown" // Default sector
    }
    
    // Validate the parsed data
    if err := nt.Validate(); err != nil {
        return nt, fmt.Errorf("validation failed: %w", err)
    }
    
    return nt, nil
}

// Anomaly represents a detected anomaly event
type Anomaly struct {
    Ticker    string  `json:"ticker" validate:"required,ticker"`
    Price     float64 `json:"price" validate:"required,price"`
    ZScore    float64 `json:"z_score" validate:"required,zscore"`
    Timestamp int64   `json:"timestamp" validate:"required,timestamp"` // milliseconds since epoch (UTC)
}

// Validate validates the Anomaly struct
func (a Anomaly) Validate() error {
    if errors := validation.ValidateStruct(a); len(errors) > 0 {
        return errors
    }
    return nil
}

// Sanitize cleans and validates the Anomaly data
func (a *Anomaly) Sanitize() {
    a.Ticker = validation.SanitizeString(a.Ticker)
    a.Price = validation.SanitizePrice(a.Price)
    a.Timestamp = validation.SanitizeTimestamp(a.Timestamp)
    
    // Sanitize z-score
    if a.ZScore < 0 {
        a.ZScore = 0
    }
    if a.ZScore > 100 {
        a.ZScore = 100
    }
}

// ToMap converts Anomaly to a map for Redis storage
func (a Anomaly) ToMap() map[string]interface{} {
    return map[string]interface{}{
        "ticker":    a.Ticker,
        "price":     fmt.Sprintf("%.8f", a.Price),
        "z":         a.ZScore,
        "ts_ms":     a.Timestamp,
    }
}

// ToJSON converts to JSON string
func (a Anomaly) ToJSON() (string, error) {
    data, err := json.Marshal(a)
    if err != nil {
        return "", fmt.Errorf("json marshal error: %w", err)
    }
    return string(data), nil
}

// FromJSON creates Anomaly from JSON string
func AnomalyFromJSON(data string) (Anomaly, error) {
    var anomaly Anomaly
    if err := json.Unmarshal([]byte(data), &anomaly); err != nil {
        return anomaly, fmt.Errorf("json unmarshal error: %w", err)
    }
    
    // Sanitize and validate
    anomaly.Sanitize()
    if err := anomaly.Validate(); err != nil {
        return anomaly, fmt.Errorf("validation failed: %w", err)
    }
    
    return anomaly, nil
}

// FromMap creates Anomaly from Redis stream message
func AnomalyFromMap(m map[string]interface{}) (Anomaly, error) {
    var a Anomaly
    
    // Validate required schema
    schema := map[string]string{
        "ticker": "string",
        "price":  "float64",
        "z":      "float64",
        "ts_ms":  "int64",
    }
    
    if errors := validation.ValidateMap(m, schema); len(errors) > 0 {
        return a, errors
    }
    
    // Ticker
    if ticker, ok := m["ticker"].(string); ok {
        a.Ticker = validation.SanitizeString(ticker)
    } else {
        return a, fmt.Errorf("missing or invalid 'ticker'")
    }
    
    // Price
    switch v := m["price"].(type) {
    case float64:
        a.Price = validation.SanitizePrice(v)
    case string:
        if price, err := strconv.ParseFloat(v, 64); err == nil {
            a.Price = validation.SanitizePrice(price)
        } else {
            return a, fmt.Errorf("price parse error: %w", err)
        }
    default:
        return a, fmt.Errorf("missing or invalid 'price'")
    }
    
    // Z-Score
    switch v := m["z"].(type) {
    case float64:
        a.ZScore = v
        if a.ZScore < 0 {
            a.ZScore = 0
        }
        if a.ZScore > 100 {
            a.ZScore = 100
        }
    case string:
        if zscore, err := strconv.ParseFloat(v, 64); err == nil {
            a.ZScore = zscore
            if a.ZScore < 0 {
                a.ZScore = 0
            }
            if a.ZScore > 100 {
                a.ZScore = 100
            }
        } else {
            return a, fmt.Errorf("z-score parse error: %w", err)
        }
    default:
        return a, fmt.Errorf("missing or invalid 'z'")
    }
    
    // Timestamp
    switch v := m["ts_ms"].(type) {
    case int64:
        a.Timestamp = validation.SanitizeTimestamp(v)
    case string:
        if ts, err := strconv.ParseInt(v, 10, 64); err == nil {
            a.Timestamp = validation.SanitizeTimestamp(ts)
        } else {
            return a, fmt.Errorf("timestamp parse error: %w", err)
        }
    case float64:
        a.Timestamp = validation.SanitizeTimestamp(int64(v))
    default:
        return a, fmt.Errorf("missing or invalid 'ts_ms'")
    }
    
    // Validate the parsed data
    if err := a.Validate(); err != nil {
        return a, fmt.Errorf("validation failed: %w", err)
    }
    
    return a, nil
}
