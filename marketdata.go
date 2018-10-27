package cexio

import (
	"encoding/json"
	"fmt"
	"github.com/buger/goterm"
	"log"
	"sort"
	"strconv"
	"time"
)

type Level struct {
	Price float32
	Qty   float32
}

const (
	kMaxDepth         = 6
	kMaxPrice float32 = 9999999.9999
	kBuy              = 0
	kSell             = 1
	kNumSide          = 2
)

type Side int

type Levels struct {
	Data [kMaxDepth]Level
}

func (levels *Levels) Len() int { return len(levels.Data) }

func (levels *Levels) Swap(i, j int) {
	levels.Data[i], levels.Data[j] = levels.Data[j], levels.Data[i]
}

func (levels *Levels) Less(i, j int) bool {
	return levels.Data[i].Price < levels.Data[j].Price
}

type Orderbook struct {
	Id        int
	Pair      string
	Bids      Levels
	Asks      Levels
	Low       float32
	High      float32
	LastPrice float32
	Volume    float32
	Bid       float32
	Ask       float32
}

func (orderbook *Orderbook) update(price, qty float32, level int, side Side) {
	if side == kBuy {
		orderbook.Bids.Data[level].Price = price
		orderbook.Bids.Data[level].Qty = qty
	} else {
		orderbook.Asks.Data[level].Price = price
		orderbook.Asks.Data[level].Qty = qty
	}
}

func (orderbook *Orderbook) initalize() {
	for i := 0; i < kMaxDepth; i++ {
		orderbook.update(-kMaxPrice, 0, i, kBuy)
		orderbook.update(kMaxPrice, 0, i, kSell)
	}
}

func (orderbook *Orderbook) allLevelUpdate(levels [][]float32, side Side) {
	for index, data := range levels {
		orderbook.update(data[0], data[1], index, side)
	}
}

func (orderbook *Orderbook) removeLevel(price float32, side Side) {
	price_levels := orderbook.Bids.Data
	max_price := -kMaxPrice
	if side == kSell {
		price_levels = orderbook.Asks.Data
		max_price = kMaxPrice
	}

	for i, level := range price_levels {
		if level.Price == price {
			orderbook.update(max_price, 0, i, side)
		}
	}
}

func (orderbook *Orderbook) updateLevel(price, qty float32, side Side) bool {
	price_levels := orderbook.Bids.Data
	if side == kSell {
		price_levels = orderbook.Asks.Data
	}

	for i, level := range price_levels {
		if level.Price == price {
			orderbook.update(price, qty, i, side)
			return true
		}
	}

	return false
}

var ob_map = make(map[string]*Orderbook)

func PrintOrderbook() {
	goterm.Clear()
	goterm.MoveCursor(1, 1)
	for _, orderbook := range ob_map {
		ob := goterm.NewTable(0, 10, 5, ' ', 0)
		fmt.Fprintf(ob, "%s\t%d\n", orderbook.Pair, orderbook.Id)
		fmt.Fprintf(ob, "%s\t%s\t%s\t%s\n", "BidSz", "Bid", "Ask", "AskSz")
		for i := 0; i < kMaxDepth-1; i++ {
			fmt.Fprintf(ob, "%0.4f\t%0.4f\t", orderbook.Bids.Data[i].Qty, orderbook.Bids.Data[i].Price)
			fmt.Fprintf(ob, "%0.4f\t%0.4f\n", orderbook.Asks.Data[i].Price, orderbook.Asks.Data[i].Qty)
		}
		fmt.Fprintf(ob, "\n")
		goterm.Println(ob)
	}

	goterm.Flush()
}

func CreateSnapshot(m *Message) {
	orderbook := &Orderbook{}
	orderbook.Pair = m.Data.Pair.(string)
	orderbook.Id = m.Data.Id
	orderbook.initalize()
	orderbook.allLevelUpdate(m.Data.Bids, kBuy)
	orderbook.allLevelUpdate(m.Data.Asks, kSell)
	ob_map[m.Data.Pair.(string)] = orderbook
	l.Infof("Created Orderbook %+v", orderbook)
}

