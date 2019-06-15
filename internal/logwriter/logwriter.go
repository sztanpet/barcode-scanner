package logwriter

import (
	"fmt"
	"strings"
	"sync"

	"code.sztanpet.net/zvpsz/barcode-scanner/internal/telegram"
	"github.com/juju/loggo"
)

var defaultWriter *writer

type writer struct {
	bot *telegram.Bot

	mu      sync.Mutex
	builder strings.Builder
}

func Setup(bot *telegram.Bot) error {
	_, err := loggo.RemoveWriter("default")
	if err != nil {
		return err
	}

	defaultWriter = &writer{
		bot: bot,
	}
	err = loggo.RegisterWriter("default", defaultWriter)
	if err != nil {
		return err
	}

	return nil
}

func (w *writer) Write(e loggo.Entry) {
	line := w.formatEntry(e)
	if w.bot != nil {
		needNotification := e.Level >= loggo.WARNING
		err := w.bot.Send(line, !needNotification)
		if err != nil {
			fmt.Printf("bot send error: %v\n", err)
		}
	}

	fmt.Print(e.Timestamp.Format("[2006-01-02 15:04:05] "))
	fmt.Printf("%v:%v ", e.Filename, e.Line)
	fmt.Print(line)
	fmt.Print("\n")
}

func (w *writer) formatEntry(e loggo.Entry) string {
	w.mu.Lock()
	defer w.mu.Unlock()
	defer w.builder.Reset()

	// who can remember the order of the levels right?
	// indicate the level like T1 for TRACE D2 for debug, etc
	_, _ = w.builder.WriteRune('[')
	_ = w.builder.WriteByte(e.Level.String()[0])
	_, _ = w.builder.WriteString(string(48 + int(e.Level))) // poor mans strconv.Itoa
	_, _ = w.builder.WriteRune('|')
	_, _ = w.builder.WriteString(e.Module)
	_, _ = w.builder.WriteString("] ")
	_, _ = w.builder.WriteString(e.Message)
	return w.builder.String()
}

// Config configures logging according to the specification
// specification is a loggo.ConfigString
func (w *writer) Config(specification string) error {
	if err := loggo.ConfigureLoggers(specification); err != nil {
		return err
	}

	return nil
}
