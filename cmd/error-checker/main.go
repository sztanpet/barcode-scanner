package main

import (
	"context"
	"flag"
	"os"
	"strings"
	"time"

	"code.sztanpet.net/zvpsz/barcode-scanner/internal/config"
	"code.sztanpet.net/zvpsz/barcode-scanner/internal/logwriter"
	"code.sztanpet.net/zvpsz/barcode-scanner/internal/telegram"
	"github.com/juju/loggo"
)

type app struct {
	ctx  context.Context
	exit context.CancelFunc
	cfg  *config.Config
	bot  *telegram.Bot
	bin  string
}

var logger = loggo.GetLogger("error-checker")
var binary = flag.String("binary", "", "the base name of the binary to check")
var logs = flag.String("logs", "", "the base name of the binaries for which logs should be checked, separated by commas\nif empty, defaults to the logs for the binary")

func init() {
	loggo.GetLogger("").SetLogLevel(loggo.TRACE)
}

func main() {
	cfg := config.Get()
	ctx, exit := context.WithCancel(context.Background())

	bot := telegram.New(ctx, cfg)
	err := logwriter.Setup(bot, cfg)
	if err != nil {
		logger.Criticalf("Failed initializing telegram bot: %v", err)
		os.Exit(1)
	}

	flag.Parse()
	if *binary == "" {
		logger.Criticalf("-binary not specified!")
		os.Exit(1)
	}
	if *logs == "" {
		*logs = *binary
	}
	binaries := strings.Split(*logs, ",")

	a := &app{
		ctx:  ctx,
		exit: exit,
		cfg:  cfg,
		bot:  bot,
		bin:  *binary,
	}
	a.handleSignals()
	a.handleLogs(binaries)
	a.handleServiceError()

	<-ctx.Done()
	time.Sleep(250 * time.Millisecond)
}
