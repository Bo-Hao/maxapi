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
		sync.RWMutex
	}

	// CXMM parameters
	BaseOrderUnitBranch struct {
		BaseOrderUnit string
		sync.RWMutex
	}

	// exchange information
	ExchangeInfoBranch struct {
		ExInfo ExchangeInfo
		sync.RWMutex
	}

	// web socket client
	WsClient struct {
		OnErr      bool
		onErrMutex sync.RWMutex
		Conn       *websocket.Conn
		connMutex  sync.Mutex

		LastUpdatedIdBranch struct {
			LastUpdatedId decimal.Decimal
			sync.RWMutex
		}

		TmpBranch struct {
			Trades []Trade
			Orders map[int64]WsOrder
			sync.RWMutex
		}
	}

	// api client
	ApiClient *APIClient

	// limit unfilled orders
	OrdersBranch struct {
		Orders       map[int64]WsOrder
		OrderNumbers map[string]NumbersOfOrder
		sync.RWMutex
	}

	// filled orders
	FilledOrdersBranch struct {
		Filled  map[int64]WsOrder
		Partial map[int64]WsOrder
		sync.RWMutex
	}

	TradeBranch struct {
		UnhedgeTrades []Trade
		Trades        []Trade
		sync.RWMutex
	}

	// All markets pairs
	MarketsBranch struct {
		Markets []Market
		sync.RWMutex
	}

	// Account
	AccountBranch struct {
		Account Member
		sync.RWMutex
	}

	// local balance
	BalanceBranch struct {
		Balance map[string]Balance // currency balance
		sync.RWMutex
	}

	TradeReportBranch struct {
		TradeReports []Trade
		sync.RWMutex
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
	Market         string
	Base           string
	Quote          string
	Profit         float64
	Volume         float64
	Timestamp      int64
	AbsVolume      float64
	MaxFee         float64
	MaxFeeCurrency string
	MaxMaker       bool

	// hedged info
	TotalProfit        float64
	MarketTransactTime int64
	AvgPrice           float64
	TransactVolume     float64
	MarketSide         string
	Fee                float64
	FeeCurrency        string
}

type NumbersOfOrder struct {
	NBid int
	NAsk int
}

type WsOrder struct {
	Id              int64  `json:"i,omitempty"`
	Side            string `json:"sd,omitempty"`
	OrdType         string `json:"ot,omitempty"`
	Price           string `json:"p,omitempty"`
	StopPrice       string `json:"sp,omitempty"`
	AvgPrice        string `json:"ap,omitempty"`
	State           string `json:"S,omitempty"`
	Market          string `json:"M,omitempty"`
	CreatedAt       int64  `json:"T,omitempty"`
	Volume          string `json:"v,omitempty"`
	RemainingVolume string `json:"rv,omitempty"`
	ExecutedVolume  string `json:"ev,omitempty"`
	TradesCount     int64  `json:"tc,omitempty"`
}

type Trade struct {
	Id          int64  `json:"i,omitempty"`
	Price       string `json:"p,omitempty"`
	Volume      string `json:"v,omitempty"`
	Market      string `json:"M,omitempty"`
	Timestamp   int64  `json:"T,omitempty"`
	Side        string `json:"sd,omitempty"`
	Fee         string `json:"f,omitempty"`
	FeeCurrency string `json:"fc,omitempty"`
	Maker       bool   `json:"m,omitempty"`
}
