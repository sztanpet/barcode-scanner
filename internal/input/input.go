package input

import (
	"context"
	"fmt"

	"github.com/mattn/go-tty"
)

type Input struct {
	ctx context.Context
	tty *tty.TTY
}

const (
	KeyArrowLeft       = '\x02'
	KeyArrowRight      = '\x06'
	KeyArrowUp         = '\x10'
	KeyArrowDown       = '\x0e'
	KeySpace           = ' '
	KeyEnter           = '\r'
	KeyBackspace       = '\b'
	KeyDelete          = '\x7f'
	KeyInterrupt       = '\x03'
	KeyEndTransmission = '\x04'
	KeyEscape          = '\x1b'
	KeyDeleteWord      = '\x17' // Ctrl+W
	KeyDeleteLine      = '\x18' // Ctrl+X
	SpecialKeyHome     = '\x01'
	SpecialKeyEnd      = '\x11'
	SpecialKeyDelete   = '\x12'
	ignoreKey          = '\000'
)

func New(ctx context.Context) (*Input, error) {
	t, err := tty.Open()
	if err != nil {
		return nil, err
	}

	_, err = t.Raw()
	if err != nil {
		return nil, err
	}

	return &Input{
		ctx: ctx,
		tty: t,
	}, nil
}

// copy-pasted most of this from
// https://github.com/AlecAivazis/survey/blob/9c5cb2fb1f72d65b04a5c29e82126b0b0b6fa444/terminal/runereader_posix.go#L63

// ReadRune reads input from the active tty and deals with escape sequences
func (i *Input) ReadRune() (r rune, err error) {
	for {
		select {
		case <-i.ctx.Done():
			return '\r', nil
		default:
		}

		r, err = i.tty.ReadRune()
		if err != nil {
			return r, err
		}

		// parse escape sequences
		if r == '\033' {

			// not buffered anything? just a pure escape
			if !i.tty.Buffered() {
				return KeyEscape, nil
			}

			r, err = i.tty.ReadRune()
			if err != nil {
				return r, err
			}

			if r != '[' {
				return r, fmt.Errorf("Unexpected escape sequence: %q", r)
			}

			r, err = i.tty.ReadRune()
			if err != nil {
				return r, err
			}

			switch r {
			case 'D':
				return KeyArrowLeft, nil
			case 'C':
				return KeyArrowRight, nil
			case 'A':
				return KeyArrowUp, nil
			case 'B':
				return KeyArrowDown, nil
			case 'H': // Home button
				return SpecialKeyHome, nil
			case 'F': // End button
				return SpecialKeyEnd, nil
			case '3': // Delete Button
				// discard the following '~' key from buffer
				_, _ = i.tty.ReadRune()
				return SpecialKeyDelete, nil
			default:
				r = ignoreKey
			}
		}

		if r != ignoreKey {
			break
		}
	}

	return r, nil
}
