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

	s.WriteTitle("WIFI SETUP")
	s.WriteLine(1, "SSID:")
	s.WriteLine(2, "fooBarŰÁÉÚŐÍÓÜÖ")
	s.WriteHelp("(enter when done)")

	time.Sleep(5 * time.Second)
}
