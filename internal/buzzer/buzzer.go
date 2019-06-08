package buzzer

import (
	"io"
	"os"
	"time"
)

const pwmBase = "/sys/class/pwm/pwmchip0"
const port = "/pwm0"
const beepDurr = 150 * time.Millisecond

var exported bool

func Setup() error {
	// echo 0 > /sys/class/pwm/pwmchip0/export
	// 2068hz
	// echo 241779 > /sys/class/pwm/pwmchip0/pwm0/duty_cycle
	// echo 483558 > /sys/class/pwm/pwmchip0/pwm0/period

	// already exported?
	if _, err := os.Stat(pwmBase + port); err == nil {
		exported = true
	}

	if !exported {
		err := write(pwmBase+"/export", "0")
		if err != nil {
			return err
		}
		exported = true
	}

	err := write(pwmBase+port+"/duty_cycle", "241779")
	if err != nil {
		return err
	}

	err = write(pwmBase+port+"/period", "483558")
	if err != nil {
		return err
	}

	return nil
}

func Beep() (err error) {
	err = write(pwmBase+port+"/enable", "1")
	if err != nil {
		return err
	}

	<-time.After(beepDurr)

	err = write(pwmBase+port+"/enable", "0")
	return
}

func write(path, value string) error {
	f, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	defer f.Close()

	n, err := f.WriteString(value)
	if err != nil {
		return err
	}

	if n < len(value) {
		return io.ErrShortWrite
	}

	return nil
}
