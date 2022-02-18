package maxapi

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"

	"golang.org/x/oauth2"
)

var (
	jsonCheck = regexp.MustCompile("(?i:[application|text]/json)")
	xmlCheck  = regexp.MustCompile("(?i:[application|text]/xml)")
)

type contextKey string

var (
	// ContextOAuth2 takes a oauth2.TokenSource as authentication for the request.
	ContextOAuth2 = contextKey("token")

	// ContextBasicAuth takes BasicAuth as authentication for the request.
	ContextBasicAuth = contextKey("basic")

	// ContextAccessToken takes a string oauth2 access token as authentication for the request.
	ContextAccessToken = contextKey("accesstoken")

	// ContextAPIKey takes an APIKey as authentication for the request
	ContextAPIKey = contextKey("apikey")
)

// BasicAuth provides basic http authentication to a request passed via context using ContextBasicAuth
type BasicAuth struct {
	UserName string `json:"userName,omitempty"`
	Password string `json:"password,omitempty"`
}

// APIClient manages communication with the MAX RESTful API List API v1.0.0
// In most cases there should be only one, shared, APIClient.
type APIClient struct {
	cfg    *Configuration
	common service // Reuse a single struct instead of allocating one for each service on the heap.

	// API Services
	PrivateApi *PrivateApiService
	PublicApi  *PublicApiService
}

// NewAPIClient creates a new API client. Requires a userAgent string describing your application.
// optionally a custom http.Client to allow for advanced features such as caching.
func NewAPIClient(cfg *Configuration) *APIClient {
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = http.DefaultClient
	}

	c := &APIClient{}
	c.cfg = cfg
	c.common.client = c

	// API Services
	c.PrivateApi = (*PrivateApiService)(&c.common)
	c.PublicApi = (*PublicApiService)(&c.common)

	return c
}

type Configuration struct {
	BasePath      string            `json:"basePath,omitempty"`
	Host          string            `json:"host,omitempty"`
	Scheme        string            `json:"scheme,omitempty"`
	DefaultHeader map[string]string `json:"defaultHeader,omitempty"`
	UserAgent     string            `json:"userAgent,omitempty"`
	HTTPClient    *http.Client
}

func NewConfiguration() *Configuration {
	cfg := &Configuration{
		BasePath:      "https://max-api.maicoin.com",
		DefaultHeader: make(map[string]string),
		UserAgent:     "Swagger-Codegen/1.0.0/go",
	}
	return cfg
}

type service struct {
	client *APIClient
}

type PublicApiService service
type PrivateApiService service

// contains is a case insenstive match, finding needle in a haystack
func contains(haystack []string, needle string) bool {
	for _, a := range haystack {
		if strings.EqualFold(a, needle) {
			return true
		}
	}
	return false
}

// selectHeaderContentType select a content type from the available list.
func selectHeaderContentType(contentTypes []string) string {
	if len(contentTypes) == 0 {
		return ""
	}
	if contains(contentTypes, "application/json") {
		return "application/json"
	}
	return contentTypes[0] // use the first content type specified in 'consumes'
}

// selectHeaderAccept join all accept types and return
func selectHeaderAccept(accepts []string) string {
	if len(accepts) == 0 {
		return ""
	}

	if contains(accepts, "application/json") {
		return "application/json"
	}

	return strings.Join(accepts, ",")
}

type Market struct {
	// unique market id, check /api/v2/markets for available markets
	Id string `json:"id,omitempty"`

	// market name
	Name string `json:"name,omitempty"`

	// base unit
	BaseUnit string `json:"base_unit,omitempty"`

	// fixed precision of base unit
	BaseUnitPrecision int64 `json:"base_unit_precision,omitempty"`

	// quote unit
	QuoteUnit string `json:"quote_unit,omitempty"`

	// fixed precision of quote unit
	QuoteUnitPrecision int64 `json:"quote_unit_precision,omitempty"`
}

// get all available currencies.
type Currency struct {

	// unique currency id, check /api/v2/currencies for available currencies
	Id string `json:"id,omitempty"`

	// fixed precision of the currency
	Precision int64 `json:"precision,omitempty"`
}

