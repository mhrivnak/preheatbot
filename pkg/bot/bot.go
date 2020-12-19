package bot

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	tb "gopkg.in/tucnak/telebot.v2"

	"github.com/mhrivnak/preheatbot/pkg/heaterstore"
)

type Bot struct {
	sync.Mutex
	tbBot *tb.Bot
	store *heaterstore.Store
	// {"<username>/<heaterID>": {"<randomUUID>": <channel>}}
	// Stores the channel used to tell an API handler that a value has
	// been set. The UUID is internally used to identify a channel when
	// the http client disconnects.
	heaterChanMap map[string]map[string]chan<- heaterstore.Record
}

func New(token string, store *heaterstore.Store) *Bot {
	b, err := tb.NewBot(tb.Settings{
		Token:    token,
		Poller:   &tb.LongPoller{Timeout: 10 * time.Second},
		Reporter: tbdebug,
	})
	if err != nil {
		log.Fatal(err.Error())
	}

	bot := Bot{
		tbBot:         b,
		store:         store,
		heaterChanMap: make(map[string]map[string]chan<- heaterstore.Record),
	}

	b.Handle("/hello", func(m *tb.Message) {
		if m.Private() && bot.store.UserExists(m.Sender.Username) {
			b.Send(m.Sender, "Hello from the hangar!")
		} else {
			b.Send(m.Sender, "I don't recognize you, "+m.Sender.Username)
		}
	})

	b.Handle(tb.OnText, bot.TextHandler)

	b.Handle("/on", bot.OnOffHandler("on"))
	b.Handle("on", bot.OnOffHandler("on"))

	b.Handle("/off", bot.OnOffHandler("off"))
	b.Handle("off", bot.OnOffHandler("off"))

	b.Handle("/status", bot.StatusHandler)
	b.Handle("status", bot.StatusHandler)
	return &bot
}

func (b *Bot) Start() {
	b.tbBot.Start()
}

// Subscribe returns a channel that will return one Record the next time the
// value for the specified heater is changed. The channel will then be closed.
func (b *Bot) Subscribe(ctx context.Context, username, heater string) <-chan heaterstore.Record {
	b.Lock()
	defer b.Unlock()
	heaterID := heaterID(username, heater)
	_, ok := b.heaterChanMap[heaterID]
	if !ok {
		log.Debugf("created new channel map for %s", heaterID)
		b.heaterChanMap[heaterID] = make(map[string]chan<- heaterstore.Record)
	}
	newChan := make(chan heaterstore.Record, 1)
	chanID := uuid.New().String()
	b.heaterChanMap[heaterID][chanID] = newChan
	go b.unsubscribe(ctx, heaterID, chanID)
	return newChan
}

// unsubscribe ensures that the channel specified gets closed and removed
// from the heaterChanMap. It is particularly useful when an API client
// disconnects before receiving a response.
func (b *Bot) unsubscribe(ctx context.Context, heaterID, chanID string) {
	<-ctx.Done()
	b.Lock()
	defer b.Unlock()
	_, ok := b.heaterChanMap[heaterID]
	if !ok {
		return
	}
	heaterChan, ok := b.heaterChanMap[heaterID][chanID]
	if !ok {
		return
	}
	close(heaterChan)
	delete(b.heaterChanMap[heaterID], chanID)
}

// Publish sends the Record to each channel that corresponds to the specified
// heater. It then removes each channel from the heaterChanMap.
func (b *Bot) Publish(username, heater string, r heaterstore.Record) int {
	b.Lock()
	defer b.Unlock()
	var count int
	id := heaterID(username, heater)
	heaterChans, ok := b.heaterChanMap[id]
	if ok {
		for _, heaterChan := range heaterChans {
			heaterChan <- r
		}
		count = len(heaterChans)
	}
	b.heaterChanMap[id] = make(map[string]chan<- heaterstore.Record, 0)

	log.Infof("Set %d connections for %s to %s", count, id, r.Value)
	return count
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
				record, err := b.store.Set(m.Sender.Username, ids[0], value)
				if err != nil {
					log.Errorf("error setting pending value: %s", err.Error())
					return
				}
				b.Publish(m.Sender.Username, ids[0], record)
				b.tbBot.Send(m.Sender, fmt.Sprintf("I set %s to %s", ids[0], value))
				return
			}

			b.store.SetPendingValue(m.Sender.Username, value)

			b.tbBot.Send(m.Sender, "Which heater?", menu(ids))
		} else {
			b.tbBot.Send(m.Sender, "I don't recognize you, "+m.Sender.Username)
		}
	}
}

func (b *Bot) TextHandler(m *tb.Message) {
	if m.Private() && b.store.UserExists(m.Sender.Username) {
		msg, err := json.Marshal(m)
		if err != nil {
			log.Errorf("error marshaling JSON: %s", err.Error())
			return
		}
		log.Debug(string(msg))

		// If there is a pending value, assume this text is telling us which
		// heater to apply it to.
		pendingValue, err := b.store.GetPendingValue(m.Sender.Username)
		if err != nil {
			log.Errorf("error getting pending value: %s", err.Error())
			return
		}
		log.Debugf("found pending value \"%s\" for user %s", pendingValue, m.Sender.Username)
		record, err := b.store.Set(m.Sender.Username, m.Text, pendingValue)
		if err != nil {
			log.Errorf("error setting pending value: %s", err.Error())
			return
		}
		err = b.store.DelPendingValue(m.Sender.Username)
		if err != nil {
			log.Errorf("failed to remove pending value for %s: %s", m.Sender.Username, err.Error())
		}
		count := b.Publish(m.Sender.Username, m.Text, record)
		b.tbBot.Send(m.Sender, fmt.Sprintf("I set %d connections for %s to %s", count, m.Text, pendingValue))
	}
}

func (b *Bot) StatusHandler(m *tb.Message) {
	if m.Private() && b.store.UserExists(m.Sender.Username) {
		message := ""
		ids, err := b.store.IDs(m.Sender.Username)
		if err != nil {
			log.Errorf("error getting IDs: %s", err.Error())
			return
		}
		for _, heater := range ids {
			record, err := b.store.Get(m.Sender.Username, heater)
			if err != nil {
				log.Errorf("error getting record: %s", err.Error())
				return
			}
			message = message + fmt.Sprintf("%s: %s\n", heater, record.Value)
		}
		b.tbBot.Send(m.Sender, message)
	} else {
		log.Infof("Got message from unknown user %s", m.Sender.Username)
		b.tbBot.Send(m.Sender, "I don't recognize you, "+m.Sender.Username)
	}
}

// menu creates a telegram keyboard with options for each heater
func menu(heaters []string) *tb.ReplyMarkup {
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

func heaterID(username, heater string) string {
	return fmt.Sprintf("%s/%s", username, heater)
}

// tbdebug logs errors from telebot. telebot's default behavior is to print
// errors with stack traces, which we don't want. More info:
// https://github.com/tucnak/telebot/issues/342
func tbdebug(err error) {
	log.WithFields(log.Fields{"error_source": "telebot"}).Error(err.Error())
}
