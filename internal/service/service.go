package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
	"time"

	"github.com/bufbuild/connect-go"
	"github.com/dcaraxes/gotenberg-go-client/v8"
	"github.com/dcaraxes/gotenberg-go-client/v8/document"
	"github.com/phin1x/go-ipp"
	longrunningv1 "github.com/tierklinik-dobersberg/apis/gen/go/tkd/longrunning/v1"
	v1 "github.com/tierklinik-dobersberg/apis/gen/go/tkd/printing/v1"
	"github.com/tierklinik-dobersberg/apis/gen/go/tkd/printing/v1/printingv1connect"
	"github.com/tierklinik-dobersberg/apis/pkg/auth"
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

func (svc *Service) ListPrinters(ctx context.Context, req *connect.Request[v1.ListPrintersRequest]) (*connect.Response[v1.ListPrintersResponse], error) {
	printers, err := svc.providers.CUPS.ListPrinters()
	if err != nil {
		return nil, err
	}

	res := &v1.ListPrintersResponse{
		Printers: make([]*v1.Printer, len(printers)),
	}

	for idx, p := range printers {
		res.Printers[idx] = p.ToProto()
	}

	return connect.NewResponse(res), nil
}

func (svc *Service) PrintDocument(ctx context.Context, req *connect.Request[v1.Document]) (*connect.Response[longrunningv1.Operation], error) {
	user := auth.From(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("unauthentication"))
	}

	// first, get a reader to the document content
	reader, size, err := svc.resolveContent(req.Msg)
	if err != nil {
		return nil, err
	}

	var closer io.Closer = reader
	var content io.Reader = reader

	mime := req.Msg.ContentType

	// finally, if there's no content-type, try to autodetect it
	if req.Msg.ContentType == "" {
		// try to read the first bytes
		buf := make([]byte, 512)
		read, err := io.ReadFull(reader, buf)

		switch {
		case err == nil:
		case errors.Is(err, io.ErrUnexpectedEOF):
			buf = buf[:read]
		case errors.Is(err, io.EOF):
			return nil, fmt.Errorf("empty document content")
		default:
			return nil, fmt.Errorf("failed to read document content: %w", err)
		}

		mime = http.DetectContentType(buf)
		slog.Info("auto detected content type for document", "name", req.Msg.Name, "content-type", mime)

		content = io.MultiReader(
			bytes.NewReader(buf),
			reader,
		)
	}

	// close the document source
	defer func() {
		if err := closer.Close(); err != nil {
			slog.Error("failed to close document source", "name", req.Msg.Name, "error", err)
		}
	}()

	// check if we need to convert the given mime-type to PDF:
	// TODO(ppacher): this could actually be part of the long-running-operation.
	wrapped, mime, err := svc.mayConvertToPDF(ctx, req.Msg.Name, mime, content, req.Msg.Orientation)
	if err != nil {
		return nil, err
	}

	// if the returned wrapped reader is also a closer, defer closing
	if c, ok := wrapped.(io.Closer); ok && wrapped != content {
		if err := c.Close(); err != nil {
			slog.Error("failed to close wrapped content source", "name", req.Msg.Name, "error", err)
		}
	}

	doc := ipp.Document{
		Document: wrapped,
		Size:     int(size),
		Name:     req.Msg.Name,
		MimeType: mime,
	}

	/*
		orientation := cups.OrientationPortrait
		if req.Msg.Orientation == v1.Orientation_ORIENTATION_LANDSCAPE {
			orientation = cups.OrientationLandscape
		}

		color := cups.ColorModeAuto
		switch req.Msg.ColorMode {
		case v1.ColorMode_COLORMODE_AUTO:
			color = cups.ColorModeAuto
		case v1.ColorMode_COLORMODE_COLOR:
			color = cups.ColorModeColor
		case v1.ColorMode_COLORMODE_GRAYSCALE:
			color = cups.ColorModeGrayScale
		}
	*/

	operation, err := svc.providers.CUPS.PrintWithOperation(
		ctx,
		svc.providers.LongRunning,
		doc,
		req.Msg.Printer,
		map[string]any{
			ipp.AttributeRequestingUserName: user.Username,
			// ipp.AttributeOrientationRequested: string(orientation),
			// cups.AttributePrintColorMode:      string(color),
		},
	)
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(operation), nil
}

