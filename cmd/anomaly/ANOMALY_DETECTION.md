# Anomaly Detection System

## Overview

The anomaly detection system in this financial market data project uses statistical analysis to identify unusual price movements in real-time. It's designed to detect price spikes, drops, and other market anomalies that deviate significantly from normal trading patterns.

## Architecture

```
┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│   Ingest    │───▶│ Normalize   │───▶│   Anomaly   │
│   Service   │    │   Service   │    │  Detector   │
└─────────────┘    └─────────────┘    └─────────────┘
                           │                   │
                           ▼                   ▼
                    ┌─────────────┐    ┌─────────────┐
                    │   Redis     │    │   Redis     │
                    │  Streams    │    │  Anomalies  │
                    └─────────────┘    └─────────────┘
                                              │
                                              ▼
                                       ┌─────────────┐
                                       │    API      │
                                       │  (GraphQL)  │
                                       └─────────────┘
```

## How It Works

### 1. Data Flow
1. **Ingest Service** receives raw market data from external feeds
2. **Normalize Service** cleans and standardizes the data
3. **Anomaly Detector** subscribes to normalized data via Redis pub/sub
4. **API Service** provides GraphQL interface to query anomalies

### 2. Statistical Detection Algorithm

The system uses a **rolling window approach** with Z-score analysis:

```go
// Key components:
type rollingWindow struct {
    buf        []float64  // Fixed-size ring buffer
    sum, sqsum float64    // Running sum and sum of squares
    idx        int        // Current position
    full       bool       // Whether buffer is full
}
```

**Detection Process:**
1. **Window Management**: Maintains a fixed-size window (default: 20 data points) per ticker
2. **Statistics Calculation**: Computes mean and standard deviation using running sums
3. **Z-Score Calculation**: `z = |(current_price - mean) / std_deviation|`
4. **Threshold Comparison**: Triggers anomaly if `z >= threshold` (default: 3.0)

### 3. Configuration

Environment variables control detection sensitivity:

```bash
# Detection sensitivity (default: 3.0)
export ANOMALY_THRESHOLD=2.5  # Lower = more sensitive

# Window size for calculations (default: 20)
export ANOMALY_WINDOW_SIZE=15 # Smaller = faster detection

# Redis connection
export REDIS_URL="redis://localhost:6379"
```

## Types of Anomalies Detected

### 1. Price Spikes
- **Detection**: Sudden price increases beyond normal variation
- **Example**: Stock jumps 50% in one tick
- **Z-Score**: Positive deviation from mean

### 2. Price Drops
- **Detection**: Sudden price decreases beyond normal variation
- **Example**: Stock drops 40% in one tick
- **Z-Score**: Negative deviation from mean

### 3. Volatility Anomalies
- **Detection**: Unusual price volatility patterns
- **Example**: Erratic price movements
- **Z-Score**: Based on price change magnitude

## Data Storage

### Redis Streams
- **`anomalies:stream`**: Time-series storage of all anomalies
- **`anomalies:{TICKER}`**: Sorted sets for ticker-specific queries

### Anomaly Structure
```json
{
  "ticker": "AAPL",
  "price": 150.00,
  "z": 4.2,
  "ts_ms": 1703123456789
}
```

## API Interface

### GraphQL Queries

**Get All Anomalies:**
```graphql
query {
  anomalies {
    id
    ticker
    price
    threshold
    type
    timestamp
    severity
  }
}
```

**Filter by Severity:**
```graphql
query {
  anomalies(severity: "high") {
    id
    ticker
    price
    type
    severity
  }
}
```

**Filter by Ticker:**
```graphql
query {
  anomalies(ticker: "AAPL") {
    id
    ticker
    price
    type
    timestamp
  }
}
```

### GraphQL Mutations

**Create Manual Anomaly:**
```graphql
mutation {
  createAnomaly(input: {
    ticker: "TEST"
    price: 999.99
    threshold: 5.0
    type: "spike"
    severity: "high"
  }) {
    id
    ticker
    price
    type
    severity
  }
}
```

## Testing the System

### 1. Run the Test Script
```bash
./test_anomaly_detection.sh
```

This script will:
- Simulate normal market data
- Inject price spikes and drops
- Query anomalies via GraphQL
- Test filtering and creation

### 2. Manual Testing

**Start Services:**
```bash
# Terminal 1: Start anomaly detector
go run cmd/anomaly/main.go

# Terminal 2: Start API server
go run cmd/api/main.go

# Terminal 3: Simulate data
redis-cli XADD "quotes:pubsub" "*" ticker "AAPL" price "100.00" timestamp "$(date +%s%3N)"
```

**Query Anomalies:**
```bash
curl -X POST http://localhost:8080/graphql \
  -H "Content-Type: application/json" \
  -d '{"query": "query { anomalies { ticker price type severity } }"}'
```

## Performance Characteristics

### Time Complexity
- **Window Updates**: O(1) using ring buffer
- **Statistics Calculation**: O(1) using running sums
- **Anomaly Detection**: O(1) per data point

### Memory Usage
- **Per Ticker**: ~160 bytes (20 float64 values + metadata)
- **Scalability**: Linear with number of tickers

### Latency
- **Detection**: < 1ms per data point
- **Storage**: < 5ms for Redis operations
- **Query**: < 10ms for GraphQL responses

## Monitoring & Metrics

### Prometheus Metrics
- `fin_line_anomaly_detected_total`: Total anomalies detected
- `fin_line_anomaly_errors_total`: Detection errors
- `fin_line_anomaly_processing_duration_seconds`: Processing time

### Health Checks
```bash
curl http://localhost:8080/health
curl http://localhost:8080/metrics
```

## Troubleshooting

### Common Issues

1. **No Anomalies Detected**
   - Check `ANOMALY_THRESHOLD` (try lowering to 2.0)
   - Verify data is flowing through Redis streams
   - Check anomaly detector logs

2. **Too Many False Positives**
   - Increase `ANOMALY_THRESHOLD` (try 3.5 or 4.0)
   - Increase `ANOMALY_WINDOW_SIZE` for more stable baseline

3. **High Latency**
   - Check Redis connection performance
   - Monitor system resources
   - Consider reducing window size

### Debug Commands
```bash
# Check Redis streams
redis-cli XRANGE "quotes:pubsub" - + | tail -5
redis-cli XRANGE "anomalies:stream" - + | tail -5

# Check anomaly sets
redis-cli ZRANGE "anomalies:AAPL" -10 -1

# Monitor Redis in real-time
redis-cli MONITOR
```

## Future Enhancements

1. **Machine Learning**: Replace statistical methods with ML models
2. **Volume Analysis**: Include volume data in anomaly detection
3. **Pattern Recognition**: Detect complex market patterns
4. **Alerting**: Real-time notifications for critical anomalies
5. **Backtesting**: Historical anomaly analysis capabilities 