type Member struct {

	// unique serial number
	Sn string `json:"sn,omitempty"`

	// user name
	Name string `json:"name,omitempty"`

	// user language
	Language string `json:"language,omitempty"`

	// valid phone set
	PhoneSet bool `json:"phone_set,omitempty"`

	// phone country code
	CountryCode string `json:"country_code,omitempty"`

	// taiwanese identity number
	IdentityNumber string `json:"identity_number,omitempty"`

	// invoice carrier id
	InvoiceCarrierId string `json:"invoice_carrier_id,omitempty"`

	// invoice carrier type
	InvoiceCarrierType string `json:"invoice_carrier_type,omitempty"`

	// is deleted
	IsDeleted bool `json:"is_deleted,omitempty"`

	// is frozen
	IsFrozen bool `json:"is_frozen,omitempty"`

	// is activated
	IsActivated bool `json:"is_activated,omitempty"`

	// is user profile verified
	ProfileVerified bool `json:"profile_verified,omitempty"`

	// is kyc approved
	KycApproved bool `json:"kyc_approved,omitempty"`

	// member kyc state: unverified, verifying, profile_verifying, verified, rejected
	KycState string `json:"kyc_state,omitempty"`

	// user mobile phone number
	PhoneNumber string `json:"phone_number,omitempty"`

	// is user aggreement checked
	UserAgreementChecked bool `json:"user_agreement_checked,omitempty"`

	// which tou version user agree with
	UserAgreementVersion string `json:"user_agreement_version,omitempty"`

	Bank *Bank `json:"bank,omitempty"`

	Documents *MemberDocs `json:"documents,omitempty"`

	// user email
	Email string `json:"email,omitempty"`

	Accounts []Account `json:"accounts,omitempty"`

	// type_guest, type_coin_1, type_coin_2, type_fiat
	MemberType string `json:"member_type,omitempty"`

	// member level
	Level int64 `json:"level,omitempty"`
}

type Bank struct {

	// bank branch code
	Branch string `json:"branch,omitempty"`

	// bank account
	Account string `json:"account,omitempty"`

	// bank account state
	State string `json:"state,omitempty"`
}

type MemberDocs struct {

	// ID photo front status
	PhotoIdFrontState string `json:"photo_id_front_state,omitempty"`

	// ID photo back status
	PhotoIdBackState string `json:"photo_id_back_state,omitempty"`

	// supplemental document status
	CellphoneBillState string `json:"cellphone_bill_state,omitempty"`

	// selfie status
	SelfieWithIdState string `json:"selfie_with_id_state,omitempty"`
}

type Account struct {

	// currency id, e.g. twd, btc, ...
	Currency string `json:"currency,omitempty"`

	// available balance
	Balance string `json:"balance,omitempty"`

	// locked funds
	Locked string `json:"locked,omitempty"`

	Type string `json:"type,omitempty"`

	// fiat currency that using to calculate
	FiatCurrency string `json:"fiat_currency,omitempty"`

	// fiat currency balance according to currency
	FiatBalance string `json:"fiat_balance,omitempty"`
}

// get ticker of specific market
type Ticker struct {

	// timestamp in seconds since Unix epoch
	At int64 `json:"at,omitempty"`

	// highest buy price
	Buy string `json:"buy,omitempty"`

	// lowest sell price
	Sell string `json:"sell,omitempty"`

	// price before 24 hours
	Open string `json:"open,omitempty"`

	// lowest price within 24 hours
	Low string `json:"low,omitempty"`

	// highest price within 24 hours
	High string `json:"high,omitempty"`

	// last traded price
	Last string `json:"last,omitempty"`

	// traded volume within 24 hours
	Vol string `json:"vol,omitempty"`
}

