package logwriter

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"code.sztanpet.net/zvpsz/barcode-scanner/internal/config"
	"code.sztanpet.net/zvpsz/barcode-scanner/internal/file"
	"code.sztanpet.net/zvpsz/barcode-scanner/internal/telegram"
	"github.com/juju/loggo"
)

type writer struct {
	bot *telegram.Bot
}

var logPath string

func Setup(bot *telegram.Bot, cfg *config.Config) error {
	if path, err := os.Executable(); err != nil {
		panic("os.Executable() failed! " + err.Error())
	} else {
		logPath = filepath.Join(
			cfg.StatePath,
			filepath.Base(path)+".log",
		)
	}

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

	fp := e.Filename
	ix := strings.Index(e.Filename, "barcode-scanner/")
	if ix != -1 {
		fp = fp[ix+len("barcode-scanner/"):]
	}

	l := fmt.Sprintf("%v%v:%v %v\n",
		e.Timestamp.Format("[2006-01-02 15:04:05] "),
		fp, e.Line,
		line,
	)
	if err := file.Append(logPath, []byte(l)); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write log file: %v\n", err)
	}

	go func() {
		if w.bot != nil {
			needNotification := e.Level >= loggo.WARNING
			err := w.bot.Send(line, !needNotification)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%v bot send error: %v\n", e.Timestamp.Format("[2006-01-02 15:04:05]"), err)
			}
		}
	}()
}

func (w *writer) formatEntry(e loggo.Entry) string {
	// who can remember the order of the levels right?
	// indicate the level like T1 for TRACE D2 for debug, etc
	return fmt.Sprintf(
		"[%v%v|%v:%v:%v] %v",
		string(e.Level.String()[0]),
		int(e.Level),
		e.Module,
		filepath.Base(e.Filename),
		e.Line,
		e.Message,
	)
}
