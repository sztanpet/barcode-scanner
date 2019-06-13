package main

import (
	"bytes"
	"context"

	"code.sztanpet.net/barcode-scanner/internal/buzzer"
	"code.sztanpet.net/barcode-scanner/internal/display"
	"code.sztanpet.net/barcode-scanner/internal/input"
	"code.sztanpet.net/barcode-scanner/internal/storage"
	"github.com/juju/loggo"
)

type app struct {
	ctx         context.Context
	exit        context.CancelFunc
	state       State
	currentLine bytes.Buffer

	screen  *display.Screen
	storage *storage.Storage
}

var logger = loggo.GetLogger("main")

func main() {
	ctx, exit := context.WithCancel(context.Background())
	a := &app{
		ctx:  ctx,
		exit: exit,
	}
	_ = a

	_ = buzzer.Setup()
	_ = buzzer.SuccessBeep()
	in, _ := input.New(ctx)
	for {
		r, _ := in.ReadRune()
		a.transitionState(r)
	}
}
