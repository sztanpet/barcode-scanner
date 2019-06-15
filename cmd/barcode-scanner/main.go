package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/signal"

	"code.sztanpet.net/zvpsz/barcode-scanner/internal/buzzer"
	"code.sztanpet.net/zvpsz/barcode-scanner/internal/display"
	"code.sztanpet.net/zvpsz/barcode-scanner/internal/storage"
	"code.sztanpet.net/zvpsz/barcode-scanner/internal/tty"
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

func (a *app) handleSignals() {
	c := make(chan os.Signal, 1)
	signal.Notify(c)
	go func(c chan os.Signal) {
		s := <-c
		// TODO exit unconditionally on any signal?
		fmt.Println("Got signal:", s)
	}(c)
}

var logger = loggo.GetLogger("main")

func main() {
	ctx, exit := context.WithCancel(context.Background())
	a := &app{
		ctx:  ctx,
		exit: exit,
	}
	a.handleSignals()
	_ = a

	_ = buzzer.Setup()
	_ = buzzer.SuccessBeep()
	in, _ := tty.Open(ctx)
	for {
		r, _, _ := in.ReadRune()
		a.transitionState(r)
	}
}
