package cups

import (
	"fmt"

	"github.com/hashicorp/go-multierror"
	ipp "github.com/phin1x/go-ipp"
)

func getFirstValue[T any](states []ipp.Attribute, tag int8) (T, error) {
	var empty T
	var errs = new(multierror.Error)

	if len(states) == 0 {
		return empty, fmt.Errorf("no attribute values available")
	}

	for _, s := range states {
		if tag == ipp.TagCupsInvalid || s.Tag == tag {
			val, ok := s.Value.(T)
			if !ok {
				errs.Errors = append(errs.Errors, fmt.Errorf("%s: expected value type %T for tag type %02x but got %T", s.Name, empty, tag, s.Value))
				continue
			}

			return val, nil
		}
	}

	errs.Errors = append(errs.Errors, fmt.Errorf("failed to find value"))
	return empty, errs.ErrorOrNil()
}
