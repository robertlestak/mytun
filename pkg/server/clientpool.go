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
		"client_ip": c.IP,
		"client_port": c.Port,
	}).Trace("Adding client to pool")
	if Clients[clientId] != nil {
		log.WithField("client_id", clientId).Error("Client ID already exists")
		return ErrClientExists
	}
	Clients[clientId] = c
	ClientLastConnect[clientId] = time.Now()
	log.WithField("client_id", clientId).WithField("total_clients", len(Clients)).Trace("Client added to pool")
	return nil
}

func RemoveClient(clientId string) {
	log.WithFields(log.Fields{
		"client-id": clientId,
	}).Trace("Removing client from pool")
	delete(Clients, clientId)
	delete(ClientLastConnect, clientId)
	log.WithField("client_id", clientId).WithField("total_clients", len(Clients)).Trace("Client removed from pool")
}