// get a specific order.
type Order struct {
	// unique order id
	Id int64 `json:"id,omitempty"`

	// 'sell' or 'buy'
	Side string `json:"side,omitempty"`

	// 'limit', 'market', 'stop_limit', or 'stop_market'
	OrdType string `json:"ord_type,omitempty"`

	// price of a unit
	Price string `json:"price,omitempty"`

	// price to trigger a stop order
	StopPrice string `json:"stop_price,omitempty"`

	// average execution price
	AvgPrice string `json:"avg_price,omitempty"`

	// 'wait', 'done', 'cancel', or 'convert'; 'wait' means waiting for fulfillment; 'done' means fullfilled; 'cancel' means cancelled; 'convert' means the stop order is triggered
	State string `json:"state,omitempty"`

	// market id, check /api/v2/markets for available markets
	Market string `json:"market,omitempty"`

	// created timestamp (second)
	CreatedAt int64 `json:"created_at,omitempty"`

	// total amount to sell/buy, an order could be partially executed
	Volume string `json:"volume,omitempty"`

	// remaining volume
	RemainingVolume string `json:"remaining_volume,omitempty"`

	// executed volume
	ExecutedVolume string `json:"executed_volume,omitempty"`

	// trade count
	TradesCount int64 `json:"trades_count,omitempty"`
}

// get ticker of all markets
type Tickers struct {
	Btctwd *Ticker `json:"btctwd,omitempty"`

	Ethtwd *Ticker `json:"ethtwd,omitempty"`

	Ltctwd *Ticker `json:"ltctwd,omitempty"`

	Mithtwd *Ticker `json:"mithtwd,omitempty"`

	Bchtwd *Ticker `json:"bchtwd,omitempty"`

	Usdttwd *Ticker `json:"usdttwd,omitempty"`

	Ethbtc *Ticker `json:"ethbtc,omitempty"`

	Mitheth *Ticker `json:"mitheth,omitempty"`

	Trxtwd *Ticker `json:"trxtwd,omitempty"`

	Trxbtc *Ticker `json:"trxbtc,omitempty"`

	Trxeth *Ticker `json:"trxeth,omitempty"`

	Trxusdt *Ticker `json:"trxusdt,omitempty"`

	Btcusdt *Ticker `json:"btcusdt,omitempty"`

	Ethusdt *Ticker `json:"ethusdt,omitempty"`

	Bchusdt *Ticker `json:"bchusdt,omitempty"`

	Ltcusdt *Ticker `json:"ltcusdt,omitempty"`

	Mithbtc *Ticker `json:"mithbtc,omitempty"`

	Mithusdt *Ticker `json:"mithusdt,omitempty"`

	Cccxbtc *Ticker `json:"cccxbtc,omitempty"`

	Cccxeth *Ticker `json:"cccxeth,omitempty"`

	Cccxtwd *Ticker `json:"cccxtwd,omitempty"`

	Cccxusdt *Ticker `json:"cccxusdt,omitempty"`

	Eosbtc *Ticker `json:"eosbtc,omitempty"`

	Eoseth *Ticker `json:"eoseth,omitempty"`

	Eostwd *Ticker `json:"eostwd,omitempty"`

	Eosusdt *Ticker `json:"eosusdt,omitempty"`

	Palbtc *Ticker `json:"palbtc,omitempty"`

	Paleth *Ticker `json:"paleth,omitempty"`

	Paltwd *Ticker `json:"paltwd,omitempty"`

	Batbtc *Ticker `json:"batbtc,omitempty"`

	Bateth *Ticker `json:"bateth,omitempty"`

	Battwd *Ticker `json:"battwd,omitempty"`

	Batusdt *Ticker `json:"batusdt,omitempty"`

	Zrxbtc *Ticker `json:"zrxbtc,omitempty"`

	Zrxeth *Ticker `json:"zrxeth,omitempty"`

	Zrxtwd *Ticker `json:"zrxtwd,omitempty"`

	Zrxusdt *Ticker `json:"zrxusdt,omitempty"`

	Gntbtc *Ticker `json:"gntbtc,omitempty"`

	Gnteth *Ticker `json:"gnteth,omitempty"`

	Gnttwd *Ticker `json:"gnttwd,omitempty"`

	Gntusdt *Ticker `json:"gntusdt,omitempty"`

	Omgbtc *Ticker `json:"omgbtc,omitempty"`

	Omgeth *Ticker `json:"omgeth,omitempty"`

	Omgtwd *Ticker `json:"omgtwd,omitempty"`

	Omgusdt *Ticker `json:"omgusdt,omitempty"`

	Kncbtc *Ticker `json:"kncbtc,omitempty"`

	Knceth *Ticker `json:"knceth,omitempty"`

	Knctwd *Ticker `json:"knctwd,omitempty"`

	Kncusdt *Ticker `json:"kncusdt,omitempty"`
}

