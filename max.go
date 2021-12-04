package maxapi

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
	"github.com/shopspring/decimal"
)

func (Mc *MaxClient) Run() {
	go func() {
		Mc.SubscribeWS()
	}()

	go func() {
		for {
			Mc.shutingMutex.Lock()
			if Mc.shuting {
				Mc.ShutDown()
			}
			Mc.shutingMutex.Unlock()

			err := Mc.BalanceGlobal2Local()
			if err != nil {
				LogWarningToDailyLogFile(err, ". in routine checking")
			}

			err = Mc.OrderGlobal2Local()
			if err != nil {
				LogWarningToDailyLogFile(err, ". in routine checking")
			}
			time.Sleep(60 * time.Second)
		}
	}()

	go func() {
		for {
			Mc.shutingMutex.Lock()
			if Mc.shuting {
				Mc.ShutDown()
			}
			Mc.shutingMutex.Unlock()

			err := Mc.MarketsGolbal2Local()
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

	return &MaxClient{
		apiKey:    APIKEY,
		apiSecret: APISECRET,

		ctx:        ctx,
		cancelFunc: cancel,
		shuting:    false,

		ApiClient: apiclient,

		LimitOrders:         map[int32]WsOrder{},
		FilledOrders:        map[int32]WsOrder{},
		PartialFilledOrders: map[int32]WsOrder{},

		Markets:      markets,
		LocalBalance: map[string]Balance{},
	}
}

func (Mc *MaxClient) ShutDown() {
	fmt.Println("Shut Down the program")
	Mc.CancelAllOrders()
	Mc.cancelFunc()

	Mc.shutingMutex.Lock()
	Mc.shuting = true
	Mc.shutingMutex.Unlock()
	time.Sleep(3 * time.Second)
	os.Exit(1)
}

type MaxClient struct {
	apiKey    string
	apiSecret string

	ctx          context.Context
	cancelFunc   context.CancelFunc
	shuting      bool
	shutingMutex sync.Mutex

	// CXMM parameters
	BaseOrderUnit      string
	baseOrderUnitMutex sync.RWMutex

	// exchange information
	ExchangeInfo ExchangeInfo
	exchangeInfoMutex sync.RWMutex

	// web socket client
	WsClient WebsocketClient

	// api client
	ApiClient *APIClient

	// limit unfilled orders
	LimitOrders      map[int32]WsOrder
	limitOrdersMutex sync.RWMutex

	// filled orders
	FilledOrders        map[int32]WsOrder
	PartialFilledOrders map[int32]WsOrder
	filledOrdersMutex   sync.RWMutex

	// All markets pairs
	Markets      []Market
	marketsMutex sync.RWMutex

	// Account
	Account      Member
	accountMutex sync.RWMutex

	// local balance
	LocalBalance      map[string]Balance // currency balance
	localBalanceMutex sync.RWMutex
}

type ExchangeInfo struct {
	MinOrderUnit float64
	LimitApi     int
	CurrentNApi  int
}

type WebsocketClient struct {
	OnErr      bool
	onErrMutex sync.RWMutex
	Conn       *websocket.Conn

	LastUpdatedId      decimal.Decimal
	LastUpdatedIdMutex sync.RWMutex

	TmpTrades      []Trade
	TmpOrders      map[int32]WsOrder
	TmpOrdersMutex sync.RWMutex
}

func (Mc *MaxClient) MarketsGolbal2Local() error {
	Mc.marketsMutex.Lock()
	defer Mc.marketsMutex.Unlock()
	markets, _, err := Mc.ApiClient.PublicApi.GetApiV2Markets(Mc.ctx)
	if err != nil {
		return errors.New("fail to get market")
	}
	Mc.Markets = markets
	return nil
}

// GET account and sent the balance to the local balance
func (Mc *MaxClient) BalanceGlobal2Local() error {
	Member, err := Mc.ApiGetAccount()
	if err != nil {
		LogFatalToDailyLogFile(err)
	}

	Mc.localBalanceMutex.Lock()
	defer Mc.localBalanceMutex.Unlock()

	Accounts := Member.Accounts
	for i := 0; i < len(Accounts); i++ {
		currency := Accounts[i].Currency
		balance, err := strconv.ParseFloat(Accounts[i].Balance, 64)
		if err != nil {
			LogFatalToDailyLogFile(err)
			return errors.New("fail to parse balance to float64")
		}
		locked, err := strconv.ParseFloat(Accounts[i].Locked, 64)
		if err != nil {
			LogFatalToDailyLogFile(err)
			return errors.New("fail to parse locked balance to float64")
		}

		b := Balance{
			Name:      currency,
			Avaliable: balance,
			Locked:    locked,
		}
		Mc.LocalBalance[currency] = b
	}

	return nil
}

func (Mc *MaxClient) OrderGlobal2Local() error {
	newOrders := map[int32]WsOrder{}
	for i := 0; i < len(Mc.Markets); i++ {
		marketId := Mc.Markets[i].Id
		orders, _, err := Mc.ApiClient.PrivateApi.GetApiV2Orders(Mc.ctx, Mc.apiKey, Mc.apiSecret, marketId, nil)
		if err != nil {
			return errors.New("fail to get order list")
		}

		for j := 0; j < len(orders); j++ {
			newOrders[orders[j].Id] = WsOrder(orders[j])
		}
	}

	Mc.limitOrdersMutex.Lock()
	defer Mc.limitOrdersMutex.Unlock()
	Mc.LimitOrders = newOrders
	return nil
}

func (Mc *MaxClient) DetectFilledOrders() (map[string]HedgingOrder, bool) {
	hedgingOrders := map[string]HedgingOrder{}
	if len(Mc.FilledOrders) == 0 {
		return hedgingOrders, false
	}

	Mc.filledOrdersMutex.Lock()
	defer Mc.filledOrdersMutex.Unlock()

	for _, order := range Mc.FilledOrders {
		preEv := 0.
		if _, in := Mc.PartialFilledOrders[order.Id]; in {
			f, err := strconv.ParseFloat(Mc.PartialFilledOrders[order.Id].ExecutedVolume, 64)
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
			delete(Mc.PartialFilledOrders, order.Id)
		} else {
			Mc.PartialFilledOrders[order.Id] = order
		}

	}

	Mc.FilledOrders = map[int32]WsOrder{}
	return hedgingOrders, true
}

type HedgingOrder struct {
	// order
	Market    string
	Base      string
	Quote     string
	Profit    float64
	Volume    float64
	Timestamp int32
	AbsVolume float64

	// hedged info
	TotalProfit        float64
	MarketTransactTime int32
	AvgPrice           float64
	TransactVolume     float64
	MarketSide string
	Fee                float64
	FeeCurrency        string
	
}

// ########### assistant functions ###########

func (Mc *MaxClient) checkBaseQuote(market string) (base, quote string, err error) {
	markets := Mc.Markets
	for _, m := range markets {
		if m.Id == market {
			base = m.BaseUnit
			quote = m.QuoteUnit
			err = nil
			return
		}
	}
	err = errors.New("market not exist")
	return
}

// volume is the volume of base currency. return true denote enough for trading.
func (Mc *MaxClient) checkBalanceEnoughLocal(market, side string, price, volume float64) (enough bool) {
	Mc.localBalanceMutex.RLock()
	defer Mc.localBalanceMutex.RUnlock()

	base, quote, err := Mc.checkBaseQuote(market)
	if err != nil {
		LogErrorToDailyLogFile(err)
		return false
	}
	switch side {
	case "sell":
		baseBalance := Mc.LocalBalance[base].Avaliable
		if baseBalance > volume {
			enough = true
		}
	case "buy":
		needed := price * volume
		quoteBalance := Mc.LocalBalance[quote].Avaliable
		if quoteBalance >= needed {
			enough = true
		}
	} // end switch
	return
}

func (Mc *MaxClient) updateLocalBalance(market, side string, price, volume float64, gain bool) error {
	Mc.localBalanceMutex.Lock()
	defer Mc.localBalanceMutex.Unlock()
	base, quote, err := Mc.checkBaseQuote(market)
	if err != nil {
		LogFatalToDailyLogFile(err)
		return errors.New("fail to update local balance")
	}

	switch side {
	case "sell":
		bb := Mc.LocalBalance[base]
		if gain {
			bb.Avaliable += volume
			bb.Locked -= volume
		} else {
			bb.Avaliable -= volume
			bb.Locked += volume
		}
		Mc.LocalBalance[base] = bb
	case "buy":
		bq := Mc.LocalBalance[quote]
		needed := price * volume
		if gain {
			bq.Avaliable += needed
			bq.Locked -= needed
		} else {
			bq.Avaliable -= needed
			bq.Locked += needed
		}
		Mc.LocalBalance[quote] = bq
	}
	return nil
}

type Balance struct {
	Name      string
	Avaliable float64
	Locked    float64
}
