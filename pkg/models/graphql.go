package models

// GraphQL specific types and interfaces can be added here
// This file is a placeholder for GraphQL-related model definitions

// MarketQuote represents a market quote for GraphQL
type MarketQuote struct {
	Ticker    string  `json:"ticker"`
	Price     float64 `json:"price"`
	Timestamp string  `json:"timestamp"`
}