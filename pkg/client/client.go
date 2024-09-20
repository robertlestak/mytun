package client

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

type Client struct {
	ID       string `yaml:"id" json:"id"`
	Insecure bool   `yaml:"insecure" json:"insecure"`
	Endpoint string `yaml:"endpoint" json:"endpoint"`
	IP       string `yaml:"ip" json:"ip"`
	Port     int    `yaml:"port" json:"port"`
	Domain   string `yaml:"domain" json:"domain"`
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
	proto := "https://"
	if c.Insecure {
		proto = "http://"
	}
	endpoint := fmt.Sprintf("%s%s/connect", proto, c.Endpoint)
	if c.IP == "" {
		return errors.New("IP is required")
	}
	jd, err := json.Marshal(c)
	if err != nil {
		return err
	}
	resp, err := http.Post(endpoint, "application/json", bytes.NewBuffer(jd))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("server returned %d", resp.StatusCode)
	}
	done := make(chan struct{})
	// the response is a single string which is the client id
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	c.ID = strings.TrimSpace(string(b))
	var clientEndpoint string
	if c.Domain != "" {
		clientEndpoint = fmt.Sprintf("%s%s.%s", proto, c.ID, c.Domain)
	} else {
		clientEndpoint = fmt.Sprintf("%s%s", proto, c.ID)
	}
	fmt.Printf("tunnel open: %s\n", clientEndpoint)
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)
	for {
		select {
		case <-interrupt:
			fmt.Println("closing tunnel")
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
	return nil
}
