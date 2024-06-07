package main

import (
    "fmt"
    "net/http"
    "github.com/gorilla/websocket"
)

type Client struct {
    ID        string           `json:"id"`
    Conn      *websocket.Conn
    Interests []string         `json:"interests"`
}

type Message struct {
    Type      string   `json:"type"`
    ID        string   `json:"id,omitempty"`
    Text      string   `json:"text,omitempty"`
    Interests []string `json:"interests,omitempty"`
}

var clients = make(map[*Client]bool)
var matches = make(map[string]*Client)  
var register = make(chan *Client)
var unregister = make(chan *Client)


var upgrader = websocket.Upgrader{
    CheckOrigin: func(r *http.Request) bool {
        return true
    },
}

func main() {
    http.HandleFunc("/ws", handleConnections)
    go handleMessages()

    err := http.ListenAndServe(":8080", nil)
    if err != nil {
        fmt.Println("Error starting server: ", err)
    }
}

func handleConnections(w http.ResponseWriter, r *http.Request) {
    conn, err := upgrader.Upgrade(w, r, nil)
    if err != nil {
        fmt.Println("Failed to upgrade:", err)
        return
    }
    defer conn.Close()

    client := &Client{Conn: conn}
    register <- client

    for {
        var msg Message
        err := conn.ReadJSON(&msg)
        if err != nil {
            fmt.Println("Error reading json:", err)
            break
        }

        switch msg.Type {
        case "register":
            client.ID = msg.ID
            client.Interests = msg.Interests
            clients[client] = true
            tryMatch(client)
        case "message":
            if partner, ok := matches[client.ID]; ok {
                partner.Conn.WriteJSON(msg)  // Forward the message to the matched partner
            }
        }
    }

    unregister <- client
}

func tryMatch(newClient *Client) {
    for client := range clients {
        if client != newClient && sharesInterest(client, newClient) && matches[client.ID] == nil && matches[newClient.ID] == nil {

            matches[client.ID] = newClient
            matches[newClient.ID] = client
            matchedMessage := Message{Type: "match", ID: client.ID}
            client.Conn.WriteJSON(matchedMessage)
            newClient.Conn.WriteJSON(matchedMessage)
            break
        }
    }
}

func sharesInterest(c1, c2 *Client) bool {
    for _, interest1 := range c1.Interests {
        for _, interest2 := range c2.Interests {
            if interest1 == interest2 {
                return true
            }
        }
    }
    return false
}

func handleMessages() {
    for {
        select {
        case client := <-register:
            fmt.Println("New Client Registered:", client.ID)
        case client := <-unregister:
            if _, ok := clients[client]; ok {
                if matchedClient, exists := matches[client.ID]; exists {
                    delete(matches, matchedClient.ID)  // Clean up the match
                    delete(matches, client.ID)
                }
                client.Conn.Close()
                delete(clients, client)
                fmt.Println("Client Left:", client.ID)
            }

        }
    }
}
