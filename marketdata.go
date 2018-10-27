package cexio

import (
	"encoding/json"
	"log"
	"sort"
	"time"
	"github.com/buger/goterm"
	"fmt"
)

type Level struct {
	Price float32
	Qty   float32
}

const (
	kMaxDepth = 6
	kMaxPrice = 9999999.9999
	kBuy      = 0
	kSell     = 1
	kNumSide  = 2
)

type Levels struct {
	Data [kMaxDepth]Level
}

type Side int

type Orderbook struct {
	Id        int
	Pair      string
	Bids      Levels
	Asks      Levels
	Low       float64
	High      float64
	LastPrice float64
	Volume    float64
	Bid       float32
	Ask       float32
}

func (levels *Levels) Len() int { return len(levels.Data) }

func (levels *Levels) Swap(i, j int) {
	levels.Data[i], levels.Data[j] = levels.Data[j], levels.Data[i]
}

func (levels *Levels) Less(i, j int) bool {
	return levels.Data[i].Price < levels.Data[j].Price
}

func (orderbook *Orderbook) Update(price, qty float32, level int, side Side) {
	if side == kBuy {
		orderbook.Bids.Data[level].Price = price
		orderbook.Bids.Data[level].Qty = qty
	} else {
		orderbook.Asks.Data[level].Price = price
		orderbook.Asks.Data[level].Qty = qty
	}
}

// var orderbook = Orderbook{}
var ob_map = make(map[string]*Orderbook)

func PrintOrderbook() {
	// orderbook := ob_map[m.Data.Pair]
	goterm.Clear()
	goterm.MoveCursor(1,1)
	for _, orderbook := range ob_map {
		ob  := goterm.NewTable(0, 10, 5, ' ', 0)
		fmt.Fprintf(ob, "%s\t%d\n", orderbook.Pair, orderbook.Id)
		fmt.Fprintf(ob, "%s\t%s\t%s\t%s\n", "BidSz", "Bid", "Ask", "AskSz")
		for i:=0; i<kMaxDepth-1; i++ {
			fmt.Fprintf(ob, "%0.4f\t%0.4f\t", orderbook.Bids.Data[i].Qty, orderbook.Bids.Data[i].Price)
			fmt.Fprintf(ob, "%0.4f\t%0.4f\n", orderbook.Asks.Data[i].Price, orderbook.Asks.Data[i].Qty)
		}
		fmt.Fprintf(ob, "\n")
		goterm.Println(ob)
	}

	goterm.Flush()
}

func CreateSnapshot(m *Message) {
	orderbook := ob_map[m.Data.Pair.(string)]
	orderbook.Pair = m.Data.Pair.(string)
	for i := 0; i < kMaxDepth; i++ {
		orderbook.Update(-kMaxPrice, 0, i, kBuy)
		orderbook.Update(kMaxPrice, 0, i, kSell)
	}

	for index, data := range m.Data.Bids {
		orderbook.Update(data[0], data[1], index, kBuy)
	}

	for index, data := range m.Data.Asks {
		orderbook.Update(data[0], data[1], index, kSell)
	}
	orderbook.Id = m.Data.Id
	l.Infof("Created Orderbook %+v", orderbook)
}

