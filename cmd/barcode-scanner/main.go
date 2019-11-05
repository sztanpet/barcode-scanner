package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"code.sztanpet.net/zvpsz/barcode-scanner/internal/config"
	"code.sztanpet.net/zvpsz/barcode-scanner/internal/display"
	"code.sztanpet.net/zvpsz/barcode-scanner/internal/file"
	"code.sztanpet.net/zvpsz/barcode-scanner/internal/status"
	"code.sztanpet.net/zvpsz/barcode-scanner/internal/storage"
	"code.sztanpet.net/zvpsz/barcode-scanner/internal/telegram"
	"code.sztanpet.net/zvpsz/barcode-scanner/internal/tty"
	"code.sztanpet.net/zvpsz/barcode-scanner/internal/update"
	"github.com/juju/loggo"
)

type direction int

func (d *direction) String() string {
	switch *d {
	case 0:
		return "EGRESS"
	case 1:
		return "INGRESS"
	}

	panic("unknown value for direction: " + strconv.Itoa(int(*d)))
}

const (
	EGRESS  direction = 0
	INGRESS direction = 1
)

var settingsPath = "barcode-scanner/settings"
var specialBarcodeRe = regexp.MustCompile(`(?i)(?:^(INGRESS|EGRESS)-(\d+)$|^(W(?:S|P))\$(.+)$)`)

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

func init() {
	loggo.GetLogger("").SetLogLevel(loggo.TRACE)
}

func main() {
	cfg := config.Get()
	ctx, exit := context.WithCancel(context.Background())
	a := &app{
		ctx:     ctx,
		exit:    exit,
		cfg:     cfg,
		currier: "0",
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
	a.setupWiFi()
	a.setupDeviceID()

	go a.inputLoop()
	go a.idleLoop()

	a.setupHardware()
	a.onBootup()

	// canceling the context is the normal way to exit
	<-ctx.Done()
	a.onShutdown()
	time.Sleep(250 * time.Millisecond)
	os.Exit(0)
}

// the bot can receive messages, so lets control the logging levels with it
// the command message has to begin with the following prefix
// and the rest is the logging specification as documented at
// https://godoc.org/github.com/juju/loggo#ParseConfigString
//
// ex: <root>=ERROR; foo.bar=WARNING
// full command would thus be:
// !log barcode-scanner <root>=ERROR; foo.bar=WARNING
func (a *app) handleTelegramMessage(msg string) {
	if msg == "!restart barcode-scanner" {
		logger.Warningf("restarting due to command")
		time.Sleep(500 * time.Millisecond)
		a.exit()
		return
	}

	const pfx = "!log barcode-scanner "
	if len(msg) < len(pfx) || !strings.EqualFold(msg[:len(pfx)], pfx) {
		return
	}

	spec := msg[len(pfx)-1:]
	if _, err := loggo.ParseConfigString(spec); err != nil {
		logger.Warningf("failed parsing spec: %v, error was: %v", spec, err)
		return
	}

	loggo.DefaultContext().ResetLoggerLevels()
	if err := loggo.ConfigureLoggers(spec); err != nil {
		logger.Errorf("failed to apply log spec: %v, error was: %v", spec, err)
		return
	}

	logger.Debugf("logging spec successfully applied, spec was: %v", spec)
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

		// mark activity
		select {
		case a.activity <- struct{}{}:
		default:
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
			if a.screen.ShouldBlank() {
				a.screen.Blank()
			}

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

func (a *app) persistSettingsLocked() {
	s := &struct {
		Direction int
		Currier   string
		IdleStart time.Time
	}{
		Direction: int(a.dir),
		Currier:   a.currier,
		IdleStart: a.idleStart,
	}

	path := filepath.Join(a.cfg.StatePath, settingsPath)
	if err := file.Serialize(path, s); err != nil {
		logger.Warningf("Failed to serialize settings: %v", err)
	}
}

func (a *app) inExtendedIdle() bool {
	return !a.idleStart.IsZero() && time.Now().After(a.idleStart.Add(6*time.Hour))
}

func (a *app) loadSettings() {
	path := filepath.Join(a.cfg.StatePath, settingsPath)
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

	a.mu.Lock()
	defer a.mu.Unlock()

	a.dir = direction(s.Direction)
	a.currier = s.Currier
	a.idleStart = s.IdleStart
	logger.Debugf("Restored settings (dir=%v, currier=%v, idleStart=%v)", s.Direction, s.Currier, s.IdleStart)
}
