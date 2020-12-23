package api

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/mhrivnak/preheatbot/pkg/heaterstore"
	log "github.com/sirupsen/logrus"
)

type API struct {
	server     http.Server
	store      *heaterstore.Store
	subscriber Subscriber
}

type Subscriber interface {
	Subscribe(username, heater string) <-chan string
}

func New(subscriber Subscriber, store *heaterstore.Store) *http.Server {
	log.Info("Starting API")

	r := mux.NewRouter()
	api := API{
		server: http.Server{
			Addr:         ":8080",
			Handler:      r,
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 10 * time.Second,
		},
		store:      store,
		subscriber: subscriber,
	}

	r.HandleFunc("/api/users/{username}/heaters/{heater}", api.HeaterHandler).Methods("GET")

	return &api.server
}

func (a *API) HeaterHandler(w http.ResponseWriter, r *http.Request) {
	var value string
	vars := mux.Vars(r)
	username := vars["username"]
	heater := vars["heater"]

	if r.URL.Query().Get("longpoll") == "" {
		var err error
		value, err = a.store.Get(username, heater)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, "error reading current value")
			log.WithError(err).Error("error reading current value")
			return
		}
	} else {
		log.Infof("getting channel for %s/%s", username, heater)
		myChan := a.subscriber.Subscribe(username, heater)
		value = <-myChan
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, value)
}
