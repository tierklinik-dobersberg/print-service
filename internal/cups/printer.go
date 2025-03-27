package cups

import (
	"fmt"
	"log/slog"

	ipp "github.com/phin1x/go-ipp"
	printingv1 "github.com/tierklinik-dobersberg/apis/gen/go/tkd/printing/v1"
)

type PrinterState int8

const (
	PrinterStateUnknown = PrinterState(iota)
	PrinterStateIdle
	PrinterStateProcessing
	PrinterStateStopped
)

func (state PrinterState) String() string {
	switch state {
	case PrinterStateIdle:
		return "idle"
	case PrinterStateProcessing:
		return "processing"
	case PrinterStateStopped:
		return "stopped"

	default:
		return "unknown"
	}
}

func (state PrinterState) ToProto() printingv1.PrinterState {
	switch state {
	case PrinterStateIdle:
		return printingv1.PrinterState_PRINTERSTATE_IDLE
	case PrinterStateProcessing:
		return printingv1.PrinterState_PRINTERSTATE_PRINTING
	case PrinterStateStopped:
		return printingv1.PrinterState_PRINTERSTATE_STOPPED

	default:
		return printingv1.PrinterState_PRINTERSTATE_UNSPECIFIED
	}
}

type Printer struct {
	Name         string
	URI          string
	State        PrinterState
	StateReason  string
	StateMessage string
	Location     string
	Info         string
	Model        string
}

func (p Printer) ToProto() *printingv1.Printer {
	return &printingv1.Printer{
		Name:        p.Name,
		Model:       p.Model,
		Location:    p.Location,
		Description: p.Info,
	}
}

func (cli *Client) ListPrinters() ([]Printer, error) {
	res, err := cli.cli.GetPrinters(nil)
	if err != nil {
		return nil, err
	}

	result := make([]Printer, 0, len(res))
	for name, attrs := range res {
		p, err := cli.newPrinter(name, attrs)
		if err != nil {
			slog.Error("failed to create printer", "name", name, "error", err)
			continue
		}

		result = append(result, p)
	}

	return result, nil
}

func (cli *Client) newPrinter(name string, attrs ipp.Attributes) (Printer, error) {
	l := slog.Default().With("name", name)

	p := Printer{
		Name: name,
	}

	var err error
	if name == "" {
		p.Name, err = getFirstValue[string](attrs[ipp.AttributePrinterName], ipp.TagName)
		if err != nil {
			l.Warn("failed to get printer name", "error", err)
		}
	}

	p.URI, _ = getFirstValue[string](attrs[ipp.AttributePrinterURI], ipp.TagUri)

	p.State, err = parsePrinterState(attrs[ipp.AttributePrinterState])
	if err != nil {
		l.Warn("failed to get printer state", "error", err)
	}

	p.StateReason, err = getFirstValue[string](attrs[ipp.AttributePrinterStateReasons], ipp.TagKeyword)
	if err != nil {
		l.Warn("failed to get printer state reason", "error", err)
	}

	p.StateMessage, _ = getFirstValue[string](attrs[ipp.AttributePrinterStateMessage], ipp.TagString)
	p.Location, _ = getFirstValue[string](attrs[ipp.AttributePrinterLocation], ipp.TagText)
	p.Info, _ = getFirstValue[string](attrs[ipp.AttributePrinterInfo], ipp.TagText)
	p.Model, _ = getFirstValue[string](attrs[ipp.AttributePrinterMakeAndModel], ipp.TagText)

	return p, nil
}

func parsePrinterState(states []ipp.Attribute) (PrinterState, error) {
	val, err := getFirstValue[int](states, ipp.TagEnum)
	if err != nil {
		return PrinterStateUnknown, err
	}

	switch val {
	case int(ipp.PrinterStateIdle):
		return PrinterStateIdle, nil
	case int(ipp.PrinterStateProcessing):
		return PrinterStateProcessing, nil
	case int(ipp.PrinterStateStopped):
		return PrinterStateStopped, nil
	}

	return PrinterStateUnknown, fmt.Errorf("unexpected or unsupported printer state value: %02x", val)
}
