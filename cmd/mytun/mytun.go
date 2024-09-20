package main

import (
	"flag"
	"os"

	"github.com/robertlestak/mytun/pkg/client"
	"github.com/robertlestak/mytun/pkg/server"
	log "github.com/sirupsen/logrus"
)

var (
	mytunFlagset       = flag.NewFlagSet("mytun", flag.ExitOnError)
	serverFlagset      = flag.NewFlagSet("server", flag.ExitOnError)
	internalListenAddr = serverFlagset.String("internal-addr", ":8080", "Listen address")
	publicListenAddr   = serverFlagset.String("public-addr", ":8081", "Listen address")
	clientFlagset      = flag.NewFlagSet("client", flag.ExitOnError)
	endpoint           = clientFlagset.String("endpoint", "localhost:8080", "Endpoint")
	insecure           = clientFlagset.Bool("insecure", false, "Insecure")
	clientIp           = clientFlagset.String("ip", "", "IP")
	clientId           = clientFlagset.String("id", "", "ID. If not provided, a random UUID will be generated")
	domain             = clientFlagset.String("domain", "localhost", "Domain")
	port               = clientFlagset.Int("port", 3000, "Port")
)

func init() {
	ll, err := log.ParseLevel(os.Getenv("LOG_LEVEL"))
	if err != nil {
		ll = log.InfoLevel
	}
	log.SetLevel(ll)
}

func serverCmd() error {
	l := log.WithFields(log.Fields{
		"app": "mytun",
		"cmd": "server",
	})
	l.Debug("Starting server")
	serverFlagset.Parse(os.Args[2:])
	go func() {
		if err := server.PublicServer(*publicListenAddr); err != nil {
			l.WithError(err).Fatal("Failed to start public server")
		}
	}()
	if err := server.InternalServer(*internalListenAddr); err != nil {
		l.WithError(err).Fatal("Failed to start server")
	}
	return nil
}

func clientCmd() error {
	l := log.WithFields(log.Fields{
		"app": "mytun",
		"cmd": "client",
	})
	l.Debug("Starting client")
	if len(os.Args) < 2 {
		l.Fatal("No command provided")
	}
	clientFlagset.Parse(os.Args[2:])
	cl := &client.Client{
		Endpoint: *endpoint,
		Insecure: *insecure,
		IP:       *clientIp,
		ID:       *clientId,
		Port:     *port,
		Domain:   *domain,
	}
	cl.ReadFromContext()
	if err := cl.Connect(); err != nil {
		l.WithError(err).Fatal("Failed to connect")
	}
	return nil
}

func main() {
	l := log.WithFields(log.Fields{
		"app": "mytun",
	})
	l.Debug("Starting mytun")
	logLevelStr := mytunFlagset.String("log-level", log.GetLevel().String(), "Log level")
	mytunFlagset.Parse(os.Args[1:])
	logLevel, err := log.ParseLevel(*logLevelStr)
	if err != nil {
		logLevel = log.InfoLevel
	}
	log.SetLevel(logLevel)
	var cmd = "start"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}
	l.WithField("cmd", cmd).Debug("Running command")
	switch cmd {
	case "start":
		if err := clientCmd(); err != nil {
			l.WithError(err).Fatal("Failed to run client")
		}
	case "server":
		if err := serverCmd(); err != nil {
			l.WithError(err).Fatal("Failed to run server")
		}
	default:
		l.WithField("cmd", cmd).Fatal("Unknown command")
	}
}
