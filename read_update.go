package maxapi

import (
	"errors"
	"strings"
)

func (Mc *MaxClient) ReadBaseOrderUnit() string {
	Mc.BaseOrderUnitBranch.RLock()
	defer Mc.BaseOrderUnitBranch.RUnlock()
	BaseOrderUnit := Mc.BaseOrderUnitBranch.BaseOrderUnit

	return BaseOrderUnit
}

func (Mc *MaxClient) ReadExchangeInfo() ExchangeInfo {
	Mc.ExchangeInfoBranch.RLock()
	E := Mc.ExchangeInfoBranch.ExInfo
	Mc.ExchangeInfoBranch.RUnlock()
	return E
}

func (Mc *MaxClient) ReadNumberOfOrder(market string) (NAsk, NBid int) {
	Mc.OrdersBranch.RLock()
	defer Mc.OrdersBranch.RUnlock()
	NOO := Mc.OrdersBranch.OrderNumbers[strings.ToLower(market)]
	NAsk = NOO.NAsk
	NBid = NOO.NBid
	return
}

func (Mc *MaxClient) ReadOrders() map[int64]WsOrder {
	Mc.OrdersBranch.RLock()
	defer Mc.OrdersBranch.RUnlock()
	return Mc.OrdersBranch.Orders
}

func (Mc *MaxClient) ReadFilledOrders() map[int64]WsOrder {
	Mc.FilledOrdersBranch.RLock()
	defer Mc.FilledOrdersBranch.RUnlock()
	return Mc.FilledOrdersBranch.Filled
}

func (Mc *MaxClient) ReadPartialFilledOrders() map[int64]WsOrder {
	Mc.FilledOrdersBranch.RLock()
	defer Mc.FilledOrdersBranch.RUnlock()
	return Mc.FilledOrdersBranch.Partial
}

func (Mc *MaxClient) ReadBalance() map[string]Balance {
	Mc.BalanceBranch.RLock()
	defer Mc.BalanceBranch.RUnlock()
	return Mc.BalanceBranch.Balance
}

func (Mc *MaxClient) ReadMarkets() []Market {
	Mc.MarketsBranch.RLock()
	defer Mc.MarketsBranch.RUnlock()
	return Mc.MarketsBranch.Markets
}

func (Mc *MaxClient) ReadTrades() []Trade {
	Mc.TradeBranch.RLock()
	defer Mc.TradeBranch.RUnlock()
	return Mc.TradeBranch.Trades
}

func (Mc *MaxClient) TakeUnhedgeTrades() []Trade {
	Mc.TradeBranch.Lock()
	defer Mc.TradeBranch.Unlock()
	unhedgeTrades := Mc.TradeBranch.UnhedgeTrades
	Mc.TradeBranch.Trades = append(Mc.TradeBranch.Trades, unhedgeTrades...)
	if len(Mc.TradeBranch.Trades) > 105 {
		Mc.TradeBranch.Trades = Mc.TradeBranch.Trades[len(Mc.TradeBranch.Trades)-105:]
	}
	Mc.TradeBranch.UnhedgeTrades = []Trade{}
	return unhedgeTrades
}

func (Mc *MaxClient) ReadUnhedgeTrades() []Trade {
	Mc.TradeBranch.RLock()
	defer Mc.TradeBranch.RUnlock()
	return Mc.TradeBranch.UnhedgeTrades
}

// update part

func (Mc *MaxClient) UpdateBaseOrderUnit(Unit string) {
	Mc.BaseOrderUnitBranch.Lock()
	defer Mc.BaseOrderUnitBranch.Unlock()
	Mc.BaseOrderUnitBranch.BaseOrderUnit = Unit
}

func (Mc *MaxClient) UpdateExchangeInfo(exInfo ExchangeInfo) {
	Mc.ExchangeInfoBranch.Lock()
	defer Mc.ExchangeInfoBranch.Unlock()
	Mc.ExchangeInfoBranch.ExInfo = exInfo
}

func (Mc *MaxClient) UpdateOrders(wsOrders map[int64]WsOrder) {
	Mc.OrdersBranch.Lock()
	Mc.OrdersBranch.Orders = wsOrders
	Mc.OrdersBranch.Unlock()

	newOrderNubmer := make(map[string]NumbersOfOrder)
	for _, v := range wsOrders {
		m := v.Market
		NBid, NAsk := 0, 0
		if strings.EqualFold(v.Side, "BUY") || strings.EqualFold(v.Side, "bid") {
			NBid = 1
		} else {
			NAsk = 1
		}
		if NOO, ok := newOrderNubmer[m]; !ok {
			newOrderNubmer[m] = NumbersOfOrder{
				NBid: NBid,
				NAsk: NAsk,
			}
		} else {
			NOO.NAsk += NAsk
			NOO.NBid += NBid
			newOrderNubmer[m] = NOO
		}
	}
	Mc.OrdersBranch.Lock()
	Mc.OrdersBranch.OrderNumbers = newOrderNubmer
	Mc.OrdersBranch.Unlock()
}

