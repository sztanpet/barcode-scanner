package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"code.sztanpet.net/zvpsz/barcode-scanner/internal/buzzer"
	"code.sztanpet.net/zvpsz/barcode-scanner/internal/display"
	"code.sztanpet.net/zvpsz/barcode-scanner/internal/logwriter"
	"code.sztanpet.net/zvpsz/barcode-scanner/internal/storage"
	"code.sztanpet.net/zvpsz/barcode-scanner/internal/telegram"
	"code.sztanpet.net/zvpsz/barcode-scanner/internal/update"
)

func (a *app) handleSignals() {
	if a.ctx.Err() != nil {
		return
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	go func() {
		s := <-c
		// exit unconditionally on any signal
		logger.Warningf("Got signal: %s, exiting cleanly", s)
		a.exit()
	}()
}

func (a *app) setupLogging() {
	if a.ctx.Err() != nil {
		return
	}

	err := logwriter.Setup(a.bot, a.cfg)
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
	a.upd.Cleanup()

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

	go func() {
		err := a.bot.HandleMessage(a.handleTelegramMessage, false)

		if err != nil {
			logger.Criticalf("Handlemessage error: %v", err)
		}
	}()
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

	a.screen.Clear()
	a.screen.WriteTitle("STARTUP")
	a.screen.WriteLine(1, "")
	a.screen.WriteLine(2, "OK")
	a.screen.WriteHelp("scanner ready")
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

func (a *app) setupSettings() {
	a.loadSettings()
	a.addIdleTask(func() {
		if a.inExtendedIdle() {
			a.dir = EGRESS
			a.currier = "0"
		}
	})
}
