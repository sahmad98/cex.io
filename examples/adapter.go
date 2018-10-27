package main

import (
	"fmt"
	"log"
	"github.com/sahmad98/cex.io"
	"time"
)

func main() {
	context := cexio.GetApplicationContext()
	md := cexio.NewMarketDataAdapter(context)
	err := context.Authenticate()
	if err != nil {
		log.Fatal(err)
	}

	var input string
	fmt.Println("Enter to subscribe")
	fmt.Scanln(&input)
	md.Subscribe("BTC", "USD", 5)
	//md.Subscribe("ETH", "USD", 5)
	//md.Subscribe("ETH", "BTC", 5)
	go func() {
		for {
			cexio.PrintOrderbook()
			time.Sleep(1 * time.Second)
		}
	}()
	time.Sleep(6000 * time.Second)
}
