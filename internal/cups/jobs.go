package cups

import (
	"fmt"
	"log/slog"
	"strconv"

	"github.com/hashicorp/go-multierror"
	"github.com/phin1x/go-ipp"
	printingv1 "github.com/tierklinik-dobersberg/apis/gen/go/tkd/printing/v1"
)

type JobState int

const (
	JobStateUnknown = JobState(iota)
	JobStatePending
	JobStateHeld
	JobStateProcessing
	JobStateStopped
	JobStateCanceled
	JobStateAborted
	JobStateComplete
)

func (j JobState) ToProto() printingv1.PrintState {
	switch j {
	case JobStateUnknown:
		return printingv1.PrintState_PRINTSTATE_UNSPECIFIED
	case JobStatePending:
		return printingv1.PrintState_PRINTSTATE_PENDING
	case JobStateHeld:
		return printingv1.PrintState_PRINTSTATE_PENDING
	case JobStateProcessing:
		return printingv1.PrintState_PRINTSTATE_PRINTING
	case JobStateStopped:
		return printingv1.PrintState_PRINTSTATE_CANCELLED
	case JobStateCanceled:
		return printingv1.PrintState_PRINTSTATE_CANCELLED
	case JobStateAborted:
		return printingv1.PrintState_PRINTSTATE_CANCELLED
	case JobStateComplete:
		return printingv1.PrintState_PRINTSTATE_COMPLETED
	}

	return printingv1.PrintState_PRINTSTATE_UNSPECIFIED
}

func (j JobState) String() string {
	switch j {
	case JobStateUnknown:
		return "unknown"
	case JobStatePending:
		return "pending"
	case JobStateHeld:
		return "held"
	case JobStateProcessing:
		return "processing"
	case JobStateStopped:
		return "stopped"
	case JobStateCanceled:
		return "canceled"
	case JobStateAborted:
		return "aborted"
	case JobStateComplete:
		return "complete"
	}

	return fmt.Sprintf("unknown-state-%02x", int(j))
}

type Job struct {
	ID          int
	Name        string
	State       JobState
	PrinterURI  string
	PrinterName string
	Progress    int

	OperationID string
}

func (j Job) ToProto() *printingv1.Job {
	return &printingv1.Job{
		Id:       strconv.Itoa(j.ID),
		Name:     j.Name,
		State:    j.State.ToProto(),
		Progress: int32(j.Progress),
		Printer:  j.PrinterURI,
	}
}

func (cli *Client) ListJobs(printer string) ([]Job, error) {
	jobs, err := cli.cli.GetJobs(printer, "", "all", false, 0, 0, nil)
	if err != nil {
		return nil, err
	}

	merr := new(multierror.Error)

	result := make([]Job, 0, len(jobs))
	for id, attr := range jobs {
		j, err := cli.newJob(id, attr)

		if err != nil {
			merr.Errors = append(merr.Errors, fmt.Errorf("job-%d: %w", id, err))
			continue
		}

		result = append(result, j)
	}

	return result, nil
}

func (cli *Client) GetJobById(id int) (Job, error) {
	res, err := cli.cli.GetJobAttributes(id, nil)
	if err != nil {
		return Job{}, err
	}

	return cli.newJob(id, res)
}

func (cli *Client) newJob(jobId int, attr ipp.Attributes) (Job, error) {
	job := Job{
		ID: jobId,
	}

	var err error

	l := slog.Default().With("jobId", jobId)

	job.State, err = getJobState(attr[ipp.AttributeJobState])
	if err != nil {
		l.Error("job.State", "error", err.Error())
	}

	job.Name, err = getFirstValue[string](attr[ipp.AttributeJobName], ipp.TagName)
	if err != nil {
		job.Name, err = getFirstValue[string](attr[ipp.AttributeDocumentName], ipp.TagName)
		if err != nil {
			l.Error("job.Name", "error", err.Error())
		}
	}

	job.PrinterURI, err = getFirstValue[string](attr[ipp.AttributePrinterURI], ipp.TagUri)
	if err != nil {
		l.Error("job.PrinterURI", "error", err.Error())
	} else {
		job.PrinterName = cli.getPrinterName(job.PrinterURI)
	}

	job.Progress, err = getFirstValue[int](attr[ipp.AttributeJobMediaProgress], ipp.TagCupsInvalid)
	if err != nil {
		l.Error("job.PrinterURI", "error", err.Error())
	}

	job.OperationID, err = getFirstValue[string](attr[AttributeLongRunningOperationID], ipp.TagString)
	if err != nil {
		l.Error("job.OperationID", "error", err.Error())
	}

	return job, nil
}

func getJobState(attrs []ipp.Attribute) (JobState, error) {
	val, err := getFirstValue[int](attrs, ipp.TagEnum)
	if err != nil {
		return JobStateUnknown, fmt.Errorf("failed to get job state: %w", err)
	}

	switch int8(val) {
	case ipp.JobStatePending:
		return JobStatePending, nil
	case ipp.JobStateHeld:
		return JobStateHeld, nil
	case ipp.JobStateProcessing:
		return JobStateProcessing, nil
	case ipp.JobStateStopped:
		return JobStateStopped, nil
	case ipp.JobStateCanceled:
		return JobStateCanceled, nil
	case ipp.JobStateAborted:
		return JobStateAborted, nil
	case ipp.JobStateCompleted:
		return JobStateComplete, nil

	default:
		return JobStateUnknown, fmt.Errorf("unsupported job state value %02x", val)
	}
}