func (Mc *MaxClient) AddOrder(market string, side string, change int) {
	Mc.OrdersBranch.Lock()
	defer Mc.OrdersBranch.Unlock()
	if NOO, ok := Mc.OrdersBranch.OrderNumbers[strings.ToLower(market)]; !ok {
		if change < 0 {
			change = 0
		}
		if strings.EqualFold(side, "BUY") || strings.EqualFold(side, "bid") {
			Mc.OrdersBranch.OrderNumbers[strings.ToLower(market)] = NumbersOfOrder{
				NBid: change,
				NAsk: 0,
			}
		} else {
			Mc.OrdersBranch.OrderNumbers[strings.ToLower(market)] = NumbersOfOrder{
				NBid: 0,
				NAsk: change,
			}
		}
	} else {
		if strings.EqualFold(side, "BUY") || strings.EqualFold(side, "bid") {
			NOO.NBid += change
			if NOO.NBid < 0 {
				NOO.NBid = 0
			}
		} else {
			NOO.NAsk += change
			if NOO.NAsk < 0 {
				NOO.NAsk = 0
			}
		}
		Mc.OrdersBranch.OrderNumbers[strings.ToLower(market)] = NOO
	}
}

func (Mc *MaxClient) AddTrades(trades []Trade) {
	Mc.TradeBranch.Lock()
	defer Mc.TradeBranch.Unlock()
	Mc.TradeBranch.Trades = append(Mc.TradeBranch.Trades, trades...)
}

func (Mc *MaxClient) UpdateTrades(trades []Trade) {
	Mc.TradeBranch.Lock()
	defer Mc.TradeBranch.Unlock()
	Mc.TradeBranch.Trades = trades
}

func (Mc *MaxClient) UpdateUnhedgeTrades(unhedgetrades []Trade) {
	Mc.TradeBranch.Lock()
	defer Mc.TradeBranch.Unlock()
	Mc.TradeBranch.UnhedgeTrades = unhedgetrades
}

func (Mc *MaxClient) TradesArrived(trades []Trade) {
	Mc.TradeBranch.Lock()
	Mc.TradeBranch.UnhedgeTrades = append(Mc.TradeBranch.UnhedgeTrades, trades...)
	Mc.TradeBranch.Unlock()
}

func (Mc *MaxClient) UpdateFilledOrders(wsOrders map[int64]WsOrder) {
	Mc.FilledOrdersBranch.Lock()
	defer Mc.FilledOrdersBranch.Unlock()
	Mc.FilledOrdersBranch.Filled = wsOrders
}

func (Mc *MaxClient) UpdatePartialFilledOrders(wsOrders map[int64]WsOrder) {
	Mc.FilledOrdersBranch.Lock()
	defer Mc.FilledOrdersBranch.Unlock()
	Mc.FilledOrdersBranch.Partial = wsOrders
}

func (Mc *MaxClient) UpdateBalances(balances map[string]Balance) {
	Mc.BalanceBranch.Lock()
	defer Mc.BalanceBranch.Unlock()
	Mc.BalanceBranch.Balance = balances
}

func (Mc *MaxClient) UpdateBalance(asset string, change float64) {
	Mc.BalanceBranch.Lock()
	defer Mc.BalanceBranch.Unlock()
	b := Mc.BalanceBranch.Balance[strings.ToLower(asset)]
	b.Locked += change
	Mc.BalanceBranch.Balance[strings.ToLower(asset)] = b
}

func (Mc *MaxClient) LockBalance(asset string, lock float64) {
	Mc.BalanceBranch.Lock()
	defer Mc.BalanceBranch.Unlock()
	b := Mc.BalanceBranch.Balance[strings.ToLower(asset)]
	b.Locked += lock
	b.Avaliable -= lock
	Mc.BalanceBranch.Balance[strings.ToLower(asset)] = b
}

// ########### assistant functions ###########
func (Mc *MaxClient) checkBaseQuote(market string) (base, quote string, err error) {
	markets := Mc.ReadMarkets()
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
	Mc.BalanceBranch.RLock()
	defer Mc.BalanceBranch.RUnlock()

	base, quote, err := Mc.checkBaseQuote(market)
	if err != nil {
		LogErrorToDailyLogFile(err)
		return false
	}
	balance := Mc.ReadBalance()
	switch side {
	case "sell":
		baseBalance := balance[base].Avaliable
		if baseBalance > volume {
			enough = true
		}
	case "buy":
		needed := price * volume
		quoteBalance := balance[quote].Avaliable
		if quoteBalance >= needed {
			enough = true
		}
	} // end switch
	return
}

func (Mc *MaxClient) updateLocalBalance(market, side string, price, volume float64, gain bool) error {
	Mc.BalanceBranch.Lock()
	defer Mc.BalanceBranch.Unlock()
	base, quote, err := Mc.checkBaseQuote(market)
	if err != nil {
		LogFatalToDailyLogFile(err)
		return errors.New("fail to update local balance")
	}

	switch side {
	case "sell":
		bb := Mc.BalanceBranch.Balance[base]
		if gain {
			bb.Avaliable += volume
			bb.Locked -= volume
		} else {
			bb.Avaliable -= volume
			bb.Locked += volume
		}
		Mc.BalanceBranch.Balance[base] = bb
	case "buy":
		bq := Mc.BalanceBranch.Balance[quote]
		needed := price * volume
		if gain {
			bq.Avaliable += needed
			bq.Locked -= needed
		} else {
			bq.Avaliable -= needed
			bq.Locked += needed
		}
		Mc.BalanceBranch.Balance[quote] = bq
	}
	return nil
}
