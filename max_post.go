package maxapi

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

func (Mc *MaxClient) GetAccount() (Member, error) {
	member, _, err := Mc.ApiClient.PrivateApi.GetApiV2MembersAccounts(context.Background(), Mc.apiKey, Mc.apiSecret)
	if err != nil {
		return Member{}, errors.New("fail to get account")
	}
	Mc.AccountBranch.Lock()
	Mc.AccountBranch.Account = member
	Mc.AccountBranch.Unlock()
	return member, nil
}

// Get balance into a map with key denote asset.
func (Mc *MaxClient) GetBalance() (map[string]Balance, error) {
	localbalance := map[string]Balance{}

	member, _, err := Mc.ApiClient.PrivateApi.GetApiV2MembersAccounts(context.Background(), Mc.apiKey, Mc.apiSecret)
	if err != nil {
		return map[string]Balance{}, err
	}
	Accounts := member.Accounts
	for i := 0; i < len(Accounts); i++ {
		currency := Accounts[i].Currency
		balance, err := strconv.ParseFloat(Accounts[i].Balance, 64)
		if err != nil {
			LogFatalToDailyLogFile(err)
			return map[string]Balance{}, errors.New("fail to parse balance to float64")
		}
		locked, err := strconv.ParseFloat(Accounts[i].Locked, 64)
		if err != nil {
			LogFatalToDailyLogFile(err)
			return map[string]Balance{}, errors.New("fail to parse locked balance to float64")
		}

		b := Balance{
			Name:      currency,
			Avaliable: balance,
			Locked:    locked,
		}
		localbalance[currency] = b
	}

	Mc.BalanceBranch.Lock()
	Mc.BalanceBranch.Balance = localbalance
	defer Mc.BalanceBranch.Unlock()

	return localbalance, nil
}

// Get orders of the coresponding $market
func (Mc *MaxClient) GetOrders(market string) (map[int64]WsOrder, error) {
	orders, _, err := Mc.ApiClient.PrivateApi.GetApiV2Orders(context.Background(), Mc.apiKey, Mc.apiSecret, market, nil)
	if err != nil {
		return map[int64]WsOrder{}, err
	}

	NBid, NAsk := 0, 0
	wsOrders := map[int64]WsOrder{}
	for i := 0; i < len(orders); i++ {
		order := WsOrder(orders[i])
		wsOrders[order.Id] = order

		if strings.EqualFold(orders[i].Side, "bid") {
			NBid += 1
		} else {
			NAsk += 1
		}
	}
	Mc.OrdersBranch.Lock()
	defer Mc.OrdersBranch.Unlock()
	Mc.OrdersBranch.OrderNumbers[strings.ToLower(market)] = NumbersOfOrder{
		NBid: NBid,
		NAsk: NAsk,
	}

	oldOrders := Mc.OrdersBranch.Orders
	for key, value := range oldOrders {
		if _, ok := wsOrders[key]; !ok && strings.EqualFold(value.Market, market) {
			delete(oldOrders, key)
		} else if ok {
			oldOrders[key] = wsOrders[key]
		}
	}
	Mc.OrdersBranch.Orders = oldOrders

	return wsOrders, nil
}

// Get orders of all markets.
func (Mc *MaxClient) GetAllOrders() (map[int64]WsOrder, error) {
	newOrders := map[int64]WsOrder{}
	newOrderNumbers := make(map[string]NumbersOfOrder)

	orders, _, err := Mc.ApiClient.PrivateApi.GetApiV2Orders(context.Background(), Mc.apiKey, Mc.apiSecret, "all", nil)

	if err != nil {
		return map[int64]WsOrder{}, err
	}

	// mux
	Mc.OrdersBranch.Lock()
	defer Mc.OrdersBranch.Unlock()
	for i := 0; i < len(orders); i++ {
		newOrders[orders[i].Id] = WsOrder(orders[i])
		side := orders[i].Side
		market := orders[i].Market

		if NOO, ok := newOrderNumbers[strings.ToLower(market)]; !ok {
			if strings.EqualFold(side, "buy") {
				Mc.OrdersBranch.OrderNumbers[strings.ToLower(market)] = NumbersOfOrder{
					NBid: 1,
					NAsk: 0,
				}
			} else {
				Mc.OrdersBranch.OrderNumbers[strings.ToLower(market)] = NumbersOfOrder{
					NBid: 0,
					NAsk: 1,
				}
			}
		} else {
			if strings.EqualFold(side, "buy") {
				NOO.NBid += 1
			} else {
				NOO.NAsk += 1
			}
			Mc.OrdersBranch.OrderNumbers[strings.ToLower(market)] = NOO
		}
	}

	Mc.OrdersBranch.Orders = newOrders
	Mc.OrdersBranch.OrderNumbers = newOrderNumbers

	return newOrders, nil
}

