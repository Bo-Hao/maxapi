package maxapi

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
)

func (Mc *MaxClient) Run(ctx context.Context) {
	go func() {
		Mc.PriviateWebsocket(ctx)
	}()

	go func() {
		for {
			time.Sleep(30 * time.Second)
			select {
			case <-ctx.Done():
				return
			default:
				message := []byte("ping")
				Mc.WsClient.Conn.WriteMessage(websocket.TextMessage, message)
			}
		}
	}()

	go func() {
		for {
			select {
			case <-ctx.Done():
				Mc.ShutDown()
				return
			default:
				Mc.ShutingBranch.RLock()
				if Mc.ShutingBranch.shut {
					Mc.ShutDown()
				}
				Mc.ShutingBranch.RUnlock()

				_, err := Mc.GetBalance()
				if err != nil {
					LogWarningToDailyLogFile(err, ". in routine checking")
				}

				_, err = Mc.GetAllOrders()
				if err != nil {
					LogWarningToDailyLogFile(err, ". in routine checking")
				}
				time.Sleep(60 * time.Second)
			}

		}
	}()

	go func() {
		for {
			select {
			case <-ctx.Done():
				Mc.ShutDown()
				return
			default:
				Mc.ShutingBranch.RLock()
				if Mc.ShutingBranch.shut {
					Mc.ShutDown()
				}
				Mc.ShutingBranch.RUnlock()

				_, err := Mc.GetMarkets()
				if err != nil {
					LogWarningToDailyLogFile(err, ". in routine checking")
				}
				time.Sleep(12 * time.Hour)
			}
		}
	}()

	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		fmt.Println("Ctrl + C Pressed: shutting max websocket")
		Mc.ShutDown()
	}()
}

func (Mc *MaxClient) RunWithTradeChannel(ctx context.Context, tradeChan chan []Trade) {
	go func() {
		Mc.PriviateWebsocketWithChannel(ctx, tradeChan)
	}()

	go func() {
		for {
			time.Sleep(50 * time.Second)
			select {
			case <-ctx.Done():
				return
			default:
				message := []byte("ping")
				Mc.WsClient.Conn.WriteMessage(websocket.TextMessage, message)
			}
		}
	}()

	go func() {
		for {
			select {
			case <-ctx.Done():
				Mc.ShutDown()
				return
			default:
				Mc.ShutingBranch.RLock()
				if Mc.ShutingBranch.shut {
					Mc.ShutDown()
				}
				Mc.ShutingBranch.RUnlock()

				_, err := Mc.GetBalance()
				if err != nil {
					LogWarningToDailyLogFile(err, ". in routine checking balance")
				}

				_, err = Mc.GetAllOrders()
				if err != nil {
					LogWarningToDailyLogFile(err, ". in routine checking orders")
				}
				time.Sleep(time.Minute)
			}

		}
	}()

	go func() {
		for {
			select {
			case <-ctx.Done():
				Mc.ShutDown()
				return
			default:
				Mc.ShutingBranch.RLock()
				if Mc.ShutingBranch.shut {
					Mc.ShutDown()
				}
				Mc.ShutingBranch.RUnlock()

				_, err := Mc.GetMarkets()
				if err != nil {
					LogWarningToDailyLogFile(err, ". in routine checking")
				}
				time.Sleep(12 * time.Hour)
			}
		}
	}()

	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		fmt.Println("Ctrl + C Pressed: shutting max websocket")
		Mc.ShutDown()
	}()
}

func NewMaxClient(ctx context.Context, APIKEY, APISECRET string) *MaxClient {
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
	_, cancel := context.WithCancel(ctx)
	m.cancelFunc = &cancel
	m.ShutingBranch.shut = false
	m.ApiClient = apiclient
	m.OrdersBranch.Orders = make(map[int64]WsOrder)
	m.OrdersBranch.OrderNumbers = make(map[string]NumbersOfOrder)
	m.FilledOrdersBranch.Partial = make(map[int64]WsOrder)
	m.FilledOrdersBranch.Filled = make(map[int64]WsOrder)
	m.MarketsBranch.Markets = markets
	m.BalanceBranch.Balance = make(map[string]Balance)

	return &m
}

