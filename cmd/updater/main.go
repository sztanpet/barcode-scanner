package main

import (
	"context"
	"os"
	"time"

	"code.sztanpet.net/zvpsz/barcode-scanner/internal/config"
	"code.sztanpet.net/zvpsz/barcode-scanner/internal/logwriter"
	"code.sztanpet.net/zvpsz/barcode-scanner/internal/telegram"
	"github.com/juju/loggo"
)

var logger = loggo.GetLogger("updater")
var updateDurr = 5 * time.Minute

func main() {
	cfg := config.Get()
	ctx, exit := context.WithCancel(context.Background())

	bot := telegram.New(ctx, cfg)
	err := logwriter.Setup(bot)
	if err != nil {
		logger.Criticalf("Failed initializing telegram bot: %v", err)
		os.Exit(1)
	}

	a := &app{
		ctx:  ctx,
		exit: exit,
		cfg:  cfg,
	}
	a.handleSignals()
	err = a.setupUpdate([]string{
		"barcode-scanner",
		"updater",
		"error-checker",
	})
	if err != nil {
		logger.Criticalf("Failed setupUpdate: %v", err)
		os.Exit(1)
	}

	a.loop()

	os.Exit(0)
}
