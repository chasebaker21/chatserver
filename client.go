package main

import (
	"github.com/gorilla/websocket"
)

type client struct {
	// socket is the web socket for this client.
	socket *websocket.Conn

	// receive is a channel to receive message from other clients.
	receive chan []byte

	// room is the room this client is chatting in.
	room *room

	// user is the id given to user when joining session
	user string
}

func (c *client) read() {
	defer func(socket *websocket.Conn) {
		err := socket.Close()
		if err != nil {
			return
		}
	}(c.socket)
	for {
		_, msg, err := c.socket.ReadMessage()
		if err != nil {
			return
		}
		c.room.forward <- msg
	}
}

func (c *client) write() {
	defer func(socket *websocket.Conn) {
		err := socket.Close()
		if err != nil {
			return
		}
	}(c.socket)
	for msg := range c.receive {
		err := c.socket.WriteMessage(websocket.TextMessage, msg)
		if err != nil {
			return
		}
	}
}
