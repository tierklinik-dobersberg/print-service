package config

import (
	"io/fs"

	"github.com/dcaraxes/gotenberg-go-client/v8"
	"github.com/tierklinik-dobersberg/apis/gen/go/tkd/events/v1/eventsv1connect"
	"github.com/tierklinik-dobersberg/apis/gen/go/tkd/longrunning/v1/longrunningv1connect"
	"github.com/tierklinik-dobersberg/apis/pkg/discovery"
	"github.com/tierklinik-dobersberg/print-service/internal/cups"
)

type Providers struct {
	Config *Config

	Catalog discovery.Discoverer

	CUPS *cups.Client

	EventService eventsv1connect.EventServiceClient

	LongRunning longrunningv1connect.LongRunningServiceClient

	Storage fs.FS

	Gotenberg *gotenberg.Client
}