// callAPI do the request.
func (c *APIClient) callAPI(request *http.Request) (*http.Response, error) {
	return c.cfg.HTTPClient.Do(request)
}

// Prevent trying to import "fmt"
func reportError(format string, a ...interface{}) error {
	return fmt.Errorf(format, a...)
}

// detectContentType method is used to figure out `Request.Body` content type for request header
func detectContentType(body interface{}) string {
	contentType := "text/plain; charset=utf-8"
	kind := reflect.TypeOf(body).Kind()

	switch kind {
	case reflect.Struct, reflect.Map, reflect.Ptr:
		contentType = "application/json; charset=utf-8"
	case reflect.String:
		contentType = "text/plain; charset=utf-8"
	default:
		if b, ok := body.([]byte); ok {
			contentType = http.DetectContentType(b)
		} else if kind == reflect.Slice {
			contentType = "application/json; charset=utf-8"
		}
	}

	return contentType
}

// Add a file to the multipart request
func addFile(w *multipart.Writer, fieldName, path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	part, err := w.CreateFormFile(fieldName, filepath.Base(path))
	if err != nil {
		return err
	}
	_, err = io.Copy(part, file)

	return err
}

// Set request body from an interface{}
func setBody(body interface{}, contentType string) (bodyBuf *bytes.Buffer, err error) {
	if bodyBuf == nil {
		bodyBuf = &bytes.Buffer{}
	}

	if reader, ok := body.(io.Reader); ok {
		_, err = bodyBuf.ReadFrom(reader)
	} else if b, ok := body.([]byte); ok {
		_, err = bodyBuf.Write(b)
	} else if s, ok := body.(string); ok {
		_, err = bodyBuf.WriteString(s)
	} else if jsonCheck.MatchString(contentType) {
		err = json.NewEncoder(bodyBuf).Encode(body)
	} else if xmlCheck.MatchString(contentType) {
		xml.NewEncoder(bodyBuf).Encode(body)
	}

	if err != nil {
		return nil, err
	}

	if bodyBuf.Len() == 0 {
		err = fmt.Errorf("invalid body type %s", contentType)
		return nil, err
	}

	return bodyBuf, nil
}

