package main

import (
	"errors"
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
	listenAddr := os.Getenv("LISTENADDR")
	if listenAddr == "" {
		listenAddr = ":8080"
		log.Infof("using default listen address %s", listenAddr)
	}

	store := heaterstore.Store{Dir: datadir}
	b := bot.New(token, &store)
	server := api.New(b, &store, listenAddr)
	exitChan := make(chan error)

	// start bot
	go func() {
		b.Start()
		exitChan <- errors.New("bot routine exited unexpectedly")
	}()

	// start API
	go func() {
		err := server.ListenAndServe()
		if err != nil {
			exitChan <- err
		}
		exitChan <- errors.New("http listener returned unexpectedly")
	}()

	err := <-exitChan
	log.WithError(err).Fatal("Exiting")
}
