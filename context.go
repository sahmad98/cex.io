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

// Request/Response structure used to parse outgoing/incoming json
// string into a golang structure.
type Message struct {
	Type string `json:"e"` // Field to specify the type of message
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
		Timestamp int64       `json:"time"`
	} `json:"data"`
	// Custom data for performace calculations
	RecvTimestamp int64
}

type Context struct {
	Connection      *websocket.Conn
	RecvChannel     chan *Message
	SendChannel     chan Message
	SendJsonChannel chan []byte
	Logger          *logger.Logger
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

func initChannels(context *Context, q_size int) {
	context.RecvChannel = make(chan *Message, q_size)
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

func runWebsocketReader(context *Context) {
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
			context.RecvChannel <- &response
		}
	}
}

func runWebsocketSender(context *Context) {
	for request := range context.SendChannel {
		json_string, error := json.Marshal(request)
		if error != nil {
			l.Errorf("Unable to convert to json payload: %s", error)
		}
		context.SendJsonChannel <- json_string
	}
}

func runWebsocketJsonSender(context *Context) {
	for request := range context.SendJsonChannel {
		l.Infof("SEND: %s", request)
		error := context.Connection.WriteMessage(websocket.TextMessage, request)
		if error != nil {
			l.Error("Unable to send message: %s", error)
		}
	}
}

func runGoRoutines(context *Context) {
	go runWebsocketJsonSender(context)
	go runWebsocketSender(context)
	go runWebsocketReader(context)
}

func GetApplicationContext() *Context {
	initLogger()
	readConfig()
	context := &Context{}
	initConnection(context)
	initChannels(context, 16)
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
	close(context.RecvChannel)
	close(context.SendChannel)
	l.Infof("Context Cleanup")
	l.Close()
}
