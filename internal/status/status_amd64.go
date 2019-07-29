// +build amd64

package status

import (
	"context"

	"code.sztanpet.net/zvpsz/barcode-scanner/internal/file"
	"code.sztanpet.net/zvpsz/barcode-scanner/internal/telegram"
)

type Status struct {
	bot *telegram.Bot
}

func New(ctx context.Context, bot *telegram.Bot) *Status {
	return &Status{
		bot: bot,
	}
}

func (s *Status) Check() {
	_ = s.bot.Send("status.Check", true)
}

func (s *Status) CheckFile(path string) {
	var postfix = ") exists: "
	if file.Exists(path) {
		postfix += "yes"
	} else {
		postfix += "no"
	}
	_ = s.bot.Send("status.CheckFile("+path+postfix, true)
}
