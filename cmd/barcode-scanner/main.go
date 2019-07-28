package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"code.sztanpet.net/zvpsz/barcode-scanner/internal/buzzer"
	"code.sztanpet.net/zvpsz/barcode-scanner/internal/config"
	"code.sztanpet.net/zvpsz/barcode-scanner/internal/display"
	"code.sztanpet.net/zvpsz/barcode-scanner/internal/file"
	"code.sztanpet.net/zvpsz/barcode-scanner/internal/logwriter"
	"code.sztanpet.net/zvpsz/barcode-scanner/internal/status"
	"code.sztanpet.net/zvpsz/barcode-scanner/internal/storage"
	"code.sztanpet.net/zvpsz/barcode-scanner/internal/telegram"
	"code.sztanpet.net/zvpsz/barcode-scanner/internal/tty"
	"code.sztanpet.net/zvpsz/barcode-scanner/internal/update"
	"github.com/juju/loggo"
)

type direction int

func (d *direction) String() string {
	return strconv.Itoa(int(*d))
}

const (
	EGRESS  direction = 0
	INGRESS direction = 1
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
	upd     *update.Binary

	state       State
	currentLine bytes.Buffer
	activity    chan struct{}
	idleTasks   []func()
	idleStart   time.Time

	mu      sync.RWMutex
	dir     direction
	currier string
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

	// handle signals first
	a.handleSignals()

	a.setupTelegram()
	// logging sends messages to telegram, so it depends on it
	a.setupLogging()
	// sends to telegram
	a.status = status.New(a.ctx, a.bot)
	// depends on statePath => config
	a.setupStorage()
	// no deps
	a.setupScreen()

	// updates are low-prio and only depend on statePath
	a.setupUpdate()
	// restore settings set by the user, only the inputLoop uses the info
	a.setupSettings()

	go a.inputLoop()
	go a.idleLoop()

	a.setupBuzzer()

	// canceling the context is the normal way to exit
	<-ctx.Done()
	time.Sleep(250 * time.Millisecond)
	os.Exit(0)
}

func (a *app) handleSignals() {
	if a.ctx.Err() != nil {
		return
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c)
	go func() {
		s := <-c
		// exit unconditionally on any signal
		logger.Warningf("Got signal: %s, exiting cleanly", s)
		a.exit()
		fmt.Println("exited")
	}()
}

func (a *app) setupLogging() {
	if a.ctx.Err() != nil {
		return
	}

	err := logwriter.Setup(a.bot)
	if err != nil {
		panic("logwriter setup failed, impossible: " + err.Error())
	}
}

func (a *app) setupUpdate() {
	if a.ctx.Err() != nil {
		return
	}

	binPath, err := os.Executable()
	if err != nil {
		logger.Criticalf("os.Executable failed: %v", err)
		panic("os.Executable failed: " + err.Error())
	}
	upd, err := update.NewBinary(binPath, a.cfg)
	if err != nil {
		logger.Criticalf("update.NewBinary failed: %v", err)
		panic("update.NewBinary failed: " + err.Error())
	}
	a.upd = upd

	a.addIdleTask(func() {
		if a.upd.ShouldRestart() {
			logger.Warningf("update available, exiting cleanly")
			a.exit()
		}
	})
}

func (a *app) setupStorage() {
	if a.ctx.Err() != nil {
		return
	}

	storage, err := storage.New(a.ctx, a.cfg)
	if err != nil {
		logger.Criticalf("failed to initialize storage: %v", err)
		os.Exit(1)
	}

	a.storage = storage
}

func (a *app) setupTelegram() {
	if a.ctx.Err() != nil {
		return
	}

	a.bot = telegram.New(a.ctx, a.cfg)
	_ = a.bot.Send("BS-start @ "+time.Now().Format(time.RFC3339), true)

	// TODO make telegram persist unsendable messages and retry automatically?
	go func() {
		err := a.bot.HandleMessage(handleTelegramMessage, false)

		if err != nil {
			logger.Criticalf("Handlemessage error: %v", err)
		}
	}()
}

