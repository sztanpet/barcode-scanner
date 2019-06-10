package main

import (
	"fmt"
	"time"

	"code.sztanpet.net/barcode-scanner/internal/buzzer"
)

func main() {
	// http://blog.oddbit.com/post/2017-09-26-some-notes-on-pwm-on-the-raspberry-pi/
	// echo 0 > /sys/class/pwm/pwmchip0/export
	// 2068hz
	// echo 241779 > /sys/class/pwm/pwmchip0/pwm0/duty_cycle
	// echo 483558 > /sys/class/pwm/pwmchip0/pwm0/period
	// 150ms on
	err := buzzer.Setup()
	if err != nil {
		fmt.Printf("err: %v\n", err)
	}
	for {
		err = buzzer.Beep()
		if err != nil {
			fmt.Printf("beep err: %v", err)
		}
		<-time.After(500 * time.Millisecond)
	}
}
