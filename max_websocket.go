package maxapi

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"reflect"
	"strconv"
	"time"

	"github.com/gorilla/websocket"
)

func (Mc *MaxClient) SubscribeWS() {
	var url string = "wss://max-stream.maicoin.com/ws"
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		log.Print(err)
	}
	log.Println("Connected:", url)

	subMsg, err := GetMaxSubscribePrivateMessage(Mc.apiKey, Mc.apiSecret)
	if err != nil {
		log.Print(errors.New("fail to construct subscribtion message"))
	}

	err = conn.WriteMessage(websocket.TextMessage, subMsg)
	if err != nil {
		log.Print(errors.New("fail to subscribe websocket"))
	}
	Mc.WsClient.Conn = conn
	Mc.WsClient.OnErr = false
	defer conn.Close()

	// mainloop
	for {
		select {
		case <-Mc.ctx.Done():
			fmt.Println("stop")
			Mc.CancelAllOrders()
			Mc.WsClient.Conn.Close()
			return
		default:
			if Mc.WsClient.Conn == nil {
				Mc.WsClient.OnErr = true
				message := "max websocket reconnecting"
				log.Print(message)
			}

			_, msg, err := conn.ReadMessage()
			if err != nil {
				log.Println("read:", err)
				Mc.WsClient.OnErr = true
				message := "max websocket reconnecting"
				log.Print(message)
			}

			var msgMap map[string]interface{}
			err = json.Unmarshal(msg, &msgMap)
			if err != nil {
				log.Print(err)
				Mc.WsClient.OnErr = true
			}

			errh := Mc.handleMAXSocketMsg(msg)
			if errh != nil {
				Mc.WsClient.OnErr = true
				message := "max websocket reconnecting"
				log.Print(message)
			}
			time.Sleep(1 * time.Second)
		} // end select

		// if there is something wrong that the WS should be reconnected.
		if Mc.WsClient.OnErr {
			break
		}
	} // end for

	Mc.WsClient.Conn.Close()
	// if it is manual work.
	if Mc.WsClient.OnErr {
		Mc.WsClient.TmpOrdersMutex.Lock()
		Mc.WsClient.TmpOrders = Mc.LimitOrders
		Mc.WsClient.TmpOrdersMutex.Unlock()
		Mc.SubscribeWS()
	}
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

