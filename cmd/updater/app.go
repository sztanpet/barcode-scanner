package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
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
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	go func(c chan os.Signal) {
		s := <-c
		logger.Warningf("Caught signal: %v, exiting", s)
		a.exit()
	}(c)
}

func (a *app) setupUpdate(binaryNames []string) error {
	// basic assumption:
	// all binaries have to be in the same directory
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

		b, err := update.NewBinary(bpath, a.cfg)
		if err != nil {
			logger.Criticalf("Could not create updater for: %v: %v", n, err)
			return err
		}

		if n == "updater" {
			b.Cleanup()
		}

		a.binaries = append(a.binaries, b)
	}

	logger.Tracef("updaters successfully set up: %v", binaryNames)
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

			logger.Tracef("Checking for updates")
			a.checkBinaries()

			a.checkService()
		}
	}
}

func (a *app) checkBinaries() {
	for _, b := range a.binaries {
		err := b.Check()
		if err != nil {
			logger.Warningf("Could not check for updates for %v: %v", b.Name, err)
		}
		time.Sleep(5 * time.Second)
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

func (a *app) checkService() {
	// check exit code of `pidof barcode-scanner`
	// if not running, reset service timer, and restart service?
	cmd := exec.CommandContext(a.ctx, "pidof", "barcode-scanner")
	out, err := cmd.CombinedOutput()
	if err != nil {
		logger.Warningf("pidof barcode-scanner failed: %v", err)
		return
	}

	if len(out) != 0 {
		logger.Tracef("barcode-scanner running with pid: %s", out)
		return
	}

	cmd = exec.CommandContext(a.ctx, "systemctl", "reset-failed")
	_, err = cmd.CombinedOutput()
	if err != nil {
		logger.Warningf("systemctl reset-failed failed: %v", err)
		return
	}

	logger.Infof("barcode-scanner was not running, systemctl reset-failed")
}
