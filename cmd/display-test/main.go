package main

import (
	"context"
	"fmt"
	"time"

	"code.sztanpet.net/zvpsz/barcode-scanner/internal/display"
)

func main() {
	// TODO burn-in!
	// TODO led
	s, err := display.NewScreen(context.Background())
	if err != nil {
		fmt.Printf("error: %v\n", err)
		return
	}

	defer s.Blank()

	_ = s.WriteTitle("WIFI SETUP")
	_ = s.WriteLine(1, "SSID:")
	_ = s.WriteLine(2, "fooBarŰÁÉÚŐÍÓÜÖ")
	_ = s.WriteHelp("(enter when done)")

	time.Sleep(5 * time.Second)
}
