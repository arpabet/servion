package main

import (
	"fmt"
	"github.com/gorilla/websocket"
	"go.arpabet.com/servion"
	"log"
	"net/http"
)

type implWebsocketHandler struct {
	upgrader *websocket.Upgrader
}

func WebsocketHander() servion.HttpHandler {
	return &implWebsocketHandler{
		upgrader: &websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // allow all origins for now
			},
		},
	}
}

func (t *implWebsocketHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	conn, err := t.upgrader.Upgrade(writer, request, nil)
	if err != nil {
		log.Printf("[WebSocket] Error upgrading connection: %v", err)
		return
	}
	defer conn.Close()
	log.Println("Client connected")

	for {
		// Read message from client
		_, msg, err := conn.ReadMessage()
		if err != nil {
			log.Println("Read error:", err)
			break
		}

		log.Println("Received:", string(msg))

		// Echo message back to client
		err = conn.WriteMessage(websocket.TextMessage, msg)
		if err != nil {
			fmt.Println("Write error:", err)
			break
		}
	}
	log.Println("Client disconnected")
}

func (t *implWebsocketHandler) Pattern() string {
	return "/v1/examples/websocket"
}
