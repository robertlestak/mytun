package server

import (
	log "github.com/sirupsen/logrus"
)

type Client struct {
	ID   string `json:"id"`
	IP   string `json:"ip"`
	Port int    `json:"port"`
}

var (
	Clients     = make(map[string]*Client)
	ClientsDone = make(map[string]chan struct{})
)

func AddClient(clientId string, c *Client) {
	log.WithFields(log.Fields{
		"client-id": clientId,
	}).Debug("Adding client")
	Clients[clientId] = c
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
