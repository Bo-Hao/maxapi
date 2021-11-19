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
		if Mc.shuting {
			Mc.ShutDown()
		}

		err := Mc.BalanceGlobal2Local()
		if err != nil {
			LogWarningToDailyLogFile(err, ". in routine checking")
		}

		err = Mc.OrderGlobal2Local()
		if err != nil {
			LogWarningToDailyLogFile(err, ". in routine checking")
		}
		time.Sleep(60 * time.Second)
	}()

	go func() {
		if Mc.shuting {
			Mc.ShutDown()
		}

		err := Mc.MarketsGolval2Local()
		if err != nil {
			LogWarningToDailyLogFile(err, ". in routine checking")
		}
		time.Sleep(12 * time.Hour)
	}()

	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		Mc.ShutDown()
	}()

}

func NewMaxClient(APIKEY, APISECRET string) MaxClient {
	ctx, cancel := context.WithCancel(context.Background())

	// api client
	cfg := NewConfiguration()
	apiclient := NewAPIClient(cfg)

	// Get markets []Market
	markets, _, err := apiclient.PublicApi.GetApiV2Markets(ctx)
	if err != nil {
		LogFatalToDailyLogFile(err)
	}

	return MaxClient{
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
		LocalBalance: map[string]float64{},
		LocalLocked:  map[string]float64{},
	}
}

func (Mc *MaxClient) ShutDown() {
	fmt.Println("Shut Down the program")
	Mc.CancelAllOrders()
	Mc.cancelFunc()
	Mc.shuting = true
	time.Sleep(3*time.Second)
	os.Exit(1)

}

type MaxClient struct {
	apiKey    string
	apiSecret string

	ctx        context.Context
	cancelFunc context.CancelFunc
	shuting    bool

	// CXMM parameters
	BaseOrderUnit string

	// exchange information
	ExchangeInfo ExchangeInfo

	// web socket client
	WsClient WebsocketClient

	// api client
	ApiClient *APIClient

	// limit unfilled orders
	LimitOrders      map[int32]WsOrder
	LimitOrdersMutex sync.RWMutex

	// filled orders
	FilledOrders        map[int32]WsOrder
	PartialFilledOrders map[int32]WsOrder
	FilledOrdersMutex   sync.RWMutex

	// All markets pairs
	Markets      []Market
	MarketsMutex sync.RWMutex

	// Account
	Account Member

	// local balance
	LocalBalance      map[string]float64 // currency balance
	LocalBalanceMutex sync.RWMutex
	LocalLocked       map[string]float64 // locked currency balance
	
}

type ExchangeInfo struct {
	MinOrderUnit float64
	LimitApi     int
	CurrentNApi  int
}

type WebsocketClient struct {
	OnErr bool
	Conn  *websocket.Conn

	LastUpdatedId      decimal.Decimal
	LastUpdatedIdMutex sync.RWMutex

	TmpTrades      []Trade
	TmpOrders      map[int32]WsOrder
	TmpOrdersMutex sync.RWMutex
}

func (Mc *MaxClient) CancelAllOrders() ([]WsOrder, error) {
	Mc.LimitOrdersMutex.Lock()
	defer Mc.LimitOrdersMutex.Unlock()
	canceledOrders, _, err := Mc.ApiClient.PrivateApi.PostApiV2OrdersClear(Mc.ctx, Mc.apiKey, Mc.apiSecret, nil)
	if err != nil {
		return []WsOrder{}, errors.New("fail to cancel all orders")
	}
	canceledWsOrders := make([]WsOrder, 0, len(canceledOrders))

	LogInfoToDailyLogFile("Cancel ", len(canceledOrders), " Orders by CancelAllOrders.")

	// data update
	// local balance update
	for i := 0; i < len(canceledOrders); i++ {
		canceledWsOrders = append(canceledWsOrders, WsOrder(canceledOrders[i]))
		order := canceledOrders[i]
		side := order.Side
		market := order.Market
		price, err := strconv.ParseFloat(order.Price, 64)
		if err != nil {
			LogWarningToDailyLogFile(err)
		}
		volume, err := strconv.ParseFloat(order.Volume, 64)
		if err != nil {
			LogWarningToDailyLogFile(err)
		}
		Mc.updateLocalBalance(market, side, price, volume, true) // true means this is a cancel order.
	}

	return canceledWsOrders, nil
}

