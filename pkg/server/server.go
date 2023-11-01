package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/robertlestak/mytun/pkg/request"
	log "github.com/sirupsen/logrus"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "OK")
}

func handleSocketConnection(w http.ResponseWriter, r *http.Request) {
	l := log.WithFields(log.Fields{
		"app": "mytun",
		"cmd": "server.handleSocketConnection",
	})
	l.Debug("Handling socket connection")
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer conn.Close()
	clientId := uuid.New().String()
	// remove dashes
	clientId = strings.Replace(clientId, "-", "", -1)
	// trim to the first 8 characters
	clientId = clientId[:8]
	AddClient(clientId, conn)
	defer RemoveClient(clientId)
	log.WithFields(log.Fields{
		"client-id": clientId,
	}).Debug("Added client")
	// send the client their client ID
	if err := conn.WriteMessage(websocket.TextMessage, []byte(clientId)); err != nil {
		log.WithError(err).Error("Failed to send client ID")
		return
	}
	// now that the client has their id, keep the connection open
	// so we can proxy requests to the client
	// since we are reading the client message in the handleRequest function,
	// we can't read the client message here, otherwise the handleRequest function
	// won't be able to read it. Instead, we'll just wait for the client to close
	// the connection.
	<-ClientDone(clientId)
	log.WithFields(log.Fields{
		"client-id": clientId,
	}).Debug("Client done")
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

	// Extract request data
	requestData := request.RequestData{
		Method: r.Method,
		URL:    r.URL.String(),
		Header: r.Header,
	}
	requestBody, err := io.ReadAll(r.Body)
	if err != nil {
		log.WithError(err).Error("Failed to read request body")
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	// Set the request body in the RequestData struct
	requestData.Body = requestBody

	// Send the request data to the client via the WebSocket
	jsonRequest, err := json.Marshal(requestData)
	if err != nil {
		log.WithError(err).Error("Failed to marshal request data")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	if err := c.WriteMessage(websocket.TextMessage, jsonRequest); err != nil {
		log.WithError(err).Error("Failed to send request to client")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	// Receive the client's response from the WebSocket
	messageType, response, err := c.ReadMessage()
	if err != nil {
		log.WithError(err).Error("Failed to read response from client")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	if messageType != websocket.TextMessage {
		log.Error("Unexpected response type from client")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	var responseData request.ResponseData
	if err := json.Unmarshal(response, &responseData); err != nil {
		log.WithError(err).Error("Failed to unmarshal response data")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(responseData.StatusCode)
	for k, v := range responseData.Header {
		w.Header().Set(k, v[0])
	}
	if _, err := w.Write(responseData.Body); err != nil {
		log.WithError(err).Error("Failed to write response body")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
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

func SocketServer(listenAddr string) error {
	l := log.WithFields(log.Fields{
		"app":         "mytun",
		"cmd":         "server.SocketServer",
		"listen-addr": listenAddr,
	})
	l.Debug("Starting server")
	r := mux.NewRouter()
	r.HandleFunc("/health", handleHealthCheck)
	r.HandleFunc("/ws", handleSocketConnection)
	r.HandleFunc("/close/{id}", handleClientClosure).Methods("POST")
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
