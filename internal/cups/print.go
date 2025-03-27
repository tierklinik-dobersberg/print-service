package cups

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/bufbuild/connect-go"
	ipp "github.com/phin1x/go-ipp"
	longrunningv1 "github.com/tierklinik-dobersberg/apis/gen/go/tkd/longrunning/v1"
	"github.com/tierklinik-dobersberg/apis/gen/go/tkd/longrunning/v1/longrunningv1connect"
	printingv1 "github.com/tierklinik-dobersberg/apis/gen/go/tkd/printing/v1"
	"github.com/tierklinik-dobersberg/apis/pkg/auth"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"
)

func (cli *Client) Print(doc ipp.Document, printer string, customAttrs map[string]any) (int, error) {
	if printer == "" {
		if cli.defaultPrinterName != "" {
			printer = cli.defaultPrinterName
		} else {
			return -1, fmt.Errorf("no printer specified and no default printer available")
		}
	}

	jobId, err := cli.cli.PrintJob(doc, printer, customAttrs)
	if err != nil {
		return -1, err
	}

	return jobId, nil
}

type UpdateFunc func(job Job)

func (cli *Client) PrintAndWait(doc ipp.Document, printer string, customAttrs map[string]any, update UpdateFunc) (JobState, error) {
	jobId, err := cli.Print(doc, printer, customAttrs)
	if err != nil {
		return JobStateUnknown, err
	}

	for {
		<-time.After(time.Second)

		job, err := cli.GetJobById(jobId)
		if err != nil {
			return JobStateUnknown, err
		}

		if update != nil {
			update(job)
		}

		switch job.State {
		case JobStatePending, JobStateHeld, JobStateProcessing:
			continue

		default:
			return job.State, nil
		}
	}
}

func (cli *Client) PrintWithOperation(ctx context.Context, lrun longrunningv1connect.LongRunningServiceClient, doc ipp.Document, printer string, customAttrs map[string]any) (*longrunningv1.Operation, error) {
	// first, create a new operation
	req := connect.NewRequest(&longrunningv1.RegisterOperationRequest{
		Owner:        "tkd.printing.v1.PrintService",
		Creator:      auth.From(ctx).Username,
		InitialState: longrunningv1.OperationState_OperationState_PENDING,
		Ttl:          durationpb.New(time.Second * 30),
		GracePeriod:  durationpb.New(time.Second * 30),
		Description:  doc.Name,
		Kind:         "tkd.printing.v1/print-job",
		Annotations:  map[string]string{},
	})

	operationResponse, err := lrun.RegisterOperation(ctx, req)
	if err != nil {
		return nil, err
	}

	if customAttrs == nil {
		customAttrs = make(map[string]any)
	}

	customAttrs[AttributeLongRunningOperationID] = operationResponse.Msg.Operation.UniqueId

	update := func(ctx context.Context, j Job) {
		_, err = lrun.UpdateOperation(ctx, connect.NewRequest(&longrunningv1.UpdateOperationRequest{
			UniqueId:  operationResponse.Msg.Operation.UniqueId,
			AuthToken: operationResponse.Msg.AuthToken,
			Running:   j.State == JobStateProcessing,
			Annotations: map[string]string{
				"state":      j.State.String(),
				"percent":    fmt.Sprintf("%d%%", j.Progress),
				"jobID":      strconv.Itoa(j.ID),
				"printer":    j.PrinterName,
				"printerUri": j.PrinterURI,
			},
		}))
		if err != nil {
			slog.Error("failed to update job operation", "error", err.Error(), "job-id", j.ID, "operation-id", operationResponse.Msg.Operation.UniqueId)
		}
	}

	id, err := cli.Print(doc, printer, customAttrs)
	if err != nil {
		if _, err := lrun.CompleteOperation(ctx, connect.NewRequest(&longrunningv1.CompleteOperationRequest{
			UniqueId:  operationResponse.Msg.Operation.UniqueId,
			AuthToken: operationResponse.Msg.AuthToken,
			Result: &longrunningv1.CompleteOperationRequest_Error{
				Error: &longrunningv1.OperationError{
					Message: err.Error(),
				},
			},
		})); err != nil {
			slog.Error("failed to complete operation", "error", err.Error())
		}

		return nil, err
	}

	go func() {
		for {
			<-time.After(time.Second * 15)

			j, err := cli.GetJobById(id)
			if err != nil {
				slog.Error("failed to get job", "error", err.Error())
				return
			}

			switch j.State {
			case JobStatePending, JobStateProcessing, JobStateHeld:
				update(ctx, j)
				continue

			default:
				// finally, makr the operation as done
				result := &printingv1.PrintOperationState{
					State: j.State.ToProto(),
					Document: &printingv1.Document{
						Name:        doc.Name,
						ContentType: doc.MimeType,
						Printer:     j.PrinterName,
					},
				}

				resultPb, err := anypb.New(result)
				if err != nil {
					slog.Error("failed to perpare print result", "error", err)
				}

				if _, err := lrun.CompleteOperation(ctx, connect.NewRequest(&longrunningv1.CompleteOperationRequest{
					UniqueId:  operationResponse.Msg.Operation.UniqueId,
					AuthToken: operationResponse.Msg.AuthToken,
					Result: &longrunningv1.CompleteOperationRequest_Success{
						Success: &longrunningv1.OperationSuccess{
							Message: j.State.String(),
							Result:  resultPb,
						},
					},
				})); err != nil {
					slog.Error("failed to complete operation", "error", err.Error())
				}

				return
			}
		}
	}()

	return operationResponse.Msg.Operation, nil
}
