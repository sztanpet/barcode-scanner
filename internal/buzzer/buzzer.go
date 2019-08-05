// +build !amd64

// buzzer uses the linux pwm driver to generate tones for a piezzo buzzer
// more info: blog.oddbit.com/post/2017-09-26-some-notes-on-pwm-on-the-raspberry-pi
package buzzer

import (
	"io"
	"os"
	"sync"
	"time"
)

const pwmBase = "/sys/class/pwm/pwmchip0"
const port = "/pwm0"
const beepDurr = 150 * time.Millisecond

var exported bool
var lastBeep time.Time
var running sync.Mutex
var once sync.Once

func Setup() (err error) {
	running.Lock()
	defer running.Unlock()

	once.Do(func() {
		go checkLastBeep()
	})

	if err = ensureExported(); err != nil {
		return err
	}

	deNoise()
	return
}

// TODO something two-tone, need to refactor this shit for that, maybe one day
func StartupBeep() (err error) {
	running.Lock()
	defer running.Unlock()
	defer markLastBeep()
	defer disable()

	if err = ensureExported(); err != nil {
		return err
	}

	enable()
	<-time.After(beepDurr / 3)
	disable()
	return
}

func SuccessBeep() (err error) {
	running.Lock()
	defer running.Unlock()
	defer markLastBeep()
	defer disable()

	if err = ensureExported(); err != nil {
		return err
	}

	enable()
	<-time.After(beepDurr)
	disable()
	return
}

func FailBeep() (err error) {
	running.Lock()
	defer running.Unlock()
	defer markLastBeep()
	defer disable()

	if err = ensureExported(); err != nil {
		return err
	}

	for i := 0; i < 4; i++ {
		enable()
		<-time.After(beepDurr / 2)
		disable()
		<-time.After(beepDurr / 2)
	}

	return
}

func markLastBeep() {
	lastBeep = time.Now()
}

func checkLastBeep() {
	t := time.NewTicker(5 * time.Minute)
	for {
		now := <-t.C

		running.Lock()
		if !exported {
			running.Unlock()
			continue
		}

		if lastBeep.Add(5 * time.Minute).After(now) {
			running.Unlock()
			continue
		}

		deNoise()
		running.Unlock()
	}
}

// deNoise silences any noise on the buzzer while idle.
// Depending on CPU usage seemingly, the transistor controlling the
// piezo buzzer drifts into it's active region because the noise on the
// pwm output becomes so big. This causes the components to heat up unnecessarily.
// The problem can be sidestepped by momentarily switching the output on.
func deNoise() {
	enable()
	time.Sleep(10 * time.Millisecond)
	disable()
	markLastBeep()
}

func ensureExported() error {
	// echo 0 > /sys/class/pwm/pwmchip0/export
	// 2068hz
	// echo 241779 > /sys/class/pwm/pwmchip0/pwm0/duty_cycle
	// echo 483558 > /sys/class/pwm/pwmchip0/pwm0/period

	if exported {
		return nil
	}

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

	err := write(pwmBase+port+"/period", "483558")
	if err != nil {
		return err
	}

	err = write(pwmBase+port+"/duty_cycle", "241779")
	if err != nil {
		return err
	}

	err = write(pwmBase+port+"/polarity", "normal")
	if err != nil {
		return err
	}

	deNoise()
	return nil
}

func unexport() {
	_ = write(pwmBase+"/unexport", "0")
	exported = false
}

func enable() {
	if !exported {
		return
	}

	if err := write(pwmBase+port+"/enable", "1"); err != nil {
		unexport()
	}
}

func disable() {
	if !exported {
		return
	}

	if err := write(pwmBase+port+"/enable", "0"); err != nil {
		unexport()
	}
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
