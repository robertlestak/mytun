package server

import (
	"context"
	"errors"
	"net"
	"time"

	"github.com/coder/websocket"
	log "github.com/sirupsen/logrus"
)

type Client struct {
	ID     string          `json:"id"`
	IP     string          `json:"ip"`
	Port   int             `json:"port"`
	Conn   net.Conn        `json:"-"`
	WSConn *websocket.Conn `json:"-"`
	WSCtx  context.Context `json:"-"`
}

var (
	Clients           = make(map[string]*Client)
	ClientLastConnect = make(map[string]time.Time)
	ErrClientExists   = errors.New("client id already exists")
)

func AddClient(clientId string, c *Client) error {
	log.WithFields(log.Fields{
		"client-id": clientId,
	}).Debug("Adding client")
	if Clients[clientId] != nil {
		return ErrClientExists
	}
	Clients[clientId] = c
	ClientLastConnect[clientId] = time.Now()
	return nil
}

func RemoveClient(clientId string) {
	log.WithFields(log.Fields{
		"client-id": clientId,
	}).Debug("Removing client")
	delete(Clients, clientId)
	delete(ClientLastConnect, clientId)
}
