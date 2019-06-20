// +build windows

package tty

import (
	"context"
	"errors"
)

type TTY struct {
}

func Open(ctx context.Context) (*TTY, error) {
	return nil, errors.New("unimplemented")
}

func (t *TTY) RestoreTermMode() {
}

func (t *TTY) Buffered() bool {
	return false
}

func (t *TTY) ReadRune() (r rune, size int, err error) {
	return ignoreKey, 1, errors.New("unimplemented")
}
