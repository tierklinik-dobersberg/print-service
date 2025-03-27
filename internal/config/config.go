package config

import (
	"context"
	"fmt"
	"io/fs"
	"net/http"
	"os"

	"github.com/dcaraxes/gotenberg-go-client/v8"
	"github.com/sethvargo/go-envconfig"
	"github.com/tierklinik-dobersberg/apis/gen/go/tkd/events/v1/eventsv1connect"
	"github.com/tierklinik-dobersberg/apis/gen/go/tkd/longrunning/v1/longrunningv1connect"
	"github.com/tierklinik-dobersberg/apis/pkg/discovery"
	"github.com/tierklinik-dobersberg/apis/pkg/discovery/wellknown"
	"github.com/tierklinik-dobersberg/print-service/internal/cups"
)

type Config struct {
	AllowedOrigins []string `env:"ALLOWED_ORIGINS,default=*"`
	ListenAddress  string   `env:"LISTEN,default=:8081"`
	StoragePath    string   `env:"STORAGE_PATH"`
	Gotenberg      string   `env:"GOTENBERG"`
	CUPSServer     struct {
		Address  string `json:"address" env:"CUPS_ADDRESS,default=localhost:631"`
		Username string `json:"username" env:"CUPS_USER"`
		Password string `json:"password" env:"CUPS_PASSWORD"`
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
	var lrun longrunningv1connect.LongRunningServiceClient
	if catalog != nil {
		var err error

		events, err = wellknown.EventService.Create(ctx, catalog)
		if err != nil {
			return nil, fmt.Errorf("tkd.events.v1.EventService: %w", err)
		}

		lrun, err = wellknown.LongRunningService.Create(ctx, catalog)
		if err != nil {
			return nil, fmt.Errorf("tkd.longrunning.v1.LongRunningService: %w", err)
		}
	}

	cli, err := cups.NewClient(cfg.CUPSServer.Address, cfg.CUPSServer.Username, cfg.CUPSServer.Password)
	if err != nil {
		return nil, fmt.Errorf("failed to configure CUPS client: %w", err)
	}

	var storage fs.FS
	if cfg.StoragePath != "" {
		root, err := os.OpenRoot(cfg.StoragePath)
		if err != nil {
			return nil, fmt.Errorf("failed to open storage path: %w", err)
		}

		storage = root.FS()
	}

	var gotenbergClient *gotenberg.Client
	if cfg.Gotenberg != "" {
		gotenbergClient, err = gotenberg.NewClient(cfg.Gotenberg, http.DefaultClient)
		if err != nil {
			return nil, fmt.Errorf("failed to create gotenberg client: %w", err)
		}
	}

	return &Providers{
		Config:       cfg,
		Catalog:      catalog,
		CUPS:         cli,
		EventService: events,
		LongRunning:  lrun,
		Storage:      storage,
		Gotenberg:    gotenbergClient,
	}, nil
}
