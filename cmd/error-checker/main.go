package main

import (
	"context"

	"code.sztanpet.net/zvpsz/barcode-scanner/internal/config"
	"code.sztanpet.net/zvpsz/barcode-scanner/internal/logwriter"
	"code.sztanpet.net/zvpsz/barcode-scanner/internal/telegram"
)

func main() {
	cfg := config.Get()
	ctx, exit := context.WithCancel(context.Background())
	bot := telegram.New(ctx, cfg)

	err := logwriter.Setup(bot)
	if err != nil {
		panic("logwriter setup failed, impossible")
	}

	// https://www.freedesktop.org/software/systemd/man/systemd.exec.html#%24EXIT_CODE
	// $EXIT_CODE is one of "exited", "killed", "dumped"
	// $SERVICE_RESULT:
	//    "success", "protocol", "timeout", "exit-code",
	//    "signal", "core-dump", "watchdog", "start-limit-hit", "resources"
	// $EXIT_STATUS: 0-255, or signal name
	exit()
}
