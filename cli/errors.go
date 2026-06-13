package cli

import (
	"errors"

	"github.com/tamnd/dzone-cli/dzone"
)

func isNotFound(err error) bool {
	return errors.Is(err, dzone.ErrUnknownSection)
}

func mapFetchErr(err error) error {
	if err == nil {
		return nil
	}
	if isNotFound(err) {
		return codeError(exitUsage, err)
	}
	return codeError(exitError, err)
}
