package main

import (
	"context"
	"fmt"
	"log"

	"code.sztanpet.net/barcode-scanner/internal/input"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	i, err := input.New(ctx)
	if err != nil {
		log.Fatalf("input err: %v", err)
	}

	for {
		r, err := i.ReadRune()
		if err != nil {
			log.Fatalf("readrune error: %v", err)
		}

		fmt.Printf("read: %q %x\n", r, r)
		if r == 4 {
			fmt.Printf("caught ctrl+d, exiting\n")
			cancel()
			break
		}
	}
}
