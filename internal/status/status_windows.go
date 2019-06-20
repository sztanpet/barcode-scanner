// +build windows

package status

import (
	"context"

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

func (s *Status) Check(filepath string) {
	_ = s.bot.Send("yup, check on windows", true)
}
