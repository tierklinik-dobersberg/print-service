package cmds

import (
	"os"

	"github.com/bufbuild/connect-go"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	printingv1 "github.com/tierklinik-dobersberg/apis/gen/go/tkd/printing/v1"
	"github.com/tierklinik-dobersberg/apis/pkg/cli"
)

func GetPrintCommand(root *cli.Root) *cobra.Command {
	var (
		isUrl       bool
		name        string
		contentType string
		printer     string
	)

	cmd := &cobra.Command{
		Use:  "print <path-or-url>",
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			req := &printingv1.Document{
				Name:        name,
				ContentType: contentType,
				Printer:     printer,
			}

			if isUrl {
				req.Source = &printingv1.Document_Url{
					Url: args[0],
				}
			} else {
				content, err := os.ReadFile(args[0])
				if err != nil {
					logrus.Fatalf("failed to read file: %s", err)
				}

				req.Source = &printingv1.Document_Data{
					Data: content,
				}
			}

			res, err := root.PrintService().PrintDocument(root.Context(), connect.NewRequest(req))
			if err != nil {
				logrus.Fatalf(err.Error())
			}

			root.Print(res.Msg)
		},
	}

	f := cmd.Flags()
	{
		f.BoolVarP(&isUrl, "url", "u", false, "Whether or not the path is a URL")
		f.StringVarP(&name, "name", "n", "", "The name of the document (optional)")
		f.StringVarP(&contentType, "content-type", "C", "", "The content-type of the document (optional)")
		f.StringVarP(&printer, "printer", "p", "", "The printer to use (optional)")
	}

	return cmd
}
