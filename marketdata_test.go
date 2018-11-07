package cexio

import (
	"github.com/golang-collections/go-datastructures/queue"
	"testing"
)

func BenchmarkRingBufferGetPut(b *testing.B) {
	x := queue.NewRingBuffer(16)
	for i := 0; i < b.N; i++ {
		x.Put(10)
		x.Get()
	}
}

func BenchmarkUpdateTicker(b *testing.B) {
	md := MarketDataAdapter{}
	orderbook := Orderbook{}
	m := Message{}
	m.Data.Low = "1235.223"
	m.Data.High = "1254.223"
	m.Data.Bid = float32(125.25)
	m.Data.Ask = float32(132.25)
	m.Data.Last = "125.25"
	m.Data.Volume = "125.25"

	for i := 0; i < b.N; i++ {
		md.UpdateTicker(&m, &orderbook)
	}
}

func BenchmarkParseFloat(b *testing.B) {
	for i := 0; i < b.N; i++ {
		ParseFloat32("125.25")
	}
}

func BenchmarkOrderbookLookup(b *testing.B) {
	ob1 := Orderbook{}
	ob1.Pair = "BTCUSD"
	ob2 := Orderbook{}
	ob2.Pair = "ETHUSD"
	ob3 := Orderbook{}
	ob3.Pair = "BTCETH"
	ob_map["BTCUSD"] = &ob1
	ob_map["ETHUSD"] = &ob2
	ob_map["BTCETH"] = &ob3
	pairs := []string{"BTCUSD", "ETHUSD", "BTCETH"}
	for i := 0; i < b.N; i++ {
		_ = ob_map[pairs[i%3]]
	}
}

func BenchmarkOrderbookRemoveLevel(b *testing.B) {
	ob := Orderbook{}
	ob.initalize()
	for i := 0; i < b.N; i++ {
		ob.Bids.Data[4].Price = 6556.25
		ob.removeLevel(6556.25, kBuy)
	}
}

func BenchmarkOrderbookUpdateLevel(b *testing.B) {
	ob := Orderbook{}
	ob.initalize()
	for i := 0; i < b.N; i++ {
		ob.Asks.Data[4].Price = 6556.25
		ob.updateLevel(6556.25, 0.0225, kBuy)
	}
}
