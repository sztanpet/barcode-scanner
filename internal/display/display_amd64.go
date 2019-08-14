// +build amd64

package display

import (
	"context"
	"sync"
	"time"
)

// The ScreenTimeout duration after which the display is blanked to prevent burn-in.
var ScreenTimeout = 1 * time.Hour

// lineCount defines how many lines of text fit on the screen
const lineCount = 4

type Screen struct {
	ctx context.Context

	mu         sync.Mutex
	lines      []string
	lastActive time.Time
}

func NewScreen(ctx context.Context) (*Screen, error) {
	ret := &Screen{
		ctx:        ctx,
		lines:      make([]string, lineCount),
		lastActive: time.Now(),
	}

	return ret, nil
}

// WriteTitle draws the text in black on a white background into the first line (line #0)
func (s *Screen) WriteTitle(text string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lines[0] = text
}

// WriteLine writes the text in white on black into the indicated line (usually #1 or #2)
func (s *Screen) WriteLine(line int, text string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lines[line] = text
}

// WriteHelp writes help text in black on white into the last line (line #3)
func (s *Screen) WriteHelp(text string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.lines[lineCount-1] = text
}

func (s *Screen) Clear() {
}

func (s *Screen) Draw() {
}

// Blank blanks the screen without clearing the image
func (s *Screen) Blank() {
}

func (s *Screen) ShouldBlank() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	blankAfter := s.lastActive.Add(ScreenTimeout)
	return time.Now().After(blankAfter)
}