func (Mc *MaxClient) GetMarkets() ([]Market, error) {
	Mc.MarketsBranch.RLock()
	defer Mc.MarketsBranch.RUnlock()
	markets, _, err := Mc.ApiClient.PublicApi.GetApiV2Markets(context.Background())
	if err != nil {
		return []Market{}, err
	}
	Mc.MarketsBranch.Markets = markets
	return markets, nil
}

func (Mc *MaxClient) CancelAllOrders() ([]WsOrder, error) {
	Mc.OrdersBranch.Lock()
	defer Mc.OrdersBranch.Unlock()
	canceledOrders, _, err := Mc.ApiClient.PrivateApi.PostApiV2OrdersClear(context.Background(), Mc.apiKey, Mc.apiSecret, nil)
	if err != nil {
		return []WsOrder{}, errors.New("fail to cancel all orders")
	}
	canceledWsOrders := make([]WsOrder, 0, len(canceledOrders))
	/* if len(canceledOrders) > 0 {
		LogInfoToDailyLogFile("Cancel ", len(canceledOrders), " Orders by CancelAllOrders.")
	} */

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
	Mc.UpdatePartialFilledOrders(map[int64]WsOrder{})

	return canceledWsOrders, nil
}

func (Mc *MaxClient) CancelOrder(market string, id, clientId interface{}) (wsOrder WsOrder, err error) {
	var canceledorder Order
	if clientId == nil && id == nil {
		return WsOrder{}, errors.New("no order found")
	} else if clientId == nil {
		canceledorder, _, err = Mc.ApiClient.PrivateApi.PostApiV2OrderDelete(context.Background(), Mc.apiKey, Mc.apiSecret, id.(int64))
		if err != nil {
			return WsOrder{}, errors.New("fail to cancel order" + strconv.Itoa(int(id.(int64))))
		}
		LogInfoToDailyLogFile("Cancel Order ", id, "by CancelOrder func.")
	} else if id == nil {
		canceledorder, _, err = Mc.ApiClient.PrivateApi.PostApiV2OrderDeleteClientId(context.Background(), Mc.apiKey, Mc.apiSecret, clientId.(string))
		if err != nil {
			return WsOrder{}, errors.New("fail to cancel order" + clientId.(string))
		}
		LogInfoToDailyLogFile("Cancel Order with client_id ", clientId.(string), "by CancelOrder func.")
	}

	// data update
	// local balance update
	order := canceledorder
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

	//del partial fill order those in map
	Mc.FilledOrdersBranch.Lock()
	defer Mc.FilledOrdersBranch.Unlock()
	delete(Mc.FilledOrdersBranch.Partial, order.Id)

	return WsOrder(canceledorder), nil
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
	// bug exist
	canceledOrders, _, err := Mc.ApiClient.PrivateApi.PostApiV2OrdersClear(context.Background(), Mc.apiKey, Mc.apiSecret, params)
	if err != nil {
		fmt.Println(err)
		return []WsOrder{}, err
	}
	canceledWsOrders := make([]WsOrder, 0, len(canceledOrders))

	// local balance update
	cancelOrdersKey := []int64{}
	for i := 0; i < len(canceledOrders); i++ {
		// update local orders
		wsOrder := WsOrder(canceledOrders[i])
		canceledWsOrders = append(canceledWsOrders, wsOrder)
		order := canceledOrders[i]
		cancelOrdersKey = append(cancelOrdersKey, order.Id)
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

	//del partial fill order those in map
	Mc.FilledOrdersBranch.Lock()
	defer Mc.FilledOrdersBranch.Unlock()
	for i := 0; i < len(cancelOrdersKey); i++ {
		delete(Mc.FilledOrdersBranch.Partial, cancelOrdersKey[i])
	}

	return canceledWsOrders, nil
}

func (Mc *MaxClient) PlaceLimitOrder(market string, side string, price, volume float64) (WsOrder, error) {
	/* if isEnough := Mc.checkBalanceEnoughLocal(market, side, price, volume); !isEnough {
		return WsOrder{}, errors.New("balance is not enough for trading")
	} */

	params := make(map[string]interface{})
	params["price"] = fmt.Sprint(price)
	params["ord_type"] = "limit"
	vol := fmt.Sprint(volume)

	order, _, err := Mc.ApiClient.PrivateApi.PostApiV2Orders(context.Background(), Mc.apiKey, Mc.apiSecret, market, side, vol, params)
	if err != nil {
		return WsOrder{}, err
	}

	// data update
	// local balance update
	Mc.updateLocalBalance(market, side, price, volume, false) // false means this is a normal order
	return WsOrder(order), nil
}

func (Mc *MaxClient) PlacePostOnlyOrder(market string, side string, price, volume float64) (WsOrder, error) {
	/* if isEnough := Mc.checkBalanceEnoughLocal(market, side, price, volume); !isEnough {
		return WsOrder{}, errors.New("balance is not enough for trading")
	} */

	params := make(map[string]interface{})
	params["price"] = fmt.Sprint(price)
	params["ord_type"] = "post_only"
	vol := fmt.Sprint(volume)

	order, _, err := Mc.ApiClient.PrivateApi.PostApiV2Orders(context.Background(), Mc.apiKey, Mc.apiSecret, market, side, vol, params)
	if err != nil {
		return WsOrder{}, err
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
	orders, _, err := Mc.ApiClient.PrivateApi.PostApiV2OrdersMulti(context.Background(), Mc.apiKey, Mc.apiSecret, market, sides, ordersVolume, optionalMap)
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
	params["ord_type"] = "market"
	vol := fmt.Sprint(volume)
	order, _, err := Mc.ApiClient.PrivateApi.PostApiV2Orders(context.Background(), Mc.apiKey, Mc.apiSecret, market, side, vol, params)
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

/*
func (Mc MaxClient) PlaceMakerOrder(market string, side string, volume float64) error {
	orderbook := SpotLocalOrderbook(strings.ToLower(market), logrus.New())
	time.Sleep(2 * time.Second)

	for {}
	asks, ok := orderbook.GetAsks()
	if ok {
		fmt.Println("ask")
		fmt.Println(asks)
	}

	bids, ok := orderbook.GetBids()
	if ok {
		fmt.Println("bid")
		fmt.Println(bids)
	}
}
*/

// for modularized arbitrage framework
// GetBalances() ([][]string, bool)     // []string{asset, available, total}
func (Mc *MaxClient) GetBalances() (balances [][]string, ok bool) {
	member, _, err := Mc.ApiClient.PrivateApi.GetApiV2MembersAccounts(context.Background(), Mc.apiKey, Mc.apiSecret)
	if err != nil {
		return [][]string{}, false
	}
	Accounts := member.Accounts
	for i := 0; i < len(Accounts); i++ {
		currency := Accounts[i].Currency
		available, err := strconv.ParseFloat(Accounts[i].Balance, 64)
		if err != nil {
			available = 0
		}
		locked, err := strconv.ParseFloat(Accounts[i].Locked, 64)

		if err != nil {
			locked = 0
		}

		// handle the case here.
		balances = append(balances, []string{strings.ToUpper(currency), fmt.Sprint(available), fmt.Sprint(available + locked)})
	}
	return balances, true
}

// GetOpenOrders() ([][]string, bool)   // []string{oid, symbol, product, subaccount, price, qty, side, execType, UnfilledQty}
func (Mc *MaxClient) GetOpenOrders() (openOrders [][]string, ok bool) {
	orders, _, err := Mc.ApiClient.PrivateApi.GetApiV2Orders(context.Background(), Mc.apiKey, Mc.apiSecret, "all", nil)
	if err != nil {
		return [][]string{}, false
	}

	for _, v := range orders {
		// []string{oid, symbol, product, subaccount, price, qty, side, execType, unfilledQty}
		oid := fmt.Sprint(v.Id)
		symbol := v.Market
		product := "spot"
		price := v.Price
		qty := v.Volume
		side := v.Side
		execType := v.OrdType
		unfilledQty := v.RemainingVolume

		openOrder := []string{oid, symbol, product, "", price, qty, side, execType, unfilledQty}

		openOrders = append(openOrders, openOrder)
	}

	return openOrders, true
}

// GetTradeReports() ([][]string, bool) // []string{oid, symbol, product, subaccount, price, qty, side, execType, fee, filledQty}
func (Mc *MaxClient) GetTradeReports() ([][]string, bool) {
	Mc.TradeReportBranch.Lock()
	trades := Mc.TradeReportBranch.TradeReports
	Mc.TradeReportBranch.TradeReports = []Trade{}
	Mc.TradeReportBranch.Unlock()

	var tradeReports [][]string
	for i := range trades {
		trade := trades[i]
		oid := ""
		symbol := strings.ToUpper(trade.Market)
		product := "spot"
		subaccount := ""
		price := trade.Price
		qty := trade.Volume
		side := trade.Side

		maker := trade.Maker

		execType := "limit"
		if !maker {
			execType = "market"
		}

		fee := trade.Fee
		filledQty := trade.Volume

		tradeReport := []string{oid, symbol, product, subaccount, price, qty, side, execType, fee, filledQty}

		tradeReports = append(tradeReports, tradeReport)
	}

	return tradeReports, true
}
