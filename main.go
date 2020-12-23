package main

import (
	"os"

	log "github.com/sirupsen/logrus"

	"github.com/mhrivnak/preheatbot/pkg/api"
	"github.com/mhrivnak/preheatbot/pkg/bot"
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
	log.SetLevel(log.InfoLevel)
	debug := os.Getenv("DEBUG")
	if debug != "" {
		log.SetLevel(log.DebugLevel)
	}

	store := heaterstore.Store{Dir: datadir}
	b := bot.New(token, &store)
	server := api.New(b, &store)
	go server.ListenAndServe()
	b.Start()
}
