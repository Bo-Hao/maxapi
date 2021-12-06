package maxapi

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

func (Mc *MaxClient) PriviateWebsocket() {
	var url string = "wss://max-stream.maicoin.com/ws"
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		LogFatalToDailyLogFile(err)
	}
	LogInfoToDailyLogFile("Connected:", url)

	subMsg, err := GetMaxSubscribePrivateMessage(Mc.apiKey, Mc.apiSecret)
	if err != nil {
		LogFatalToDailyLogFile(errors.New("fail to construct subscribtion message"))
	}

	err = conn.WriteMessage(websocket.TextMessage, subMsg)
	if err != nil {
		LogFatalToDailyLogFile(errors.New("fail to subscribe websocket"))
	}
	Mc.WsClient.Conn = conn
	Mc.WsClient.OnErr = false
	defer conn.Close()

	// mainloop
	mainloop:
	for {
		select {
		case <-Mc.ctx.Done():
			Mc.WsClient.OnErr = false
			Mc.ShutDown()
			return
		default:
			Mc.WsClient.onErrMutex.Lock()
			if Mc.WsClient.Conn == nil {
				Mc.WsClient.OnErr = true
				message := "max websocket reconnecting"
				LogInfoToDailyLogFile(message)
			}

			_, msg, err := conn.ReadMessage()
			if err != nil {
				LogErrorToDailyLogFile("read:", err)
				Mc.WsClient.OnErr = true
				message := "max websocket reconnecting"
				LogInfoToDailyLogFile(message)
			}

			var msgMap map[string]interface{}
			err = json.Unmarshal(msg, &msgMap)
			if err != nil {
				LogWarningToDailyLogFile(err)
				Mc.WsClient.OnErr = true
			}

			errh := Mc.handleMaxSocketMsg(msg)
			if errh != nil {
				Mc.WsClient.OnErr = true
				message := "max websocket reconnecting"
				LogInfoToDailyLogFile(message)
			}
			Mc.WsClient.onErrMutex.Unlock()
		} // end select

		// if there is something wrong that the WS should be reconnected.
		if Mc.WsClient.OnErr {
			break mainloop
		}
	} // end for

	Mc.WsClient.Conn.Close()
	// if it is manual work.
	Mc.WsClient.onErrMutex.RLock()
	if Mc.WsClient.OnErr {
		Mc.WsClient.TmpBranch.mux.Lock()
		Mc.WsClient.TmpBranch.Orders = Mc.ReadOrders()
		Mc.WsClient.TmpBranch.mux.Unlock()
		Mc.PriviateWebsocket()
	}
	Mc.WsClient.onErrMutex.RUnlock()
}

func GetMaxSubscribeMessage(product, channel string, symbols []string) ([]byte, error) {
	param := make(map[string]interface{})
	param["action"] = "sub"

	var args []map[string]interface{}
	for _, symbol := range symbols {
		subscriptions := make(map[string]interface{})
		subscriptions["channel"] = channel
		subscriptions["market"] = symbol
		subscriptions["depth"] = 1
		args = append(args, subscriptions)
	}

	param["subscriptions"] = args
	req, err := json.Marshal(param)
	if err != nil {
		return nil, err
	}
	return req, nil
}

// provide private subscribtion message.
func GetMaxSubscribePrivateMessage(apikey, apisecret string) ([]byte, error) {
	// making signature
	h := hmac.New(sha256.New, []byte(apisecret))
	nonce := time.Now().UnixMilli()               // millisecond.
	h.Write([]byte(strconv.FormatInt(nonce, 10))) // int64 to string.
	signature := hex.EncodeToString(h.Sum(nil))

	// prepare authentication message.
	param := make(map[string]interface{})
	param["action"] = "auth"
	param["apiKey"] = apikey
	param["nonce"] = nonce
	param["signature"] = signature
	//param["filters"] = []string{filter}
	param["id"] = "User"

	req, err := json.Marshal(param)
	if err != nil {
		return nil, err
	}
	return req, nil
}

// ##### #####

