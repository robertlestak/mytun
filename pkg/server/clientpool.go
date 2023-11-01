package server

import (
	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
)

var (
	Clients     = make(map[string]*websocket.Conn)
	ClientsDone = make(map[string]chan struct{})
)

func AddClient(clientId string, conn *websocket.Conn) {
	log.WithFields(log.Fields{
		"client-id": clientId,
	}).Debug("Adding client")
	Clients[clientId] = conn
	ClientsDone[clientId] = make(chan struct{})
}

func RemoveClient(clientId string) {
	log.WithFields(log.Fields{
		"client-id": clientId,
	}).Debug("Removing client")
	delete(Clients, clientId)
	delete(ClientsDone, clientId)
	if ClientsDone[clientId] != nil {
		close(ClientsDone[clientId])
	}
}

func ClientDone(clientId string) chan struct{} {
	return ClientsDone[clientId]
}
