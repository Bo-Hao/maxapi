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
	"time"

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
	bidsMap := make(map[string]string)
	for i := 0; i < len(updateAsks); i ++{
		price := updateAsks[i][0]
		volume := updateAsks[i][1]
		bidsMap[price] = volume
	}

	for i := 0; i < len(oldAsks); i ++{
		price := oldAsks[i][0]
		volume := oldAsks[i][1]
		if _, ok := bidsMap[price]; !ok {
			bidsMap[price]= volume
		}else{
			if volume == "0"{
				delete(bidsMap, price)
			}else {
				bidsMap[price] = volume
			}	
		}
	}

	for k, v := range bidsMap{
		allAsks  = append(allAsks, []string{k, v})
	}

	// sort them ascently
	sort.Slice(allAsks, func(i, j int) bool { 
		pi, _ := strconv.ParseFloat(allAsks[i][0], 64)
		pj, _ := strconv.ParseFloat(allAsks[j][0], 64)
		return pi < pj
	})

	if len(allAsks) >= 10 {
		allAsks = allAsks[:10]
	}

	return allAsks, nil
}

func updateBids(updateBids [][]string, oldBids [][]string) ([][]string, error) {
	allBids := make([][]string, 0, len(updateBids)+len(oldBids))
	bidsMap := make(map[string]string)
	for i := 0; i < len(updateBids); i ++{
		price := updateBids[i][0]
		volume := updateBids[i][1]
		bidsMap[price] = volume
	}

	for i := 0; i < len(oldBids); i ++{
		price := oldBids[i][0]
		volume := oldBids[i][1]
		if _, ok := bidsMap[price]; !ok {
			bidsMap[price]= volume
		}else{
			if volume == "0"{
				delete(bidsMap, price)
			}else {
				bidsMap[price] = volume
			}	
		}
	}

	for k, v := range bidsMap{
		allBids  = append(allBids, []string{k, v})
	}

	sort.Slice(allBids, func(i, j int) bool { 
		pi, _ := strconv.ParseFloat(allBids[i][0], 64)
		pj, _ := strconv.ParseFloat(allBids[j][0], 64)
		return pi > pj
	})


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
