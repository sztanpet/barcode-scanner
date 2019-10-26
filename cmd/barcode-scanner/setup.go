package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"code.sztanpet.net/zvpsz/barcode-scanner/internal/buzzer"
	"code.sztanpet.net/zvpsz/barcode-scanner/internal/display"
	"code.sztanpet.net/zvpsz/barcode-scanner/internal/gpio"
	"code.sztanpet.net/zvpsz/barcode-scanner/internal/logwriter"
	"code.sztanpet.net/zvpsz/barcode-scanner/internal/storage"
	"code.sztanpet.net/zvpsz/barcode-scanner/internal/telegram"
	"code.sztanpet.net/zvpsz/barcode-scanner/internal/update"
	"code.sztanpet.net/zvpsz/barcode-scanner/internal/wifi"
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
			logger.Criticalf("barcode-scanner restarting cleanly because of update")
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

func (a *app) setupDeviceID() {
	go func() {
		for {
			did, err := a.storage.SetupDevice(a.cfg)
			if err == nil {
				logger.Tracef("got deviceid: %v", did)
				return
			}
			logger.Tracef("failed to get deviceid retrying in a minute, err: %v", err)
			time.Sleep(1 * time.Minute)
		}
	}()
}

func (a *app) setupTelegram() {
	if a.ctx.Err() != nil {
		return
	}

	a.bot = telegram.New(a.ctx, a.cfg)
	_ = a.bot.Send("BS-start @ "+time.Now().Format(time.RFC3339), true)

	go func() {
		for {
			_ = a.bot.HandleMessage(a.handleTelegramMessage, false)
			time.Sleep(1 * time.Minute)
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

func (a *app) setupHardware() {
	if a.cfg.HardwareVersion < 2 {
		a.setupBuzzer()
	} else {
		a.setupGPIO()
	}
}

func (a *app) setupBuzzer() {
	if a.ctx.Err() != nil {
		return
	}

	if err := buzzer.Setup(); err != nil {
		logger.Warningf("buzzer setup error: %v", err)
	}
}

func (a *app) setupGPIO() {
	if a.ctx.Err() != nil {
		return
	}

	if err := gpio.Setup(); err != nil {
		logger.Criticalf("GPIO setup failed: %v", err)
		return
	}
}

func (a *app) setupSettings() {
	a.loadSettings()
	a.addIdleTask(func() {
		if a.inExtendedIdle() && (a.dir != EGRESS || a.currier != "0") {
			a.dir = EGRESS
			a.currier = "0"
			a.persistSettingsLocked()
			a.writeBarcodeTitle()
			a.screen.Blank()
		}
	})
}

func (a *app) setupWiFi() {
	go func() {
		// try connecting right away if not connected
		if !wifi.IsConnected() {
			_ = wifi.Setup(a.ctx, a.cfg)
		}

		// otherwise check every N minutes if still connected and try reconnecting if not
		t := time.NewTicker(5 * time.Minute)
		for {
			select {
			case <-a.ctx.Done():
				return
			case <-t.C:
				// check 3 times if we are not connected (to detect a transient wifi connection)
				// should reduce spurious wifi re-initialization
				ok := false
				for i := 3; i > 0; i-- {
					if a.ctx.Err() != nil || wifi.IsConnected() {
						ok = true
						break
					}

					time.Sleep(30 * time.Second)
				}

				if !ok {
					logger.Warningf("no internet connection detected, running wifi.Setup")
					_ = wifi.Setup(a.ctx, a.cfg)
				}
			}
		}
	}()
}
