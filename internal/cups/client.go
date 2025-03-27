package cups

import (
	"errors"
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"

	ipp "github.com/phin1x/go-ipp"
)

type Client struct {
	cli *ipp.CUPSClient

	defaultPrinterName string
	defaultPrinterURI  string

	host     string
	port     int
	username string
	password string
}

func NewClient(address string, user string, password string) (*Client, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		var addrErr *net.AddrError

		if errors.As(err, &addrErr) && strings.Contains(addrErr.Err, "missing port") {
			host, port, err = net.SplitHostPort(address + ":631")
		}
	}
	if err != nil {
		return nil, err
	}

	p, err := strconv.ParseInt(port, 10, 0)
	if err != nil {
		return nil, fmt.Errorf("invalid port: %w", err)
	}

	cli := &Client{
		cli:      ipp.NewCUPSClient(host, int(p), user, password, false),
		host:     host,
		port:     int(p),
		username: user,
		password: password,
	}

	// immediately test if the connection succeeds
	if err := cli.cli.TestConnection(); err != nil {
		return nil, err
	}

	// fetch the default printer
	defaultPrinter, err := cli.GetDefaultPrinter()
	if err == nil {
		cli.defaultPrinterName = defaultPrinter.Name
		cli.defaultPrinterURI = defaultPrinter.URI
	}

	return cli, nil
}

func (cli *Client) GetDefaultPrinter() (Printer, error) {
	req := ipp.NewRequest(ipp.OperationCupsGetDefault, 1)
	req.OperationAttributes[ipp.AttributeRequestedAttributes] = append(ipp.DefaultPrinterAttributes, ipp.AttributePrinterURI)

	adapter := ipp.NewHttpAdapter(cli.host, cli.port, cli.username, cli.password, false)

	resp, err := cli.cli.SendRequest(adapter.GetHttpUri("", nil), req, nil)
	if err != nil {
		return Printer{}, err
	}

	if len(resp.PrinterAttributes) == 0 {
		log.Printf("%#v", resp)
		return Printer{}, fmt.Errorf("no default printer found")
	}

	return cli.newPrinter("", resp.PrinterAttributes[0])
}

func (cli *Client) Ping() error {
	return cli.cli.TestConnection()
}

func (cli *Client) getPrinterName(uri string) string {
	if strings.Contains(uri, fmt.Sprintf("%s:%d", cli.host, cli.port)) {
		return strings.TrimPrefix(uri, "ipp://"+cli.host+":"+strconv.Itoa(cli.port)+"/printers/")
	}

	if strings.Contains(uri, cli.host) {
		return strings.TrimPrefix(uri, "ipp://"+cli.host+"/printers/")
	}

	if strings.Contains(uri, fmt.Sprintf("localhost:%d", cli.port)) {
		return strings.TrimPrefix(uri, "ipp://localhost"+":"+strconv.Itoa(cli.port)+"/printers/")
	}

	if strings.Contains(uri, "localhost") {
		return strings.TrimPrefix(uri, "ipp://localhost/printers/")
	}

	return uri
}

func init() {
	// add default attributes and custom attribute mappings
	ipp.AttributeTagMapping[AttributeLongRunningOperationID] = ipp.TagString
	ipp.DefaultJobAttributes = append(ipp.DefaultJobAttributes, AttributeLongRunningOperationID)

	ipp.AttributeTagMapping[AttributePrintColorMode] = ipp.TagKeyword
	ipp.AttributeTagMapping[AttributePrintColorModeDefault] = ipp.TagKeyword
	ipp.DefaultJobAttributes = append(ipp.DefaultJobAttributes, AttributePrintColorModeDefault)
}