func (svc *Service) ListJobs(ctx context.Context, req *connect.Request[v1.ListJobsRequest]) (*connect.Response[v1.ListJobsResponse], error) {
	var printers []string

	if len(req.Msg.Printers) == 0 {
		all, err := svc.providers.CUPS.ListPrinters()
		if err != nil {
			return nil, fmt.Errorf("failed to get printers: %w", err)
		}

		for _, p := range all {
			printers = append(printers, p.Name)
		}
	} else {
		printers = req.Msg.Printers
	}

	var jobs []*v1.Job

	for _, p := range printers {
		pj, err := svc.providers.CUPS.ListJobs(p)
		if err != nil {
			return nil, fmt.Errorf("failed to get jobs for printer %q: %w", p, err)
		}

		for _, j := range pj {
			jobs = append(jobs, j.ToProto())
		}
	}

	return connect.NewResponse(&v1.ListJobsResponse{
		Jobs: jobs,
	}), nil
}

func isOfficeRequest(name string) bool {
	ext := filepath.Ext(name)

	switch ext {
	case ".doc", ".docx", ".ppt", ".pptx", ".odt", ".xls", ".xlsx", ".fodt", ".ods", ".fods", ".odp", ".fodp", ".odf", ".epub":
		return true

	default:
		return false
	}
}

func (svc *Service) mayConvertToPDF(ctx context.Context, name, mime string, reader io.Reader, orientation v1.Orientation) (io.Reader, string, error) {
	// Skip PDF, postscript, octet-stream,  plain text and image documents
	switch mime {
	case "application/pdf", "application/postscript", "text/plain":
		return reader, mime, nil

		// this might be an office request
	case "text/html":
		return svc.renderHTML(ctx, name, reader, orientation)

	default:
		if isOfficeRequest(name) {
			return svc.renderOffice(ctx, name, reader, orientation)
		}

		return reader, mime, nil
	}
}

func (svc *Service) renderHTML(ctx context.Context, name string, reader io.Reader, orientation v1.Orientation) (io.Reader, string, error) {
	indexDoc, err := document.FromReader(name, reader)
	if err != nil {
		return nil, "", err
	}

	req := gotenberg.NewHTMLRequest(indexDoc)
	req.WaitDelay(time.Second * 3)
	req.SkipNetworkIdleEvent()

	// TODO(ppacher): make this configurable
	req.PaperSize(gotenberg.A4)
	req.Margins(gotenberg.NormalMargins)
	req.PrintBackground()
	req.FailOnConsoleExceptions()

	switch orientation {
	case v1.Orientation_ORIENTATION_LANDSCAPE:
		req.Landscape()
	case v1.Orientation_ORIENTATION_PORTRAIT:
		// nothing to do, portrait is default
	}

	res, err := svc.providers.Gotenberg.Send(ctx, req)
	if err != nil {
		return nil, "", err
	}

	content, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, "", err
	}
	slog.Info("successfully converted HTML document to PDF", "size", len(content))

	return io.NopCloser(bytes.NewReader(content)), "application/pdf", nil
}

func (svc *Service) renderOffice(ctx context.Context, name string, reader io.Reader, orientation v1.Orientation) (io.Reader, string, error) {
	indexDoc, err := document.FromReader(name, reader)
	if err != nil {
		return nil, "", err
	}

	req := gotenberg.NewOfficeRequest(indexDoc)

	switch orientation {
	case v1.Orientation_ORIENTATION_LANDSCAPE:
		req.Landscape()
	case v1.Orientation_ORIENTATION_PORTRAIT:
		// nothing to do, portrait is default
	}

	res, err := svc.providers.Gotenberg.Send(ctx, req)
	if err != nil {
		return nil, "", err
	}

	return res.Body, "application/pdf", nil
}
