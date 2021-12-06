package maxapi

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

func (Mc *MaxClient) Run() {
	go func() {
		Mc.PriviateWebsocket()
	}()

	go func() {
		for {
			Mc.ShutingBranch.mux.RLock()
			if Mc.ShutingBranch.shut {
				Mc.ShutDown()
			}
			Mc.ShutingBranch.mux.RUnlock()

			_, err := Mc.GetBalance()
			if err != nil {
				LogWarningToDailyLogFile(err, ". in routine checking")
			}

			_, err = Mc.GetAllOrders()
			if err != nil {
				LogWarningToDailyLogFile(err, ". in routine checking")
			}
			time.Sleep(300 * time.Second)
		}
	}()

	go func() {
		for {
			Mc.ShutingBranch.mux.RLock()
			if Mc.ShutingBranch.shut {
				Mc.ShutDown()
			}
			Mc.ShutingBranch.mux.RUnlock()

			_, err := Mc.GetMarkets()
			if err != nil {
				LogWarningToDailyLogFile(err, ". in routine checking")
			}
			time.Sleep(12 * time.Hour)
		}
	}()

	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		fmt.Println("Ctrl + C is pressed")
		Mc.ShutDown()
	}()
}

func NewMaxClient(APIKEY, APISECRET string) *MaxClient {
	ctx, cancel := context.WithCancel(context.Background())

	// api client
	cfg := NewConfiguration()
	apiclient := NewAPIClient(cfg)

	// Get markets []Market
	markets, _, err := apiclient.PublicApi.GetApiV2Markets(ctx)
	if err != nil {
		LogFatalToDailyLogFile(err)
	}
	m := MaxClient{}
	m.apiKey = APIKEY
	m.apiSecret = APISECRET
	m.ctx = ctx
	m.cancelFunc = cancel
	m.ShutingBranch.shut = false
	m.ApiClient = apiclient
	m.OrdersBranch.Orders = map[int32]WsOrder{}
	m.FilledOrdersBranch.Partial = map[int32]WsOrder{}
	m.FilledOrdersBranch.Filled = map[int32]WsOrder{}
	m.MarketsBranch.Markets = markets
	m.BalanceBranch.Balance = map[string]Balance{}

	return &m
}

func (Mc *MaxClient) ShutDown() {
	fmt.Println("Shut Down the program")
	Mc.CancelAllOrders()
	Mc.cancelFunc()

	Mc.ShutingBranch.mux.Lock()
	Mc.ShutingBranch.shut = true
	Mc.ShutingBranch.mux.Unlock()
	time.Sleep(3 * time.Second)
	os.Exit(1)
}

// Detect if there is Unhedge position.
func (Mc *MaxClient) DetectUnhedgeOrders() (map[string]HedgingOrder, bool) {
	// check if there is filled order but not hedged.
	Mc.FilledOrdersBranch.mux.RLock()
	filledLen := len(Mc.FilledOrdersBranch.Filled)
	Mc.FilledOrdersBranch.mux.RUnlock()
	if filledLen == 0 {
		return map[string]HedgingOrder{}, false
	}
	hedgingOrders := map[string]HedgingOrder{}

	Mc.FilledOrdersBranch.mux.Lock()
	defer Mc.FilledOrdersBranch.mux.Unlock()

	for _, order := range Mc.FilledOrdersBranch.Filled {
		preEv := 0.
		if _, in := Mc.FilledOrdersBranch.Partial[order.Id]; in {
			f, err := strconv.ParseFloat(Mc.FilledOrdersBranch.Partial[order.Id].ExecutedVolume, 64)
			if err != nil {
				LogWarningToDailyLogFile("fail to convert previous executed volume: ", err)
			} else {
				preEv = f
			}
		}

		market := order.Market
		price, err := strconv.ParseFloat(order.Price, 64)
		if err != nil {
			LogFatalToDailyLogFile(err, order.Price, "is the element can't be convert")
		}
		volume, err := strconv.ParseFloat(order.ExecutedVolume, 64)
		if err != nil {
			LogFatalToDailyLogFile(err, order.ExecutedVolume, "is the element can't be convert")
		}
		if volume > preEv {
			volume = volume - preEv
		}

		s := 1.
		if order.Side == "buy" || order.Side == "bid" {
			s = -1.
		}

		timestamp := int32(time.Now().UnixMilli())

		if hedgingOrder, ok := hedgingOrders[market]; ok {
			hedgingOrder.Profit += price * volume * s
			hedgingOrder.Volume += volume * s
			hedgingOrder.AbsVolume += volume
		} else {
			base, quote, err := Mc.checkBaseQuote(market)
			if err != nil {
				LogFatalToDailyLogFile(err)
			}
			ho := HedgingOrder{
				Market:    market,
				Base:      base,
				Quote:     quote,
				Profit:    price * volume * s,
				Volume:    volume * s,
				AbsVolume: volume,
				Timestamp: timestamp,
			}
			hedgingOrders[market] = ho
		}

		// dealing with partial filled orders
		if order.State == "done" {
			delete(Mc.FilledOrdersBranch.Partial, order.Id)
		} else {
			Mc.FilledOrdersBranch.Partial[order.Id] = order
		}

	}

	Mc.FilledOrdersBranch.Filled = map[int32]WsOrder{}
	return hedgingOrders, true
}