func (Mc *MaxClient) handleMAXSocketMsg(msg []byte) error {
	var msgMap map[string]interface{}
	err := json.Unmarshal(msg, &msgMap)
	if err != nil {
		log.Print(err)
		return errors.New("fail to unmarshal message")
	}

	event, ok := msgMap["e"]
	if !ok {
		log.Print("there is no event in message")
		return errors.New("fail to obtain message")
	}

	// distribute the msg
	var err2 error
	switch event {
	case "authenticated":
		log.Print("websocket subscribtion authenticated")
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

	Mc.LimitOrdersMutex.Lock()
	defer Mc.LimitOrdersMutex.Unlock()
	Mc.DoneOrdersMutex.Lock()
	defer Mc.DoneOrdersMutex.Unlock()

	for i := 0; i < len(wsOrders); i++ {
		if _, ok := Mc.LimitOrders[wsOrders[i].Id]; !ok {
			Mc.LimitOrders[wsOrders[i].Id] = wsOrders[i]
			fmt.Println("new order arrived: ", wsOrders[i])
		} else {
			switch wsOrders[i].State {
			case "cancel":
				delete(Mc.LimitOrders, wsOrders[i].Id)
			case "done":
				Mc.DoneOrders[wsOrders[i].Id] = wsOrders[i]
				delete(Mc.LimitOrders, wsOrders[i].Id)
			default:

				fmt.Println(wsOrders[i], " waiting...")
				if _, ok := Mc.LimitOrders[wsOrders[i].Id]; !ok {
					Mc.LimitOrders[wsOrders[i].Id] = wsOrders[i]
				}
			}
		}
	}
	return nil
}

// Trade
//	trade_snapshot
func (Mc *MaxClient) parseTradeSnapshotMsg(msgMap map[string]interface{}) error {
	snapshotTrades := make([]Trade, 0, 100)
	switch reflect.TypeOf(msgMap["t"]).Kind() {
	case reflect.Slice:
		s := reflect.ValueOf(msgMap["t"])
		for i := 0; i < s.Len(); i++ {
			wsTrade := s.Index(i).Interface().(map[string]interface{})

			id, err := strconv.Atoi(wsTrade["i"].(string))
			if err != nil {
				return errors.New("fail to convert id from string to int")
			}

			T, err := strconv.Atoi(wsTrade["T"].(string))
			if err != nil {
				return errors.New("fail to convert T from string to int")
			}

			side, err := sellbuyTransfer(wsTrade["sd"].(string))
			if err != nil {
				return err
			}
			maker := true
			if wsTrade["m"] == "false" {
				maker = false
			}

			newTrade := Trade{
				Id:          int32(id),
				Price:       wsTrade["p"].(string),
				Volume:      wsTrade["v"].(string),
				Market:      wsTrade["M"].(string),
				Timestamp:   int32(T),
				Side:        side,
				Fee:         wsTrade["f"].(string),
				FeeCurrency: wsTrade["fc"].(string),
				Maker:       maker,
			}
			snapshotTrades = append(snapshotTrades, newTrade)
		} // end for
	} // end switch

	// checking trades situation.
	/* err := Mc.trackingTrades(snapshotTrades)
	if err != nil {
		log.Print("fail to check the trades during disconnection")
	} */

	return nil
}

func (Mc *MaxClient) trackingOrders(snapshotWsOrders map[int32]WsOrder) error {
	Mc.LimitOrdersMutex.Lock()
	defer Mc.LimitOrdersMutex.Unlock()
	Mc.WsClient.TmpOrdersMutex.Lock()
	defer Mc.WsClient.TmpOrdersMutex.Unlock()

	// if there is not orders in the tmp memory, it is not possible to track the trades during WS is disconnected.
	if len(Mc.WsClient.TmpOrders) == 0 {
		Mc.LimitOrders = snapshotWsOrders
		return nil
	}

	untrackedWsOrders := map[int32]WsOrder{}
	trackedWsOrders := map[int32]WsOrder{}

	for wsorderId, wsorder := range Mc.WsClient.TmpOrders {
		if _, ok := snapshotWsOrders[wsorderId]; ok && wsorder.State != "Done" {
			trackedWsOrders[wsorderId] = wsorder
		} else {
			untrackedWsOrders[wsorderId] = wsorder
		}
	}

	Mc.LimitOrders = trackedWsOrders

	Mc.DoneOrdersMutex.Lock()
	defer Mc.DoneOrdersMutex.Unlock()
	for id, odr := range untrackedWsOrders {
		if _, ok := Mc.DoneOrders[id]; !ok {
			Mc.DoneOrders[id] = odr
		}
	}

	return nil
}

//	trade_update
func (Mc *MaxClient) parseTradeUpdateMsg(msgMap map[string]interface{}) error {
	var newTrades []Trade
	switch reflect.TypeOf(msgMap["t"]).Kind() {
	case reflect.Slice:
		s := reflect.ValueOf(msgMap["t"])
		for i := 0; i < s.Len(); i++ {
			wsTrade := s.Index(i).Interface().(map[string]interface{})
			id, err := strconv.Atoi(wsTrade["i"].(string))
			if err != nil {
				return errors.New("fail to convert id from string to int")
			}

			T, err := strconv.Atoi(wsTrade["T"].(string))
			if err != nil {
				return errors.New("fail to convert T from string to int")
			}

			side, err := sellbuyTransfer(wsTrade["sd"].(string))
			if err != nil {
				return err
			}
			maker := true
			if wsTrade["m"] == "false" {
				maker = false
			}

			newTrade := Trade{
				Id:          int32(id),
				Price:       wsTrade["p"].(string),
				Volume:      wsTrade["v"].(string),
				Market:      wsTrade["M"].(string),
				Timestamp:   int32(T),
				Side:        side,
				Fee:         wsTrade["f"].(string),
				FeeCurrency: wsTrade["fc"].(string),
				Maker:       maker,
			}
			newTrades = append(newTrades, newTrade)
		} // end for
	} // end switch

	//Mc.UnhedgedTrades = append(Mc.UnhedgedTrades, newTrades...)
	return nil
}

type Trade struct {
	Id          int32
	Price       string
	Volume      string
	Market      string
	Timestamp   int32
	Side        string
	Fee         string
	FeeCurrency string
	Maker       bool
}

// Account
//	account_snapshot and //	account_update
func (Mc *MaxClient) parseAccountMsg(msgMap map[string]interface{}) error {
	Mc.LocalBalanceMutex.Lock()
	defer Mc.LocalBalanceMutex.Unlock()
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

			Mc.LocalBalance[wsCurrency["cu"].(string)] = wsBalance
			Mc.LocalLocked[wsCurrency["cu"].(string)] = wsLocked
		} // end for
	} // end switch

	fmt.Println("WS bottom:", Mc.LocalBalance)
	return nil
}

func sellbuyTransfer(side string) (string, error) {
	switch side {
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
