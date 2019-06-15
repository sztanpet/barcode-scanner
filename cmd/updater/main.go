package main

import (
	"context"
	"os"
	"strconv"
	"time"

	"code.sztanpet.net/zvpsz/barcode-scanner/internal/logwriter"
	"code.sztanpet.net/zvpsz/barcode-scanner/internal/telegram"
	"github.com/juju/loggo"
)

var logger = loggo.GetLogger("updater")
var updateDurr = 5 * time.Minute

func main() {
	baseURL, token, channelID := getEnvVars()
	ctx, exit := context.WithCancel(context.Background())

	bot, err := telegram.New(ctx, token, channelID)
	if err != nil {
		logger.Criticalf("Failed initializing telegram bot: %v", err)
		os.Exit(1)
	}
	err = logwriter.Setup(bot)
	if err != nil {
		logger.Criticalf("Failed initializing telegram bot: %v", err)
		os.Exit(1)
	}

	a := &app{
		ctx:     ctx,
		exit:    exit,
		baseURL: baseURL,
	}
	a.handleSignals()
	err = a.setupUpdate()
	if err != nil {
		logger.Criticalf("Failed setupUpdate: %v", err)
		os.Exit(1)
	}

	a.loop()
	// TODO health checks and update revert on detecting problems?

	os.Exit(0)
}

func getEnvVars() (baseURL string, token string, channelID int64) {
	baseURL = os.Getenv("UPDATE_BASEURL")
	if baseURL == "" {
		logger.Criticalf("Empty UPDATE_BASEURL env var!")
		os.Exit(1)
	}

	token = os.Getenv("TELEGRAM_TOKEN")
	if token == "" {
		logger.Criticalf("Empty TELEGRAM_TOKEN env var!")
		os.Exit(1)
	}

	cid := os.Getenv("TELEGRAM_CHANNELID")
	if cid == "" {
		logger.Criticalf("Empty TELEGRAM_CHANNELID env var!")
		os.Exit(1)
	}

	var err error
	channelID, err = strconv.ParseInt(cid, 10, 64)
	if err != nil {
		logger.Criticalf("Failed parsing TELEGRAM_CHANNELID env var!")
		os.Exit(1)
	}

	return
}
