package main

import (
	"os"
	"path/filepath"
	"time"

	"code.sztanpet.net/barcode-scanner/internal/file"
	"code.sztanpet.net/barcode-scanner/internal/update"
	"github.com/juju/loggo"
)

var logger = loggo.GetLogger("main.updater")
var updateDurr = 5 * time.Minute

const baseURL = "https://update.sztanpet.net"

func main() {
	// basic assumption:
	// both binaries have to be in the same directory
	updpath, err := os.Executable()
	if err != nil {
		logger.Criticalf("os.Executable err: %v", err)
		os.Exit(1)
	}

	baseDir := filepath.Dir(updpath)
	bspath := baseDir + "barcode-scanner"
	if !file.Exists(bspath) {
		logger.Criticalf("could not find barcode-scanner in the same dir as the updater: %v", err)
		os.Exit(1)
	}

	uu, err := update.NewBinary(updpath, baseURL+"/updater")
	if err != nil {
		logger.Criticalf("Could not create update updater: %v", err)
		os.Exit(1)
	}
	uu.Cleanup()

	ubs, err := update.NewBinary(bspath, baseURL+"/barcode-scanner")
	if err != nil {
		logger.Criticalf("Could not create barcode-scanner updater: %v", err)
		os.Exit(1)
	}

	t := time.NewTicker(updateDurr)
	for {
		<-t.C
		err = uu.Check()
		if err != nil {
			logger.Warningf("Could not check for updates for the updater: %v", err)
		}

		if uu.ShouldRestart() {
			logger.Warningf("Restarting due to update")
			break
		}

		err = ubs.Check()
		if err != nil {
			logger.Warningf("Could not check for updates for barcode-scanner: %v", err)
		}

		// TODO health checks and update revert on detecting problems?
	}

	os.Exit(0)
}
