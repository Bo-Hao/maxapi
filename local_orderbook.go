package maxapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

type OrderBookBranch struct {
	ctx         context.Context
	cancel      *context.CancelFunc
	conn        *websocket.Conn
	onErrBranch struct {
		onErr bool
		mutex sync.RWMutex
	}

	bids          bookBranch
	asks          bookBranch
	lastUpdatedId int32
}

type bookBranch struct {
	mux   sync.RWMutex
	Book  [][]string
	Micro []string
}

type bookstruct struct {
	channcel  string     `json:"c,omitempty"`
	event     string     `json:"e,omitempty"`
	market    string     `json:"M,omitempty"`
	asks      [][]string `json:"a,omitempty"`
	bids      [][]string `json:"b,omitempty"`
	timestamp int32      `json:"T,omitempty"`
}

func SpotLocalOrderbook(symbol string, logger *logrus.Logger) *OrderBookBranch {
	var o OrderBookBranch
	go o.Maintain(symbol)
	return &o
}

func (o *OrderBookBranch) Maintain(symbol string) {
	var url string = "wss://max-stream.maicoin.com/ws"

	ctx, cancel := context.WithCancel(context.Background())
	o.cancel = &cancel
	o.ctx = ctx

	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		LogFatalToDailyLogFile(err)
	}
	LogInfoToDailyLogFile("Connected:", url)
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

mainloop:
	for {
		select {
		case <-o.ctx.Done():
			o.conn.Close()
			break mainloop
		default:
			_, msg, err := o.conn.ReadMessage()
			if err != nil {
				LogErrorToDailyLogFile("read:", err)
				o.onErrBranch.mutex.Lock()
				o.onErrBranch.onErr = true
				o.onErrBranch.mutex.Unlock()
				message := "max websocket reconnecting"
				LogInfoToDailyLogFile(message)
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
				message := "max websocket reconnecting"
				LogInfoToDailyLogFile(message)
			}

			time.Sleep(1 * time.Second)
		} // end select

		// if there is something wrong that the WS should be reconnected.

		o.onErrBranch.mutex.Lock()
		if o.onErrBranch.onErr {
			break
		}
		o.onErrBranch.mutex.Unlock()
	} // end for
	o.conn.Close()

	o.onErrBranch.mutex.RLock()
	if o.onErrBranch.onErr {
		o.Maintain(symbol)
	}
	o.onErrBranch.mutex.RUnlock()
}

// default for the depth 10.
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

func (o *OrderBookBranch) handleMaxBookSocketMsg(msg []byte) error {
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

	channel, ok := msgMap["c"]
	if !ok {
		LogWarningToDailyLogFile("there is no channel in message")
		return errors.New("fail to obtain message")
	}

	if channel != "book" {
		return errors.New("Not book channel")
	}

	// distribute the msg
	var err2 error
	switch event {
	case "subscribed":
		LogInfoToDailyLogFile("websocket subscribed")
	case "snapshot":
		err2 = o.parseOrderbookUpdateMsg(msgMap)
	case "update":
		err2 = o.parseOrderbookUpdateMsg(msgMap)
	}

	if err2 != nil {
		return errors.New("fail to parse message")
	}
	return nil
}

func (o *OrderBookBranch) parseOrderbookUpdateMsg(msgMap map[string]interface{}) error {
	jsonbody, _ := json.Marshal(msgMap)
	var book bookstruct
	json.Unmarshal(jsonbody, &book)

	/* Mc.limitOrdersMutex.Lock()
	defer Mc.limitOrdersMutex.Unlock()
	Mc.filledOrdersMutex.Lock()
	defer Mc.filledOrdersMutex.Unlock() */

	/* for i := 0; i < len(wsOrders); i++ {
		if _, ok := Mc.LimitOrders[wsOrders[i].Id]; !ok {
			Mc.LimitOrders[wsOrders[i].Id] = wsOrders[i]
			//fmt.Println("new order arrived: ", wsOrders[i])
		} else {
			switch wsOrders[i].State {
			case "cancel":
				//fmt.Println("order canceled: ", wsOrders[i])
				delete(Mc.LimitOrders, wsOrders[i].Id)
			case "done":
				//fmt.Println("order done: ", wsOrders[i])
				Mc.FilledOrders[wsOrders[i].Id] = wsOrders[i]
				delete(Mc.LimitOrders, wsOrders[i].Id)
			default:
				//fmt.Println("order partial fill: ", wsOrders[i])
				if _, ok := Mc.LimitOrders[wsOrders[i].Id]; !ok {
					Mc.LimitOrders[wsOrders[i].Id] = wsOrders[i]
				}
				Mc.FilledOrders[wsOrders[i].Id] = wsOrders[i]
			}
		}
	} */
	fmt.Println(book)
	return nil
}
