package cexio

import (
	"github.com/golang-collections/go-datastructures/queue"
	"strings"
)

type Currency string

const (
	BTCUSD    = "BTC:USD"
	ETHUSD    = "ETH:USD"
	SEPERATOR = ":"
)

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
		Qty       float32     `json:"amount"`
		Price     string      `json:"price"`
		Side      string      `json:"type"`
	} `json:"data"`
	// Custom data for performace calculations
	RecvTimestamp int64
}

type Buffer struct {
	buffer *queue.RingBuffer
}

func NewMessageBuffer(size int) *Buffer {
	buffer := &Buffer{}
	buffer.buffer = queue.NewRingBuffer(uint64(size))
	return buffer
}

func (buffer *Buffer) Get() Message {
	data, _ := buffer.buffer.Get()
	return data.(Message)
}

func (buffer *Buffer) Put(message *Message) {
	buffer.buffer.Put(*message)
}

func getTickerSymbol(message *Message) string {
	data := [2]string{}
	data[0] = message.Data.Pair.([]interface{})[0].(string)
	data[1] = message.Data.Pair.([]interface{})[1].(string)
	return strings.Join(data[0:2], SEPERATOR)
}
