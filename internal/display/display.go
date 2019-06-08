package display

import (
	"fmt"
	"image"
	"sync"
	"time"

	"golang.org/x/image/font"
	"golang.org/x/image/font/inconsolata"
	"golang.org/x/image/math/fixed"
	"periph.io/x/periph/conn/i2c/i2creg"
	"periph.io/x/periph/devices/ssd1306"
	"periph.io/x/periph/devices/ssd1306/image1bit"
	"periph.io/x/periph/host"
)

// lineCount defines how many lines of text fit on the screen
const lineCount = 4

// textFont is the font used for displaying text
var textFont = inconsolata.Bold8x16

// The ScreenTimeout after which the display is blanked to prevent burn-in.
var ScreenTimeout = 10 * time.Minute

type Screen struct {
	dev        *ssd1306.Dev
	img        *image1bit.VerticalLSB
	lastActive time.Time

	mu    sync.Mutex
	lines []string
}

func NewScreen() (*Screen, error) {
	// TODO logging, error cleanup
	if _, err := host.Init(); err != nil {
		fmt.Printf("no display detected, skipping: %v", err)
		return nil, err
	}

	b, err := i2creg.Open("")
	if err != nil {
		fmt.Printf("could not open i2c bus, display disabled: %v", err)
		return nil, err
	}

	opts := ssd1306.DefaultOpts
	opts.Rotated = false
	dev, err := ssd1306.NewI2C(b, &opts)
	if err != nil {
		fmt.Printf("could not find ssd1306 screen, display disabled: %v", err)
		return nil, err
	}

	img := image1bit.NewVerticalLSB(dev.Bounds())

	return &Screen{
		dev:   dev,
		img:   img,
		lines: make([]string, lineCount),
	}, nil
}

func (s *Screen) Lines() []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	ret := make([]string, 0, lineCount)
	_ = copy(ret, s.lines)

	return ret
}

func (s *Screen) WriteLine(linenum int, text string) error {
	s.MarkActivity()
	s.mu.Lock()
	s.lines[linenum] = text
	s.mu.Unlock()

	height := s.img.Bounds().Dy() - textFont.Descent
	// by default, 0th line is at the bottom, 3rd is at the top,
	// invert it, because it feels better
	// 0th line should be the top, 3rd line should be at the bottom
	height -= (3 - linenum) * textFont.Height
	drawer := font.Drawer{
		Dst:  s.img,
		Src:  &image.Uniform{image1bit.On},
		Face: textFont,
		Dot:  fixed.P(0, height),
	}

	drawer.DrawString(text)
	return s.Draw()
}

func (s *Screen) Draw() error {
	return s.dev.Draw(s.dev.Bounds(), s.img, image.Point{})
}

func (s *Screen) Blank() error {
	s.MarkActivity()
	img := image1bit.NewVerticalLSB(s.dev.Bounds())
	return s.dev.Draw(s.dev.Bounds(), img, image.Point{})
}

func (s *Screen) MarkActivity() {
	s.lastActive = time.Now()
}

func (s *Screen) shouldBlank() bool {
	blankAfter := s.lastActive.Add(ScreenTimeout)
	return time.Now().Before(blankAfter)
}

func (s *Screen) HandleScreenSaver() {
	t := time.NewTicker(1 * time.Minute)
	for {
		<-t.C
		if s.shouldBlank() {
			_ = s.Blank()
		} else {
			_ = s.Draw()
		}
	}
}
