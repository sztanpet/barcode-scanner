package gpio

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
)

// orangepi pc plus gpio numbering:
// (position of letter in alphabet - 1) * 32 + pin number
// Beeper - PA20 => 20
const base = "/sys/class/gpio"
const beepDurr = 150 * time.Millisecond
const flashDurr = 500 * time.Millisecond

type dir string

var (
	in  dir = "in"
	out dir = "out"
)

type pin struct {
	mu  sync.Mutex
	pin string
}

func (p *pin) String() string {
	return "GPIO PIN: " + p.pin
}

func (p *pin) Enable() error {
	return write(gpioPath(p.pin, "value"), "1")
}

func (p *pin) Disable() {
	_ = write(gpioPath(p.pin, "value"), "0")
}

func (p *pin) Flash() {
	if err := p.Enable(); err != nil {
		return
	}

	time.Sleep(flashDurr)
	p.Disable()
}

func (p *pin) export() error {
	path := base + "/gpio" + p.pin
	if _, err := os.Stat(path); err == nil {
		return nil // already exported
	}

	if err := write(base+"/export", p.pin); err != nil {
		return fmt.Errorf("Failed to export: %v %v", p, err)
	}

	return nil
}

func (p *pin) direction(d dir) error {
	if err := write(gpioPath(p.pin, "direction"), string(d)); err != nil {
		return fmt.Errorf("Failed to set direction 'out': %v %v", p, err)
	}

	return nil
}

// directionPin is always on, and is controlled by setting the GPIO direction instead
type directionPin struct {
	pin
}

func (p *directionPin) Enable() error {
	return p.pin.direction(in)
}

func (p *directionPin) Disable() {
	_ = p.pin.direction(out)
}

type beepPin struct {
	pin
}

func (p *beepPin) StartupBeep() (err error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if err := p.Enable(); err != nil {
		return err
	}
	<-time.After(beepDurr / 3)
	p.Disable()
	return
}

func (p *beepPin) SuccessBeep() (err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	defer p.Disable()

	if err := p.Enable(); err != nil {
		return err
	}
	time.Sleep(beepDurr)
	p.Disable()
	return
}

func (p *beepPin) FailBeep() (err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	defer p.Disable()

	for i := 0; i < 4; i++ {
		if err := p.Enable(); err != nil {
			return err
		}
		time.Sleep(beepDurr / 2)
		p.Disable()
		time.Sleep(beepDurr / 2)
	}

	return
}

var (
	Beeper   = beepPin{pin{pin: "20"}}
	GreenLED = pin{pin: "8"}
	BlueLED  = pin{pin: "9"}

	// red pin is special! always on by default
	RedLED = directionPin{pin{pin: "10"}}
)

func Setup() (err error) {
	if err := Beeper.export(); err != nil {
		return err
	}
	if err := Beeper.direction(out); err != nil {
		return err
	}
	if err := GreenLED.export(); err != nil {
		return err
	}
	if err := GreenLED.direction(out); err != nil {
		return err
	}
	if err := BlueLED.export(); err != nil {
		return err
	}
	if err := BlueLED.direction(out); err != nil {
		return err
	}
	if err := RedLED.export(); err != nil {
		return err
	}

	return
}

func gpioPath(p string, file string) string {
	return base + "/gpio" + p + "/" + file
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

func successFlash() (err error) {
	GreenLED.Disable()
	defer func() {
		err = GreenLED.Enable()
	}()

	if err := BlueLED.Enable(); err != nil {
		return err
	}
	time.Sleep(flashDurr)
	BlueLED.Disable()
	return
}

func failFlash() (err error) {
	GreenLED.Disable()
	defer func() {
		err = GreenLED.Enable()
	}()

	if err := RedLED.Enable(); err != nil {
		return err
	}
	time.Sleep(flashDurr)
	RedLED.Disable()
	return
}

func Success(ctx context.Context) error {
	g, _ := errgroup.WithContext(ctx)
	g.Go(Beeper.SuccessBeep)
	g.Go(successFlash)
	return g.Wait()
}

func Fail(ctx context.Context) error {
	g, _ := errgroup.WithContext(ctx)
	g.Go(Beeper.FailBeep)
	g.Go(failFlash)
	return g.Wait()
}
