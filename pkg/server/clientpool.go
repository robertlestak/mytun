package server

import (
	"errors"

	log "github.com/sirupsen/logrus"
)

type Client struct {
	ID   string `json:"id"`
	IP   string `json:"ip"`
	Port int    `json:"port"`
}

var (
	Clients         = make(map[string]*Client)
	ClientsDone     = make(map[string]chan struct{})
	ErrClientExists = errors.New("client id already exists")
)

func AddClient(clientId string, c *Client) error {
	log.WithFields(log.Fields{
		"client-id": clientId,
	}).Debug("Adding client")
	if Clients[clientId] != nil {
		return ErrClientExists
	}
	Clients[clientId] = c
	ClientsDone[clientId] = make(chan struct{})
	return nil
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
