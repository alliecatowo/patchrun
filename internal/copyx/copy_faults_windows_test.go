//go:build windows

package copyx

import "errors"

func makeFifo(path string) error { return errors.New("fifos not supported on windows") }
