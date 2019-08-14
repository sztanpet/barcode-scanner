// +build !amd64

package display

import (
	"context"
	"image"
	"image/color"
	"image/draw"
	"sync"
	"time"

	"code.sztanpet.net/zvpsz/barcode-scanner/internal/sh1106"
	"github.com/golang/freetype/truetype"
	"github.com/juju/loggo"
	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gosmallcaps"
	"golang.org/x/image/font/inconsolata"
	"golang.org/x/image/math/fixed"
	"periph.io/x/periph/conn/i2c/i2creg"
	"periph.io/x/periph/devices/ssd1306/image1bit"
	"periph.io/x/periph/host"
)

var logger = loggo.GetLogger("main.display")

// The ScreenTimeout duration after which the display is blanked to prevent burn-in.
var ScreenTimeout = 90 * time.Minute

// lineCount defines how many lines of text fit on the screen
const lineCount = 4

var mediumFont = inconsolata.Bold8x16
var titleFont = inconsolata.Regular8x16
var helpFont font.Face

const white = image1bit.On
const black = image1bit.Off

type Screen struct {
	ctx context.Context
	dev *sh1106.Dev

	mu         sync.Mutex
	img        *image1bit.VerticalLSB
	lines      []string
	lastActive time.Time
}

func init() {
	// tiny help text font on the bottom
	ff, err := truetype.Parse(gosmallcaps.TTF)
	if err != nil {
		panic(err.Error())
	}
	helpFont = truetype.NewFace(ff, &truetype.Options{
		Size:    12,
		Hinting: font.HintingNone,
	})
}

func NewScreen(ctx context.Context) (*Screen, error) {
	if _, err := host.Init(); err != nil {
		logger.Criticalf("no display detected, skipping: %v", err)
		return nil, err
	}

	b, err := i2creg.Open("")
	if err != nil {
		logger.Criticalf("could not open i2c bus, display disabled: %v", err)
		return nil, err
	}

	opts := sh1106.DefaultOpts
	opts.Rotated = false // TODO finalize physical position of screen
	dev, err := sh1106.NewI2C(b, &opts)
	if err != nil {
		logger.Criticalf("could not find sh1106 screen, display disabled: %v", err)
		return nil, err
	}

	_ = dev.SetContrast(0xFF)

	img := image1bit.NewVerticalLSB(dev.Bounds())

	ret := &Screen{
		ctx:        ctx,
		dev:        dev,
		img:        img,
		lines:      make([]string, lineCount),
		lastActive: time.Now(),
	}
	ret.Clear()

	return ret, nil
}

func (s *Screen) writeUnlocked(f font.Face, line int, c color.Color, centered bool, text string) {
	s.markActivity()
	m := f.Metrics()
	height := s.img.Bounds().Dy() - m.Descent.Round()
	// by default, 0th line is at the bottom, 3rd is at the top,
	// invert it, because it feels better
	// 0th line should be the top, 3rd line should be at the bottom
	height -= (3 - line) * m.Height.Round()
	drawer := font.Drawer{
		Dst:  s.img,
		Src:  &image.Uniform{c},
		Face: f,
		Dot:  fixed.P(0, height),
	}
	/*
		TODO
		if text == "" {
			text = " "
		}
	*/
	bounds, adv := drawer.BoundString(text)

	// adjust text start position
	if centered {
		x := s.img.Bounds().Dx()/2 - adv.Round()/2
		drawer.Dot = fixed.P(x, height)
	}

	// if we need to write in black, the background needs to be white,
	bg := black
	if c == black {
		bg = white
	}

	// add 2 pixels to the height because it looks better that way
	r := image.Rect(bounds.Min.X.Round(), bounds.Min.Y.Round(), s.img.Bounds().Dx(), height+1)
	draw.Draw(s.img, r, &image.Uniform{bg}, image.ZP, draw.Src)

	drawer.DrawString(text)
}

// WriteTitle draws the text in black on a white background into the first line (line #0)
func (s *Screen) WriteTitle(text string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lines[0] = text

	s.writeUnlocked(titleFont, 0, black, true, text)
	s.drawUnlocked()
}

// WriteLine writes the text in white on black into the indicated line (usually #1 or #2)
func (s *Screen) WriteLine(line int, text string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lines[line] = text

	s.writeUnlocked(mediumFont, line, white, false, text)
	s.drawUnlocked()
}

// WriteHelp writes help text in black on white into the last line (line #3)
func (s *Screen) WriteHelp(text string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.lines[lineCount-1] = text
	s.writeUnlocked(helpFont, lineCount-1, black, true, text)
	s.drawUnlocked()
}

// Clear clears the image
func (s *Screen) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.img = image1bit.NewVerticalLSB(s.dev.Bounds())
	s.drawUnlocked()
}

// Draw display the image
func (s *Screen) Draw() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.drawUnlocked()
}

func (s *Screen) drawUnlocked() {
	if err := s.dev.Draw(s.dev.Bounds(), s.img, image.Point{}); err != nil {
		logger.Errorf("draw error: %v", err)
	}
}

// Blank blanks the screen without clearing the image
func (s *Screen) Blank() {
	logger.Tracef("blanking screen")
	if err := s.dev.Halt(); err != nil {
		logger.Errorf("halt error: %v", err)
	}
}

func (s *Screen) markActivity() {
	if time.Now().Sub(s.lastActive) > 1*time.Hour {
		logger.Tracef("activity after more than an hour")
	}

	s.lastActive = time.Now()
}

func (s *Screen) ShouldBlank() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	blankAfter := s.lastActive.Add(ScreenTimeout)
	return time.Now().After(blankAfter)
}
