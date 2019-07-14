package telegram

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"code.sztanpet.net/zvpsz/barcode-scanner/internal/config"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"golang.org/x/time/rate"
)

// MaxSendDurr configures the limiter to send at most 1 message per MaxSendDurr
var MaxSendDurr = 500 * time.Millisecond

type Bot struct {
	ctx       context.Context
	token     string
	channelID int64
	limiter   *rate.Limiter

	mu  sync.Mutex
	api *tgbotapi.BotAPI
}

func New(ctx context.Context, cfg *config.Config) *Bot {
	api, err := tgbotapi.NewBotAPI(cfg.TelegramToken)
	if err != nil {
		api = nil
	}

	t := &Bot{
		ctx:       ctx,
		token:     cfg.TelegramToken,
		channelID: cfg.TelegramChannelID,
		api:       api,
		// limmit message spam to once every MaxSendDurr
		limiter: rate.NewLimiter(rate.Every(MaxSendDurr), 1),
	}
	return t
}

func (t *Bot) ensureAPI() (err error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.api != nil {
		return nil
	}

	t.api, err = tgbotapi.NewBotAPI(t.token)
	return
}

// Send sends a message to the channel, optionally sending notifications depending on disableNotification
// internally ratelimited to once every 500ms
func (t *Bot) Send(txt string, disableNotification bool) (err error) {
	err = t.ensureAPI()
	if err != nil {
		return err
	}
	t.mu.Lock()
	defer t.mu.Unlock()

	const postfixLength = 4
	const maxMessageSize = 4096 // https://github.com/yagop/node-telegram-bot-api/issues/165
	// 9*4092 bytes should be enough for everybody...
	if len(txt) > 9*(maxMessageSize-postfixLength) {
		return errors.New("Message too long")
	}

	s := []byte(txt)
	i := 1
	// send until there is something to send
	for len(s) > 0 {
		err = t.limiter.Wait(t.ctx)
		if err != nil {
			return err
		}

		end := maxMessageSize - postfixLength
		if len(s) < end {
			end = len(s)
		}
		tt := s
		// do we need to cut the message?
		if len(s) >= maxMessageSize {
			tt = append(s[:0:0], s[:end]...) // copy s
			tt = append(                     // append " (" + i + ")"
				tt,
				' ',
				'(',
				[]byte(string(48 + i))[0], // ascii 0 + i = "i"
				')',
			)
			i++
		}

		// adjust s
		if len(s) >= end {
			s = s[end:]
		}

		msg := tgbotapi.NewMessage(t.channelID, string(tt))
		msg.DisableNotification = disableNotification
		_, err = t.api.Send(msg)
	}

	return err
}

func (t *Bot) SendFile(data []byte, filename string, disableNotification bool) error {
	err := t.ensureAPI()
	if err != nil {
		return err
	}
	t.mu.Lock()
	defer t.mu.Unlock()

	r := tgbotapi.FileBytes{
		Name:  filename,
		Bytes: data,
	}

	d := tgbotapi.NewDocumentUpload(t.channelID, r)
	d.DisableNotification = disableNotification
	_, err = t.api.Send(d)
	return err
}

// HandleUpdates receives bot events, and calls callback with received messages
// old bot events are replayed on calling the method, except when onlyNewUpdates is true
func (t *Bot) HandleUpdates(callback func(msg string), onlyNewUpdates bool) error {
	err := t.ensureAPI()
	if err != nil {
		return err
	}
	t.mu.Lock()
	defer t.mu.Unlock()

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