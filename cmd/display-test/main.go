package main

import (
	"fmt"
	"time"

	"code.sztanpet.net/zvpsz/barcode-scanner/internal/display"
)

func main() {
	// TODO burn-in!
	// TODO led
	s, err := display.NewScreen()
	if err != nil {
		fmt.Printf("error: %v\n", err)
		return
	}

	// 4 sor max, a nulladik lesz a tetejen
	_ = s.WriteLine(0, "NULLADIK")
	_ = s.WriteLine(1, "HULLO")
	_ = s.WriteLine(2, "BUFF")
	_ = s.WriteLine(3, "DIKKDIKKDIKK")

	<-time.After(5 * time.Second)
	_ = s.Blank()
}
