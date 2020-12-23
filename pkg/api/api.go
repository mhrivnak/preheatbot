package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"

	"github.com/mhrivnak/preheatbot/pkg/heaterstore"
)

type API struct {
	server     http.Server
	store      *heaterstore.Store
	subscriber Subscriber
}

type Subscriber interface {
	Subscribe(username, heater string) <-chan heaterstore.Record
}

func New(subscriber Subscriber, store *heaterstore.Store) *http.Server {
	log.Info("Starting API")

	r := mux.NewRouter()
	api := API{
		server: http.Server{
			Addr:    ":8080",
			Handler: r,
		},
		store:      store,
		subscriber: subscriber,
	}

	r.HandleFunc("/api/users/{username}/heaters/{heater}", api.HeaterHandler).Methods("GET")

	return &api.server
}

func (a *API) HeaterHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	username := vars["username"]
	heater := vars["heater"]
	hasVersion := -1
	hasVersionString := r.URL.Query().Get("version")
	if hasVersionString != "" {
		var err error
		hasVersion, err = strconv.Atoi(hasVersionString)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, "error parsing version string")
			return
		}
	}

	record, err := a.store.Get(username, heater)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, "error reading current value")
		log.WithError(err).Error("error reading current value")
		return
	}

	// If the client wants to long-poll and already has the current version,
	// then wait for the next version before responding.
	if r.URL.Query().Get("longpoll") != "" && record.Version == hasVersion {
		log.Infof("getting channel for %s/%s", username, heater)
		myChan := a.subscriber.Subscribe(username, heater)
		record = <-myChan
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	err = json.NewEncoder(w).Encode(record)
	if err != nil {
		log.WithError(err).Error("error serializing current value")
		return
	}
	log.Debug("Responded")
}
