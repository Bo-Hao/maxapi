package maxapi

func (Mc *MaxClient) GetBaseOrderUnit() string {
	Mc.baseOrderUnitMutex.RLock()
	BaseOrderUnit := Mc.BaseOrderUnit
	defer Mc.baseOrderUnitMutex.RUnlock()
	return BaseOrderUnit
}

func (Mc *MaxClient) GetExchangeInfo() ExchangeInfo {
	return Mc.ExchangeInfo
}

func (Mc *MaxClient) GetLimitOrders() map[int32]WsOrder {
	Mc.limitOrdersMutex.RLock()
	LimitOrders := Mc.LimitOrders
	defer Mc.limitOrdersMutex.RUnlock()
	return LimitOrders
}

func (Mc *MaxClient) GetFilledOrders() map[int32]WsOrder {
	Mc.filledOrdersMutex.RLock()
	FilledOrders := Mc.FilledOrders
	defer Mc.filledOrdersMutex.RUnlock()
	return FilledOrders
}

func (Mc *MaxClient) GetPartialFilledOrders() map[int32]WsOrder {
	Mc.filledOrdersMutex.RLock()
	PartialFilledOrders := Mc.PartialFilledOrders
	defer Mc.filledOrdersMutex.RUnlock()
	return PartialFilledOrders
}

func (Mc *MaxClient) GetLocalBalance() map[string]Balance {
	Mc.localBalanceMutex.RLock()
	LocalBalance := Mc.LocalBalance
	defer Mc.localBalanceMutex.RUnlock()
	return LocalBalance
}