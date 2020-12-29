package api

import (
	"context"
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
	Subscribe(ctx context.Context, username, heater string) <-chan heaterstore.Record
}

func New(subscriber Subscriber, store *heaterstore.Store, listenAddr string) *http.Server {
	log.Info("Starting API")

	r := mux.NewRouter()
	api := API{
		server: http.Server{
			Addr:    listenAddr,
			Handler: r,
		},
		store:      store,
		subscriber: subscriber,
	}

	r.HandleFunc("/v1/users/{username}/heaters/{heater}", api.HeaterHandler).Methods("GET")

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
	if err != nil && a.store.IsNotExist(err) {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, "error reading current value")
		log.WithError(err).Error("error reading current value")
		return
	}

	// If the client wants to long-poll and already has the current version,
	// then wait for the next version before responding.
	if r.URL.Query().Get("longpoll") != "" && record.Version == hasVersion {
		log.Infof("starting long poll wait for %s/%s", username, heater)
		myChan := a.subscriber.Subscribe(r.Context(), username, heater)
		var stillOpen bool
		record, stillOpen = <-myChan
		if !stillOpen {
			log.Infof("long poll request on %s was canceled", r.RequestURI)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	err = json.NewEncoder(w).Encode(record)
	if err != nil {
		log.WithError(err).Error("error serializing current value")
		return
	}
	log.Infof("Sent version %d to %s/%s", record.Version, username, heater)
}
