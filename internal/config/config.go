package config

import (
	"context"
	"fmt"

	"github.com/sethvargo/go-envconfig"
	"github.com/tierklinik-dobersberg/apis/gen/go/tkd/events/v1/eventsv1connect"
	"github.com/tierklinik-dobersberg/apis/pkg/discovery"
	"github.com/tierklinik-dobersberg/apis/pkg/discovery/wellknown"
)

type Config struct {
	AllowedOrigins []string `env:"ALLOWED_ORIGINS,default=*"`
	ListenAddress  string   `env:"LISTEN,default=:8081"`
	CUPSServer     struct {
		Address  string `json:"address"`
		Username string `json:"username"`
		Password string `json:"password"`
	}
}

func LoadConfig(ctx context.Context) (*Config, error) {
	var cfg Config

	if err := envconfig.Process(ctx, &cfg); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &cfg, nil
}

func (cfg *Config) ConfigureProviders(ctx context.Context, catalog discovery.Discoverer) (*Providers, error) {
	var events eventsv1connect.EventServiceClient
	if catalog != nil {
		var err error

		events, err = wellknown.EventService.Create(ctx, catalog)
		if err != nil {
			return nil, err
		}
	}

	return &Providers{
		Config:       cfg,
		Catalog:      catalog,
		EventService: events,
	}, nil
}
