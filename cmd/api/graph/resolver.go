package graph

import (
	"github.com/alim08/fin_line/pkg/redisclient"
)

type Resolver struct {
	redis *redisclient.Client
}

func NewResolver(redis *redisclient.Client) *Resolver {
	return &Resolver{
		redis: redis,
	}
} 