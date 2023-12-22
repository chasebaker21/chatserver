package main

import (
	"encoding/json"
	"github.com/gorilla/websocket"
	"log"
	"net/http"
	"strconv"
	"sync"
)

// clients holds all current clients in this room
// join is a channel for clients wishing to join the room
// leave is a channel for clients wishing to leave the room
// forward is a channel that holds incoming message that should be forwarded to the other clients
type room struct {
	clients        map[*client]bool
	join           chan *client
	leave          chan *client
	forward        chan []byte
	chatMsgChannel chan ClientMessage
}

type ClientMessage struct {
	Type    string `json:"type"`
	Message string `json:"message"`
	User    string `json:"user"`
}

var userIDCounter int
var userIDMutex sync.Mutex

const (
	socketBufferSize  = 1024
	messageBufferSize = 256
)

// Create a new room
func newRoom() *room {
	room := &room{
		forward:        make(chan []byte),
		join:           make(chan *client),
		leave:          make(chan *client),
		clients:        make(map[*client]bool),
		chatMsgChannel: make(chan ClientMessage),
	}
	room.startChatProcessor()
	return room
}

// Starts the chat room and handles any incoming messages
func (r *room) run() {
	for {
		select {
		case client := <-r.join:
			r.clients[client] = true
			r.broadcastUserJoined(client)
		case client := <-r.leave:
			delete(r.clients, client)
			close(client.receive)
			r.broadcastUserLeft(client)
		case msg := <-r.forward:
			clientMsg := ClientMessage{
				Type:    "chatMessage",
				Message: string(msg),
				User:    "SenderUsername", // TODO replace with actual username/identifier
			}
			r.chatMsgChannel <- clientMsg
		}
	}
}

var upgrader = &websocket.Upgrader{
	ReadBufferSize:  socketBufferSize,
	WriteBufferSize: socketBufferSize,
	CheckOrigin: func(r *http.Request) bool {
		// Will only allow connection to localhost:3000
		return r.Header.Get("Origin") == "http://localhost:3000"
	}}

// ServeHTTP upgrades the HTTP connection to a WebSocket connection
func (r *room) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	socket, err := upgrader.Upgrade(w, req, nil)
	if err != nil {
		log.Fatal("ServeHTTP", err)
		return
	}
	client := &client{
		socket:  socket,
		receive: make(chan []byte, messageBufferSize),
		room:    r,
		user:    generateUniqueUser(),
	}
	r.join <- client
	defer func() { r.leave <- client }()
	go client.write()
	client.read()
}

func generateUniqueUser() string {
	userIDMutex.Lock()
	defer userIDMutex.Unlock()
	userIDCounter++
	return "User" + strconv.Itoa(userIDCounter)
}

func (r *room) broadcastUserJoined(newClient *client) {
	message := ClientMessage{
		Type:    "userJoined",
		Message: newClient.user + ` joined the chat.`,
		User:    newClient.user,
	}
	r.broadcastMessage(newClient, message)
}

func (r *room) broadcastUserLeft(leftClient *client) {
	message := ClientMessage{
		Type:    "userLeft",
		Message: leftClient.user + ` left the chat.`,
		User:    leftClient.user,
	}
	r.broadcastMessage(leftClient, message)
}

func (r *room) broadcastMessage(sender *client, message interface{}) {
	msgBytes, err := json.Marshal(message)
	if err != nil {
		log.Println("Error marshaling message:", err)
		return
	}

	for client := range r.clients {
		// Stops sender from receiving its own message
		if client != sender {
			client.receive <- msgBytes
		}
	}
}

// Continuously waits for messages on the chatMsgChannel and broadcasts those messages to all clients in the room.
func (r *room) startChatProcessor() {
	go func() {
		for {
			select {
			case chatMsg := <-r.chatMsgChannel:
				r.broadcastMessage(nil, chatMsg)
			}
		}
	}()
}
