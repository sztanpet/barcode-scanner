package main

import (
	"bytes"
	"context"
	"os"
	"os/signal"
	"time"

	"code.sztanpet.net/zvpsz/barcode-scanner/internal/config"
	"code.sztanpet.net/zvpsz/barcode-scanner/internal/logwriter"
	"code.sztanpet.net/zvpsz/barcode-scanner/internal/status"

	"code.sztanpet.net/zvpsz/barcode-scanner/internal/buzzer"
	"code.sztanpet.net/zvpsz/barcode-scanner/internal/display"
	"code.sztanpet.net/zvpsz/barcode-scanner/internal/storage"
	"code.sztanpet.net/zvpsz/barcode-scanner/internal/telegram"
	"code.sztanpet.net/zvpsz/barcode-scanner/internal/tty"
	"github.com/juju/loggo"
)

type direction int

const (
	EGRESS direction = iota
	INGRESS
)

// TODO https://vincent.bernat.ch/en/blog/2017-systemd-golang
// TODO reverter binary
type app struct {
	ctx     context.Context
	exit    context.CancelFunc
	cfg     *config.Config
	screen  *display.Screen
	status  *status.Status
	storage *storage.Storage
	bot     *telegram.Bot

	state       State
	currentLine bytes.Buffer
	dir         direction
	currier     string
	activity    chan struct{}
}

var logger = loggo.GetLogger("barcode-scanner")
var (
	idleDurr   = 1 * time.Hour
	statusDurr = 5 * time.Minute
)

func main() {
	cfg := config.Get()
	ctx, exit := context.WithCancel(context.Background())
	a := &app{
		ctx:  ctx,
		exit: exit,
		cfg:  cfg,
	}
	// logging sends messages to telegram, so it depends on it
	// TODO make telegram persist unsendable messages and retry automatically?
	a.setupTelegram()
	a.setupLogging()
	a.status = status.New(a.ctx, a.bot)

	a.handleSignals()

	// depends on statePath
	a.setupStorage()

	// no deps
	a.setupScreen()

	// updates are low-prio and only depend on statePath
	a.setupUpdate()

	go a.inputLoop()
	go a.idleLoop()

	// we got here successfully, beep
	err := buzzer.Setup()
	if err != nil {
		logger.Warningf("buzzer setup error: %v", err)
	}
	err = buzzer.StartupBeep()
	if err != nil {
		logger.Warningf("buzzer beep error: %v", err)
	}

	// canceling the context is the normal way to exit
	<-ctx.Done()
	time.Sleep(250 * time.Millisecond)
	os.Exit(0)
}

func (a *app) handleSignals() {
	c := make(chan os.Signal, 1)
	signal.Notify(c)
	go func(c chan os.Signal) {
		s := <-c
		// exit unconditionally on any signal
		logger.Warningf("Got signal: %s, exiting clearnly", s)
		a.exit()
	}(c)
}

func (a *app) setupLogging() {
	_, _ = loggo.RemoveWriter("default")
	err := logwriter.Setup(a.bot)
	if err != nil {
		panic("logwriter setup failed, impossible")
	}
}

func (a *app) setupUpdate() {
	// TODO
}

func (a *app) setupStorage() {
	storage, err := storage.New(a.ctx, a.cfg)
	if err != nil {
		logger.Criticalf("failed to initialize storage: %v", err)
		os.Exit(1)
	}

	a.storage = storage
}

func (a *app) setupTelegram() {
	bot, err := telegram.New(a.ctx, a.cfg)
	if err != nil {
		return
	}

	a.bot = bot

	err = bot.HandleUpdates(func(msg string) {
		// TODO logging specification handling
		/*
			if err := loggo.ConfigureLoggers(specification); err != nil {
				return err
			}
		*/

	}, false)
	if err != nil {
		// TODO?
	}
}

func (a *app) setupScreen() {
	screen, err := display.NewScreen(a.ctx)
	if err != nil {
		// screen handles its own logging, just exit
		os.Exit(1)
	}
	a.screen = screen

	// TODO show something
}

func (a *app) inputLoop() {
	in, err := tty.Open(a.ctx)
	if err != nil {
		logger.Criticalf("tty open error: %v", err)
		os.Exit(1)
	}
	defer in.RestoreTermMode()

loop:
	for {
		select {
		case <-a.ctx.Done():
			break loop
		default:
		}

		r, _, err := in.ReadRune()
		if err != nil {
			logger.Debugf("read rune error: %v", err)
			continue
		}

		// provide a way to exit the app directly from the keyboard
		if r == 4 {
			logger.Debugf("ctrl+d pressed, exiting")
			a.exit()
			return
		}

		a.transitionState(r)
	}
}

func (a *app) idleLoop() {
	// TODO more frequent runner for status checking? probably
	st := time.NewTicker(statusDurr)
	it := time.NewTimer(idleDurr)
	for {
		select {
		case <-st.C:
			// TODO error log path
			a.status.Check("TODO")
		case <-it.C:
			// TODO handle idle actions: update, etc
		case <-a.activity:
			// reset timer, not idle
			if !it.Stop() {
				<-it.C
			}
			_ = it.Reset(idleDurr)
		}
	}
}