func (Mc *MaxClient) handleMaxSocketMsg(msg []byte) error {
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
	case "authenticated":
		LogInfoToDailyLogFile("websocket subscribtion authenticated")
	case "order_snapshot":
		err2 = Mc.parseOrderSnapshotMsg(msgMap)
	case "trade_snapshot":
		err2 = Mc.parseTradeSnapshotMsg(msgMap)
	case "account_snapshot":
		err2 = Mc.parseAccountMsg(msgMap)
	case "order_update":
		err2 = Mc.parseOrderUpdateMsg(msgMap)
	case "trade_update":
		err2 = Mc.parseTradeUpdateMsg(msgMap)
	case "account_update":
		err2 = Mc.parseAccountMsg(msgMap)
	default:
		err2 = errors.New("event not exist")
	}
	if err2 != nil {
		return errors.New("fail to parse message")
	}
	return nil
}

// Order
//	order_snapshot
func (Mc *MaxClient) parseOrderSnapshotMsg(msgMap map[string]interface{}) error {
	snapshotWsOrders := map[int32]WsOrder{}
	jsonbody, _ := json.Marshal(msgMap["o"])
	var wsOrders []WsOrder
	json.Unmarshal(jsonbody, &wsOrders)

	for i := 0; i < len(wsOrders); i++ {
		snapshotWsOrders[wsOrders[i].Id] = wsOrders[i]
	}

	// checking trades situation.
	err := Mc.trackingOrders(snapshotWsOrders)
	if err != nil {
		log.Print("fail to check the trades during disconnection")
	}

	Mc.trackingOrders(snapshotWsOrders)
	return nil
}

//	order_update
func (Mc *MaxClient) parseOrderUpdateMsg(msgMap map[string]interface{}) error {
	jsonbody, _ := json.Marshal(msgMap["o"])
	var wsOrders []WsOrder
	json.Unmarshal(jsonbody, &wsOrders)

	Mc.OrdersBranch.mux.Lock()
	defer Mc.OrdersBranch.mux.Unlock()
	Mc.FilledOrdersBranch.mux.Lock()
	defer Mc.FilledOrdersBranch.mux.Unlock()

	for i := 0; i < len(wsOrders); i++ {
		if _, ok := Mc.OrdersBranch.Orders[wsOrders[i].Id]; !ok {
			Mc.OrdersBranch.Orders[wsOrders[i].Id] = wsOrders[i]
			//fmt.Println("new order arrived: ", wsOrders[i])
		} else {
			switch wsOrders[i].State {
			case "cancel":
				//fmt.Println("order canceled: ", wsOrders[i])
				delete(Mc.OrdersBranch.Orders, wsOrders[i].Id)
			case "done":
				//fmt.Println("order done: ", wsOrders[i])
				Mc.FilledOrdersBranch.Filled[wsOrders[i].Id] = wsOrders[i]
				delete(Mc.OrdersBranch.Orders, wsOrders[i].Id)
			default:
				//fmt.Println("order partial fill: ", wsOrders[i])
				if _, ok := Mc.OrdersBranch.Orders[wsOrders[i].Id]; !ok {
					Mc.OrdersBranch.Orders[wsOrders[i].Id] = wsOrders[i]
				}
				Mc.FilledOrdersBranch.Filled[wsOrders[i].Id] = wsOrders[i]
			}
		}
	}
	return nil
}

// Trade
//	trade_snapshot
func (Mc *MaxClient) parseTradeSnapshotMsg(msgMap map[string]interface{}) error {
	jsonbody, _ := json.Marshal(msgMap["t"])
	var newTrades []Trade
	json.Unmarshal(jsonbody, &newTrades)

	return nil
}

