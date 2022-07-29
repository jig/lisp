package errorstack

import "errors"

func Dump(err error) string {
	if parentErr := errors.Unwrap(err); parentErr != nil {
		return err.Error() + "\n" + Dump(errors.Unwrap(err))
	}
	return err.Error()
}