func (Mc *MaxClient) CancelOrder(market string, id int32) (WsOrder, error) {
	CanceledOrder, _, err := Mc.ApiClient.PrivateApi.PostApiV2OrderDelete(Mc.ctx, Mc.apiKey, Mc.apiSecret, id)
	if err != nil {
		fmt.Println(err)
		return WsOrder{}, errors.New("fail to cancel order" + strconv.Itoa(int(id)))
	}

	LogInfoToDailyLogFile("Cancel Order ", id, "by CancelOrder func.")

	// data update
	// local balance update
	order := CanceledOrder
	side := order.Side
	price, err := strconv.ParseFloat(order.Price, 64)
	if err != nil {
		LogWarningToDailyLogFile(err)
	}
	volume, err := strconv.ParseFloat(order.Volume, 64)
	if err != nil {
		LogWarningToDailyLogFile(err)
	}

	Mc.updateLocalBalance(market, side, price, volume, true) // true means this is a cancel order

	return WsOrder(CanceledOrder), nil
}

/*	"side" (string) set tp cancel only sell (asks) or buy (bids) orders
	"market" (string) specify market like btctwd / ethbtc
	both params can be set as nil.
*/
func (Mc *MaxClient) CancelOrders(market, side interface{}) ([]WsOrder, error) {
	params := make(map[string]interface{})
	if market != nil {
		params["market"] = market.(string)
	}
	if side != nil {
		params["side"] = side.(string)
	}

	canceledOrders, _, err := Mc.ApiClient.PrivateApi.PostApiV2OrdersClear(Mc.ctx, Mc.apiKey, Mc.apiSecret, params)
	if err != nil {
		return []WsOrder{}, errors.New("fail to cancel orders")
	}
	canceledWsOrders := make([]WsOrder, 0, len(canceledOrders))

	LogInfoToDailyLogFile("Cancel ", len(canceledOrders), " Orders.")

	// local balance update
	for i := 0; i < len(canceledOrders); i++ {
		// update local orders
		wsOrder := WsOrder(canceledOrders[i])
		canceledWsOrders = append(canceledWsOrders, wsOrder)
		order := canceledOrders[i]
		side := order.Side
		market := order.Market
		price, err := strconv.ParseFloat(order.Price, 64)
		if err != nil {
			LogWarningToDailyLogFile(err)
		}
		volume, err := strconv.ParseFloat(order.Volume, 64)
		if err != nil {
			LogWarningToDailyLogFile(err)
		}
		Mc.updateLocalBalance(market, side, price, volume, true) // true means this is a cancel order.
	}

	return canceledWsOrders, nil
}

func (Mc *MaxClient) PlaceLimitOrder(market string, side string, price, volume float64) (WsOrder, error) {
	if isEnough := Mc.checkBalanceEnoughLocal(market, side, price, volume); !isEnough {
		return WsOrder{}, errors.New("balance is not enough for trading")
	}

	params := make(map[string]interface{})
	params["price"] = fmt.Sprint(price)
	params["ordType"] = "limit"
	vol := fmt.Sprint(volume)

	order, _, err := Mc.ApiClient.PrivateApi.PostApiV2Orders(Mc.ctx, Mc.apiKey, Mc.apiSecret, market, side, vol, params)
	if err != nil {
		return WsOrder{}, errors.New("fail to place limit order")
	}

	// data update
	// local balance update
	Mc.updateLocalBalance(market, side, price, volume, false) // false means this is a normal order
	return WsOrder(order), nil
}

// temporarily cannot work
func (Mc *MaxClient) PlaceMultiLimitOrders(market string, sides []string, prices, volumes []float64) ([]WsOrder, error) {
	// check not zero
	if len(sides) == 0 || len(prices) == 0 || len(volumes) == 0 {
		return []WsOrder{}, errors.New("fail to construct multi limit orders")
	}

	// check length
	if len(sides) != len(prices) || len(prices) != len(volumes) {
		return []WsOrder{}, errors.New("fail to construct multi limit orders")
	}

	optionalMap := map[string]interface{}{}
	ordersPrice := make([]string, 0, len(prices))
	ordersVolume := make([]string, 0, len(prices))

	totalVolumeBuy, totalVolumeSell := 0., 0.
	weightedPriceBuy, weightedPriceSell := 0., 0.
	countSideBuy, countSideSell := 0, 0
	for i := 0; i < len(prices); i++ {
		ordersPrice = append(ordersPrice, fmt.Sprintf("%g", prices[i]))
		ordersVolume = append(ordersVolume, fmt.Sprintf("%g", volumes[i]))

		switch sides[i] {
		case "buy":
			totalVolumeBuy += volumes[i]
			weightedPriceBuy += prices[i] * volumes[i]
			countSideBuy++
		case "sell":
			totalVolumeSell += volumes[i]
			weightedPriceSell += prices[i] * volumes[i]
			countSideSell++
		}
	}

	// check if the balance enough or not.
	buyEnough, sellEnough := false, false
	if totalVolumeBuy == 0. {
		buyEnough = true
	} else {
		weightedPriceBuy /= totalVolumeBuy
		buyEnough = Mc.checkBalanceEnoughLocal(market, "buy", weightedPriceBuy, totalVolumeBuy)
	}

	if totalVolumeSell == 0. {
		sellEnough = true
	} else {
		weightedPriceSell /= totalVolumeSell
		sellEnough = Mc.checkBalanceEnoughLocal(market, "sell", weightedPriceSell, totalVolumeSell)
	}

	// if there is one side lack of balance.
	if !buyEnough || !sellEnough {
		return []WsOrder{}, errors.New("there is no enough balance for placing multi orders")
	}

	optionalMap["orders[price]"] = ordersPrice

	// main api function
	orders, _, err := Mc.ApiClient.PrivateApi.PostApiV2OrdersMulti(Mc.ctx, Mc.apiKey, Mc.apiSecret, market, sides, ordersVolume, optionalMap)
	if err != nil {
		fmt.Println(err)
		return []WsOrder{}, errors.New("fail to place multi-limit orders")
	}

	// data update
	// local balance update
	Mc.updateLocalBalance(market, "buy", weightedPriceBuy, totalVolumeBuy, false)    // false means this is a normal order
	Mc.updateLocalBalance(market, "sell", weightedPriceSell, totalVolumeSell, false) // false means this is a normal order

	wsOrders := make([]WsOrder, 0, len(orders))
	for i := 0; i < len(orders); i++ {
		wsOrders = append(wsOrders, WsOrder(orders[i]))
	}
	return wsOrders, nil
}

