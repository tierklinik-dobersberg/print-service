package config

import (
	"github.com/tierklinik-dobersberg/apis/gen/go/tkd/events/v1/eventsv1connect"
	"github.com/tierklinik-dobersberg/apis/pkg/discovery"
)

type Providers struct {
	Config *Config

	Catalog discovery.Discoverer

	EventService eventsv1connect.EventServiceClient
}
