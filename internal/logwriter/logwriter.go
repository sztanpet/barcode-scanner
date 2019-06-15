package logwriter

import (
	"fmt"
	"os"
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
			fmt.Fprintf(os.Stderr, "bot send error: %v\n", err)
		}
	}

	fmt.Fprintf(os.Stderr, e.Timestamp.Format("[2006-01-02 15:04:05] "))
	fmt.Fprintf(os.Stderr, "%v:%v ", e.Filename, e.Line)
	fmt.Fprintf(os.Stderr, line)
	fmt.Fprintf(os.Stderr, "\n")
}

func (w *writer) formatEntry(e loggo.Entry) string {
	w.mu.Lock()
	defer w.mu.Unlock()
	defer w.builder.Reset()

	_, _ = w.builder.WriteRune('[')
	_ = w.builder.WriteByte(e.Level.String()[0])
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
