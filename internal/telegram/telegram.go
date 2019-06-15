package telegram

import (
	"context"
	"io"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"golang.org/x/time/rate"
)

// MaxSendDurr configures the limiter to send at most 1 message per MaxSendDurr
var MaxSendDurr = 500 * time.Millisecond

type Bot struct {
	ctx       context.Context
	token     string
	channelID int64
	api       *tgbotapi.BotAPI
	limiter   *rate.Limiter
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
		// limmit message spam to once every MaxSendDurr
		limiter: rate.NewLimiter(rate.Every(MaxSendDurr), 1),
	}
	return t, nil
}

// Send sends a message to the channel, optionally sending notifications depending on disableNotification
// internally ratelimited to once every 500ms
func (t *Bot) Send(txt string, disableNotification bool) (err error) {
	const postfixLength = 4
	const maxMessageSize = 4096 // https://github.com/yagop/node-telegram-bot-api/issues/165
	// 4*4096 bytes should be enough for everybody...
	if len(txt) > 9*maxMessageSize {
		panic("message too long")
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

func (t *Bot) SendFile(reader io.Reader, msg string, disableNotification bool) error {
	/*
		TODO: upload to ix.io; with baseauth username channelid and password hashed channelid
		and send message with link
		// -F 'read:1=2'
		$ echo Hello world. | curl -F 'f:1=<-' ix.io
		0000: 50 4f 53 54 20 2f 20 48 54 54 50 2f 31 2e 31 0d POST / HTTP/1.1.
		0010: 0a 48 6f 73 74 3a 20 69 78 2e 69 6f 0d 0a 55 73 .Host: ix.io..Us
		0020: 65 72 2d 41 67 65 6e 74 3a 20 63 75 72 6c 2f 37 er-Agent: curl/7
		0030: 2e 36 35 2e 31 0d 0a 41 63 63 65 70 74 3a 20 2a .65.1..Accept: *
		0040: 2f 2a 0d 0a 43 6f 6e 74 65 6e 74 2d 4c 65 6e 67 /*..Content-Leng
		0050: 74 68 3a 20 31 35 31 0d 0a 43 6f 6e 74 65 6e 74 th: 151..Content
		0060: 2d 54 79 70 65 3a 20 6d 75 6c 74 69 70 61 72 74 -Type: multipart
		0070: 2f 66 6f 72 6d 2d 64 61 74 61 3b 20 62 6f 75 6e /form-data; boun
		0080: 64 61 72 79 3d 2d 2d 2d 2d 2d 2d 2d 2d 2d 2d 2d dary=-----------
		0090: 2d 2d 2d 2d 2d 2d 2d 2d 2d 2d 2d 2d 2d 34 32 61 -------------42a
		00a0: 37 36 64 38 64 37 64 63 33 31 36 38 33 0d 0a 0d 76d8d7dc31683...
		00b0: 0a                                              .
		0000: 2d 2d 2d 2d 2d 2d 2d 2d 2d 2d 2d 2d 2d 2d 2d 2d ----------------
		0010: 2d 2d 2d 2d 2d 2d 2d 2d 2d 2d 34 32 61 37 36 64 ----------42a76d
		0020: 38 64 37 64 63 33 31 36 38 33 0d 0a 43 6f 6e 74 8d7dc31683..Cont
		0030: 65 6e 74 2d 44 69 73 70 6f 73 69 74 69 6f 6e 3a ent-Disposition:
		0040: 20 66 6f 72 6d 2d 64 61 74 61 3b 20 6e 61 6d 65  form-data; name
		0050: 3d 22 66 3a 31 22 0d 0a 0d 0a 48 65 6c 6c 6f 20 ="f:1"....Hello
		0060: 77 6f 72 6c 64 2e 0a 0d 0a 2d 2d 2d 2d 2d 2d 2d world....-------
		0070: 2d 2d 2d 2d 2d 2d 2d 2d 2d 2d 2d 2d 2d 2d 2d 2d ----------------
		0080: 2d 2d 2d 34 32 61 37 36 64 38 64 37 64 63 33 31 ---42a76d8d7dc31
		0090: 36 38 33 2d 2d 0d 0a                            683--..
	*/

	return nil
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
