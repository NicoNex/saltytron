package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/NicoNex/echotron/v3"
	"go.salty.im/saltyim"

	_ "embed"
)

const (
	NicoNex          = 41876271
	forbiddenSticker = "CAACAgIAAxkBAAEH8aZj_s6gLC33wJViUxYshH4XthTuWgACHgADr8ZRGtsCxVn3qdEpLgQ"
)

var (
	//go:embed token
	token    string
	saltyKey string
	dsp      *echotron.Dispatcher
	api      = echotron.NewAPI(token)
	commands = []echotron.BotCommand{
		{Command: "/recipient", Description: "Set the recipient the bot will send messages to."},
	}
)

type empty struct{}

func (e empty) Update(_ *echotron.Update) {}

type stateFn func(*echotron.Update) stateFn

type bot struct {
	chatID    int64
	state     stateFn
	recipient string
	salty     *saltyim.Client
	echotron.API
}

func newBot(chatID int64) echotron.Bot {
	if chatID != NicoNex {
		go func() {
			api.SendSticker(forbiddenSticker, chatID, nil)
			dsp.DelSession(chatID)
		}()
		return &empty{}
	}

	id, err := saltyim.GetIdentity(saltyim.WithIdentityPath(saltyKey))
	if err != nil {
		log.Fatal("saltyim.GetIdentity", err)
	}

	client, err := saltyim.NewClient(saltyim.WithIdentity(id))
	if err != nil {
		log.Fatal("saltyim.NewClient", err)
	}

	b := &bot{
		chatID: chatID,
		salty:  client,
		API:    api,
	}
	b.state = b.handleMessage
	go b.listen()

	return b
}

func (b *bot) listen() {
	for m := range b.salty.Subscribe(context.Background()) {
		if _, err := b.SendMessage(format(m.Text), b.chatID, nil); err != nil {
			log.Println("b.listen", err)
		}
	}
}

func (b *bot) handleRecipient(u *echotron.Update) stateFn {
	switch m := message(u); {
	case strings.HasPrefix(m, "/cancel"):
		b.messagef("ok")
		return b.handleMessage
	default:
		b.recipient = m
		b.messagef("Recipient set as %q", m)
		return b.handleMessage
	}
}

func (b *bot) handleMessage(u *echotron.Update) stateFn {
	switch m := message(u); {
	case strings.HasPrefix(m, "/recipient"):
		b.messagef("Send me the recipient name or /cancel.")
		return b.handleRecipient
	default:
		if b.recipient != "" {
			if err := b.salty.Send(b.recipient, m); err != nil {
				b.messagef("Send failed!")
				log.Println("handleMessage", "b.salty.Send", err)
			}
		} else {
			b.messagef("No recipient set, set one with /recipient")
		}
		return b.handleMessage
	}
}

func (b bot) messagef(f string, a ...any) (res echotron.APIResponseMessage, err error) {
	return b.SendMessage(fmt.Sprintf(f, a...), b.chatID, nil)
}

func (b *bot) Update(update *echotron.Update) {
	b.state = b.state(update)
}

func format(msg string) string {
	toks := strings.SplitN(msg, "\t", 3)
	if len(toks) != 3 {
		return msg
	}

	var tstr = toks[0]
	if t, err := time.Parse("2006-01-02T15:04:05Z", toks[0]); err == nil {
		tstr = t.Format(time.DateTime)
	} else {
		log.Println("format", "time.Parse", err)
	}

	return fmt.Sprintf(
		"%s <%s>\n%s",
		tstr,
		strings.Trim(toks[1], "()"),
		toks[2],
	)
}

// Returns the message from the given update.
func message(u *echotron.Update) string {
	if u.Message != nil {
		return u.Message.Text
	} else if u.EditedMessage != nil {
		return u.EditedMessage.Text
	} else if u.CallbackQuery != nil {
		return u.CallbackQuery.Data
	}
	return ""
}

func main() {
	flag.StringVar(&saltyKey, "k", "", "The key to be used by salty.im")
	flag.StringVar(&token, "t", token, "The token the bot will use")
	flag.Parse()

	if saltyKey == "" {
		fmt.Println("please provide the path to the salty.im key")
		flag.Usage()
		return
	}

	api.SetMyCommands(nil, commands...)
	dsp = echotron.NewDispatcher(token, newBot)
	for {
		log.Println("dispatcher", dsp.Poll())
		time.Sleep(5 * time.Second)
	}
}