func (Mc *MaxClient) PlaceMarketOrder(market string, side string, volume float64) (WsOrder, error) {
	params := make(map[string]interface{})
	params["ordType"] = "market"
	vol := fmt.Sprint(volume)
	order, _, err := Mc.ApiClient.PrivateApi.PostApiV2Orders(Mc.ctx, Mc.apiKey, Mc.apiSecret, market, side, vol, params)
	if err != nil {
		return WsOrder{}, errors.New("fail to place market orders")
	}

	// local balance update
	price, err := strconv.ParseFloat(order.Price, 64)
	if err != nil {
		LogWarningToDailyLogFile(err)
	}
	Mc.updateLocalBalance(market, side, price, volume, false) // false means this is a normal order

	return WsOrder(order), nil
}

func (Mc *MaxClient) GetAccount() (Member, error) {
	member, _, err := Mc.ApiClient.PrivateApi.GetApiV2MembersAccounts(Mc.ctx, Mc.apiKey, Mc.apiSecret)
	if err != nil {
		fmt.Println(err)
		return Member{}, errors.New("fail to get account")
	}

	Mc.Account = member
	return member, nil
}

func (Mc *MaxClient) GetBalance() (map[string]float64, map[string]float64, error) {
	localbalance := map[string]float64{}
	locallocked := map[string]float64{}
	
	member, _, err := Mc.ApiClient.PrivateApi.GetApiV2MembersAccounts(Mc.ctx, Mc.apiKey, Mc.apiSecret)
	if err != nil {
		fmt.Println(err)
		return map[string]float64{}, map[string]float64{}, errors.New("fail to get balance")
	}
	Accounts := member.Accounts
	for i := 0; i < len(Accounts); i++ {
		currency := Accounts[i].Currency
		balance, err := strconv.ParseFloat(Accounts[i].Balance, 64)
		if err != nil {
			LogFatalToDailyLogFile(err)
			return map[string]float64{}, map[string]float64{}, errors.New("fail to parse balance to float64")
		}
		locked, err := strconv.ParseFloat(Accounts[i].Locked, 64)
		if err != nil {
			LogFatalToDailyLogFile(err)
			return map[string]float64{}, map[string]float64{}, errors.New("fail to parse locked balance to float64")
		}

		localbalance[currency] = balance
		locallocked[currency] = locked
	}

	return localbalance, locallocked, nil
}

func (Mc *MaxClient) GetOrders(market string) (map[int32]WsOrder, error) {
	orders, _, err := Mc.ApiClient.PrivateApi.GetApiV2Orders(Mc.ctx, Mc.apiKey, Mc.apiSecret, market, nil)
	if err != nil {
		return map[int32]WsOrder{}, errors.New("fail to get order list")
	}

	wsOrders := map[int32]WsOrder{}
	for i := 0; i < len(orders); i++ {
		order := WsOrder(orders[i])
		wsOrders[order.Id] = order
	}

	return wsOrders, nil
}

func (Mc *MaxClient) MarketsGolval2Local() error {
	Mc.MarketsMutex.Lock()
	defer Mc.MarketsMutex.Unlock()
	markets, _, err := Mc.ApiClient.PublicApi.GetApiV2Markets(Mc.ctx)
	if err != nil {
		return errors.New("fail to get market")
	}
	Mc.Markets = markets
	return nil
}

