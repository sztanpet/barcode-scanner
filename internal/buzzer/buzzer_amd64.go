// +build amd64

// buzzer uses the linux pwm driver to generate tones for a piezzo buzzer
// more info: blog.oddbit.com/post/2017-09-26-some-notes-on-pwm-on-the-raspberry-pi
package buzzer

import (
	"time"
)

const beepDurr = 150 * time.Millisecond

func Setup() error {
	return nil
}

// TODO something two-tone, need to refactor this shit for that, maybe one day
func StartupBeep() (err error) {
	<-time.After(beepDurr / 3)
	return
}

func SuccessBeep() (err error) {
	<-time.After(beepDurr)
	return
}

func FailBeep() (err error) {
	for i := 0; i < 4; i++ {
		<-time.After(beepDurr / 2)
		<-time.After(beepDurr / 2)
	}

	return
}
