package bot

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	tb "gopkg.in/tucnak/telebot.v2"

	"github.com/mhrivnak/preheatbot/pkg/heaterstore"
)

type Bot struct {
	sync.Mutex
	tbBot         *tb.Bot
	store         *heaterstore.Store
	heaterChanMap map[string][]chan<- string
}

func New(token string, store *heaterstore.Store) *Bot {
	b, err := tb.NewBot(tb.Settings{
		Token:  token,
		Poller: &tb.LongPoller{Timeout: 10 * time.Second},
	})
	if err != nil {
		log.Fatal(err.Error())
	}

	bot := Bot{
		tbBot:         b,
		store:         store,
		heaterChanMap: make(map[string][]chan<- string),
	}

	b.Handle("/hello", func(m *tb.Message) {
		if m.Private() && bot.store.UserExists(m.Sender.Username) {
			b.Send(m.Sender, "Hello from the hangar!")
		} else {
			b.Send(m.Sender, "I don't recognize you, "+m.Sender.Username)
		}
	})

	b.Handle(tb.OnText, func(m *tb.Message) {
		if m.Private() && bot.store.UserExists(m.Sender.Username) {
			msg, err := json.Marshal(m)
			if err != nil {
				log.Errorf("error marshaling JSON: %s", err.Error())
				return
			}
			log.Debug(string(msg))

			// If there is a pending value, assume this text is telling us which
			// heater to apply it to.
			pendingValue, err := bot.store.GetPendingValue(m.Sender.Username)
			if err != nil {
				log.Errorf("error getting pending value: %s", err.Error())
				return
			}
			log.Debugf("found pending value \"%s\" for user %s", pendingValue, m.Sender.Username)
			err = bot.store.Set(m.Sender.Username, m.Text, pendingValue)
			if err != nil {
				log.Errorf("error setting pending value: %s", err.Error())
				return
			}
			err = bot.store.DelPendingValue(m.Sender.Username)
			if err != nil {
				log.Errorf("failed to remove pending value for %s: %s", m.Sender.Username, err.Error())
			}
			count := bot.Set(m.Sender.Username, m.Text, pendingValue)
			b.Send(m.Sender, fmt.Sprintf("I set %d connections for %s to %s", count, m.Text, pendingValue))
		}
	})

	b.Handle("/on", bot.OnOffHandler("on"))
	b.Handle("on", bot.OnOffHandler("on"))

	b.Handle("/off", bot.OnOffHandler("off"))
	b.Handle("off", bot.OnOffHandler("off"))

	b.Handle("/status", func(m *tb.Message) {
		if m.Private() && bot.store.UserExists(m.Sender.Username) {
			message := ""
			ids, err := bot.store.IDs(m.Sender.Username)
			if err != nil {
				log.Errorf("error getting IDs: %s", err.Error())
				return
			}
			for _, heater := range ids {
				value, err := bot.store.Get(m.Sender.Username, heater)
				if err != nil {
					log.Errorf("error getting value: %s", err.Error())
					return
				}
				message = message + fmt.Sprintf("%s: %s\n", heater, value)
			}
			b.Send(m.Sender, message)
		} else {
			log.Infof("Got message from unknown user %s", m.Sender.Username)
			b.Send(m.Sender, "I don't recognize you, "+m.Sender.Username)
		}
	})
	return &bot
}

func (b *Bot) Start() {
	b.tbBot.Start()
}

func (b *Bot) Subscribe(username, heater string) <-chan string {
	b.Lock()
	defer b.Unlock()
	id := fmt.Sprintf("%s/%s", username, heater)
	chans, ok := b.heaterChanMap[id]
	if !ok {
		log.Debugf("created new channel slice for %s", id)
		chans = make([]chan<- string, 0)
	}
	newChan := make(chan string, 1)
	b.heaterChanMap[id] = append(chans, newChan)
	return newChan
}

func (b *Bot) Set(username, heater, value string) int {
	b.Lock()
	defer b.Unlock()
	var count int
	id := fmt.Sprintf("%s/%s", username, heater)
	heaterChans, ok := b.heaterChanMap[id]
	if ok {
		for _, heaterChan := range heaterChans {
			heaterChan <- value
		}
		count = len(heaterChans)
	}
	b.heaterChanMap[id] = make([]chan<- string, 0)

	log.Infof("Set %d connections for %s to %s", count, id, value)
	return count
}

func (b *Bot) menu(heaters []string) *tb.ReplyMarkup {
	// display a keyboard with options for each heater
	menu := tb.ReplyMarkup{
		ResizeReplyKeyboard: true,
		OneTimeKeyboard:     true,
	}
	rows := []tb.Row{}
	for _, heater := range heaters {
		rows = append(rows, menu.Row(menu.Text(heater)))
	}
	menu.Reply(rows...)
	return &menu
}

func (b *Bot) OnOffHandler(value string) func(*tb.Message) {
	return func(m *tb.Message) {
		if m.Private() && b.store.UserExists(m.Sender.Username) {
			ids, err := b.store.IDs(m.Sender.Username)
			if err != nil {
				log.Errorf("error getting IDs: %s", err.Error())
				return
			}
			// If the user has just one heater, assume that's the one to act on
			if len(ids) == 1 {
				err = b.store.Set(m.Sender.Username, ids[0], value)
				if err != nil {
					log.Errorf("error setting pending value: %s", err.Error())
					return
				}
				b.tbBot.Send(m.Sender, fmt.Sprintf("I set %s to %s", ids[0], value))
				return
			}

			b.store.SetPendingValue(m.Sender.Username, value)

			b.tbBot.Send(m.Sender, "Which heater?", b.menu(ids))
		} else {
			b.tbBot.Send(m.Sender, "I don't recognize you, "+m.Sender.Username)
		}
	}
}