// GET account and sent the balance to the local balance
func (Mc *MaxClient) BalanceGlobal2Local() error {
	Member, err := Mc.GetAccount()
	if err != nil {
		LogFatalToDailyLogFile(err)
	}

	Mc.LocalBalanceMutex.Lock()
	defer Mc.LocalBalanceMutex.Unlock()

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

		Mc.LocalBalance[currency] = balance
		Mc.LocalLocked[currency] = locked
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

	Mc.LimitOrdersMutex.Lock()
	defer Mc.LimitOrdersMutex.Unlock()
	Mc.LimitOrders = newOrders
	return nil
}

func (Mc *MaxClient) DetectFilledOrders() map[string]HedgingOrder {
	hedgingOrders := map[string]HedgingOrder{}
	if len(Mc.FilledOrders) == 0 {
		return hedgingOrders
	}

	Mc.FilledOrdersMutex.Lock()
	defer Mc.FilledOrdersMutex.Unlock()

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
			LogErrorToDailyLogFile(err, order.Price, "is the element can't be convert")
		}
		volume, err := strconv.ParseFloat(order.ExecutedVolume, 64)
		if err != nil {
			LogErrorToDailyLogFile(err, order.ExecutedVolume, "is the element can't be convert")
		}
		if volume > preEv {
			volume = volume - preEv
		}

		s := 1.
		if order.Side == "buy" || order.Side == "bid" {
			s = -1.
		}

		timestamp := int32(time.Now().UnixMilli())

		if hegingOrder, ok := hedgingOrders[market]; ok {
			hegingOrder.PartialProfit += price * volume * s
			hegingOrder.TotalVolume += volume * s
		} else {
			base, quote, err := Mc.checkBaseQuote(market)
			if err != nil {
				LogFatalToDailyLogFile(err)
			}
			ho := HedgingOrder{
				Market:        market,
				Base:          base,
				Quote:         quote,
				PartialProfit: price * volume * s,
				TotalVolume:   volume * s,
				Timestamp:     timestamp,
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

	for _, hedgingOrder := range hedgingOrders {
		if hedgingOrder.TotalVolume != 0 {
			hedgingOrder.State = "wait"
			if hedgingOrder.TotalVolume > 0 {
				hedgingOrder.HedgeSide = "sell"
				hedgingOrder.HedgeVolume = hedgingOrder.TotalVolume
			} else {
				hedgingOrder.HedgeSide = "buy"
				hedgingOrder.HedgeVolume = hedgingOrder.TotalVolume * -1
			}
		} else {
			hedgingOrder.State = "done"
			hedgingOrder.TotalProfit = hedgingOrder.PartialProfit
		}
	}

	Mc.FilledOrders = map[int32]WsOrder{}
	return hedgingOrders
}

type HedgingOrder struct {
	// order
	Market        string
	Base          string
	Quote         string
	PartialProfit float64
	TotalVolume   float64
	Timestamp     int32

	// hedging state
	State       string
	TotalProfit float64

	// the order which should be sent
	HedgeVolume float64
	HedgeSide   string
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
	Mc.LocalBalanceMutex.RLock()
	defer Mc.LocalBalanceMutex.RUnlock()

	base, quote, err := Mc.checkBaseQuote(market)
	if err != nil {
		LogErrorToDailyLogFile(err)
		return false
	}
	switch side {
	case "sell":
		baseBalance := Mc.LocalBalance[base]
		if baseBalance > volume {
			enough = true
		}
	case "buy":
		needed := price * volume
		quoteBalance := Mc.LocalBalance[quote]
		if quoteBalance >= needed {
			enough = true
		}
	} // end switch
	return
}

func (Mc *MaxClient) updateLocalBalance(market, side string, price, volume float64, gain bool) error {
	Mc.LocalBalanceMutex.Lock()
	defer Mc.LocalBalanceMutex.Unlock()
	base, quote, err := Mc.checkBaseQuote(market)
	if err != nil {
		LogFatalToDailyLogFile(err)
		return errors.New("fail to update local balance")
	}
	switch side {
	case "sell":
		if gain {
			Mc.LocalBalance[base] += volume
			Mc.LocalLocked[base] -= volume
		} else {
			Mc.LocalBalance[base] -= volume
			Mc.LocalLocked[base] += volume

		}
	case "buy":
		needed := price * volume
		if gain {
			Mc.LocalBalance[quote] += needed
			Mc.LocalLocked[quote] -= needed
		} else {
			Mc.LocalBalance[quote] -= needed
			Mc.LocalLocked[quote] += needed
		}
	}
	return nil
}
