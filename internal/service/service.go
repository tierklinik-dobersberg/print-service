package service

import (
	"github.com/tierklinik-dobersberg/apis/gen/go/tkd/printing/v1/printingv1connect"
	"github.com/tierklinik-dobersberg/print-service/internal/config"
)

type Service struct {
	printingv1connect.UnimplementedPrintServiceHandler

	providers *config.Providers
}

func New(providers *config.Providers) *Service {
	svc := &Service{
		providers: providers,
	}

	return svc
}
