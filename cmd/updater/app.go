package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	"code.sztanpet.net/zvpsz/barcode-scanner/internal/config"
	"code.sztanpet.net/zvpsz/barcode-scanner/internal/file"
	"code.sztanpet.net/zvpsz/barcode-scanner/internal/update"
)

type app struct {
	ctx      context.Context
	exit     context.CancelFunc
	cfg      *config.Config
	binaries []*update.Binary
}

func (a *app) handleSignals() {
	c := make(chan os.Signal, 1)
	signal.Notify(c)
	go func(c chan os.Signal) {
		s := <-c
		logger.Debugf("Caught signal: %v, exiting", s)
		a.exit()
	}(c)
}

func (a *app) setupUpdate(binaryNames []string) error {
	// basic assumption:
	// both binaries have to be in the same directory
	updpath, err := os.Executable()
	if err != nil {
		logger.Criticalf("os.Executable err: %v", err)
		return err
	}

	baseDir := filepath.Dir(updpath)
	for _, n := range binaryNames {
		bpath := filepath.Join(baseDir, n)
		if !file.Exists(bpath) {
			logger.Criticalf("could not find %v at %v", n, bpath)
			return fmt.Errorf("could not find %v at %v", n, bpath)
		}

		b, err := update.NewBinary(updpath, a.cfg)
		if err != nil {
			logger.Criticalf("Could not create update for: %v: %v", n, err)
			return err
		}

		a.binaries = append(a.binaries, b)
	}

	return nil
}

func (a *app) loop() {
	t := time.NewTicker(updateDurr)
	for {
		select {
		case <-a.ctx.Done():
			return
		case <-t.C:
			if a.shouldRestart() {
				logger.Infof("Updater restarting cleanly because of update")
				return
			}

			logger.Tracef("Checking error logs")
			a.checkLogs()

			logger.Tracef("Checking for updates")
			a.checkBinaries()
		}
	}
}

func (a *app) checkBinaries() {
	for _, b := range a.binaries {
		err := b.Check()
		if err != nil {
			logger.Warningf("Could not check for updates for %v: %v", b.Name, err)
		}
	}
}

func (a *app) checkLogs() {
	dir, _ := os.Executable()
	dir = filepath.Dir(dir)
	for _, b := range a.binaries {
		path := filepath.Join(dir, b.Name+".output")
		b.CheckFile(path)
	}
}

func (a *app) shouldRestart() bool {
	path, _ := os.Executable()
	name := filepath.Base(path)
	for _, b := range a.binaries {
		if b.Name == name {
			return b.ShouldRestart()
		}
	}

	panic("Could not find updater for binary: " + name)
}
