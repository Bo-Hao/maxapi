package maxapi

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

type OrderbookBranch struct {
	ctx         context.Context
	cancel      *context.CancelFunc
	conn        *websocket.Conn
	onErrBranch struct {
		onErr bool
		mutex sync.RWMutex
	}
	Market string

	bids                       bookBranch
	asks                       bookBranch
	lastUpdatedTimestampBranch struct {
		timestamp int32
		mux       sync.RWMutex
	}
}

type bookstruct struct {
	Channcel  string     `json:"c,omitempty"`
	event     string     `json:"e,omitempty"`
	market    string     `json:"M,omitempty"`
	asks      [][]string `json:"a,omitempty"`
	bids      [][]string `json:"b,omitempty"`
	timestamp int32      `json:"T,omitempty"`
}

type bookBranch struct {
	mux   sync.RWMutex
	Book  [][]string
	Micro []string
}

func SpotLocalOrderbook(symbol string, logger *logrus.Logger) *OrderbookBranch {
	var o OrderbookBranch
	o.Market = strings.ToLower(symbol)
	go o.maintain(symbol)
	return &o
}

func (o *OrderbookBranch) maintain(symbol string) {
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
		o.maintain(symbol)
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
		LogInfoToDailyLogFile("websocket subscribed")
	case "snapshot":
		err2 = o.parseOrderbookSnapshotMsg(msgMap)
	case "update":
		err2 = o.parseOrderbookUpdateMsg(msgMap)
	}

	if err2 != nil {
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
	if book.event != "update" {
		return errors.New("wrong event")
	}
	if book.market != o.Market {
		return errors.New("wrong market")
	}

	asks := book.asks
	bids := book.bids
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		oldAsks := o.asks.Book
		newAsks, err := updateAsks(asks, oldAsks)
		if err != nil {
			newAsks = oldAsks
		}
		o.asks.mux.Lock()
		o.asks.Book = newAsks
		o.asks.mux.Unlock()
		wg.Done()
	}()

	go func() {
		oldBids := o.bids.Book
		newBids, err := updateBids(bids, oldBids)
		if err != nil {
			newBids = oldBids
		}
		o.bids.mux.Lock()
		o.bids.Book = newBids
		o.bids.mux.Unlock()
		wg.Done()
	}()
	wg.Wait()

	o.lastUpdatedTimestampBranch.mux.Lock()
	o.lastUpdatedTimestampBranch.timestamp = book.timestamp
	o.lastUpdatedTimestampBranch.mux.Unlock()

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
	if book.event != "snapshot" {
		return errors.New("wrong event")
	}
	if book.market != o.Market {
		return errors.New("wrong market")
	}

	asks := book.asks
	bids := book.bids
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		sort.Slice(asks, func(i, j int) bool { return asks[i][0] < asks[j][0] })
		o.asks.mux.Lock()
		o.asks.Book = asks
		o.asks.mux.Unlock()
		wg.Done()
	}()

	go func() {
		sort.Slice(bids, func(i, j int) bool { return bids[i][0] > bids[j][0] })
		o.bids.mux.Lock()
		o.bids.Book = bids
		o.bids.mux.Unlock()
		wg.Done()
	}()
	wg.Wait()

	o.lastUpdatedTimestampBranch.mux.Lock()
	o.lastUpdatedTimestampBranch.timestamp = book.timestamp
	o.lastUpdatedTimestampBranch.mux.Unlock()

	return nil
}

func updateAsks(updateAsks [][]string, oldAsks [][]string) ([][]string, error) {
	// sort them ascently
	sort.Slice(updateAsks, func(i, j int) bool { return updateAsks[i][0] < updateAsks[j][0] })
	sort.Slice(oldAsks, func(i, j int) bool { return oldAsks[i][0] < oldAsks[j][0] })

	allAsks := make([][]string, 0, len(updateAsks)+len(oldAsks))
	uLen := len(updateAsks)
	oLen := len(oldAsks)

	uIdx := 0
	oIdx := 0
	for {
		uAsk := updateAsks[uIdx]
		oAsk := oldAsks[oIdx]

		if uAsk[0] == oAsk[0] {
			if uAsk[1] != "0" {
				allAsks = append(allAsks, uAsk)
			}
			uIdx++
			oIdx++
		} else {
			uP, err := strconv.ParseFloat(uAsk[0], 64)
			if err != nil {
				return [][]string{}, errors.New("fail to parse float64")
			}
			oP, err := strconv.ParseFloat(oAsk[0], 64)
			if err != nil {
				return [][]string{}, errors.New("fail to parse float64")
			}

			if uP > oP {
				allAsks = append(allAsks, oAsk)
				oIdx++
			} else if uP < oP {
				allAsks = append(allAsks, uAsk)
				uIdx++
			}
		}

		if uIdx >= uLen-1 && oIdx >= oLen-1 {
			break
		}

	}

	return allAsks, nil
}

func updateBids(updateBids [][]string, oldBids [][]string) ([][]string, error) {
	// sort them descently
	sort.Slice(updateBids, func(i, j int) bool { return updateBids[i][0] > updateBids[j][0] })
	sort.Slice(oldBids, func(i, j int) bool { return oldBids[i][0] > oldBids[j][0] })

	allBids := make([][]string, 0, len(updateBids)+len(oldBids))
	uLen := len(updateBids)
	oLen := len(oldBids)

	uIdx := 0
	oIdx := 0
	for {
		uBid := updateBids[uIdx]
		oBid := oldBids[oIdx]

		if uBid[0] == oBid[0] {
			if uBid[1] != "0" {
				allBids = append(allBids, uBid)
			}
			uIdx++
			oIdx++
		} else {
			uP, err := strconv.ParseFloat(uBid[0], 64)
			if err != nil {
				return [][]string{}, errors.New("fail to parse float64")
			}
			oP, err := strconv.ParseFloat(oBid[0], 64)
			if err != nil {
				return [][]string{}, errors.New("fail to parse float64")
			}

			if uP < oP {
				allBids = append(allBids, oBid)
				oIdx++
			} else if uP > oP {
				allBids = append(allBids, uBid)
				uIdx++
			}
		}

		if uIdx >= uLen-1 && oIdx >= oLen-1 {
			break
		}

	}

	return allBids, nil
}

func (o *OrderbookBranch) GetBids() ([][]string, bool) {
	o.bids.mux.RLock()
	defer o.bids.mux.RUnlock()

	o.lastUpdatedTimestampBranch.mux.RLock()
	lastT := o.lastUpdatedTimestampBranch.timestamp
	o.lastUpdatedTimestampBranch.mux.RUnlock()

	nowT := int32(time.Now().UnixMilli())

	// if there is nothing or late for 1 minute.
	if len(o.bids.Book) == 0 || nowT-lastT > 1000*60 {
		return [][]string{}, false
	}
	book := o.bids.Book
	return book, true
}

func (o *OrderbookBranch) GetAsks() ([][]string, bool) {
	o.asks.mux.RLock()
	defer o.asks.mux.RUnlock()

	o.lastUpdatedTimestampBranch.mux.RLock()
	lastT := o.lastUpdatedTimestampBranch.timestamp
	o.lastUpdatedTimestampBranch.mux.RUnlock()

	nowT := int32(time.Now().UnixMilli())

	// if there is nothing or late for 1 minute.
	if len(o.asks.Book) == 0 || nowT-lastT > 1000*60 {
		return [][]string{}, false
	}
	book := o.asks.Book
	return book, true
}
