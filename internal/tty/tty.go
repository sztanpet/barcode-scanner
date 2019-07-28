// +build !windows
// +build !plan9

// package tty only tries to deal with the basic linux flavor of tty handling
// based on github.com/mattn/go-tty and github.com/AlecAivazis/survey/
package tty

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"os"
	"syscall"
	"unsafe"
)

var ErrUnknownEscapeSequence = errors.New("ErrUnknownEscapeSequence")

type TTY struct {
	ctx    context.Context
	in     *os.File
	reader *bufio.Reader
	buf    bytes.Buffer
	term   syscall.Termios
}

func Open(ctx context.Context) (*TTY, error) {
	in, err := os.Open("/dev/tty")
	if err != nil {
		return nil, err
	}

	t := &TTY{
		ctx:    ctx,
		in:     in,
		reader: bufio.NewReader(in),
	}

	err = t.disableEcho()
	if err != nil {
		return nil, err
	}

	return t, nil
}

// For reading runes we just want to disable echo.
func (t *TTY) disableEcho() error {
	if _, _, err := syscall.Syscall6(syscall.SYS_IOCTL, uintptr(t.in.Fd()), syscall.TCGETS, uintptr(unsafe.Pointer(&t.term)), 0, 0, 0); err != 0 {
		return err
	}

	newState := t.term
	newState.Lflag &^= syscall.ECHO | syscall.ECHONL | syscall.ICANON | syscall.ISIG

	if _, _, err := syscall.Syscall6(syscall.SYS_IOCTL, uintptr(t.in.Fd()), syscall.TCSETS, uintptr(unsafe.Pointer(&newState)), 0, 0, 0); err != 0 {
		return err
	}

	return nil
}

func (t *TTY) RestoreTermMode() {
	_, _, _ = syscall.Syscall6(syscall.SYS_IOCTL, uintptr(t.in.Fd()), syscall.TCSETS, uintptr(unsafe.Pointer(&t.term)), 0, 0, 0)
}

func (t *TTY) Buffered() bool {
	return t.reader.Buffered() > 0
}

func (t *TTY) ReadRune() (r rune, size int, err error) {
	for {
		select {
		case <-t.ctx.Done():
			return '\n', 1, nil
		default:
		}

		r, size, err = t.reader.ReadRune()
		if err != nil {
			return r, size, err
		}

		// parse escape sequences
		if r == '\033' {

			// not buffered anything? just a pure escape
			if !t.Buffered() {
				return KeyEscape, 1, nil
			}

			r, size, err = t.reader.ReadRune()
			if err != nil {
				return r, size, err
			}

			// no support for alt+ combinations
			if r != '[' {
				return r, size, ErrUnknownEscapeSequence
			}

			r, size, err = t.reader.ReadRune()
			if err != nil {
				return r, size, err
			}

			switch r {
			case 'D':
				return KeyArrowLeft, 1, nil
			case 'C':
				return KeyArrowRight, 1, nil
			case 'A':
				return KeyArrowUp, 1, nil
			case 'B':
				return KeyArrowDown, 1, nil
			case '3': // Delete Button
				// discard the following '~' key from buffer
				_, _, _ = t.reader.ReadRune()
				return SpecialKeyDelete, 1, nil
			default:
				if t.Buffered() {
					// still a '~' in the buffer, discard
					_, _, _ = t.reader.ReadRune()
				}
				return r, size, ErrUnknownEscapeSequence
			}
		}

		if r != ignoreKey {
			break
		}
	}

	return r, size, nil
}
