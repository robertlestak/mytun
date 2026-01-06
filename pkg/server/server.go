package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
)

var pendingRequests = make(map[string]chan []byte)
var pendingMutex sync.RWMutex

func handleWebSocketConnect(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	ctx := context.Background()
	_, data, _ := conn.Read(ctx)
	
	c := &Client{}
	json.Unmarshal(data, c)
	if c.ID == "" {
		c.ID = uuid.New().String()[:8]
	}
	
	c.WSConn = conn
	c.WSCtx = ctx
	AddClient(c.ID, c)
	defer RemoveClient(c.ID)
	
	conn.Write(ctx, websocket.MessageText, []byte(c.ID))
	
	// Handle responses from client
	for {
		_, respData, err := conn.Read(ctx)
		if err != nil {
			break
		}
		
		var resp map[string]interface{}
		json.Unmarshal(respData, &resp)
		reqID := resp["id"].(string)
		
		pendingMutex.RLock()
		if ch, ok := pendingRequests[reqID]; ok {
			ch <- []byte(resp["body"].(string))
		}
		pendingMutex.RUnlock()
	}
}

func handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "OK")
}

func handleRequest(w http.ResponseWriter, r *http.Request) {
	clientId := strings.Split(r.Host, ".")[0]
	log.WithField("client_id", clientId).WithField("host", r.Host).WithField("method", r.Method).WithField("url", r.URL.String()).Trace("Incoming request")
	
	c, ok := Clients[clientId]
	if !ok {
		log.WithField("client_id", clientId).Error("Client not found")
		http.Error(w, "Client not found", http.StatusNotFound)
		return
	}
	ClientLastConnect[clientId] = time.Now()
	
	log.WithField("client_id", clientId).WithField("client_ip", c.IP).WithField("client_port", c.Port).Trace("Found client")
	
	if c.WSConn == nil {
		// Fallback to direct connection
		log.WithFields(log.Fields{
			"client-id": clientId,
			"protocol":  "http",
			"method":    r.Method,
			"url":       r.URL.String(),
		}).Info("Proxying request via HTTP")
		
		target := fmt.Sprintf("http://%s:%d", c.IP, c.Port)
		log.WithField("client_id", clientId).WithField("target", target).Trace("Creating reverse proxy")
		
		targetUrl, _ := url.Parse(target)
		proxy := httputil.NewSingleHostReverseProxy(targetUrl)
		
		// Add error handler to proxy
		proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
			log.WithError(err).WithField("client_id", clientId).WithField("target", target).Error("Reverse proxy error")
			http.Error(w, "Bad Gateway", http.StatusBadGateway)
		}
		
		log.WithField("client_id", clientId).Trace("Starting reverse proxy request")
		proxy.ServeHTTP(w, r)
		log.WithField("client_id", clientId).Trace("Reverse proxy request completed")
		return
	}
	
	log.WithFields(log.Fields{
		"client-id": clientId,
		"protocol":  "websocket",
		"method":    r.Method,
		"url":       r.URL.String(),
	}).Info("Proxying request via WebSocket")
	
	// Send request over websocket
	reqID := uuid.New().String()
	req := map[string]interface{}{
		"id":     reqID,
		"method": r.Method,
		"url":    r.URL.String(),
		"headers": r.Header,
	}
	
	respCh := make(chan []byte, 1)
	pendingMutex.Lock()
	pendingRequests[reqID] = respCh
	pendingMutex.Unlock()
	
	defer func() {
		pendingMutex.Lock()
		delete(pendingRequests, reqID)
		pendingMutex.Unlock()
	}()
	
	reqData, _ := json.Marshal(req)
	log.WithField("client_id", clientId).WithField("req_id", reqID).Trace("Sending websocket request")
	c.WSConn.Write(c.WSCtx, websocket.MessageText, reqData)
	
	// Wait for response
	select {
	case respBody := <-respCh:
		log.WithField("client_id", clientId).WithField("req_id", reqID).WithField("resp_size", len(respBody)).Trace("Received websocket response")
		w.Write(respBody)
	case <-time.After(30 * time.Second):
		log.WithField("client_id", clientId).WithField("req_id", reqID).Error("Websocket request timeout")
		http.Error(w, "Request timeout", http.StatusGatewayTimeout)
	}
}

func handleClientClosure(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	clientId := vars["id"]
	if Clients[clientId] != nil {
		RemoveClient(clientId)
	}
	fmt.Fprintf(w, "OK")
}

func handleClientConnect(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	c := &Client{}
	if err := json.NewDecoder(r.Body).Decode(c); err != nil {
		log.WithError(err).Error("Error decoding client connect request")
		http.Error(w, "Error decoding request", http.StatusBadRequest)
		return
	}
	if c.ID == "" {
		c.ID = uuid.New().String()[:8]
	}
	
	log.WithField("client_id", c.ID).WithField("client_ip", c.IP).WithField("client_port", c.Port).Trace("Client connect request")
	
	if err := AddClient(c.ID, c); err != nil {
		log.WithError(err).WithField("client_id", c.ID).Error("Failed to add client")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	
	log.WithField("client_id", c.ID).Trace("Client connected successfully")
	fmt.Fprintf(w, "%s", c.ID)
}

func InternalServer(listenAddr string, timeout time.Duration) error {
	go TimeoutConnections(timeout)
	r := mux.NewRouter()
	r.HandleFunc("/health", handleHealthCheck)
	r.HandleFunc("/close/{id}", handleClientClosure).Methods("POST")
	r.HandleFunc("/connect", handleClientConnect).Methods("POST")
	r.HandleFunc("/ws", handleWebSocketConnect)
	return http.ListenAndServe(listenAddr, r)
}

func PublicServer(listenAddr string) error {
	r := mux.NewRouter()
	r.HandleFunc("/health", handleHealthCheck)
	r.PathPrefix("/").HandlerFunc(handleRequest)
	return http.ListenAndServe(listenAddr, r)
}

func TimeoutConnections(timeout time.Duration) {
	if timeout == 0 {
		return
	}
	for {
		for c, t := range ClientLastConnect {
			if time.Since(t) > timeout {
				RemoveClient(c)
			}
		}
		time.Sleep(time.Minute)
	}
}
