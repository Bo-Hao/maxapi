package maxapi

import (
	"errors"
	"fmt"
	"strconv"
)

func (Mc *MaxClient) ApiGetAccount() (Member, error) {
	member, _, err := Mc.ApiClient.PrivateApi.GetApiV2MembersAccounts(Mc.ctx, Mc.apiKey, Mc.apiSecret)
	if err != nil {
		fmt.Println(err)
		return Member{}, errors.New("fail to get account")
	}
	Mc.accountMutex.Lock()
	Mc.Account = member
	Mc.accountMutex.Unlock()
	return member, nil
}

func (Mc *MaxClient) ApiGetBalance() (map[string]Balance, error) {
	localbalance := map[string]Balance{}

	member, _, err := Mc.ApiClient.PrivateApi.GetApiV2MembersAccounts(Mc.ctx, Mc.apiKey, Mc.apiSecret)
	if err != nil {
		fmt.Println(err)
		return map[string]Balance{}, errors.New("fail to get balance")
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

	return localbalance, nil
}

func (Mc *MaxClient) ApiGetOrders(market string) (map[int32]WsOrder, error) {
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

func (Mc *MaxClient) CancelAllOrders() ([]WsOrder, error) {
	Mc.limitOrdersMutex.Lock()
	defer Mc.limitOrdersMutex.Unlock()
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