func (Mc *MaxClient) ShutDown() {
	fmt.Println("Shut Down the program")
	Mc.CancelAllOrders()
	(*Mc.cancelFunc)()

	Mc.ShutingBranch.Lock()
	Mc.ShutingBranch.shut = true
	Mc.ShutingBranch.Unlock()
	time.Sleep(3 * time.Second)
	os.Exit(1)
}

// Detect if there is Unhedge position.
func (Mc *MaxClient) DetectUnhedgeOrders() (map[string]HedgingOrder, bool) {
	// check if there is filled order but not hedged.
	Mc.FilledOrdersBranch.RLock()
	filledLen := len(Mc.FilledOrdersBranch.Filled)
	Mc.FilledOrdersBranch.RUnlock()
	if filledLen == 0 {
		return map[string]HedgingOrder{}, false
	}
	hedgingOrders := map[string]HedgingOrder{}

	Mc.FilledOrdersBranch.Lock()
	defer Mc.FilledOrdersBranch.Unlock()

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
				Timestamp: int64(time.Now().UnixMilli()),
			}
			hedgingOrders[market] = ho
		}

		// dealing with partial filled orders
		if order.State == "done" || order.State == "cancel" {
			delete(Mc.FilledOrdersBranch.Partial, order.Id)
		} else {
			Mc.FilledOrdersBranch.Partial[order.Id] = order
		}

	}

	Mc.FilledOrdersBranch.Filled = map[int64]WsOrder{}
	return hedgingOrders, true
}

func (Mc *MaxClient) DetectUnhedgeTrades() (map[string]HedgingOrder, bool) {
	// check if there is unhedged trade.
	Mc.TradeBranch.RLock()
	tradeLen := len(Mc.TradeBranch.UnhedgeTrades)
	Mc.TradeBranch.RUnlock()
	if tradeLen == 0 {
		return map[string]HedgingOrder{}, false
	}

	hedgingOrders := map[string]HedgingOrder{}
	unhedgeTrades := Mc.TakeUnhedgeTrades()

	for i := 0; i < len(unhedgeTrades); i++ {
		trade := unhedgeTrades[i]
		market := trade.Market
		price, err := strconv.ParseFloat(trade.Price, 64)
		if err != nil {
			LogFatalToDailyLogFile(err, trade.Price, "is the element can't be convert")
		}
		volume, err := strconv.ParseFloat(trade.Volume, 64)
		if err != nil {
			LogFatalToDailyLogFile(err, trade.Volume, "is the element can't be convert")
		}

		maxFee, err := strconv.ParseFloat(trade.Fee, 64)
		if err != nil {
			LogFatalToDailyLogFile(err, trade.Fee, "is the element can't be convert")
		}

		s := 1.
		if trade.Side == "buy" || trade.Side == "bid" {
			s = -1.
		}

		if hedgingOrder, ok := hedgingOrders[market]; ok {
			hedgingOrder.Profit += price * volume * s
			hedgingOrder.Volume += volume * s
			hedgingOrder.AbsVolume += volume
			hedgingOrder.MaxFee += maxFee
		} else {
			base, quote, err := Mc.checkBaseQuote(market)
			if err != nil {
				LogFatalToDailyLogFile(err)
			}
			ho := HedgingOrder{
				Market:         market,
				Base:           base,
				Quote:          quote,
				Profit:         price * volume * s,
				Volume:         volume * s,
				Timestamp:      int64(time.Now().UnixMilli()),
				MaxFee:         maxFee,
				MaxFeeCurrency: trade.FeeCurrency,
				MaxMaker:       trade.Maker,
			}
			hedgingOrders[market] = ho
		}
	}
	return hedgingOrders, true
}
