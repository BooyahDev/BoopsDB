//go:build !linux && !darwin

package update

import (
	"errors"
	"os"
)

var errLocked = errors.New("another update is running")

func acquireLock(string) (*os.File, error) {
	return nil, errors.New("automatic updates are unsupported on this operating system")
}
