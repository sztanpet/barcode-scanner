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
		fmt.Println("exited")
	}()
}

func (a *app) setupLogging() {
	if a.ctx.Err() != nil {
		return
	}
	logger.Tracef("setupLogging start")
	err := logwriter.Setup(a.bot, a.cfg)
	if err != nil {
		panic("logwriter setup failed, impossible: " + err.Error())
	}
	logger.Tracef("setupLogging end")
}

func (a *app) setupUpdate() {
	if a.ctx.Err() != nil {
		return
	}

	logger.Tracef("setupUpdate start")
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
	logger.Tracef("setupUpdate end")
}

func (a *app) setupStorage() {
	if a.ctx.Err() != nil {
		return
	}
	logger.Tracef("setupStorage start")
	storage, err := storage.New(a.ctx, a.cfg)
	if err != nil {
		logger.Criticalf("failed to initialize storage: %v", err)
		os.Exit(1)
	}

	a.storage = storage
	logger.Tracef("setupStorage end")
}

func (a *app) setupTelegram() {
	if a.ctx.Err() != nil {
		return
	}
	logger.Tracef("setupTelegram start")

	a.bot = telegram.New(a.ctx, a.cfg)
	_ = a.bot.Send("BS-start @ "+time.Now().Format(time.RFC3339), true)

	// TODO make telegram persist unsendable messages and retry automatically?
	go func() {
		err := a.bot.HandleMessage(a.handleTelegramMessage, false)

		if err != nil {
			logger.Criticalf("Handlemessage error: %v", err)
		}
	}()
	logger.Tracef("setupTelegram end")
}

func (a *app) setupScreen() {
	if a.ctx.Err() != nil {
		return
	}
	logger.Tracef("setupScreen start")
	screen, err := display.NewScreen(a.ctx)
	if err != nil {
		// screen handles its own logging, just exit
		fmt.Printf("screen err: %v", err)
		a.exit()
		return
	}
	a.screen = screen

	a.screen.WriteTitle("STARTUP")
	a.screen.WriteLine(1, "")
	a.screen.WriteLine(2, "OK")
	a.screen.WriteHelp("Scanner ready")

	a.addIdleTask(func() {
		a.screen.Blank()
	})

	logger.Tracef("setupScreen end")
}

func (a *app) setupBuzzer() {
	if a.ctx.Err() != nil {
		return
	}

	logger.Tracef("setupBuzzer start")
	if err := buzzer.Setup(); err != nil {
		logger.Warningf("buzzer setup error: %v", err)
	}
	if err := buzzer.StartupBeep(); err != nil {
		logger.Warningf("buzzer beep error: %v", err)
	}
	logger.Tracef("setupBuzzer end")
}

func (a *app) setupSettings() {
	logger.Tracef("setupSettings start")
	a.loadSettings()
	a.addIdleTask(func() {
		if a.inExtendedIdle() {
			a.dir = EGRESS
			a.currier = "0"
		}
	})
	logger.Tracef("setupSettings end")
}