func UpdateSnapshot(m *Message) {
	orderbook := ob_map[m.Data.Pair.(string)]
	updated_bid := false
	updated_ask := false

	orderbook.Id++
	for _, update := range m.Data.Bids {
		l.Infof("Bid Update %+v", update)
		if update[1] == 0 {
			for i, bid := range orderbook.Bids.Data {
				if bid.Price == update[0] {
					// bids.Update(-kMaxPrice, 0, i)
					orderbook.Update(-kMaxPrice, 0, i, kBuy)
				}
			}
		} else {
			for i, bid := range orderbook.Bids.Data {
				if bid.Price == update[0] {
					// bids.Update(update[0], update[1], i)
					orderbook.Update(update[0], update[1], i, kBuy)
					updated_bid = true
					break
				}
			}

			if !updated_bid {
				// bids.Update(update[0], update[1], kMaxDepth-1)
				orderbook.Update(update[0], update[1], kMaxDepth-1, kBuy)
			}
		}
	}

	for _, update := range m.Data.Asks {
		l.Infof("Ask Update %+v", update)
		if update[1] == 0 {
			for i, ask := range orderbook.Asks.Data {
				if ask.Price == update[0] {
					orderbook.Update(kMaxPrice, 0, i, kSell)
					// asks.Update(kMaxPrice, 0, i)
				}
			}
		} else {
			for i, ask := range orderbook.Asks.Data {
				if ask.Price == update[0] {
					// asks.Update(update[0], update[1], i)
					orderbook.Update(update[0], update[1], i, kSell)
					updated_ask = true
					break
				}
			}

			if !updated_ask {
				// asks.Update(update[0], update[1], kMaxDepth-1)
				orderbook.Update(update[0], update[1], kMaxDepth-1, kSell)
			}

		}
	}

	sort.Sort(sort.Reverse(&orderbook.Bids))
	sort.Sort(&orderbook.Asks)
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
			l.Errorf("Auth Error %s", m.Data.Error)
		} else {
			l.Infof("Login Successful")
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
			if message.Type == "ping" {
				l.Infof("PING")
				md.PingChannel <- message
			} else if message.Type == "md_update" {
				l.Infof("MD_UPDTE")
				md.UpdateChannel <- message
			} else if message.Type == "ticker" {
				l.Infof("TICKER")
				message.Data.Pair = message.Data.Pair.([]interface{})[0].(string) + ":" + message.Data.Pair.([]interface{})[1].(string)
				l.Infof("TICKER %s", message.Data.Pair)
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
			l.Infof("PONG")
		}
	}(context, &md)

	go func(context *Context, md *MarketDataAdapter) {
		for response := range md.ResponseChannel {
			// log.Printf("ResponseChannel: %+v", response)
			if response.Type == "order-book-subscribe" {
				l.Infof("MD: %+v", response)
				_, ok := ob_map[response.Data.Pair.(string)]
				if !ok {
					ob_map[response.Data.Pair.(string)] = &Orderbook{}
				}
				CreateSnapshot(&response)
				PrintOrderbook()
				l.Infof("%+v", ob_map)
			}
			md.ResponseHandler(response)
		}
	}(context, &md)

	go func(context *Context, md *MarketDataAdapter) {
		for response := range md.UpdateChannel {
			// log.Printf("UpdateChannel: %+v", response)
			orderbook := ob_map[response.Data.Pair.(string)]
			if response.Type == "md_update" && orderbook.Id+1 == response.Data.Id {
				UpdateSnapshot(&response)
				l.Infof("Current Orderbook: %+v", orderbook)
			} else if response.Type == "ticker" {
				l.Infof("Ticker: %+v", response)
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
			l.Infof("TICKER")
		}
	}()
	// log.Printf("Subscribed:", sym1, sym2, depth)
}

func (adapter *MarketDataAdapter) Cleanup() {
	adapter.Context.Cleanup()
	close(adapter.PingChannel)
	close(adapter.ResponseChannel)
	close(adapter.UpdateChannel)
	l.Infof("MarketDataAdapater Cleaup")
}

func CalculateTriangularArb() {
	for {
		btcusd, ok_bu := ob_map["BTC:USD"]
		ethusd, ok_eu := ob_map["ETH:USD"]
		ethbtc, ok_eb := ob_map["ETH:BTC"]

		if ok_bu && ok_eu && ok_eb {
			l.Infof("Buy Hit Spread: %f\n", btcusd.Bids.Data[0].Price * ethbtc.Bids.Data[0].Price - ethusd.Asks.Data[0].Price)
			l.Infof("Sell Hit Spread: %f\n", ethusd.Bids.Data[0].Price - btcusd.Asks.Data[0].Price * ethbtc.Asks.Data[0].Price)
		}

		for i:=0;i<10000;i++{}
	}
}
