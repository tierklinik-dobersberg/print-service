package cmds

import (
	"github.com/bufbuild/connect-go"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	printingv1 "github.com/tierklinik-dobersberg/apis/gen/go/tkd/printing/v1"
	"github.com/tierklinik-dobersberg/apis/pkg/cli"
)

func GetPrinterCommand(root *cli.Root) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "printers",
		Aliases: []string{"printer", "devices", "dev"},
		Args:    cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			res, err := root.PrintService().ListPrinters(root.Context(), connect.NewRequest(&printingv1.ListPrintersRequest{}))
			if err != nil {
				logrus.Fatalf(err.Error())
			}

			root.Print(res.Msg)
		},
	}

	return cmd
}
