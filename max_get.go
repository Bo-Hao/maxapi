package maxapi

func (Mc *MaxClient) ReadBaseOrderUnit() string {
	Mc.baseOrderUnitMutex.RLock()
	BaseOrderUnit := Mc.BaseOrderUnit
	defer Mc.baseOrderUnitMutex.RUnlock()
	return BaseOrderUnit
}

func (Mc *MaxClient) ReadExchangeInfo() ExchangeInfo {
	Mc.exchangeInfoMutex.RLock()
	defer Mc.exchangeInfoMutex.RUnlock()
	return Mc.ExchangeInfo
}

func (Mc *MaxClient) ReadLimitOrders() map[int32]WsOrder {
	Mc.limitOrdersMutex.RLock()
	LimitOrders := Mc.LimitOrders
	defer Mc.limitOrdersMutex.RUnlock()
	return LimitOrders
}

func (Mc *MaxClient) ReadFilledOrders() map[int32]WsOrder {
	Mc.filledOrdersMutex.RLock()
	FilledOrders := Mc.FilledOrders
	defer Mc.filledOrdersMutex.RUnlock()
	return FilledOrders
}

func (Mc *MaxClient) ReadPartialFilledOrders() map[int32]WsOrder {
	Mc.filledOrdersMutex.RLock()
	PartialFilledOrders := Mc.PartialFilledOrders
	defer Mc.filledOrdersMutex.RUnlock()
	return PartialFilledOrders
}

func (Mc *MaxClient) ReadLocalBalance() map[string]Balance {
	Mc.localBalanceMutex.RLock()
	LocalBalance := Mc.LocalBalance
	defer Mc.localBalanceMutex.RUnlock()
	return LocalBalance
}

// update
func (Mc *MaxClient) UpdateBaseOrderUnit(Unit string)  {
	Mc.baseOrderUnitMutex.Lock()
	defer Mc.baseOrderUnitMutex.Unlock()
	Mc.BaseOrderUnit = Unit
}

func (Mc *MaxClient) UpdateExchangeInfo(exInfo ExchangeInfo) {
	Mc.exchangeInfoMutex.Lock()
	defer Mc.exchangeInfoMutex.Unlock()
	Mc.ExchangeInfo = exInfo
}

func (Mc *MaxClient) UpdateLimitOrders(wsOrders map[int32]WsOrder) {
	Mc.limitOrdersMutex.Lock()
	defer Mc.limitOrdersMutex.Unlock()
	Mc.LimitOrders = wsOrders
}

func (Mc *MaxClient) UpdateFilledOrders(wsOrders map[int32]WsOrder) {
	Mc.filledOrdersMutex.Lock()
	defer Mc.filledOrdersMutex.Unlock()
	Mc.FilledOrders = wsOrders
}

func (Mc *MaxClient) UpdatePartialFilledOrders(wsOrders map[int32]WsOrder) {
	Mc.filledOrdersMutex.Lock()
	defer Mc.filledOrdersMutex.Unlock()
	Mc.PartialFilledOrders = wsOrders
}

func (Mc *MaxClient) UpdateLocalBalance(balances map[string]Balance) {
	Mc.localBalanceMutex.Lock()
	defer Mc.localBalanceMutex.Unlock()
	Mc.LocalBalance = balances
}