func ParseFloat32(data string) float32 {
	num, _ := strconv.ParseFloat(data, 32)
	return float32(num)
}

func UpdateTicker(m *Message) {
	if orderbook, ok := ob_map[m.Data.Pair.(string)]; ok {
		orderbook.Low = ParseFloat32(m.Data.Low)
		orderbook.High = ParseFloat32(m.Data.High)
		orderbook.LastPrice = ParseFloat32(m.Data.Last)
		orderbook.Volume = ParseFloat32(m.Data.Volume)
		orderbook.Bid = m.Data.Bid
		orderbook.Ask = m.Data.Ask
	}
}

func UpdateSnapshot(m *Message) {
	orderbook := ob_map[m.Data.Pair.(string)]
	updated_bid := false
	updated_ask := false
	orderbook.Id++
	for _, update := range m.Data.Bids {
		l.Infof("Bid Update %+v", update)
		if update[1] == 0 {
			orderbook.removeLevel(update[0], kBuy)
		} else {
			updated_bid = orderbook.updateLevel(update[0], update[1], kBuy)
			if !updated_bid {
				orderbook.update(update[0], update[1], kMaxDepth-1, kBuy)
			}
		}
	}

	for _, update := range m.Data.Asks {
		l.Infof("Ask Update %+v", update)
		if update[1] == 0 {
			orderbook.removeLevel(update[0], kSell)
		} else {
			updated_ask = orderbook.updateLevel(update[0], update[1], kSell)
			if !updated_ask {
				orderbook.update(update[0], update[1], kMaxDepth-1, kSell)
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

func (md *MarketDataAdapter) pingPongRoutine() {
	for ping := range md.PingChannel {
		ping.Type = "pong"
		md.Context.SendChannel <- ping
		l.Infof("PONG")
	}
}

func getTickerSymbol(message *Message) string {
	return message.Data.Pair.([]interface{})[0].(string) + ":" + message.Data.Pair.([]interface{})[1].(string)
}

func (md *MarketDataAdapter) responseRouterRoutine() {
	for message := range md.Context.RecvChannel {
		if message.Type == "ping" {
			l.Infof("PING")
			md.PingChannel <- message
		} else if message.Type == "md_update" {
			l.Infof("MD_UPDTE")
			md.UpdateChannel <- message
		} else if message.Type == "ticker" {
			l.Infof("TICKER")
			message.Data.Pair = getTickerSymbol(&message)
			l.Infof("TICKER %s", message.Data.Pair)
			md.UpdateChannel <- message
		} else {
			md.ResponseChannel <- message
		}
	}
}

func (md *MarketDataAdapter) responseHandlerRoutine() {
	for response := range md.ResponseChannel {
		if response.Type == "order-book-subscribe" {
			l.Infof("MD: %+v", response)
			CreateSnapshot(&response)
			PrintOrderbook()
			l.Infof("%+v", ob_map)
		}
		md.ResponseHandler(response)
	}
}

func (md *MarketDataAdapter) updateHandlerRoutine() {
	for response := range md.UpdateChannel {
		// log.Printf("UpdateChannel: %+v", response)
		orderbook := ob_map[response.Data.Pair.(string)]
		if response.Type == "md_update" && orderbook.Id+1 == response.Data.Id {
			UpdateSnapshot(&response)
			l.Infof("Current Orderbook: %+v", orderbook)
		} else if response.Type == "ticker" {
			UpdateTicker(&response)
		} else {
			log.Fatal("Missed update snapshot/Resync")
		}
	}
}

func NewMarketDataAdapter(context *Context) *MarketDataAdapter {
	md := MarketDataAdapter{}
	md.Context = context
	md.PingChannel = make(chan Message, 16)
	md.ResponseChannel = make(chan Message, 16)
	md.UpdateChannel = make(chan Message, 16)
	md.UpdateHandler = func(m Message) {}
	md.ResponseHandler = ResponseHandler

	// Start Response handler goroutine which will
	// send responses on different channels
	go md.pingPongRoutine()
	go md.responseRouterRoutine()
	go md.responseHandlerRoutine()
	go md.updateHandlerRoutine()
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
