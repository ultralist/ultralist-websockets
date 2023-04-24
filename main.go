package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/go-redis/redis"
	"github.com/gorilla/websocket"
)

// Server is server stuff
type Server struct {
	Connections map[string][]*websocket.Conn
	RedisConn   *redis.Client
}

type Request struct {
	ClientID string                 `json:"client_id"`
	Channel  string                 `json:"channel"`
	Request  string                 `json:"request"`
	Data     map[string]interface{} `json:"data"`
}

func main() {
	log.SetFlags(0)
	log.SetOutput(os.Stdout)
	flag.Parse()

	server := &Server{}
	server.Connections = make(map[string][]*websocket.Conn)
	server.SetupRedisListener()

	http.HandleFunc("/up", HandleHeartbeat)
	http.HandleFunc("/ws", server.Serve)

	port := os.Getenv("PORT")
	if len(port) == 0 {
		port = "8080"
	}

	url := fmt.Sprintf(":%s", port)
	log.Println("Listening at ", url)

	log.Fatal(http.ListenAndServe(url, nil))
}

func HandleHeartbeat(w http.ResponseWriter, r *http.Request) {
  w.Header().Set("Content-Type", "application/json")
  w.WriteHeader(http.StatusOK)
  w.Write([]byte(`{"alive": true}`))
}

func (s *Server) Serve(w http.ResponseWriter, r *http.Request) {
	conn := s.upgradeConnection(w, r)
	defer conn.Close()

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			s.logDebug(fmt.Sprintf("Read error: %s", err))
			s.removeConnection(conn)
			break
		}
		s.logDebug(fmt.Sprintf("\n-------------------------\nrecv: '%s'\n", message))

		req := &Request{}
		err = json.Unmarshal(message, req)
		if err != nil {
			log.Println("Could not parse websocket request JSON, Got improperly formatted request")
			log.Println("Ignoring...")
			continue
		}

		if req.Request == "ping" {
			continue
		}

		connectionsForToken := s.Connections[req.Channel]
		connectionsForToken = append(connectionsForToken, conn)
		s.Connections[req.Channel] = connectionsForToken
		s.logDebug(fmt.Sprintf("Channel '%s' adding connection #%d", req.Channel, len(s.Connections[req.Channel])))
	}
}

func (s *Server) SetupRedisListener() {
	log.Println("Connecting to redis...")

	url := os.Getenv("REDIS_URL")
	if len(url) == 0 {
		url = "redis://127.0.0.1:6379"
	}

	opt, err := redis.ParseURL(url)
	if err != nil {
		fmt.Println("Error connecting to REDIS_URL:")
		panic(err)
	}
	s.RedisConn = redis.NewClient(opt)

	_, err = s.RedisConn.Ping().Result()
	if err != nil {
		log.Println("Error connecting to redis: ", err)
		panic(err)
	}

	s.setupPubSubListener()
}

func (s *Server) WriteResponseToWebsocket(response *Request) {
	channel := response.Channel

	s.logDebug(fmt.Sprintf("Writing response, using channel %s", channel))

	conns := s.Connections[channel]
	for _, conn := range conns {
		w, err := conn.NextWriter(websocket.TextMessage)
		if err != nil {
			log.Println("Error writing auth response: ", err)
			s.removeConnection(conn)
		} else {
			message, _ := json.Marshal(response)

			w.Write(message)
			w.Close()
		}
	}
}

func (s *Server) setupPubSubListener() {
	pubsub := s.RedisConn.Subscribe("clearlist_realtime")
	channel := pubsub.Channel()

	go func() {
		for {
			rawMessage := <-channel
			s.logDebug(fmt.Sprintf("redis message is %s", rawMessage.Payload))

			req := &Request{}
			err := json.Unmarshal([]byte(rawMessage.Payload), req)

			if err != nil {
				panic(err)
			}

			s.WriteResponseToWebsocket(req)
		}
	}()
}

func (s *Server) upgradeConnection(w http.ResponseWriter, r *http.Request) *websocket.Conn {
	checkOriginHandler := func(r *http.Request) bool {
		// TODO: lock this down to localhost, app.ultradeck.co, etc.
		return true
	}
	var upgrader = websocket.Upgrader{CheckOrigin: checkOriginHandler}

	Conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("upgrade error:", err)
		return nil
	}

	return Conn
}

func (s *Server) removeConnection(c *websocket.Conn) {
	var key string
	var indexToDelete int

	for k, connections := range s.Connections {
		for i, connection := range connections {
			if c == connection {
				s.logDebug(fmt.Sprintf("Found bad connection at [%s][%v]", k, i))
				key = k
				indexToDelete = i
			}
		}
	}

	if key == "" {
		return
	}

	conns := s.Connections[key]
	s.Connections[key] = append(conns[:indexToDelete], conns[indexToDelete+1:]...)

	if len(s.Connections[key]) == 0 {
		delete(s.Connections, key)
	}
}

func (s *Server) logDebug(str string) {
	if len(os.Getenv("DEBUG")) != 0 {
		log.Println(str)
	}
}
