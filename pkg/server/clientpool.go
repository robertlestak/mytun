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
	return nil
}

func RemoveClient(clientId string) {
	log.WithFields(log.Fields{
		"client-id": clientId,
	}).Debug("Removing client")
	delete(Clients, clientId)
}
