package cexio

import (
	"encoding/json"
	"fmt"
	"github.com/buger/goterm"
	"github.com/google/flatbuffers/go"
	buffer "github.com/sahmad98/cex.io/types"
	"github.com/spf13/viper"
	"log"
	"net"
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
	Id        int32
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

func (ob *Orderbook) getBuffer() []byte {
	builder := flatbuffers.NewBuilder(1024)

	// buffer.LevelsStart(builder)
	buffer.LevelsStartDataVector(builder, kMaxDepth)
	for i := kMaxDepth - 1; i >= 0; i-- {
		bid := buffer.CreateLevel(builder, ob.Bids.Data[i].Price, ob.Bids.Data[i].Qty)
		builder.PrependUOffsetT(bid)
	}
	// bids := buffer.LevelsEnd(builder)
	bids := builder.EndVector(kMaxDepth)

	// buffer.LevelsStart(builder)
	buffer.LevelsStartDataVector(builder, kMaxDepth)
	for i := kMaxDepth - 1; i >= 0; i-- {
		ask := buffer.CreateLevel(builder, ob.Asks.Data[i].Price, ob.Asks.Data[i].Qty)
		builder.PrependUOffsetT(ask)
	}
	// asks := buffer.LevelsEnd(builder)
	asks := builder.EndVector(kMaxDepth)
	pair := builder.CreateString(ob.Pair)

	buffer.OrderbookStart(builder)
	buffer.OrderbookAddId(builder, ob.Id)
	buffer.OrderbookAddPair(builder, pair)
	buffer.OrderbookAddBids(builder, bids)
	buffer.OrderbookAddAsks(builder, asks)
	buffer.OrderbookAddLow(builder, ob.Low)
	buffer.OrderbookAddHigh(builder, ob.High)
	buffer.OrderbookAddLastPrice(builder, ob.LastPrice)
	buffer.OrderbookAddVolume(builder, ob.Volume)
	buffer.OrderbookAddBid(builder, ob.Bid)
	buffer.OrderbookAddAsk(builder, ob.Ask)

	orderbook := buffer.OrderbookEnd(builder)
	builder.Finish(orderbook)
	buf := builder.FinishedBytes()
	return buf
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
		// goterm.Printf("%+v", orderbook.getBuffer())
	}

	goterm.Flush()
}

func (md *MarketDataAdapter) CreateSnapshot(m *Message) {
	orderbook := &Orderbook{}
	orderbook.Pair = m.Data.Pair.(string)
	orderbook.Id = int32(m.Data.Id)
	orderbook.initalize()
	orderbook.allLevelUpdate(m.Data.Bids, kBuy)
	orderbook.allLevelUpdate(m.Data.Asks, kSell)
	ob_map[m.Data.Pair.(string)] = orderbook
	l.Infof("Created Orderbook %+v", orderbook)
	md.OrderbookChannel <- *orderbook
}

func ParseFloat32(data string) float32 {
	num, _ := strconv.ParseFloat(data, 32)
	return float32(num)
}

func (md *MarketDataAdapter) UpdateTicker(m *Message) {
	if orderbook, ok := ob_map[m.Data.Pair.(string)]; ok {
		orderbook.Low = ParseFloat32(m.Data.Low)
		orderbook.High = ParseFloat32(m.Data.High)
		orderbook.LastPrice = ParseFloat32(m.Data.Last)
		orderbook.Volume = ParseFloat32(m.Data.Volume)
		orderbook.Bid = m.Data.Bid
		orderbook.Ask = m.Data.Ask
		md.OrderbookChannel <- *orderbook
	}
}

func (md *MarketDataAdapter) UpdateSnapshot(m *Message) {
	orderbook := ob_map[m.Data.Pair.(string)]
	updated_bid := false
	updated_ask := false
	orderbook.Id++
	// TODO Simplify this
	for _, update := range m.Data.Bids {
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
	l.Infof("MD_UPDTE,PERF,%d,%d", time.Now().UnixNano()-m.Data.Timestamp*time.Millisecond.Nanoseconds(), time.Now().UnixNano()-m.RecvTimestamp)
	md.OrderbookChannel <- *orderbook
}

type HandlerFunc func(message *Message)

type MarketDataAdapter struct {
	PingChannel      chan *Message
	ResponseChannel  chan *Message
	UpdateChannel    chan *Message
	OrderbookChannel chan Orderbook
	Context          *Context
	UpdateHandler    HandlerFunc
	ResponseHandler  HandlerFunc
}

func ResponseHandler(m *Message) {
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
		md.Context.SendChannel <- *ping
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
			md.UpdateChannel <- message
		} else if message.Type == "ticker" {
			message.Data.Pair = getTickerSymbol(message)
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
			md.CreateSnapshot(response)
			l.Infof("%+v", ob_map)
		}
		md.ResponseHandler(response)
	}
}

func (md *MarketDataAdapter) updateHandlerRoutine() {
	for response := range md.UpdateChannel {
		orderbook := ob_map[response.Data.Pair.(string)]
		if response.Type == "md_update" && orderbook.Id+1 == int32(response.Data.Id) {
			md.UpdateSnapshot(response)
			l.Infof("Current Orderbook: %+v", orderbook)
		} else if response.Type == "ticker" {
			md.UpdateTicker(response)
		} else {
			log.Fatal("Missed update snapshot/Resync")
		}
	}
}

func (md *MarketDataAdapter) runOrderbookPublisher() {
	is_publish_enabled := viper.GetBool("udp.enabled")
	if is_publish_enabled {
		ip := viper.GetString("udp.publish_ip")
		port := viper.GetInt("udp.publish_port")
		conn, error := net.ListenPacket("udp", ":0")
		if error != nil {
			log.Fatal("Error opening publish connection ", error)
		}
		dest, error := net.ResolveUDPAddr("udp", ip+":"+strconv.Itoa(port))
		for orderbook := range md.OrderbookChannel {
			_, err := conn.WriteTo(orderbook.getBuffer(), dest)
			l.Infof("Relay Orderbook: %+v", orderbook)
			if err != nil {
				l.Infof("Error Relaying, %s", err)
			}
		}
	}
}

func NewMarketDataAdapter(context *Context) *MarketDataAdapter {
	md := MarketDataAdapter{}
	md.Context = context
	md.PingChannel = make(chan *Message, 16)
	md.ResponseChannel = make(chan *Message, 16)
	md.UpdateChannel = make(chan *Message, 16)
	md.OrderbookChannel = make(chan Orderbook, 16)
	md.UpdateHandler = func(m *Message) {}
	md.ResponseHandler = ResponseHandler

	// Start Response handler goroutine which will
	// send responses on different channels
	go md.pingPongRoutine()
	go md.responseRouterRoutine()
	go md.responseHandlerRoutine()
	go md.updateHandlerRoutine()
	go md.runOrderbookPublisher()
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

func (adapter *MarketDataAdapter) Unsubscribe(sym1, sym2 string) {
	request := Message{}
	request.Type = "order-book-unsubscribe"
	request.Data.Pair = []string{sym1, sym2}
	adapter.Context.SendChannel <- request
}

func (adapter *MarketDataAdapter) Cleanup() {
	adapter.Context.Cleanup()
	close(adapter.PingChannel)
	close(adapter.ResponseChannel)
	close(adapter.UpdateChannel)
	l.Infof("MarketDataAdapater Cleaup")
}
