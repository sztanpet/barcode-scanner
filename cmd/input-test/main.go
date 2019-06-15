package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"

	"code.sztanpet.net/zvpsz/barcode-scanner/internal/tty"
)

func main() {
	c := make(chan os.Signal, 1)
	signal.Notify(c)
	go func(c chan os.Signal) {
		s := <-c
		fmt.Println("Got signal:", s)
	}(c)

	ctx, cancel := context.WithCancel(context.Background())
	t, err := tty.Open(ctx)
	if err != nil {
		log.Fatalf("input err: %v", err)
	}
	defer t.RestoreTermMode()

	for {
		r, _, err := t.ReadRune()
		if err != nil {
			log.Printf("readrune %q 0x%x error: %v", r, r, err)
			continue
		}

		fmt.Printf("read: %q 0x%x\n", r, r)
		if r == 4 {
			fmt.Printf("caught ctrl+d, exiting\n")
			cancel()
			break
		}
	}
}
