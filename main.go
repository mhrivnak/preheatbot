package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	tb "gopkg.in/tucnak/telebot.v2"

	"github.com/mhrivnak/preheatbot/pkg/heaterstore"
)

func main() {
	token := os.Getenv("APITOKEN")
	if token == "" {
		log.Fatal("must set envvar APITOKEN")
	}
	datadir := os.Getenv("DATADIR")
	if datadir == "" {
		log.Fatal("must set envvar DATADIR")
	}

	b, err := tb.NewBot(tb.Settings{
		Token:  token,
		Poller: &tb.LongPoller{Timeout: 10 * time.Second},
	})
	if err != nil {
		log.Fatal(err)
	}

	store := heaterstore.Store{
		Dir: datadir,
	}

	b.Handle("/hello", func(m *tb.Message) {
		if m.Private() && store.UserExists(m.Sender.Username) {
			b.Send(m.Sender, "Hello from the hangar!")
		} else {
			b.Send(m.Sender, "I don't recognize you, "+m.Sender.Username)
		}
	})

	b.Handle(tb.OnText, func(m *tb.Message) {
		if m.Private() && store.UserExists(m.Sender.Username) {
			msg, err := json.Marshal(m)
			if err != nil {
				return
			}
			log.Println(string(msg))

			pendingValue, err := store.GetPendingValue(m.Sender.Username)
			if err != nil {
				return
			}
			err = store.Set(m.Sender.Username, m.Text, pendingValue)
			if err != nil {
				return
			}
			b.Send(m.Sender, fmt.Sprintf("I set %s to %s", m.Text, pendingValue))
		}
	})

	b.Handle("/on", OnOffHandler(b, &store, "on"))
	b.Handle("on", OnOffHandler(b, &store, "on"))

	b.Handle("/off", OnOffHandler(b, &store, "off"))
	b.Handle("off", OnOffHandler(b, &store, "off"))

	b.Handle("/status", func(m *tb.Message) {
		if m.Private() && store.UserExists(m.Sender.Username) {
			message := ""
			ids, err := store.IDs(m.Sender.Username)
			if err != nil {
				log.Println(err.Error())
				return
			}
			for _, heater := range ids {
				value, err := store.Get(m.Sender.Username, heater)
				if err != nil {
					log.Println(err.Error())
					return
				}
				message = message + fmt.Sprintf("%s: %s\n", heater, value)
			}
			b.Send(m.Sender, message)
		} else {
			b.Send(m.Sender, "I don't recognize you, "+m.Sender.Username)
		}
	})

	b.Start()
}

func OnOffHandler(b *tb.Bot, store *heaterstore.Store, value string) func(*tb.Message) {
	return func(m *tb.Message) {
		if m.Private() && store.UserExists(m.Sender.Username) {
			menu := &tb.ReplyMarkup{
				ResizeReplyKeyboard: true,
				OneTimeKeyboard:     true,
			}
			rows := []tb.Row{}
			ids, err := store.IDs(m.Sender.Username)
			if err != nil {
				log.Println(err.Error())
				return
			}
			for _, heater := range ids {
				rows = append(rows, menu.Row(menu.Text(heater)))
			}
			menu.Reply(rows...)

			store.SetPendingValue(m.Sender.Username, value)

			b.Send(m.Sender, "Which heater?", menu)
		} else {
			b.Send(m.Sender, "I don't recognize you, "+m.Sender.Username)
		}
	}
}