// the bot can receive messages, so lets control the logging levels with it
// the command message has to begin with the following prefix
// and the rest is the logging specification as documented at
// https://godoc.org/github.com/juju/loggo#ParseConfigString
//
// ex: <root>=ERROR; foo.bar=WARNING
// full command would thus be:
// !log barcode-scanner <root>=ERROR; foo.bar=WARNING
func handleTelegramMessage(msg string) {
	const pfx = "!log barcode-scanner "
	if len(msg) < len(pfx) || !strings.EqualFold(msg[:len(pfx)], pfx) {
		return
	}

	spec := msg[len(pfx)-1:]
	if _, err := loggo.ParseConfigString(spec); err != nil {
		logger.Warningf("failed parsing spec: %v, error was: %v", spec, err)
		return
	}

	if err := loggo.ConfigureLoggers(spec); err != nil {
		logger.Errorf("failed to apply log spec: %v, error was: %v", spec, err)
		return
	}

	logger.Debugf("logging spec successfully applied, spec was: %v", spec)
}

func (a *app) setupScreen() {
	if a.ctx.Err() != nil {
		return
	}

	screen, err := display.NewScreen(a.ctx)
	if err != nil {
		// screen handles its own logging, just exit
		fmt.Printf("screen err: %v", err)
		a.exit()
		return
	}
	a.screen = screen

	_ = a.screen.WriteTitle("STARTUP")
	_ = a.screen.WriteLine(1, "")
	_ = a.screen.WriteLine(2, "OK")
	_ = a.screen.WriteHelp("Scanner ready")

	a.addIdleTask(func() {
		if err := a.screen.Blank(); err != nil {
			logger.Warningf("Failed to blank screen on idle, error was: %v", err)
		}
	})
}

func (a *app) setupBuzzer() {
	if a.ctx.Err() != nil {
		return
	}

	if err := buzzer.Setup(); err != nil {
		logger.Warningf("buzzer setup error: %v", err)
	}
	if err := buzzer.StartupBeep(); err != nil {
		logger.Warningf("buzzer beep error: %v", err)
	}
}

func (a *app) inputLoop() {
	in, err := tty.Open(a.ctx)
	if err != nil {
		logger.Criticalf("tty open error: %v", err)
		os.Exit(1)
	}
	defer in.RestoreTermMode()

	a.enterReadBarcode()

	for {
		select {
		case <-a.ctx.Done():
			return
		default:
		}

		r, _, err := in.ReadRune()
		if err != nil {
			// pretty expected error since we only provide support for a subset of inputs
			logger.Debugf("read rune error: %v", err)
			continue
		}

		// provide a way to exit the app directly from the keyboard
		if r == 4 {
			logger.Warningf("ctrl+d pressed, exiting")
			a.exit()
			return
		}

		a.transitionState(r)
	}
}

func (a *app) idleLoop() {
	if a.ctx.Err() != nil {
		return
	}

	a.status.Check()

	st := time.NewTicker(statusDurr)
	it := time.NewTimer(idleDurr)
	for {
		select {
		case <-a.activity:
			// activity signalled => not idle, reset timer
			if !it.Stop() {
				<-it.C
			}
			_ = it.Reset(idleDurr)
			a.idleStart = time.Time{}

		case <-st.C:
			a.status.Check()

		case <-it.C:
			if a.idleStart.IsZero() {
				a.idleStart = time.Now()
			}

			for _, task := range a.idleTasks {
				task()
			}
		}
	}
}

func (a *app) addIdleTask(f func()) {
	// TODO locking?
	a.idleTasks = append(a.idleTasks, f)
}

func (a *app) persistSettings() {
	s := &struct {
		Direction int
		Currier   string
		IdleStart time.Time
	}{
		Direction: int(a.dir),
		Currier:   a.currier,
		IdleStart: a.idleStart,
	}

	path := filepath.Join(a.cfg.StatePath, "barcode-scanner", "settings")
	if err := file.Serialize(path, s); err != nil {
		logger.Warningf("Failed to serialize settings: %v", err)
	}
}

func (a *app) setupSettings() {
	a.loadSettings()
	a.addIdleTask(func() {
		if !a.idleStart.IsZero() && time.Now().After(a.idleStart.Add(6*time.Hour)) {
			a.dir = 0
			a.currier = a.cfg.Currier
		}
	})
}

func (a *app) loadSettings() {
	path := filepath.Join(a.cfg.StatePath, "barcode-scanner", "state")
	if !file.Exists(path) {
		logger.Debugf("No state to restore, skipping")
		return
	}

	s := &struct {
		Direction int
		Currier   string
		IdleStart time.Time
	}{}

	if err := file.Unserialize(path, s); err != nil {
		logger.Warningf("Failed to unserialize settings: %v", err)
	}

	a.dir = direction(s.Direction)
	a.currier = s.Currier
	a.idleStart = s.IdleStart
	logger.Debugf("Restored settings: %#v", s)
}
