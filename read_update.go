package maxapi

import "errors"

func (Mc *MaxClient) ReadBaseOrderUnit() string {
	Mc.BaseOrderUnitBranch.mux.RLock()
	defer Mc.BaseOrderUnitBranch.mux.RUnlock()
	BaseOrderUnit := Mc.BaseOrderUnitBranch.BaseOrderUnit

	return BaseOrderUnit
}

func (Mc *MaxClient) ReadExchangeInfo() ExchangeInfo {
	Mc.ExchangeInfoBranch.mux.RLock()
	defer Mc.ExchangeInfoBranch.mux.RUnlock()
	return Mc.ExchangeInfoBranch.ExInfo
}

func (Mc *MaxClient) ReadOrders() map[int32]WsOrder {
	Mc.OrdersBranch.mux.RLock()
	defer Mc.OrdersBranch.mux.RUnlock()
	return Mc.OrdersBranch.Orders
}

func (Mc *MaxClient) ReadFilledOrders() map[int32]WsOrder {
	Mc.FilledOrdersBranch.mux.RLock()
	defer Mc.FilledOrdersBranch.mux.RUnlock()
	return Mc.FilledOrdersBranch.Filled
}

func (Mc *MaxClient) ReadPartialFilledOrders() map[int32]WsOrder {
	Mc.FilledOrdersBranch.mux.RLock()
	defer Mc.FilledOrdersBranch.mux.RUnlock()
	return Mc.FilledOrdersBranch.Partial
}

func (Mc *MaxClient) ReadBalance() map[string]Balance {
	Mc.BalanceBranch.mux.RLock()
	defer Mc.BalanceBranch.mux.RUnlock()
	return Mc.BalanceBranch.Balance
}

func (Mc *MaxClient) ReadMarkets() []Market {
	Mc.MarketsBranch.mux.RLock()
	defer Mc.MarketsBranch.mux.RUnlock()
	return Mc.MarketsBranch.Markets
}

// update part

func (Mc *MaxClient) UpdateBaseOrderUnit(Unit string) {
	Mc.BaseOrderUnitBranch.mux.Lock()
	defer Mc.BaseOrderUnitBranch.mux.Unlock()
	Mc.BaseOrderUnitBranch.BaseOrderUnit = Unit
}

func (Mc *MaxClient) UpdateExchangeInfo(exInfo ExchangeInfo) {
	Mc.ExchangeInfoBranch.mux.Lock()
	defer Mc.ExchangeInfoBranch.mux.Unlock()
	Mc.ExchangeInfoBranch.ExInfo = exInfo
}

func (Mc *MaxClient) UpdateOrders(wsOrders map[int32]WsOrder) {
	Mc.OrdersBranch.mux.Lock()
	defer Mc.OrdersBranch.mux.Unlock()
	Mc.OrdersBranch.Orders = wsOrders
}

func (Mc *MaxClient) UpdateFilledOrders(wsOrders map[int32]WsOrder) {
	Mc.FilledOrdersBranch.mux.Lock()
	defer Mc.FilledOrdersBranch.mux.Unlock()
	Mc.FilledOrdersBranch.Filled = wsOrders
}

func (Mc *MaxClient) UpdatePartialFilledOrders(wsOrders map[int32]WsOrder) {
	Mc.FilledOrdersBranch.mux.Lock()
	defer Mc.FilledOrdersBranch.mux.Unlock()
	Mc.FilledOrdersBranch.Partial = wsOrders
}

func (Mc *MaxClient) UpdateBalance(balances map[string]Balance) {
	Mc.BalanceBranch.mux.Lock()
	defer Mc.BalanceBranch.mux.Unlock()
	Mc.BalanceBranch.Balance = balances
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
	Mc.BalanceBranch.mux.RLock()
	defer Mc.BalanceBranch.mux.RUnlock()

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
	Mc.BalanceBranch.mux.Lock()
	defer Mc.BalanceBranch.mux.Unlock()
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

