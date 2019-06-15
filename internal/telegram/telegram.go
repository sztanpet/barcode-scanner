package telegram

import (
	"context"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

type Bot struct {
	ctx       context.Context
	token     string
	channelID int64
	api       *tgbotapi.BotAPI
}

func New(ctx context.Context, token string, channelID int64) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, err
	}

	t := &Bot{
		ctx:       ctx,
		token:     token,
		channelID: channelID,
		api:       api,
	}
	return t, nil
}

// Send sends a message to the channel, optionally sending notifications depending on disableNotification
func (t *Bot) Send(txt string, disableNotification bool) error {
	msg := tgbotapi.NewMessage(t.channelID, txt)
	msg.DisableNotification = disableNotification
	_, err := t.api.Send(msg)
	return err
}

// HandleUpdates receives bot events, and calls callback with received messages
// old bot events are replayed on calling the method, except when onlyNewUpdates is true
func (t *Bot) HandleUpdates(callback func(msg string), onlyNewUpdates bool) error {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates, err := t.api.GetUpdatesChan(u)
	if err != nil {
		return err
	}
	if onlyNewUpdates {
		updates.Clear()
	}

	for {
		select {
		case <-t.ctx.Done():
			return nil
		case u, ok := <-updates:
			if !ok {
				return nil
			}

			if u.Message != nil {
				// time.Unix(int64(u.Message.Date), 0)
				callback(u.Message.Text)
			}
			if u.EditedMessage != nil {
				callback(u.EditedMessage.Text)
			}
			if u.ChannelPost != nil {
				callback(u.ChannelPost.Text)
			}
			if u.EditedChannelPost != nil {
				callback(u.EditedChannelPost.Text)
			}
		}
	}
}

// SelfMessage differentiates between messages sent to the bot
func (t *Bot) SelfMessage(txt string) bool {
	return strings.Contains(txt, "@"+t.api.Self.UserName)
}
