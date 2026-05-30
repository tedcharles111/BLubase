package main

import (
	"log"
	"net/http"
	"golang.org/x/net/websocket"
)

func main() {
	http.Handle("/ws", websocket.Handler(func(ws *websocket.Conn) {
		var msg string
		for {
			err := websocket.Message.Receive(ws, &msg)
			if err != nil { return }
			websocket.Message.Send(ws, "echo: "+msg)
		}
	}))
	log.Println("Realtime WebSocket on :4000")
	log.Fatal(http.ListenAndServe(":4000", nil))
}
