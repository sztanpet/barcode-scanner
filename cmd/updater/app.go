package main

import (
	"context"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	"code.sztanpet.net/zvpsz/barcode-scanner/internal/file"
	"code.sztanpet.net/zvpsz/barcode-scanner/internal/update"
)

type app struct {
	ctx        context.Context
	exit       context.CancelFunc
	baseURL    string
	binUpdate  *update.Binary
	binBarcode *update.Binary
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

func (a *app) setupUpdate() error {
	// basic assumption:
	// both binaries have to be in the same directory
	updpath, err := os.Executable()
	if err != nil {
		logger.Criticalf("os.Executable err: %v", err)
		return err
	}

	baseDir := filepath.Dir(updpath)
	bspath := baseDir + "/barcode-scanner"
	if !file.Exists(bspath) {
		logger.Criticalf("could not find barcode-scanner in the same dir as the updater: %v", bspath)
		return err
	}

	binUpdate, err := update.NewBinary(updpath, a.baseURL+"/updater")
	if err != nil {
		logger.Criticalf("Could not create update updater: %v", err)
		return err
	}
	a.binUpdate = binUpdate

	// cleanup after ourselves
	uu.Cleanup()

	binBarcode, err := update.NewBinary(bspath, a.baseURL+"/barcode-scanner")
	if err != nil {
		logger.Criticalf("Could not create barcode-scanner updater: %v", err)
		return err
	}
	a.binBarcode = binBarcode

	return nil
}

func (a *app) loop() {
	t := time.NewTicker(updateDurr)
	for {
		select {
		case <-a.ctx.Done():
			return
		case <-t.C:
			err := a.binUpdate.Check()
			if err != nil {
				logger.Warningf("Could not check for updates for the updater: %v", err)
			}

			if a.binUpdate.ShouldRestart() {
				logger.Warningf("Restarting due to update")
				return
			}

			err = a.binBarcode.Check()
			if err != nil {
				logger.Warningf("Could not check for updates for barcode-scanner: %v", err)
			}
		}
	}
}