// prepareRequest build the request
func (c *APIClient) prepareRequest(
	ctx context.Context,
	path string, method string,
	postBody interface{},
	headerParams map[string]string,
	queryParams url.Values,
	formParams url.Values,
	fileName string,
	fileBytes []byte) (localVarRequest *http.Request, err error) {

	var body *bytes.Buffer

	// Detect postBody type and post.
	if postBody != nil {
		contentType := headerParams["Content-Type"]
		if contentType == "" {
			contentType = detectContentType(postBody)
			headerParams["Content-Type"] = contentType
		}

		body, err = setBody(postBody, contentType)
		if err != nil {
			return nil, err
		}
	}

	// add form parameters and file if available.
	if len(formParams) > 0 || (len(fileBytes) > 0 && fileName != "") {
		if body != nil {
			return nil, errors.New("cannot specify postBody and multipart form at the same time")
		}
		body = &bytes.Buffer{}
		w := multipart.NewWriter(body)

		for k, v := range formParams {
			for _, iv := range v {
				if strings.HasPrefix(k, "@") { // file
					err = addFile(w, k[1:], iv)
					if err != nil {
						return nil, err
					}
				} else { // form value
					w.WriteField(k, iv)
				}
			}
		}
		if len(fileBytes) > 0 && fileName != "" {
			w.Boundary()
			//_, fileNm := filepath.Split(fileName)
			part, err := w.CreateFormFile("file", filepath.Base(fileName))
			if err != nil {
				return nil, err
			}
			_, err = part.Write(fileBytes)
			if err != nil {
				return nil, err
			}
			// Set the Boundary in the Content-Type
			headerParams["Content-Type"] = w.FormDataContentType()
		}

		// Set Content-Length
		headerParams["Content-Length"] = fmt.Sprintf("%d", body.Len())
		w.Close()
	}

	// Setup path and query parameters
	url, err := url.Parse(path)
	if err != nil {
		return nil, err
	}

	// Adding Query Param
	query := url.Query()
	for k, v := range queryParams {
		for _, iv := range v {
			query.Add(k, iv)
		}
	}

	// Encode the parameters.
	url.RawQuery = query.Encode()

	// Generate a new request
	if body != nil {
		localVarRequest, err = http.NewRequest(method, url.String(), body)
	} else {
		localVarRequest, err = http.NewRequest(method, url.String(), nil)
	}
	if err != nil {
		return nil, err
	}

	// add header parameters, if any
	if len(headerParams) > 0 {
		headers := http.Header{}
		for h, v := range headerParams {
			headers.Set(h, v)
		}
		localVarRequest.Header = headers
	}

	// Override request host, if applicable
	if c.cfg.Host != "" {
		localVarRequest.Host = c.cfg.Host
	}

	// Add the user agent to the request.
	localVarRequest.Header.Add("User-Agent", c.cfg.UserAgent)

	if ctx != nil {
		// add context to the request
		localVarRequest = localVarRequest.WithContext(ctx)

		// Walk through any authentication.

		// OAuth2 authentication
		if tok, ok := ctx.Value(ContextOAuth2).(oauth2.TokenSource); ok {
			// We were able to grab an oauth2 token from the context
			var latestToken *oauth2.Token
			if latestToken, err = tok.Token(); err != nil {
				return nil, err
			}

			latestToken.SetAuthHeader(localVarRequest)
		}

		// Basic HTTP Authentication
		if auth, ok := ctx.Value(ContextBasicAuth).(BasicAuth); ok {
			localVarRequest.SetBasicAuth(auth.UserName, auth.Password)
		}

		// AccessToken Authentication
		if auth, ok := ctx.Value(ContextAccessToken).(string); ok {
			localVarRequest.Header.Add("Authorization", "Bearer "+auth)
		}
	}

	for header, value := range c.cfg.DefaultHeader {
		localVarRequest.Header.Add(header, value)
	}

	return localVarRequest, nil
}

// Verify optional parameters are of the correct type.
func typeCheckParameter(obj interface{}, expected string, name string) error {
	// Make sure there is an object.
	if obj == nil {
		return nil
	}

	// Check the type is as expected.
	if reflect.TypeOf(obj).String() != expected {
		return fmt.Errorf("expected %s to be of type %s but received %s", name, expected, reflect.TypeOf(obj).String())
	}
	return nil
}

// parameterToString convert interface{} parameters to string, using a delimiter if format is provided.
func parameterToString(obj interface{}, collectionFormat string) string {
	var delimiter string

	switch collectionFormat {
	case "pipes":
		delimiter = "|"
	case "ssv":
		delimiter = " "
	case "tsv":
		delimiter = "\t"
	case "csv":
		delimiter = ","
	}

	if reflect.TypeOf(obj).Kind() == reflect.Slice {
		return strings.Trim(strings.Replace(fmt.Sprint(obj), " ", delimiter, -1), "[]")
	}

	return fmt.Sprintf("%v", obj)
}

func makePayloadAndSignature(bodyMAP map[string]interface{}, apisecret string) (xMAXPAYLOAD, xMAXSIGNATURE string) {
	bodyJson, _ := json.Marshal(bodyMAP)
	xMAXPAYLOAD = base64.StdEncoding.EncodeToString((bodyJson))

	h := hmac.New(sha256.New, []byte(apisecret))
	h.Write([]byte(xMAXPAYLOAD))
	xMAXSIGNATURE = hex.EncodeToString(h.Sum(nil))
	return
}
