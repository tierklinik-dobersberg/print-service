package main

import (
	"github.com/sirupsen/logrus"
	"github.com/tierklinik-dobersberg/apis/pkg/cli"
	"github.com/tierklinik-dobersberg/print-service/cmds/printctl/cmds"
)

func main() {
	root := cli.New("printctl")

	root.AddCommand(
		cmds.GetPrintCommand(root),
		cmds.GetPrinterCommand(root),
	)

	if err := root.ExecuteContext(root.Context()); err != nil {
		logrus.Fatalf(err.Error())
	}
}
