package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
)

func handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "OK")
}

func handleRequest(w http.ResponseWriter, r *http.Request) {
	var clientId string

	// Get client for request
	// Host can be in the following format: <uuid>.<domain>.<tld>
	hsplit := strings.Split(r.Host, ".")
	clientId = hsplit[0]

	log.WithFields(log.Fields{
		"app":       "mytun",
		"cmd":       "server.handleRequest",
		"client-id": clientId,
		"method":    r.Method,
		"url":       r.URL.String(),
	}).Debug("Handling request")

	c, ok := Clients[clientId]
	if !ok {
		log.WithFields(log.Fields{
			"app":       "mytun",
			"cmd":       "server.handleRequest",
			"client-id": clientId,
		}).Error("Client not found")
		http.Error(w, "Client not found", http.StatusNotFound)
		return
	}
	// proxy the request to the client ip/port
	target := fmt.Sprintf("http://%s:%d", c.IP, c.Port)
	targetUrl, err := url.Parse(target)
	if err != nil {
		log.WithFields(log.Fields{
			"app":       "mytun",
			"cmd":       "server.handleRequest",
			"client-id": clientId,
			"target":    target,
		}).Error("Error parsing target url")
		http.Error(w, "Error parsing target url", http.StatusInternalServerError)
		return
	}
	proxy := httputil.NewSingleHostReverseProxy(targetUrl)
	proxy.ServeHTTP(w, r)
}

func handleClientClosure(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	clientId := vars["id"]
	log.WithFields(log.Fields{
		"app":       "mytun",
		"cmd":       "server.handleClientClosure",
		"client-id": clientId,
	}).Debug("Closing client")
	if Clients[clientId] != nil {
		close(ClientDone(clientId))
	}
}

func handleClientConnect(w http.ResponseWriter, r *http.Request) {
	fullId := uuid.New().String()
	clientId := fullId[:8]
	defer r.Body.Close()
	c := &Client{}
	if err := json.NewDecoder(r.Body).Decode(c); err != nil {
		http.Error(w, "Error decoding request", http.StatusBadRequest)
		return
	}
	c.ID = clientId
	AddClient(clientId, c)
	log.WithFields(log.Fields{
		"app":       "mytun",
		"cmd":       "server.handleClientConnect",
		"client-id": clientId,
		"client-ip": c.IP,
		"client-p":  c.Port,
	}).Debug("Connecting client")
	fmt.Fprintf(w, "%s", clientId)
}

func InternalServer(listenAddr string) error {
	l := log.WithFields(log.Fields{
		"app":         "mytun",
		"cmd":         "server.InternalServer",
		"listen-addr": listenAddr,
	})
	l.Debug("Starting server")
	r := mux.NewRouter()
	r.HandleFunc("/health", handleHealthCheck)
	r.HandleFunc("/close/{id}", handleClientClosure).Methods("POST")
	r.HandleFunc("/connect", handleClientConnect).Methods("POST")
	if err := http.ListenAndServe(listenAddr, r); err != nil {
		return err
	}
	return nil
}

func PublicServer(listenAddr string) error {
	l := log.WithFields(log.Fields{
		"app":         "mytun",
		"cmd":         "server.PublicServer",
		"listen-addr": listenAddr,
	})
	l.Debug("Starting server")
	r := mux.NewRouter()
	r.HandleFunc("/health", handleHealthCheck)
	r.PathPrefix("/").HandlerFunc(handleRequest)
	if err := http.ListenAndServe(listenAddr, r); err != nil {
		return err
	}
	return nil
}