func (Mc *MaxClient) trackingOrders(snapshotWsOrders map[int32]WsOrder) error {
	Mc.OrdersBranch.mux.Lock()
	defer Mc.OrdersBranch.mux.Unlock()
	Mc.WsClient.TmpBranch.mux.Lock()
	defer Mc.WsClient.TmpBranch.mux.Unlock()

	// if there is not orders in the tmp memory, it is not possible to track the trades during WS is disconnected.
	if len(Mc.WsClient.TmpBranch.Orders) == 0 {
		Mc.UpdateOrders(snapshotWsOrders)
		return nil
	}

	untrackedWsOrders := map[int32]WsOrder{}
	trackedWsOrders := map[int32]WsOrder{}

	for wsorderId, wsorder := range Mc.WsClient.TmpBranch.Orders {
		if _, ok := snapshotWsOrders[wsorderId]; ok && wsorder.State != "Done" {
			trackedWsOrders[wsorderId] = wsorder
		} else {
			untrackedWsOrders[wsorderId] = wsorder
		}
	}

	Mc.UpdateOrders(trackedWsOrders)

	Mc.FilledOrdersBranch.mux.Lock()
	defer Mc.FilledOrdersBranch.mux.Unlock()
	for id, odr := range untrackedWsOrders {
		if _, ok := Mc.FilledOrdersBranch.Filled[id]; !ok {
			Mc.FilledOrdersBranch.Filled[id] = odr
		}
	}

	return nil
}

//	trade_update
func (Mc *MaxClient) parseTradeUpdateMsg(msgMap map[string]interface{}) error {
	jsonbody, _ := json.Marshal(msgMap["t"])
	var newTrades []Trade
	json.Unmarshal(jsonbody, &newTrades)

	return nil
}

type Trade struct {
	Id          int32  `json:"i,omitempty"`
	Price       string `json:"p,omitempty"`
	Volume      string `json:"v,omitempty"`
	Market      string `json:"M,omitempty"`
	Timestamp   int32  `json:"T,omitempty"`
	Side        string `json:"sd,omitempty"`
	Fee         string `json:"f,omitempty"`
	FeeCurrency string `json:"fc,omitempty"`
	Maker       bool   `json:"m,omitempty"`
}

// Account
//	account_snapshot and //	account_update
func (Mc *MaxClient) parseAccountMsg(msgMap map[string]interface{}) error {
	Mc.BalanceBranch.mux.Lock()
	defer Mc.BalanceBranch.mux.Unlock()
	switch reflect.TypeOf(msgMap["B"]).Kind() {
	case reflect.Slice:
		s := reflect.ValueOf(msgMap["B"])
		for i := 0; i < s.Len(); i++ {
			wsCurrency := s.Index(i).Interface().(map[string]interface{})
			wsBalance, err := strconv.ParseFloat(wsCurrency["av"].(string), 64)
			if err != nil {
				wsBalance = 0.0
				return errors.New("fail to parse float")

			}

			wsLocked, err := strconv.ParseFloat(wsCurrency["l"].(string), 64)
			if err != nil {
				wsLocked = 0.0
				return errors.New("fail to parse float")
			}
			b := Balance{
				Name:      wsCurrency["cu"].(string),
				Avaliable: wsBalance,
				Locked:    wsLocked,
			}
			Mc.BalanceBranch.Balance[b.Name] = b
		} // end for
	} // end switch

	return nil
}

func sellbuyTransfer(side string) (string, error) {
	switch strings.ToLower(side) {
	case "sell":
		return "sell", nil
	case "buy":
		return "buy", nil
	case "bid":
		return "buy", nil
	case "ask":
		return "sell", nil
	}
	return "", errors.New("unrecognized side appear")
}

type WsOrder struct {
	Id              int32  `json:"i,omitempty"`
	Side            string `json:"sd,omitempty"`
	OrdType         string `json:"ot,omitempty"`
	Price           string `json:"p,omitempty"`
	StopPrice       string `json:"sp,omitempty"`
	AvgPrice        string `json:"ap,omitempty"`
	State           string `json:"S,omitempty"`
	Market          string `json:"M,omitempty"`
	CreatedAt       int32  `json:"T,omitempty"`
	Volume          string `json:"v,omitempty"`
	RemainingVolume string `json:"rv,omitempty"`
	ExecutedVolume  string `json:"ev,omitempty"`
	TradesCount     int32  `json:"tc,omitempty"`
}
