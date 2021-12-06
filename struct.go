package maxapi

import (
	"context"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/shopspring/decimal"
)

type MaxClient struct {
	apiKey    string
	apiSecret string

	cancelFunc    *context.CancelFunc
	ShutingBranch struct {
		shut bool
		mux  sync.RWMutex
	}

	// CXMM parameters
	BaseOrderUnitBranch struct {
		BaseOrderUnit string
		mux           sync.RWMutex
	}

	// exchange information
	ExchangeInfoBranch struct {
		ExInfo ExchangeInfo
		mux    sync.RWMutex
	}

	// web socket client
	WsClient struct {
		OnErr      bool
		onErrMutex sync.RWMutex
		Conn       *websocket.Conn

		LastUpdatedIdBranch struct {
			LastUpdatedId decimal.Decimal
			mux           sync.RWMutex
		}

		TmpBranch struct {
			Trades []Trade
			Orders map[int32]WsOrder
			mux    sync.RWMutex
		}
	}

	// api client
	ApiClient *APIClient

	// limit unfilled orders
	OrdersBranch struct {
		Orders map[int32]WsOrder
		mux    sync.RWMutex
	}

	// filled orders
	FilledOrdersBranch struct {
		Filled  map[int32]WsOrder
		Partial map[int32]WsOrder
		mux     sync.RWMutex
	}

	// All markets pairs
	MarketsBranch struct {
		Markets []Market
		mux     sync.RWMutex
	}

	// Account
	AccountBranch struct {
		Account Member
		mux     sync.RWMutex
	}

	// local balance
	BalanceBranch struct {
		Balance map[string]Balance // currency balance
		mux     sync.RWMutex
	}
}

type ExchangeInfo struct {
	MinOrderUnit float64
	LimitApi     int
	CurrentNApi  int
}

type Balance struct {
	Name      string
	Avaliable float64
	Locked    float64
}


// check the hedge position
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
	MarketSide         string
	Fee                float64
	FeeCurrency        string
}
