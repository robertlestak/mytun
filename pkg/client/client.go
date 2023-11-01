package client

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/robertlestak/mytun/pkg/request"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

type Client struct {
	ID       string
	Insecure bool   `yaml:"insecure"`
	Endpoint string `yaml:"endpoint"`
	Port     int    `yaml:"port"`
	Domain   string `yaml:"domain"`
}

func (c *Client) ReadFromEnv() {
	if os.Getenv("MYTUN_INSECURE") == "true" {
		c.Insecure = true
	}
	if os.Getenv("MYTUN_ENDPOINT") != "" {
		c.Endpoint = os.Getenv("MYTUN_ENDPOINT")
	}
	if os.Getenv("MYTUN_PORT") != "" {
		pv, err := strconv.Atoi(os.Getenv("MYTUN_PORT"))
		if err != nil {
			log.WithError(err).Error("Failed to parse MYTUN_PORT")
		} else {
			c.Port = pv
		}
	}
	if os.Getenv("MYTUN_DOMAIN") != "" {
		c.Domain = os.Getenv("MYTUN_DOMAIN")
	}
}

func (c *Client) ReadFromFile() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return
	}
	configPath := fmt.Sprintf("%s/.mytun.yaml", homeDir)
	f, err := os.Open(configPath)
	if err != nil {
		return
	}
	defer f.Close()
	nc := &Client{}
	if err := yaml.NewDecoder(f).Decode(nc); err != nil {
		return
	}
	if nc.Insecure {
		c.Insecure = nc.Insecure
	}
	if nc.Endpoint != "" {
		c.Endpoint = nc.Endpoint
	}
	if nc.Port != 0 {
		c.Port = nc.Port
	}
	if nc.Domain != "" {
		c.Domain = nc.Domain
	}
}

func (c *Client) ReadFromContext() {
	c.ReadFromFile()
	c.ReadFromEnv()
}

func (c *Client) setupProxy(conn *websocket.Conn, done chan struct{}) error {
	l := log.WithFields(log.Fields{
		"app":       "mytun",
		"cmd":       "client.setupProxy",
		"client-id": c.ID,
	})
	l.Debug("Setting up proxy")
	proto := "https://"
	if c.Insecure {
		proto = "http://"
	}
	var connectionString string
	if c.Domain != "" {
		connectionString = fmt.Sprintf("%s%s.%s", proto, c.ID, c.Domain)
	} else {
		connectionString = fmt.Sprintf("%s%s", proto, c.ID)
	}
	// Connect to the local HTTP server
	localServerAddr := fmt.Sprintf("http://localhost:%d", c.Port)

	fmt.Printf("tunnel open: %s\n", connectionString)

	l = l.WithField("local-server-addr", localServerAddr)
	// when we get a message from the server, send it to the local server
	go func() {
		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				l.WithError(err).Debug("Failed to read message")
				close(done)
				return
			}
			var req request.RequestData
			if err := json.Unmarshal(message, &req); err != nil {
				l.WithError(err).Error("Failed to unmarshal request")
				close(done)
				return
			}
			res, err := req.SendLocal(localServerAddr)
			if err != nil {
				l.WithError(err).Error("Failed to send request")
				// send an empty response
				res = &request.ResponseData{
					StatusCode: http.StatusBadGateway,
					Header:     make(http.Header),
					Body:       []byte(http.StatusText(http.StatusBadGateway)),
				}
			}
			resBytes, err := json.Marshal(res)
			if err != nil {
				l.WithError(err).Error("Failed to marshal response")
				close(done)
				return
			}
			if err := conn.WriteMessage(websocket.TextMessage, resBytes); err != nil {
				l.WithError(err).Error("Failed to send response")
				close(done)
				return
			}
		}
	}()

	return nil
}

func (c *Client) Close() error {
	l := log.WithFields(log.Fields{
		"app": "mytun",
		"cmd": "client.Close",
	})
	l.Debug("Closing client")
	proto := "https://"
	if c.Insecure {
		proto = "http://"
	}
	endpoint := fmt.Sprintf("%s%s/close/%s", proto, c.Endpoint, c.ID)
	if _, err := http.Post(endpoint, "text/plain", nil); err != nil {
		return err
	}
	return nil
}

func (c *Client) Connect() error {
	l := log.WithFields(log.Fields{
		"app": "mytun",
		"cmd": "client.Connect",
	})
	l.Debug("Starting client")
	wsProto := "wss://"
	if c.Insecure {
		wsProto = "ws://"
	}
	endpoint := fmt.Sprintf("%s%s/ws", wsProto, c.Endpoint)
	u, err := url.Parse(endpoint)
	if err != nil {
		return err
	}
	sc, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		return err
	}
	defer sc.Close()

	done := make(chan struct{})
	var clientId string
	for {
		_, message, err := sc.ReadMessage()
		if err != nil {
			close(done)
			return err
		}
		if clientId == "" {
			clientId = strings.TrimSpace(string(message))
			break
		}
	}
	c.ID = clientId
	c.setupProxy(sc, done)
	// Set up a signal handler to gracefully close the WebSocket connection
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	for {
		select {
		case <-interrupt:
			fmt.Println("closing tunnel")
			err := sc.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			if err != nil {
				fmt.Println("Error sending close message:", err)
			}
			if err := c.Close(); err != nil {
				fmt.Println("Error closing client:", err)
			}
			select {
			case <-done:
			case <-time.After(time.Second):
			}
			return nil
		case <-done:
			return nil
		}
	}
}
