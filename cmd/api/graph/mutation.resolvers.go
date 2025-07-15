package graph

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
)

// Input types for mutations
type CreateAnomalyInput struct {
	Ticker    string  `json:"ticker"`
	Price     float64 `json:"price"`
	Threshold float64 `json:"threshold"`
	Type      string  `json:"type"`
	Severity  *string `json:"severity,omitempty"`
}

type UpdateAnomalyInput struct {
	Price     *float64 `json:"price,omitempty"`
	Threshold *float64 `json:"threshold,omitempty"`
	Type      *string  `json:"type,omitempty"`
	Severity  *string  `json:"severity,omitempty"`
}

func (r *Resolver) CreateAnomaly(ctx context.Context, input CreateAnomalyInput) (*Anomaly, error) {
	// Validate required fields
	if input.Ticker == "" {
		return nil, fmt.Errorf("ticker is required")
	}
	if input.Price <= 0 {
		return nil, fmt.Errorf("price must be positive")
	}
	if input.Type == "" {
		return nil, fmt.Errorf("type is required")
	}

	// Set default values
	severity := "medium"
	if input.Severity != nil {
		severity = *input.Severity
	}

	timestamp := time.Now().UnixMilli()
	id := fmt.Sprintf("%s_%d", input.Ticker, timestamp)

	anomaly := &Anomaly{
		ID:        id,
		Ticker:    input.Ticker,
		Price:     input.Price,
		Threshold: input.Threshold,
		Type:      input.Type,
		Timestamp: time.UnixMilli(timestamp),
		Severity:  severity,
	}

	// Store anomaly in Redis
	// Instead of marshalling the struct (which uses ISO8601 for time.Time),
	// marshal a map with timestamp as int64
	anomalyMap := map[string]interface{}{
		"id":        anomaly.ID,
		"ticker":    anomaly.Ticker,
		"price":     anomaly.Price,
		"threshold": anomaly.Threshold,
		"type":      anomaly.Type,
		"timestamp": timestamp, // store as int64
		"severity":  anomaly.Severity,
	}
	anomalyJSON, err := json.Marshal(anomalyMap)
	if err != nil {
		return nil, err
	}

	err = r.redis.Client().LPush(ctx, "anomalies", anomalyJSON).Err()
	if err != nil {
		return nil, err
	}

	// Publish to Redis channel for real-time updates
	r.redis.Publish(ctx, "anomalies", anomalyJSON)

	return anomaly, nil
}

func (r *Resolver) UpdateAnomaly(ctx context.Context, id string, input UpdateAnomalyInput) (*Anomaly, error) {
	// Get all anomalies and find the one to update
	anomalies, err := r.redis.Client().LRange(ctx, "anomalies", 0, -1).Result()
	if err != nil && err != redis.Nil {
		return nil, err
	}

	var updatedAnomaly *Anomaly
	var anomalyIndex int64 = -1

	// Find the anomaly to update
	for i, anomalyStr := range anomalies {
		var anomalyData map[string]interface{}
		if err := json.Unmarshal([]byte(anomalyStr), &anomalyData); err != nil {
			continue
		}

		if anomalyData["id"] == id {
			// Update fields if provided
			if input.Price != nil {
				anomalyData["price"] = *input.Price
			}
			if input.Threshold != nil {
				anomalyData["threshold"] = *input.Threshold
			}
			if input.Type != nil {
				anomalyData["type"] = *input.Type
			}
			if input.Severity != nil {
				anomalyData["severity"] = *input.Severity
			}

			// Convert back to model
			updatedAnomaly = &Anomaly{
				ID:        anomalyData["id"].(string),
				Ticker:    anomalyData["ticker"].(string),
				Price:     anomalyData["price"].(float64),
				Threshold: anomalyData["threshold"].(float64),
				Type:      anomalyData["type"].(string),
				Timestamp: time.UnixMilli(int64(anomalyData["timestamp"].(float64))),
				Severity:  anomalyData["severity"].(string),
			}
			anomalyIndex = int64(i)
			break
		}
	}

	if updatedAnomaly == nil {
		return nil, fmt.Errorf("anomaly not found")
	}

	// Update the anomaly in Redis
	updatedJSON, err := json.Marshal(updatedAnomaly)
	if err != nil {
		return nil, err
	}

	err = r.redis.Client().LSet(ctx, "anomalies", anomalyIndex, updatedJSON).Err()
	if err != nil {
		return nil, err
	}

	// Publish update to Redis channel
	r.redis.Publish(ctx, "anomalies", updatedJSON)

	return updatedAnomaly, nil
}

func (r *Resolver) DeleteAnomaly(ctx context.Context, id string) (bool, error) {
	// Get all anomalies and find the one to delete
	anomalies, err := r.redis.Client().LRange(ctx, "anomalies", 0, -1).Result()
	if err != nil && err != redis.Nil {
		return false, err
	}

	var anomalyIndex int64 = -1

	// Find the anomaly to delete
	for i, anomalyStr := range anomalies {
		var anomalyData map[string]interface{}
		if err := json.Unmarshal([]byte(anomalyStr), &anomalyData); err != nil {
			continue
		}

		if anomalyData["id"] == id {
			anomalyIndex = int64(i)
			break
		}
	}

	if anomalyIndex == -1 {
		return false, fmt.Errorf("anomaly not found")
	}

	// Remove the anomaly from Redis
	err = r.redis.Client().LRem(ctx, "anomalies", 1, anomalies[anomalyIndex]).Err()
	if err != nil {
		return false, err
	}

	// Publish deletion to Redis channel
	deletionMsg := map[string]interface{}{
		"action": "delete",
		"id":     id,
	}
	deletionJSON, _ := json.Marshal(deletionMsg)
	r.redis.Publish(ctx, "anomalies", deletionJSON)

	return true, nil
} 