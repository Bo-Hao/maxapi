package maxapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"

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

	bids                       bookBranch
	asks                       bookBranch
	lastUpdatedTimestampBranch struct {
		timestamp int32
		mux       sync.RWMutex
	}
}

type bookstruct struct {
	Channcel  string     `json:"c,omitempty"`
	Event     string     `json:"e,omitempty"`
	Market    string     `json:"M,omitempty"`
	Asks      [][]string `json:"a,omitempty"`
	Bids      [][]string `json:"b,omitempty"`
	Timestamp int32      `json:"T,omitempty"`
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
	go o.maintain(ctx, symbol)
	return &o
}

func (o *OrderbookBranch) maintain(ctx context.Context, symbol string) {
	var url string = "wss://max-stream.maicoin.com/ws"

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

	NoErr:= true
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
			message := "max websocket reconnecting"
			LogInfoToDailyLogFile(message)
			NoErr = false
		}
	} // end for
	o.conn.Close()

	
	if !o.onErrBranch.onErr {
		return
	}
	o.maintain(ctx, symbol)
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
		fmt.Println(err2, "err2")
		return errors.New("fail to parse message")
	}
	return nil
}

func (o *OrderbookBranch) parseOrderbookUpdateMsg(msgMap map[string]interface{}) error {
	jsonbody, _ := json.Marshal(msgMap)
	var book bookstruct
	json.Unmarshal(jsonbody, &book)
	fmt.Println("update!!!")

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

	asks := book.Asks
	bids := book.Bids
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
	o.lastUpdatedTimestampBranch.timestamp = book.Timestamp
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
	if book.Event != "snapshot" {
		fmt.Println("event:", book.Event)
		return errors.New("wrong event")
	}
	if book.Market != o.Market {
		return errors.New("wrong market")
	}

	asks := book.Asks
	bids := book.Bids
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
	o.lastUpdatedTimestampBranch.timestamp = book.Timestamp
	o.lastUpdatedTimestampBranch.mux.Unlock()

	return nil
}

func updateAsks(updateAsks [][]string, oldAsks [][]string) ([][]string, error) {
	allAsks := make([][]string, 0, len(updateAsks)+len(oldAsks))
	uLen := len(updateAsks)
	oLen := len(oldAsks)

	if uLen == 0 {
		return oldAsks, nil
	}

	// sort them ascently
	sort.Slice(updateAsks, func(i, j int) bool { return updateAsks[i][0] < updateAsks[j][0] })
	sort.Slice(oldAsks, func(i, j int) bool { return oldAsks[i][0] < oldAsks[j][0] })

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
		if uIdx == uLen-1 {
			allAsks = append(allAsks, oldAsks[oIdx:]...)
			break
		}
		if oIdx == oLen-1 {
			allAsks = append(allAsks, updateAsks[uIdx:]...)
			break
		}

	}

	if len(allAsks) >= 10 {
		allAsks = allAsks[:10]
	}

	return allAsks, nil
}

func updateBids(updateBids [][]string, oldBids [][]string) ([][]string, error) {
	allBids := make([][]string, 0, len(updateBids)+len(oldBids))
	uLen := len(updateBids)
	oLen := len(oldBids)

	if uLen == 0 {
		return oldBids, nil
	}

	// sort them descently
	sort.Slice(updateBids, func(i, j int) bool { return updateBids[i][0] > updateBids[j][0] })
	sort.Slice(oldBids, func(i, j int) bool { return oldBids[i][0] > oldBids[j][0] })

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

		if uIdx == uLen-1 && oIdx == oLen-1 {
			break
		}

		if uIdx == uLen-1 {
			allBids = append(allBids, oldBids[oIdx:]...)
			break
		}
		if oIdx == oLen-1 {
			allBids = append(allBids, updateBids[uIdx:]...)
			break
		}

	}

	if len(allBids) >= 10 {
		allBids = allBids[:10]
	}

	return allBids, nil
}

func (o *OrderbookBranch) GetBids() ([][]string, bool) {
	o.bids.mux.RLock()
	defer o.bids.mux.RUnlock()

	// if there is nothing or late for 1 minute.
	if len(o.bids.Book) == 0 {
		return [][]string{}, false
	}
	book := o.bids.Book
	return book, true
}

func (o *OrderbookBranch) GetAsks() ([][]string, bool) {
	o.asks.mux.RLock()
	defer o.asks.mux.RUnlock()

	// if there is nothing or late for 1 minute.
	if len(o.asks.Book) == 0 {
		return [][]string{}, false
	}
	book := o.asks.Book
	return book, true
}
