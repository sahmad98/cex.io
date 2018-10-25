package cexio

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/gorilla/websocket"
	"log"
	"strconv"
	// "sync"
	"github.com/spf13/viper"
	"time"
)

// TODO Read from Config file
var WS_ENDPOINT = "wss://ws.cex.io/ws"
var API_KEY = ""
var API_SECRET = ""

type Message struct {
	Type string `json:"e"`
	Auth struct {
		Key       string `json:"key"`
		Signature string `json:"signature"`
		Timestamp int64  `json:"timestamp"`
	} `json:"auth"`
	Data struct {
		Id        int         `json:"id"`
		Pair      interface{} `json:"pair"`
		Subscribe bool        `json:"subscribe"`
		Depth     int         `json:"depth"`
		Bids      [][]float32 `json:"bids"`
		Asks      [][]float32 `json:"asks"`
		Low       string      `json:"low"`
		High      string      `json:"high"`
		Last      string      `json:"last"`
		Volume    string      `json:"volume"`
		Volume30  string      `json:"volume30d"`
		Bid       float32     `json:"bid"`
		Ask       float32     `json:"ask"`
		Ok        string      `json:"ok"`
		Error     string      `json:"error"`
	} `json:"data"`
	Timestamp int64 `json:"time"`
}

type Context struct {
	Connection      *websocket.Conn
	RecvChannel     chan Message
	SendChannel     chan Message
	SendJsonChannel chan []byte
	// Mux             sync.Mutex
}

func ReadConfig() {
	viper.SetConfigName("config")
	viper.AddConfigPath("./config")
	viper.SetConfigType("toml")
	err := viper.ReadInConfig()
	if err != nil {
		log.Fatal("Config Error: ", err)
	}

	API_KEY = viper.GetString("auth.key")
	API_SECRET = viper.GetString("auth.secret")
}

func GetApplicationContext() (*Context, error) {
	ReadConfig()
	connection, _, error := websocket.DefaultDialer.Dial(WS_ENDPOINT, nil)
	if error != nil {
		log.Fatal("Error opening websocket connection: %s", error)
		return nil, error
	}
	context := Context{}
	context.Connection = connection
	context.RecvChannel = make(chan Message, 16)
	context.SendChannel = make(chan Message, 16)
	context.SendJsonChannel = make(chan []byte, 16)
	go func(context *Context) {
		for {
			_, message, error := context.Connection.ReadMessage()
			log.Printf("RECV: %s", message)
			if error != nil {
				log.Fatal("Error reciveing messages: ", error)
				break
			}
			response := Message{}
			error = json.Unmarshal(message, &response)
			if error != nil {
				log.Fatal("Unable to parse response: %s", error)
				break
			} else {
				context.RecvChannel <- response
			}
		}
	}(&context)

	go func(context *Context) {
		for request := range context.SendChannel {
			json_string, error := json.Marshal(request)
			if error != nil {
				log.Fatal("Unable to convert to json payload: ", error)
				break
			}
			context.SendJsonChannel <- json_string
		}
	}(&context)

	go func(context *Context) {
		for request := range context.SendJsonChannel {
			log.Printf("Writing")
			error = context.Connection.WriteMessage(websocket.TextMessage, request)
			log.Printf("Written")
			if error != nil {
				log.Fatal("Unable to send message: %s", error)
				break
			}
		}
	}(&context)
	return &context, nil
}

func (context *Context) Authenticate() error {
	payload := Message{}
	payload.Type = "auth"
	payload.Auth.Key = API_KEY
	payload.Auth.Timestamp = time.Now().Unix()
	key := strconv.FormatInt(payload.Auth.Timestamp, 10) + payload.Auth.Key
	log.Printf("Key: %s", key)
	mac := hmac.New(sha256.New, []byte(API_SECRET))
	mac.Write([]byte(key))
	payload.Auth.Signature = hex.EncodeToString(mac.Sum(nil))
	log.Printf("Signature: %s", payload.Auth.Signature)
	context.SendChannel <- payload
	return nil
}

func (context *Context) Cleanup() {
	context.Connection.Close()
	close(context.RecvChannel)
	close(context.SendChannel)
	log.Printf("Context Cleanup")
}
