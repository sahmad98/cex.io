// Go package to interact with CEX.IO crypto currency trading.
// Apis provided can be used to create a algorithmics crypto
// currency trading platform.
package cexio

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/google/logger"
	"github.com/gorilla/websocket"
	"github.com/spf13/viper"
	"log"
	"os"
	"strconv"
	"time"
)

var WS_ENDPOINT = "wss://ws.cex.io/ws"
var API_KEY = ""
var API_SECRET = ""
var LOG_PATH = "."
var LOG_FILE = "marketdata.log"
var l *logger.Logger = nil

type Context struct {
	Connection        *websocket.Conn
	RecvChannel       *Buffer
	ResponseChannel   *Buffer
	SendChannel       chan Message
	SendJsonChannel   chan []byte
	Logger            *logger.Logger
	marketDataAdapter *MarketDataAdapter
	orderManager      *OrderManager
}

func readConfig() {
	viper.SetConfigName("config")
	viper.AddConfigPath("./config")
	viper.SetConfigType("toml")
	err := viper.ReadInConfig()
	if err != nil {
		log.Fatal("Config Error: ", err)
	}

	API_KEY = viper.GetString("auth.key")
	API_SECRET = viper.GetString("auth.secret")
	if viper.IsSet("log.path") {
		LOG_PATH = viper.GetString("log.path")
	}
	if viper.IsSet("log.filename") {
		LOG_FILE = viper.GetString("log.filename")
	}
	if viper.IsSet("websocket.endpoing") {
		WS_ENDPOINT = viper.GetString("websocket.endpoint")
	}
}

func initLogger() {
	file_handle, err := os.Create(LOG_PATH + "/" + LOG_FILE)
	if err != nil {
		log.Printf("Error creating logfile: ", LOG_FILE)
		panic(err)
	}
	l = logger.Init("", false, false, file_handle)
}

func (context *Context)init(q_size int) {
	context.RecvChannel = NewMessageBuffer(q_size)
	context.ResponseChannel = NewMessageBuffer(q_size)
	context.SendChannel = make(chan Message, q_size)
	context.SendJsonChannel = make(chan []byte, q_size)
}

func initConnection(context *Context) {
	connection, _, error := websocket.DefaultDialer.Dial(WS_ENDPOINT, nil)
	if error != nil {
		l.Fatalf("Error opening websocket connection: %s", error)
		panic(error)
	}
	context.Connection = connection
}

func (context *Context) runWebsocketReader() {
	for {
		_, message, error := context.Connection.ReadMessage()
		l.Infof("RECV: %s", message)
		if error != nil {
			l.Errorf("Error reciveing messages: %s", error)
		}
		response := Message{}
		error = json.Unmarshal(message, &response)
		response.RecvTimestamp = time.Now().UnixNano()
		if error != nil {
			l.Errorf("Unable to parse response: %s", error)
		} else {
			context.RecvChannel.Put(&response)
			l.Infof("PutRecvChannel: ", response)
		}
	}
}

func (context *Context) runWebsocketSender() {
	for request := range context.SendChannel {
		json_string, error := json.Marshal(request)
		if error != nil {
			l.Errorf("Unable to convert to json payload: %s", error)
		}
		context.SendJsonChannel <- json_string
	}
}

func (context *Context) runWebsocketJsonSender() {
	for request := range context.SendJsonChannel {
		l.Infof("SEND: %s", request)
		error := context.Connection.WriteMessage(websocket.TextMessage, request)
		if error != nil {
			l.Error("Unable to send message: %s", error)
		}
	}
}

func (context *Context) runResponseRouter() {
	for {
		response := context.RecvChannel.Get()
		var res_type string = response.Type
		if res_type == "ticker" || res_type == "order-book-subscribe" || res_type == "order-book-unsubscribe" || res_type == "md_update" {
			context.marketDataAdapter.ResponseChannel.Put(&response)
			l.Infof("MDResponseChannelPut: ", response)
		} else if res_type == "place-order" || res_type == "cancel-replace-order" || res_type == "get-order" || res_type == "cancel-order" || res_type == "close-position" {
			context.orderManager.ResponseChannel.Put(&response)
			l.Infof("OMResponseChannelPut: ", response)
		} else {
			context.ResponseChannel.Put(&response)
			l.Infof("ContextResponseChannelPut: ", response)
		}
	}
}

func (context *Context) SendPongMessage(m *Message) {
	for {
		m.Type = "pong"
		context.SendChannel <- *m
		l.Infof("PONG")
	}
}

func (context *Context) runResponseHandler() {
	for {
		response := context.ResponseChannel.Get()
		var res_type string = response.Type
		if res_type == "ping" {
			context.SendPongMessage(&response)
		} else if res_type == "auth" {
			if response.Data.Ok == "ok" {
				l.Infof("Authenticated Successfully")
			} else {
				l.Infof("Authentication Error: ", response.Data.Ok)
			}
		}
	}
}

func runGoRoutines(context *Context) {
	go context.runWebsocketJsonSender()
	go context.runWebsocketSender()
	go context.runWebsocketReader()
	go context.runResponseRouter()
	go context.runResponseHandler()
}

func GetApplicationContext() *Context {
	readConfig()
	initLogger()
	context := &Context{}
	initConnection(context)
	context.init(16)
	runGoRoutines(context)
	return context
}

func GenerateSignature(time int64) string {
	key := strconv.FormatInt(time, 10) + API_KEY
	mac := hmac.New(sha256.New, []byte(API_SECRET))
	mac.Write([]byte(key))
	return hex.EncodeToString(mac.Sum(nil))
}

func (context *Context) Authenticate() error {
	payload := Message{}
	payload.Type = "auth"
	payload.Auth.Key = API_KEY
	payload.Auth.Timestamp = time.Now().Unix()
	payload.Auth.Signature = GenerateSignature(payload.Auth.Timestamp)
	l.Infof("Signature: %s", payload.Auth.Signature)
	context.SendChannel <- payload
	return nil
}

func (context *Context) Cleanup() {
	context.Connection.Close()
	close(context.SendChannel)
	l.Infof("Context Cleanup")
	l.Close()
}
