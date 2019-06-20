package logwriter

import (
	"fmt"
	"path/filepath"

	"code.sztanpet.net/zvpsz/barcode-scanner/internal/telegram"
	"github.com/juju/loggo"
)

type writer struct {
	bot *telegram.Bot
}

func Setup(bot *telegram.Bot) error {
	_, err := loggo.RemoveWriter("default")
	if err != nil {
		return err
	}

	defaultWriter := &writer{
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
	// who can remember the order of the levels right?
	// indicate the level like T1 for TRACE D2 for debug, etc
	return fmt.Sprintf(
		"[%v%v|%v:%v:%v] %v",
		rune(e.Level.String()[0]),
		int(e.Level),
		e.Module,
		filepath.Base(e.Filename),
		e.Line,
		e.Message,
	)
}
