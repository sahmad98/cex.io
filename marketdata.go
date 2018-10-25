package cexio

import (
	"encoding/json"
	"log"
	"sort"
	"time"
)

type Level struct {
	Price float32
	Qty   float32
}

const (
	kMaxDepth = 6
	kMaxPrice = 9999999.9999
)

type Levels struct {
	Data [kMaxDepth]Level
	Id   int
}

func (levels *Levels) Len() int { return len(levels.Data) }

func (levels *Levels) Swap(i, j int) {
	levels.Data[i], levels.Data[j] = levels.Data[j], levels.Data[i]
}

func (levels *Levels) Less(i, j int) bool {
	return levels.Data[i].Price < levels.Data[j].Price
}

func (levels *Levels) Update(price, qty float32, index int) {
	levels.Data[index].Price = price
	levels.Data[index].Qty = qty
}

var bids = Levels{}
var asks = Levels{}

func CreateSnapshot(m *Message) {
	for i := 0; i < kMaxDepth; i++ {
		bids.Update(-kMaxPrice, 0, i)
		asks.Update(kMaxPrice, 0, i)
	}
	log.Printf("Bids: %+v", bids)
	log.Printf("Asks: %+v", asks)
	log.Printf("Bid Len: ", len(m.Data.Bids))
	log.Printf("Ask Len: ", len(m.Data.Asks))

	for index, data := range m.Data.Bids {
		bids.Update(data[0], data[1], index)
	}

	for index, data := range m.Data.Asks {
		asks.Update(data[0], data[1], index)
	}
}

func UpdateSnapshot(m *Message) {
	updated_bid := false
	updated_ask := false
	for _, update := range m.Data.Bids {
		if update[1] == 0 {
			for i, bid := range bids.Data {
				if bid.Price == update[0] {
					bids.Update(-kMaxPrice, 0, i)
				}
			}
		} else {
			for i, bid := range bids.Data {
				if bid.Price == update[0] {
					bids.Update(update[0], update[1], i)
					updated_bid = true
					break
				}
			}

			if !updated_bid {
				bids.Update(update[0], update[1], kMaxDepth-1)
			}
		}
	}

	for _, update := range m.Data.Asks {
		if update[1] == 0 {
			for i, ask := range asks.Data {
				if ask.Price == update[0] {
					asks.Update(kMaxPrice, 0, i)
				}
			}
		} else {
			for i, ask := range asks.Data {
				if ask.Price == update[0] {
					asks.Update(update[0], update[1], i)
					updated_ask = true
					break
				}
			}

			if !updated_ask {
				asks.Update(update[0], update[1], kMaxDepth-1)
			}

		}
	}

	sort.Sort(sort.Reverse(&bids))
	sort.Sort(&asks)
}

type Orderbook struct {
	Id        int
	Pair      interface{}
	Bids      [][2]float32
	Asks      [][2]float32
	Low       float64
	High      float64
	LastPrice float64
	Volume    float64
	Bid       float32
	Ask       float32
}

type HandlerFunc func(message Message)

type MarketDataAdapter struct {
	PingChannel     chan Message
	ResponseChannel chan Message
	UpdateChannel   chan Message
	Context         *Context
	UpdateHandler   HandlerFunc
	ResponseHandler HandlerFunc
}

func ResponseHandler(m Message) {
	if m.Type == "auth" {
		if m.Data.Ok != "ok" {
			log.Printf("ERROR: Auth Error, %s", m.Data.Error)
		} else {
			log.Printf("Login Successful")
		}
	}
}

func NewMarketDataAdapter(context *Context) *MarketDataAdapter {
	md := MarketDataAdapter{}
	md.PingChannel = make(chan Message, 16)
	md.ResponseChannel = make(chan Message, 16)
	md.UpdateChannel = make(chan Message, 16)
	md.Context = context

	md.UpdateHandler = func(m Message) {}
	md.ResponseHandler = ResponseHandler

	// Start Response handler goroutine which will
	// send responses on different channels
	go func(context *Context, md *MarketDataAdapter) {
		for message := range context.RecvChannel {
			// log.Printf("RECV: %+v", message)
			if message.Type == "ping" {
				log.Printf("PING")
				md.PingChannel <- message
			} else if message.Type == "md_update" {
				md.UpdateChannel <- message
			} else if message.Type == "ticker" {
				log.Printf("Ticker")
				md.UpdateChannel <- message
			} else {
				md.ResponseChannel <- message
			}
		}
	}(context, &md)

	go func(context *Context, md *MarketDataAdapter) {
		for ping := range md.PingChannel {
			ping.Type = "pong"
			md.Context.SendChannel <- ping
			log.Printf("PONG")
		}
	}(context, &md)

	go func(context *Context, md *MarketDataAdapter) {
		for response := range md.ResponseChannel {
			// log.Printf("ResponseChannel: %+v", response)
			if response.Type == "order-book-subscribe" {
				CreateSnapshot(&response)
				log.Printf("Bids: %+v", bids)
				log.Printf("Asks: %+v", asks)
				bids.Id = response.Data.Id
				asks.Id = response.Data.Id
			}
			md.ResponseHandler(response)
		}
	}(context, &md)

	go func(context *Context, md *MarketDataAdapter) {
		for response := range md.UpdateChannel {
			// log.Printf("UpdateChannel: %+v", response)
			if response.Type == "md_update" && bids.Id+1 == response.Data.Id && asks.Id+1 == response.Data.Id {
				UpdateSnapshot(&response)
				bids.Id++
				asks.Id++
				log.Printf("Bids: %+v", bids.Data[0:2])
				log.Printf("Asks: %+v", asks.Data[0:2])
				md.UpdateHandler(response)
			} else if response.Type == "ticker" {
				log.Printf("Ticker: ", response)
			} else {
				log.Fatal("Missed update snapshot/Resync")
			}
		}
	}(context, &md)

	return &md
}

type TickerRequest struct {
	Type string      `json:"e"`
	Pair interface{} `json:"data"`
}

func (adapter *MarketDataAdapter) Subscribe(sym1, sym2 string, depth int) {
	request := Message{}
	request.Type = "order-book-subscribe"
	request.Data.Pair = []string{sym1, sym2}
	request.Data.Subscribe = true
	request.Data.Depth = depth
	adapter.Context.SendChannel <- request
	go func() {
		for {
			ticker := TickerRequest{}
			ticker.Type = "ticker"
			ticker.Pair = []string{sym1, sym2}
			ticker_string, _ := json.Marshal(ticker)
			adapter.Context.SendJsonChannel <- ticker_string
			time.Sleep(2 * time.Second)
			log.Printf("TICKER")
		}
	}()
	// log.Printf("Subscribed:", sym1, sym2, depth)
}

func (adapter *MarketDataAdapter) Cleanup() {
	adapter.Context.Cleanup()
	close(adapter.PingChannel)
	close(adapter.ResponseChannel)
	close(adapter.UpdateChannel)
	log.Printf("MarketDataAdapater Cleaup")
}
