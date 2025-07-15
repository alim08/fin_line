package main

import (
	"context"
	"time"

	"github.com/alim08/fin_line/cmd/api/graph"
	"github.com/graphql-go/graphql"
	"github.com/graphql-go/graphql/language/ast"
)

// Create the GraphQL schema
func createSchema(redisClient *graph.Resolver) graphql.Schema {
	// Define scalar types
	timestampType := graphql.NewScalar(graphql.ScalarConfig{
		Name:        "Timestamp",
		Description: "Timestamp scalar type",
		Serialize: func(value interface{}) interface{} {
			switch v := value.(type) {
			case time.Time:
				return v.Format(time.RFC3339)
			case int64:
				return time.Unix(v, 0).Format(time.RFC3339)
			default:
				return nil
			}
		},
		ParseValue: func(value interface{}) interface{} {
			return value
		},
		ParseLiteral: func(valueAST ast.Value) interface{} {
			return nil // Not implemented, as we don't use literal parsing for timestamps
		},
	})

	// Define Anomaly type
	anomalyType := graphql.NewObject(graphql.ObjectConfig{
		Name: "Anomaly",
		Fields: graphql.Fields{
			"id": &graphql.Field{
				Type: graphql.String,
			},
			"ticker": &graphql.Field{
				Type: graphql.String,
			},
			"price": &graphql.Field{
				Type: graphql.Float,
			},
			"threshold": &graphql.Field{
				Type: graphql.Float,
			},
			"type": &graphql.Field{
				Type: graphql.String,
			},
			"timestamp": &graphql.Field{
				Type: timestampType,
			},
			"severity": &graphql.Field{
				Type: graphql.String,
			},
		},
	})

	// Define Quote type
	quoteType := graphql.NewObject(graphql.ObjectConfig{
		Name: "Quote",
		Fields: graphql.Fields{
			"ticker": &graphql.Field{
				Type: graphql.String,
			},
			"price": &graphql.Field{
				Type: graphql.Float,
			},
			"timestamp": &graphql.Field{
				Type: timestampType,
			},
			"sector": &graphql.Field{
				Type: graphql.String,
			},
		},
	})

	// Define MarketStats type
	marketStatsType := graphql.NewObject(graphql.ObjectConfig{
		Name: "MarketStats",
		Fields: graphql.Fields{
			"total_tickers": &graphql.Field{
				Type: graphql.Int,
			},
			"total_quotes": &graphql.Field{
				Type: graphql.Int,
			},
			"last_update": &graphql.Field{
				Type: timestampType,
			},
		},
	})

	// Define CreateAnomalyInput type
	createAnomalyInputType := graphql.NewInputObject(graphql.InputObjectConfig{
		Name: "CreateAnomalyInput",
		Fields: graphql.InputObjectConfigFieldMap{
			"ticker": &graphql.InputObjectFieldConfig{
				Type: graphql.NewNonNull(graphql.String),
			},
			"price": &graphql.InputObjectFieldConfig{
				Type: graphql.NewNonNull(graphql.Float),
			},
			"threshold": &graphql.InputObjectFieldConfig{
				Type: graphql.NewNonNull(graphql.Float),
			},
			"type": &graphql.InputObjectFieldConfig{
				Type: graphql.NewNonNull(graphql.String),
			},
			"severity": &graphql.InputObjectFieldConfig{
				Type: graphql.String,
			},
		},
	})

	// Define Query type
	queryType := graphql.NewObject(graphql.ObjectConfig{
		Name: "Query",
		Fields: graphql.Fields{
			"anomalies": &graphql.Field{
				Type: graphql.NewList(anomalyType),
				Args: graphql.FieldConfigArgument{
					"limit": &graphql.ArgumentConfig{
						Type: graphql.Int,
					},
					"severity": &graphql.ArgumentConfig{
						Type: graphql.String,
					},
					"type": &graphql.ArgumentConfig{
						Type: graphql.String,
					},
				},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					ctx := context.Background()
					
					var limit *int
					if l, ok := p.Args["limit"].(int); ok {
						limit = &l
					}
					
					var severity *string
					if s, ok := p.Args["severity"].(string); ok {
						severity = &s
					}
					
					var anomalyType *string
					if t, ok := p.Args["type"].(string); ok {
						anomalyType = &t
					}
					
					return redisClient.Anomalies(ctx, limit, severity, anomalyType)
				},
			},
			"anomaliesByTicker": &graphql.Field{
				Type: graphql.NewList(anomalyType),
				Args: graphql.FieldConfigArgument{
					"ticker": &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(graphql.String),
					},
				},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					ctx := context.Background()
					ticker := p.Args["ticker"].(string)
					return redisClient.AnomaliesByTicker(ctx, ticker)
				},
			},
			"quotes": &graphql.Field{
				Type: graphql.NewList(quoteType),
				Args: graphql.FieldConfigArgument{
					"limit": &graphql.ArgumentConfig{
						Type: graphql.Int,
					},
					"ticker": &graphql.ArgumentConfig{
						Type: graphql.String,
					},
				},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					ctx := context.Background()
					
					var limit *int
					if l, ok := p.Args["limit"].(int); ok {
						limit = &l
					}
					
					var ticker *string
					if t, ok := p.Args["ticker"].(string); ok {
						ticker = &t
					}
					
					return redisClient.Quotes(ctx, limit, ticker, nil)
				},
			},
			"latestQuotes": &graphql.Field{
				Type: graphql.NewList(quoteType),
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					ctx := context.Background()
					return redisClient.LatestQuotes(ctx)
				},
			},
			"tickers": &graphql.Field{
				Type: graphql.NewList(graphql.String),
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					ctx := context.Background()
					return redisClient.Tickers(ctx)
				},
			},
			"sectors": &graphql.Field{
				Type: graphql.NewList(graphql.String),
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					ctx := context.Background()
					return redisClient.Sectors(ctx)
				},
			},
			"marketStats": &graphql.Field{
				Type: marketStatsType,
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					ctx := context.Background()
					return redisClient.MarketStats(ctx)
				},
			},
		},
	})

	// Define Mutation type
	mutationType := graphql.NewObject(graphql.ObjectConfig{
		Name: "Mutation",
		Fields: graphql.Fields{
			"createAnomaly": &graphql.Field{
				Type: anomalyType,
				Args: graphql.FieldConfigArgument{
					"input": &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(createAnomalyInputType),
					},
				},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					ctx := context.Background()
					inputMap := p.Args["input"].(map[string]interface{})
					
					input := graph.CreateAnomalyInput{
						Ticker:    inputMap["ticker"].(string),
						Price:     inputMap["price"].(float64),
						Threshold: inputMap["threshold"].(float64),
						Type:      inputMap["type"].(string),
					}
					
					if severity, ok := inputMap["severity"].(string); ok {
						input.Severity = &severity
					}
					
					return redisClient.CreateAnomaly(ctx, input)
				},
			},
		},
	})

	// Create the schema
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    queryType,
		Mutation: mutationType,
	})
	if err != nil {
		panic(err)
	}

	return schema
} 