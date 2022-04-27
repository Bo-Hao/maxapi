package maxapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Bo-Hao/mapbook"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

type OrderbookBranch struct {
	cancel      *context.CancelFunc
	conn        *websocket.Conn
	onErrBranch struct {
		onErr bool
		mutex sync.RWMutex
	}
	Market string

	bids                       mapbook.BidBook
	asks                       mapbook.AskBook
	lastUpdatedTimestampBranch struct {
		timestamp int64
		mux       sync.RWMutex
	}
}

type bookstruct struct {
	Channcel  string     `json:"c,omitempty"`
	Event     string     `json:"e,omitempty"`
	Market    string     `json:"M,omitempty"`
	Asks      [][]string `json:"a,omitempty"`
	Bids      [][]string `json:"b,omitempty"`
	Timestamp int64      `json:"T,omitempty"`
}

type bookBranch struct {
	mux   sync.RWMutex
	Book  [][]string
	Micro []string
}

func SpotLocalOrderbook(symbol string, logger *logrus.Logger) *OrderbookBranch {
	var o OrderbookBranch
	ctx, cancel := context.WithCancel(context.Background())
	o.cancel = &cancel
	o.Market = strings.ToLower(symbol)
	o.asks = *mapbook.NewAskBook(false)
	o.bids = *mapbook.NewBidBook(false)
	go o.maintain(ctx, symbol)
	return &o
}

func (o *OrderbookBranch) PingIt(ctx context.Context) {
	go func() {
		for {
			time.Sleep(30 * time.Second)
			select {
			case <-ctx.Done():
				return
			default:
				message := []byte("ping")
				o.conn.WriteMessage(websocket.TextMessage, message)
			}
		}
	}()
}

func (o *OrderbookBranch) maintain(ctx context.Context, symbol string) {
	var url string = "wss://max-stream.maicoin.com/ws"

	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		LogFatalToDailyLogFile(err)
	}
	//LogInfoToDailyLogFile("Connected:", url)
	o.conn = conn
	o.onErrBranch.mutex.Lock()
	o.onErrBranch.onErr = false
	o.onErrBranch.mutex.Unlock()

	subMsg, err := MaxSubscribeBookMessage(symbol)
	if err != nil {
		LogFatalToDailyLogFile(errors.New("fail to construct subscribtion message"))
	}

	err = conn.WriteMessage(websocket.TextMessage, subMsg)
	if err != nil {
		LogFatalToDailyLogFile(errors.New("fail to subscribe websocket"))
	}

	NoErr := true
	for NoErr {
		select {
		case <-ctx.Done():
			o.conn.Close()
			return
		default:
			_, msg, err := o.conn.ReadMessage()
			if err != nil {
				LogErrorToDailyLogFile("read:", err)
				o.onErrBranch.mutex.Lock()
				o.onErrBranch.onErr = true
				o.onErrBranch.mutex.Unlock()
			}

			var msgMap map[string]interface{}
			err = json.Unmarshal(msg, &msgMap)
			if err != nil {
				LogWarningToDailyLogFile(err)
				o.onErrBranch.mutex.Lock()
				o.onErrBranch.onErr = true
				o.onErrBranch.mutex.Unlock()
			}

			errh := o.handleMaxBookSocketMsg(msg)
			if errh != nil {
				o.onErrBranch.mutex.Lock()
				o.onErrBranch.onErr = true
				o.onErrBranch.mutex.Unlock()
			}

		} // end select

		// if there is something wrong that the WS should be reconnected.
		if o.onErrBranch.onErr {
			//message := "max websocket reconnecting"
			//LogInfoToDailyLogFile(message)
			NoErr = false
		}
		time.Sleep(time.Millisecond)
	} // end for

	o.conn.Close()

	if !o.onErrBranch.onErr {
		return
	}
	o.maintain(ctx, symbol)
}

// default for the depth 10 (max).
func MaxSubscribeBookMessage(symbol string) ([]byte, error) {
	param := make(map[string]interface{})
	param["action"] = "sub"

	var args []map[string]interface{}
	subscriptions := make(map[string]interface{})
	subscriptions["channel"] = "book"
	subscriptions["market"] = strings.ToLower(symbol)
	subscriptions["depth"] = 10
	args = append(args, subscriptions)

	param["subscriptions"] = args
	req, err := json.Marshal(param)
	if err != nil {
		return nil, err
	}
	return req, nil
}

func (o *OrderbookBranch) handleMaxBookSocketMsg(msg []byte) error {
	var msgMap map[string]interface{}
	err := json.Unmarshal(msg, &msgMap)
	if err != nil {
		LogErrorToDailyLogFile(err)
		return errors.New("fail to unmarshal message")
	}

	event, ok := msgMap["e"]
	if !ok {
		LogWarningToDailyLogFile("there is no event in message")
		return errors.New("fail to obtain message")
	}

	// distribute the msg
	var err2 error
	switch event {
	case "subscribed":
		//LogInfoToDailyLogFile("websocket subscribed")
	case "snapshot":
		err2 = o.parseOrderbookSnapshotMsg(msgMap)
	case "update":
		err2 = o.parseOrderbookUpdateMsg(msgMap)
	}

	if err2 != nil {
		fmt.Println(err2, "err2")
		return errors.New("fail to parse message")
	}
	return nil
}

func (o *OrderbookBranch) parseOrderbookUpdateMsg(msgMap map[string]interface{}) error {
	jsonbody, _ := json.Marshal(msgMap)
	var book bookstruct
	json.Unmarshal(jsonbody, &book)

	// extract data
	if book.Channcel != "book" {
		return errors.New("wrong channel")
	}
	if book.Event != "update" {
		return errors.New("wrong event")
	}
	if book.Market != o.Market {
		return errors.New("wrong market")
	}

	wrongTime := false
	o.lastUpdatedTimestampBranch.mux.Lock()

	if book.Timestamp < o.lastUpdatedTimestampBranch.timestamp {
		wrongTime = true
	} else {
		o.lastUpdatedTimestampBranch.timestamp = book.Timestamp
	}
	o.lastUpdatedTimestampBranch.mux.Unlock()

	if wrongTime {
		return nil
	}

	o.asks.Update(book.Asks)
	o.bids.Update(book.Bids)

	return nil
}

func (o *OrderbookBranch) parseOrderbookSnapshotMsg(msgMap map[string]interface{}) error {
	jsonbody, _ := json.Marshal(msgMap)
	var book bookstruct
	json.Unmarshal(jsonbody, &book)

	// extract data
	if book.Channcel != "book" {
		return errors.New("wrong channel")
	}
	if book.Event != "snapshot" {
		fmt.Println("event:", book.Event)
		return errors.New("wrong event")
	}
	if book.Market != o.Market {
		return errors.New("wrong market")
	}

	o.asks.Snapshot(book.Asks)
	o.bids.Snapshot(book.Bids)

	o.lastUpdatedTimestampBranch.mux.Lock()
	o.lastUpdatedTimestampBranch.timestamp = book.Timestamp
	o.lastUpdatedTimestampBranch.mux.Unlock()

	return nil
}

func (o *OrderbookBranch) GetBids() ([][]string, bool) {
	return o.bids.GetAll()
}

func (o *OrderbookBranch) GetAsks() ([][]string, bool) {
	return o.asks.GetAll()
}

func (o *OrderbookBranch) Close() {
	o.conn.Close()
	(*o.cancel)()
}
