package maxapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"
)

/* PrivateApiService
create a sell/buy order
* @param ctx context.Context for authentication, logging, tracing, etc.
@param xMAXACCESSKEY access key
@param xMAXPAYLOAD encoded payload
@param xMAXSIGNATURE encrypted signature
@param market unique market id, check /api/v2/markets for available markets
@param side &#39;sell&#39; or &#39;buy&#39;
@param volume total amount to sell/buy, an order could be partially executed
@param optional (nil or map[string]interface{}) with one or more of:
    @param "price" (string) price of a unit
    @param "stopPrice" (string) price to trigger a stop order
    @param "ordType" (string) &#39;limit&#39;, &#39;market&#39;, &#39;stop_limit&#39;, or &#39;stop_market&#39;
@return Order*/
func (a *PrivateApiService) PostApiV2Orders(ctx context.Context, xMAXACCESSKEY string, xMAXSECRET string, market string, side string, volume string, localVarOptionals map[string]interface{}) (Order, *http.Response, error) {
	var (
		localVarHTTPMethod = strings.ToUpper("Post")
		localVarFileName   string
		localVarFileBytes  []byte
		successPayload     Order
	)

	// create path and map variables
	localVarPath := a.client.cfg.BasePath + "/api/v2/orders"

	localVarHeaderParams := make(map[string]string)
	localVarQueryParams := url.Values{}
	localVarFormParams := url.Values{}
	localVarPostBody := make(map[string]interface{})
	localVarPostBody["nonce"] = time.Now().UnixMilli()
	localVarPostBody["path"] = "/api/v2/orders"

	if err := typeCheckParameter(localVarOptionals["price"], "string", "price"); err != nil {
		return successPayload, nil, err
	}
	if err := typeCheckParameter(localVarOptionals["stop_price"], "string", "stop_price"); err != nil {
		return successPayload, nil, err
	}
	if err := typeCheckParameter(localVarOptionals["ord_type"], "string", "ord_type"); err != nil {
		return successPayload, nil, err
	}

	// to determine the Content-Type header
	localVarHTTPContentTypes := []string{"application/json"}

	// set Content-Type header
	localVarHTTPContentType := selectHeaderContentType(localVarHTTPContentTypes)
	if localVarHTTPContentType != "" {
		localVarHeaderParams["Content-Type"] = localVarHTTPContentType
	}

	// to determine the Accept header
	localVarHTTPHeaderAccepts := []string{
		"application/json",
	}

	// set Accept header
	localVarHTTPHeaderAccept := selectHeaderAccept(localVarHTTPHeaderAccepts)
	if localVarHTTPHeaderAccept != "" {
		localVarHeaderParams["Accept"] = localVarHTTPHeaderAccept
	}

	localVarPostBody["market"] = parameterToString(market, "")
	localVarPostBody["side"] = parameterToString(side, "")
	localVarPostBody["volume"] = parameterToString(volume, "")
	if localVarTempParam, localVarOk := localVarOptionals["price"].(string); localVarOk {
		localVarPostBody["price"] = parameterToString(localVarTempParam, "")
	}
	if localVarTempParam, localVarOk := localVarOptionals["stop_price"].(string); localVarOk {
		localVarPostBody["stop_price"] = parameterToString(localVarTempParam, "")
	}
	if localVarTempParam, localVarOk := localVarOptionals["ord_type"].(string); localVarOk {
		localVarPostBody["ord_type"] = parameterToString(localVarTempParam, "")
	}

	xMAXPAYLOAD, xMAXSIGNATURE := makePayloadAndSignature(localVarPostBody, xMAXSECRET)
	localVarHeaderParams["X-MAX-ACCESSKEY"] = parameterToString(xMAXACCESSKEY, "")
	localVarHeaderParams["X-MAX-PAYLOAD"] = parameterToString(xMAXPAYLOAD, "")
	localVarHeaderParams["X-MAX-SIGNATURE"] = parameterToString(xMAXSIGNATURE, "")

	r, err := a.client.prepareRequest(ctx, localVarPath, localVarHTTPMethod, localVarPostBody, localVarHeaderParams, localVarQueryParams, localVarFormParams, localVarFileName, localVarFileBytes)
	if err != nil {
		return successPayload, nil, err
	}

	localVarHTTPResponse, err := a.client.callAPI(r)
	if err != nil || localVarHTTPResponse == nil {
		return successPayload, localVarHTTPResponse, err
	}
	defer localVarHTTPResponse.Body.Close()
	if localVarHTTPResponse.StatusCode >= 300 {
		bodyBytes, _ := ioutil.ReadAll(localVarHTTPResponse.Body)
		return successPayload, localVarHTTPResponse, reportError("Status: %v, Body: %s", localVarHTTPResponse.Status, bodyBytes)
	}

	if err = json.NewDecoder(localVarHTTPResponse.Body).Decode(&successPayload); err != nil {
		return successPayload, localVarHTTPResponse, err
	}

	return successPayload, localVarHTTPResponse, err
}

/* PrivateApiService create multiple sell/buy orders
create multiple sell/buy orders, please put your orders as an array in json body
* @param ctx context.Context for authentication, logging, tracing, etc.
@param xMAXACCESSKEY access key
@param xMAXPAYLOAD encoded payload
@param xMAXSIGNATURE encrypted signature
@param market unique market id, check /api/v2/markets for available markets
@param ordersSide &#39;sell&#39; or &#39;buy&#39;
@param ordersVolume total amount to sell/buy, an order could be partially executed
@param optional (nil or map[string]interface{}) with one or more of:
    @param "ordersPrice" ([]string) price of a unit
    @param "ordersStopPrice" ([]string) price to trigger a stop order
    @param "ordersOrdType" ([]string) &#39;limit&#39;, &#39;market&#39;, &#39;stop_limit&#39;, or &#39;stop_market&#39;
@return []Order*/
func (a *PrivateApiService) PostApiV2OrdersMulti(ctx context.Context, xMAXACCESSKEY string, xMAXSECRET string, market string, ordersSide []string, ordersVolume []string, localVarOptionals map[string]interface{}) ([]Order, *http.Response, error) {
	var (
		localVarHTTPMethod = strings.ToUpper("Post")
		localVarFileName   string
		localVarFileBytes  []byte
		successPayload     []Order
	)

	// create path and map variables
	localVarPath := a.client.cfg.BasePath + "/api/v2/orders/multi/onebyone"

	localVarHeaderParams := make(map[string]string)
	localVarQueryParams := url.Values{}
	localVarFormParams := url.Values{}
	localVarPostBody := make(map[string]interface{})
	localVarPostBody["nonce"] = time.Now().UnixMilli()
	localVarPostBody["path"] = "/api/v2/orders/multi/onebyone"

	// to determine the Content-Type header
	localVarHTTPContentTypes := []string{"application/json"}

	// set Content-Type header
	localVarHTTPContentType := selectHeaderContentType(localVarHTTPContentTypes)
	if localVarHTTPContentType != "" {
		localVarHeaderParams["Content-Type"] = localVarHTTPContentType
	}

	// to determine the Accept header
	localVarHTTPHeaderAccepts := []string{
		"application/json",
	}

	// set Accept header
	localVarHTTPHeaderAccept := selectHeaderAccept(localVarHTTPHeaderAccepts)
	if localVarHTTPHeaderAccept != "" {
		localVarHeaderParams["Accept"] = localVarHTTPHeaderAccept
	}

	localVarPostBody["market"] = parameterToString(market, "")
	localVarPostBody["orders[side]"] = parameterToString(ordersSide, "csv")
	localVarPostBody["orders[volume]"] = parameterToString(ordersVolume, "csv")
	if localVarTempParam, localVarOk := localVarOptionals["orders[price]"].([]string); localVarOk {
		localVarPostBody["orders[price]"] = parameterToString(localVarTempParam, "csv")
	}
	if localVarTempParam, localVarOk := localVarOptionals["orders[stop_price]"].([]string); localVarOk {
		localVarPostBody["orders[stop_price]"] = parameterToString(localVarTempParam, "csv")
	}
	if localVarTempParam, localVarOk := localVarOptionals["orders[ord_type]"].([]string); localVarOk {
		localVarPostBody["orders[ord_type]"] = parameterToString(localVarTempParam, "csv")
	}

	xMAXPAYLOAD, xMAXSIGNATURE := makePayloadAndSignature(localVarPostBody, xMAXSECRET)
	localVarHeaderParams["X-MAX-ACCESSKEY"] = parameterToString(xMAXACCESSKEY, "")
	localVarHeaderParams["X-MAX-PAYLOAD"] = parameterToString(xMAXPAYLOAD, "")
	localVarHeaderParams["X-MAX-SIGNATURE"] = parameterToString(xMAXSIGNATURE, "")

	r, err := a.client.prepareRequest(ctx, localVarPath, localVarHTTPMethod, localVarPostBody, localVarHeaderParams, localVarQueryParams, localVarFormParams, localVarFileName, localVarFileBytes)
	if err != nil {
		return successPayload, nil, err
	}

	localVarHTTPResponse, err := a.client.callAPI(r)
	if err != nil || localVarHTTPResponse == nil {
		return successPayload, localVarHTTPResponse, err
	}
	defer localVarHTTPResponse.Body.Close()
	if localVarHTTPResponse.StatusCode >= 300 {
		bodyBytes, _ := ioutil.ReadAll(localVarHTTPResponse.Body)
		return successPayload, localVarHTTPResponse, reportError("Status: %v, Body: %s", localVarHTTPResponse.Status, bodyBytes)
	}

	if err = json.NewDecoder(localVarHTTPResponse.Body).Decode(&successPayload); err != nil {
		return successPayload, localVarHTTPResponse, err
	}

	return successPayload, localVarHTTPResponse, err
}

/* PrivateApiService
cancel an order
* @param ctx context.Context for authentication, logging, tracing, etc.
@param xMAXACCESSKEY access key
@param xMAXPAYLOAD encoded payload
@param xMAXSIGNATURE encrypted signature
@param id unique order id
@return Order*/
func (a *PrivateApiService) PostApiV2OrderDelete(ctx context.Context, xMAXACCESSKEY string, xMAXSECRET string, id int64) (Order, *http.Response, error) {
	var (
		localVarHTTPMethod = strings.ToUpper("Post")
		localVarFileName   string
		localVarFileBytes  []byte
		successPayload     Order
	)

	// create path and map variables
	localVarPath := a.client.cfg.BasePath + "/api/v2/order/delete"

	localVarHeaderParams := make(map[string]string)
	localVarQueryParams := url.Values{}
	localVarFormParams := url.Values{}
	localVarPostBody := make(map[string]interface{})
	localVarPostBody["nonce"] = time.Now().UnixMilli()
	localVarPostBody["path"] = "/api/v2/order/delete"

	// to determine the Content-Type header
	localVarHTTPContentTypes := []string{"application/json"}

	// set Content-Type header
	localVarHTTPContentType := selectHeaderContentType(localVarHTTPContentTypes)
	if localVarHTTPContentType != "" {
		localVarHeaderParams["Content-Type"] = localVarHTTPContentType
	}

	// to determine the Accept header
	localVarHTTPHeaderAccepts := []string{
		"application/json",
	}

	// set Accept header
	localVarHTTPHeaderAccept := selectHeaderAccept(localVarHTTPHeaderAccepts)
	if localVarHTTPHeaderAccept != "" {
		localVarHeaderParams["Accept"] = localVarHTTPHeaderAccept
	}

	localVarPostBody["id"] = parameterToString(id, "")
	xMAXPAYLOAD, xMAXSIGNATURE := makePayloadAndSignature(localVarPostBody, xMAXSECRET)
	localVarHeaderParams["X-MAX-ACCESSKEY"] = parameterToString(xMAXACCESSKEY, "")
	localVarHeaderParams["X-MAX-PAYLOAD"] = parameterToString(xMAXPAYLOAD, "")
	localVarHeaderParams["X-MAX-SIGNATURE"] = parameterToString(xMAXSIGNATURE, "")

	r, err := a.client.prepareRequest(ctx, localVarPath, localVarHTTPMethod, localVarPostBody, localVarHeaderParams, localVarQueryParams, localVarFormParams, localVarFileName, localVarFileBytes)
	if err != nil {
		return successPayload, nil, err
	}

	localVarHTTPResponse, err := a.client.callAPI(r)
	if err != nil || localVarHTTPResponse == nil {
		return successPayload, localVarHTTPResponse, err
	}
	defer localVarHTTPResponse.Body.Close()
	if localVarHTTPResponse.StatusCode >= 300 {
		bodyBytes, _ := ioutil.ReadAll(localVarHTTPResponse.Body)
		return successPayload, localVarHTTPResponse, reportError("Status: %v, Body: %s", localVarHTTPResponse.Status, bodyBytes)
	}

	if err = json.NewDecoder(localVarHTTPResponse.Body).Decode(&successPayload); err != nil {
		return successPayload, localVarHTTPResponse, err
	}

	return successPayload, localVarHTTPResponse, err
}

/* PrivateApiService
cancel an order
* @param ctx context.Context for authentication, logging, tracing, etc.
@param xMAXACCESSKEY access key
@param xMAXPAYLOAD encoded payload
@param xMAXSIGNATURE encrypted signature
@param id unique order id
@return Order*/
func (a *PrivateApiService) PostApiV2OrderDeleteClientId(ctx context.Context, xMAXACCESSKEY string, xMAXSECRET string, clientId string) (Order, *http.Response, error) {
	var (
		localVarHTTPMethod = strings.ToUpper("Post")
		localVarFileName   string
		localVarFileBytes  []byte
		successPayload     Order
	)

	// create path and map variables
	localVarPath := a.client.cfg.BasePath + "/api/v2/order/delete"

	localVarHeaderParams := make(map[string]string)
	localVarQueryParams := url.Values{}
	localVarFormParams := url.Values{}
	localVarPostBody := make(map[string]interface{})
	localVarPostBody["nonce"] = time.Now().UnixMilli()
	localVarPostBody["path"] = "/api/v2/order/delete"

	// to determine the Content-Type header
	localVarHTTPContentTypes := []string{"application/json"}

	// set Content-Type header
	localVarHTTPContentType := selectHeaderContentType(localVarHTTPContentTypes)
	if localVarHTTPContentType != "" {
		localVarHeaderParams["Content-Type"] = localVarHTTPContentType
	}

	// to determine the Accept header
	localVarHTTPHeaderAccepts := []string{
		"application/json",
	}

	// set Accept header
	localVarHTTPHeaderAccept := selectHeaderAccept(localVarHTTPHeaderAccepts)
	if localVarHTTPHeaderAccept != "" {
		localVarHeaderParams["Accept"] = localVarHTTPHeaderAccept
	}

	localVarPostBody["client_id"] = clientId
	xMAXPAYLOAD, xMAXSIGNATURE := makePayloadAndSignature(localVarPostBody, xMAXSECRET)
	localVarHeaderParams["X-MAX-ACCESSKEY"] = parameterToString(xMAXACCESSKEY, "")
	localVarHeaderParams["X-MAX-PAYLOAD"] = parameterToString(xMAXPAYLOAD, "")
	localVarHeaderParams["X-MAX-SIGNATURE"] = parameterToString(xMAXSIGNATURE, "")

	r, err := a.client.prepareRequest(ctx, localVarPath, localVarHTTPMethod, localVarPostBody, localVarHeaderParams, localVarQueryParams, localVarFormParams, localVarFileName, localVarFileBytes)
	if err != nil {
		return successPayload, nil, err
	}

	localVarHTTPResponse, err := a.client.callAPI(r)
	if err != nil || localVarHTTPResponse == nil {
		return successPayload, localVarHTTPResponse, err
	}
	defer localVarHTTPResponse.Body.Close()
	if localVarHTTPResponse.StatusCode >= 300 {
		bodyBytes, _ := ioutil.ReadAll(localVarHTTPResponse.Body)
		return successPayload, localVarHTTPResponse, reportError("Status: %v, Body: %s", localVarHTTPResponse.Status, bodyBytes)
	}

	if err = json.NewDecoder(localVarHTTPResponse.Body).Decode(&successPayload); err != nil {
		return successPayload, localVarHTTPResponse, err
	}

	return successPayload, localVarHTTPResponse, err
}

/* PrivateApiService
cancel all your orders with given market and side
* @param ctx context.Context for authentication, logging, tracing, etc.
@param xMAXACCESSKEY access key
@param xMAXPAYLOAD encoded payload
@param xMAXSIGNATURE encrypted signature
@param optional (nil or map[string]interface{}) with one or more of:
    @param "side" (string) set tp cancel only sell (asks) or buy (bids) orders
    @param "market" (string) specify market like btctwd / ethbtc
@return []Order*/
func (a *PrivateApiService) PostApiV2OrdersClear(ctx context.Context, xMAXACCESSKEY string, xMAXSECRET string, localVarOptionals map[string]interface{}) ([]Order, *http.Response, error) {
	var (
		localVarHTTPMethod = strings.ToUpper("Post")
		localVarFileName   string
		localVarFileBytes  []byte
		successPayload     []Order
	)

	// create path and map variables
	localVarPath := a.client.cfg.BasePath + "/api/v2/orders/clear"

	localVarHeaderParams := make(map[string]string)
	localVarQueryParams := url.Values{}
	localVarFormParams := url.Values{}
	localVarPostBody := make(map[string]interface{})
	localVarPostBody["nonce"] = time.Now().UnixMilli()
	localVarPostBody["path"] = "/api/v2/orders/clear"

	if err := typeCheckParameter(localVarOptionals["side"], "string", "side"); err != nil {
		return successPayload, nil, err
	}
	if err := typeCheckParameter(localVarOptionals["market"], "string", "market"); err != nil {
		return successPayload, nil, err
	}

	// to determine the Content-Type header
	localVarHTTPContentTypes := []string{"application/json"}

	// set Content-Type header
	localVarHTTPContentType := selectHeaderContentType(localVarHTTPContentTypes)
	if localVarHTTPContentType != "" {
		localVarHeaderParams["Content-Type"] = localVarHTTPContentType
	}

	// to determine the Accept header
	localVarHTTPHeaderAccepts := []string{
		"application/json",
	}

	// set Accept header
	localVarHTTPHeaderAccept := selectHeaderAccept(localVarHTTPHeaderAccepts)
	if localVarHTTPHeaderAccept != "" {
		localVarHeaderParams["Accept"] = localVarHTTPHeaderAccept
	}

	if localVarTempParam, localVarOk := localVarOptionals["side"].(string); localVarOk {
		localVarPostBody["side"] = parameterToString(localVarTempParam, "")
	}
	if localVarTempParam, localVarOk := localVarOptionals["market"].(string); localVarOk {
		localVarPostBody["market"] = parameterToString(localVarTempParam, "")
	}

	xMAXPAYLOAD, xMAXSIGNATURE := makePayloadAndSignature(localVarPostBody, xMAXSECRET)
	localVarHeaderParams["X-MAX-ACCESSKEY"] = parameterToString(xMAXACCESSKEY, "")
	localVarHeaderParams["X-MAX-PAYLOAD"] = parameterToString(xMAXPAYLOAD, "")
	localVarHeaderParams["X-MAX-SIGNATURE"] = parameterToString(xMAXSIGNATURE, "")

	r, err := a.client.prepareRequest(ctx, localVarPath, localVarHTTPMethod, localVarPostBody, localVarHeaderParams, localVarQueryParams, localVarFormParams, localVarFileName, localVarFileBytes)
	if err != nil {
		return successPayload, nil, err
	}

	localVarHTTPResponse, err := a.client.callAPI(r)
	if err != nil || localVarHTTPResponse == nil {
		return successPayload, localVarHTTPResponse, err
	}
	defer localVarHTTPResponse.Body.Close()
	if localVarHTTPResponse.StatusCode >= 300 {
		bodyBytes, _ := ioutil.ReadAll(localVarHTTPResponse.Body)
		return successPayload, localVarHTTPResponse, reportError("Status: %v, Body: %s", localVarHTTPResponse.Status, bodyBytes)
	}

	if err = json.NewDecoder(localVarHTTPResponse.Body).Decode(&successPayload); err != nil {
		return successPayload, localVarHTTPResponse, err
	}

	return successPayload, localVarHTTPResponse, err
}

/* PrivateApiService
get your profile and accounts infomation
* @param ctx context.Context for authentication, logging, tracing, etc.
@param xMAXACCESSKEY access key
@param xMAXPAYLOAD encoded payload
@param xMAXSIGNATURE encrypted signature
@return Member*/
func (a *PrivateApiService) GetApiV2MembersMe(ctx context.Context, xMAXACCESSKEY string, xMAXSECRET string) (Member, *http.Response, error) {
	var (
		localVarHTTPMethod = strings.ToUpper("Get")
		localVarFileName   string
		localVarFileBytes  []byte
		successPayload     Member
	)

	// create path and map variables
	localVarPath := a.client.cfg.BasePath + "/api/v2/members/me"

	localVarHeaderParams := make(map[string]string)
	localVarQueryParams := url.Values{}
	localVarFormParams := url.Values{}
	localVarPostBody := make(map[string]interface{})
	localVarPostBody["nonce"] = time.Now().UnixMilli()
	localVarPostBody["path"] = "/api/v2/members/me"

	xMAXPAYLOAD, xMAXSIGNATURE := makePayloadAndSignature(localVarPostBody, xMAXSECRET)

	// to determine the Content-Type header
	localVarHTTPContentTypes := []string{}

	// set Content-Type header
	localVarHTTPContentType := selectHeaderContentType(localVarHTTPContentTypes)
	if localVarHTTPContentType != "" {
		localVarHeaderParams["Content-Type"] = localVarHTTPContentType
	}

	// to determine the Accept header
	localVarHTTPHeaderAccepts := []string{
		"application/json",
	}

	// set Accept header
	localVarHTTPHeaderAccept := selectHeaderAccept(localVarHTTPHeaderAccepts)
	if localVarHTTPHeaderAccept != "" {
		localVarHeaderParams["Accept"] = localVarHTTPHeaderAccept
	}
	localVarHeaderParams["X-MAX-ACCESSKEY"] = parameterToString(xMAXACCESSKEY, "")
	localVarHeaderParams["X-MAX-PAYLOAD"] = parameterToString(xMAXPAYLOAD, "")
	localVarHeaderParams["X-MAX-SIGNATURE"] = parameterToString(xMAXSIGNATURE, "")
	r, err := a.client.prepareRequest(ctx, localVarPath, localVarHTTPMethod, localVarPostBody, localVarHeaderParams, localVarQueryParams, localVarFormParams, localVarFileName, localVarFileBytes)
	if err != nil {
		return successPayload, nil, err
	}

	localVarHTTPResponse, err := a.client.callAPI(r)
	if err != nil || localVarHTTPResponse == nil {
		return successPayload, localVarHTTPResponse, err
	}
	defer localVarHTTPResponse.Body.Close()
	if localVarHTTPResponse.StatusCode >= 300 {
		bodyBytes, _ := ioutil.ReadAll(localVarHTTPResponse.Body)
		return successPayload, localVarHTTPResponse, reportError("Status: %v, Body: %s", localVarHTTPResponse.Status, bodyBytes)
	}

	if err = json.NewDecoder(localVarHTTPResponse.Body).Decode(&successPayload); err != nil {
		return successPayload, localVarHTTPResponse, err
	}

	return successPayload, localVarHTTPResponse, err
}

/* PrivateApiService
get your profile and accounts infomation
* @param ctx context.Context for authentication, logging, tracing, etc.
@param xMAXACCESSKEY access key
@param xMAXPAYLOAD encoded payload
@param xMAXSIGNATURE encrypted signature
@return Member*/
func (a *PrivateApiService) GetApiV2MembersAccounts(ctx context.Context, xMAXACCESSKEY string, xMAXSECRET string) (Member, *http.Response, error) {
	var (
		localVarHTTPMethod = strings.ToUpper("Get")
		localVarFileName   string
		localVarFileBytes  []byte
		successPayload     Member
	)

	// create path and map variables
	localVarPath := a.client.cfg.BasePath + "/api/v2/members/me"

	localVarHeaderParams := make(map[string]string)
	localVarQueryParams := url.Values{}
	localVarFormParams := url.Values{}
	localVarPostBody := make(map[string]interface{})
	localVarPostBody["nonce"] = time.Now().UnixMilli()
	localVarPostBody["path"] = "/api/v2/members/me"

	xMAXPAYLOAD, xMAXSIGNATURE := makePayloadAndSignature(localVarPostBody, xMAXSECRET)

	// to determine the Content-Type header
	localVarHTTPContentTypes := []string{}

	// set Content-Type header
	localVarHTTPContentType := selectHeaderContentType(localVarHTTPContentTypes)
	if localVarHTTPContentType != "" {
		localVarHeaderParams["Content-Type"] = localVarHTTPContentType
	}

	// to determine the Accept header
	localVarHTTPHeaderAccepts := []string{
		"application/json",
	}

	// set Accept header
	localVarHTTPHeaderAccept := selectHeaderAccept(localVarHTTPHeaderAccepts)
	if localVarHTTPHeaderAccept != "" {
		localVarHeaderParams["Accept"] = localVarHTTPHeaderAccept
	}
	localVarHeaderParams["X-MAX-ACCESSKEY"] = parameterToString(xMAXACCESSKEY, "")
	localVarHeaderParams["X-MAX-PAYLOAD"] = parameterToString(xMAXPAYLOAD, "")
	localVarHeaderParams["X-MAX-SIGNATURE"] = parameterToString(xMAXSIGNATURE, "")
	r, err := a.client.prepareRequest(ctx, localVarPath, localVarHTTPMethod, localVarPostBody, localVarHeaderParams, localVarQueryParams, localVarFormParams, localVarFileName, localVarFileBytes)
	if err != nil {
		return successPayload, nil, err
	}

	localVarHTTPResponse, err := a.client.callAPI(r)
	if err != nil || localVarHTTPResponse == nil {
		return successPayload, localVarHTTPResponse, err
	}
	defer localVarHTTPResponse.Body.Close()
	if localVarHTTPResponse.StatusCode >= 300 {
		bodyBytes, _ := ioutil.ReadAll(localVarHTTPResponse.Body)
		return successPayload, localVarHTTPResponse, reportError("Status: %v, Body: %s", localVarHTTPResponse.Status, bodyBytes)
	}

	if err = json.NewDecoder(localVarHTTPResponse.Body).Decode(&successPayload); err != nil {
		return successPayload, localVarHTTPResponse, err
	}

	return successPayload, localVarHTTPResponse, err
}

/* PrivateApiService
get your orders, results is paginated.
* @param ctx context.Context for authentication, logging, tracing, etc.
@param xMAXACCESSKEY access key
@param xMAXPAYLOAD encoded payload
@param xMAXSIGNATURE encrypted signature
@param market unique market id, check /api/v2/markets for available markets
@param optional (nil or map[string]interface{}) with one or more of:
    @param "state" (string) filter by state, default to &#39;wait&#39;
    @param "orderBy" (string) order in created time, default to &#39;asc&#39;.
    @param "pagination" (bool) do pagination &amp; return metadata in header (default true)
    @param "page" (int64) page number, applied for pagination (default 1)
    @param "limit" (int64) returned limit (1~1000, default 100)
    @param "offset" (int64) records to skip, not applied for pagination (default 0)
@return []Order*/
func (a *PrivateApiService) GetApiV2Orders(ctx context.Context, xMAXACCESSKEY string, xMAXSECRET string, market string, localVarOptionals map[string]interface{}) ([]Order, *http.Response, error) {
	var (
		localVarHTTPMethod = strings.ToUpper("Get")
		localVarFileName   string
		localVarFileBytes  []byte
		successPayload     []Order
	)

	// create path and map variables
	localVarPath := a.client.cfg.BasePath + "/api/v2/orders"

	localVarHeaderParams := make(map[string]string)
	localVarQueryParams := url.Values{}
	localVarFormParams := url.Values{}
	localVarPostBody := make(map[string]interface{})
	localVarPostBody["nonce"] = time.Now().UnixMilli()
	localVarPostBody["path"] = "/api/v2/orders"
	localVarPostBody["market"] = market

	if err := typeCheckParameter(localVarOptionals["state"], "string", "state"); err != nil {
		return successPayload, nil, err
	}
	if err := typeCheckParameter(localVarOptionals["order_by"], "string", "order_by"); err != nil {
		return successPayload, nil, err
	}
	if err := typeCheckParameter(localVarOptionals["pagination"], "bool", "pagination"); err != nil {
		return successPayload, nil, err
	}
	if err := typeCheckParameter(localVarOptionals["page"], "int64", "page"); err != nil {
		return successPayload, nil, err
	}
	if err := typeCheckParameter(localVarOptionals["limit"], "int64", "limit"); err != nil {
		return successPayload, nil, err
	}
	if err := typeCheckParameter(localVarOptionals["offset"], "int64", "offset"); err != nil {
		return successPayload, nil, err
	}

	localVarQueryParams.Add("market", parameterToString(market, ""))
	if localVarTempParam, localVarOk := localVarOptionals["state"].(string); localVarOk {
		localVarQueryParams.Add("state", parameterToString(localVarTempParam, ""))
	}
	if localVarTempParam, localVarOk := localVarOptionals["order_by"].(string); localVarOk {
		localVarQueryParams.Add("order_by", parameterToString(localVarTempParam, ""))
	}
	if localVarTempParam, localVarOk := localVarOptionals["pagination"].(bool); localVarOk {
		localVarQueryParams.Add("pagination", parameterToString(localVarTempParam, ""))
	}
	if localVarTempParam, localVarOk := localVarOptionals["page"].(int64); localVarOk {
		localVarQueryParams.Add("page", parameterToString(localVarTempParam, ""))
	}
	if localVarTempParam, localVarOk := localVarOptionals["limit"].(int64); localVarOk {
		localVarQueryParams.Add("limit", parameterToString(localVarTempParam, ""))
	}
	if localVarTempParam, localVarOk := localVarOptionals["offset"].(int64); localVarOk {
		localVarQueryParams.Add("offset", parameterToString(localVarTempParam, ""))
	}
	// to determine the Content-Type header
	localVarHTTPContentTypes := []string{}

	// set Content-Type header
	localVarHTTPContentType := selectHeaderContentType(localVarHTTPContentTypes)
	if localVarHTTPContentType != "" {
		localVarHeaderParams["Content-Type"] = localVarHTTPContentType
	}

	// to determine the Accept header
	localVarHTTPHeaderAccepts := []string{
		"application/json",
	}

	// set Accept header
	localVarHTTPHeaderAccept := selectHeaderAccept(localVarHTTPHeaderAccepts)
	if localVarHTTPHeaderAccept != "" {
		localVarHeaderParams["Accept"] = localVarHTTPHeaderAccept
	}

	xMAXPAYLOAD, xMAXSIGNATURE := makePayloadAndSignature(localVarPostBody, xMAXSECRET)
	localVarHeaderParams["X-MAX-ACCESSKEY"] = parameterToString(xMAXACCESSKEY, "")
	localVarHeaderParams["X-MAX-PAYLOAD"] = parameterToString(xMAXPAYLOAD, "")
	localVarHeaderParams["X-MAX-SIGNATURE"] = parameterToString(xMAXSIGNATURE, "")

	r, err := a.client.prepareRequest(ctx, localVarPath, localVarHTTPMethod, localVarPostBody, localVarHeaderParams, localVarQueryParams, localVarFormParams, localVarFileName, localVarFileBytes)
	if err != nil {
		return successPayload, nil, err
	}

	localVarHTTPResponse, err := a.client.callAPI(r)
	if err != nil || localVarHTTPResponse == nil {
		return successPayload, localVarHTTPResponse, err
	}
	defer localVarHTTPResponse.Body.Close()
	if localVarHTTPResponse.StatusCode >= 300 {
		bodyBytes, _ := ioutil.ReadAll(localVarHTTPResponse.Body)
		return successPayload, localVarHTTPResponse, reportError("Status: %v, Body: %s", localVarHTTPResponse.Status, bodyBytes)
	}

	if err = json.NewDecoder(localVarHTTPResponse.Body).Decode(&successPayload); err != nil {
		return successPayload, localVarHTTPResponse, err
	}

	return successPayload, localVarHTTPResponse, err
}

// MAX public api function

/* PublicApiService
get all available markets.
* @param ctx context.Context for authentication, logging, tracing, etc.
@return []Market*/
func (a *PublicApiService) GetApiV2Markets(ctx context.Context) ([]Market, *http.Response, error) {
	var (
		localVarHTTPMethod = strings.ToUpper("Get")
		localVarFileName   string
		localVarFileBytes  []byte
		successPayload     []Market
	)

	// create path and map variables
	localVarPath := a.client.cfg.BasePath + "/api/v2/markets"

	localVarHeaderParams := make(map[string]string)
	localVarQueryParams := url.Values{}
	localVarFormParams := url.Values{}
	localVarPostBody := make(map[string]interface{})

	// to determine the Content-Type header
	localVarHTTPContentTypes := []string{}

	// set Content-Type header
	localVarHTTPContentType := selectHeaderContentType(localVarHTTPContentTypes)
	if localVarHTTPContentType != "" {
		localVarHeaderParams["Content-Type"] = localVarHTTPContentType
	}

	// to determine the Accept header
	localVarHTTPHeaderAccepts := []string{
		"application/json",
	}

	// set Accept header
	localVarHTTPHeaderAccept := selectHeaderAccept(localVarHTTPHeaderAccepts)
	if localVarHTTPHeaderAccept != "" {
		localVarHeaderParams["Accept"] = localVarHTTPHeaderAccept
	}
	r, err := a.client.prepareRequest(ctx, localVarPath, localVarHTTPMethod, localVarPostBody, localVarHeaderParams, localVarQueryParams, localVarFormParams, localVarFileName, localVarFileBytes)
	if err != nil {
		return successPayload, nil, err
	}

	localVarHTTPResponse, err := a.client.callAPI(r)
	if err != nil || localVarHTTPResponse == nil {
		return successPayload, localVarHTTPResponse, err
	}
	defer localVarHTTPResponse.Body.Close()
	if localVarHTTPResponse.StatusCode >= 300 {
		bodyBytes, _ := ioutil.ReadAll(localVarHTTPResponse.Body)
		return successPayload, localVarHTTPResponse, reportError("Status: %v, Body: %s", localVarHTTPResponse.Status, bodyBytes)
	}

	if err = json.NewDecoder(localVarHTTPResponse.Body).Decode(&successPayload); err != nil {
		return successPayload, localVarHTTPResponse, err
	}

	return successPayload, localVarHTTPResponse, err
}

/* PublicApiService
get all available currencies.
* @param ctx context.Context for authentication, logging, tracing, etc.
@return []Currency*/
func (a *PublicApiService) GetApiV2Currencies(ctx context.Context) ([]Currency, *http.Response, error) {
	var (
		localVarHTTPMethod = strings.ToUpper("Get")
		localVarFileName   string
		localVarFileBytes  []byte
		successPayload     []Currency
	)

	// create path and map variables
	localVarPath := a.client.cfg.BasePath + "/api/v2/currencies"

	localVarHeaderParams := make(map[string]string)
	localVarQueryParams := url.Values{}
	localVarFormParams := url.Values{}
	localVarPostBody := make(map[string]interface{})

	// to determine the Content-Type header
	localVarHTTPContentTypes := []string{}

	// set Content-Type header
	localVarHTTPContentType := selectHeaderContentType(localVarHTTPContentTypes)
	if localVarHTTPContentType != "" {
		localVarHeaderParams["Content-Type"] = localVarHTTPContentType
	}

	// to determine the Accept header
	localVarHTTPHeaderAccepts := []string{
		"application/json",
	}

	// set Accept header
	localVarHTTPHeaderAccept := selectHeaderAccept(localVarHTTPHeaderAccepts)
	if localVarHTTPHeaderAccept != "" {
		localVarHeaderParams["Accept"] = localVarHTTPHeaderAccept
	}
	r, err := a.client.prepareRequest(ctx, localVarPath, localVarHTTPMethod, localVarPostBody, localVarHeaderParams, localVarQueryParams, localVarFormParams, localVarFileName, localVarFileBytes)
	if err != nil {
		return successPayload, nil, err
	}

	localVarHTTPResponse, err := a.client.callAPI(r)
	if err != nil || localVarHTTPResponse == nil {
		return successPayload, localVarHTTPResponse, err
	}
	defer localVarHTTPResponse.Body.Close()
	if localVarHTTPResponse.StatusCode >= 300 {
		bodyBytes, _ := ioutil.ReadAll(localVarHTTPResponse.Body)
		return successPayload, localVarHTTPResponse, reportError("Status: %v, Body: %s", localVarHTTPResponse.Status, bodyBytes)
	}

	if err = json.NewDecoder(localVarHTTPResponse.Body).Decode(&successPayload); err != nil {
		return successPayload, localVarHTTPResponse, err
	}

	return successPayload, localVarHTTPResponse, err
}

/* PublicApiService
get ticker of all markets
* @param ctx context.Context for authentication, logging, tracing, etc.
@return Tickers*/
func (a *PublicApiService) GetApiV2Tickers(ctx context.Context) (Tickers, *http.Response, error) {
	var (
		localVarHTTPMethod = strings.ToUpper("Get")
		localVarFileName   string
		localVarFileBytes  []byte
		successPayload     Tickers
	)

	// create path and map variables
	localVarPath := a.client.cfg.BasePath + "/api/v2/tickers"

	localVarHeaderParams := make(map[string]string)
	localVarQueryParams := url.Values{}
	localVarFormParams := url.Values{}
	localVarPostBody := make(map[string]interface{})

	// to determine the Content-Type header
	localVarHTTPContentTypes := []string{}

	// set Content-Type header
	localVarHTTPContentType := selectHeaderContentType(localVarHTTPContentTypes)
	if localVarHTTPContentType != "" {
		localVarHeaderParams["Content-Type"] = localVarHTTPContentType
	}

	// to determine the Accept header
	localVarHTTPHeaderAccepts := []string{
		"application/json",
	}

	// set Accept header
	localVarHTTPHeaderAccept := selectHeaderAccept(localVarHTTPHeaderAccepts)
	if localVarHTTPHeaderAccept != "" {
		localVarHeaderParams["Accept"] = localVarHTTPHeaderAccept
	}
	r, err := a.client.prepareRequest(ctx, localVarPath, localVarHTTPMethod, localVarPostBody, localVarHeaderParams, localVarQueryParams, localVarFormParams, localVarFileName, localVarFileBytes)
	if err != nil {
		return successPayload, nil, err
	}

	localVarHTTPResponse, err := a.client.callAPI(r)
	if err != nil || localVarHTTPResponse == nil {
		return successPayload, localVarHTTPResponse, err
	}
	defer localVarHTTPResponse.Body.Close()
	if localVarHTTPResponse.StatusCode >= 300 {
		bodyBytes, _ := ioutil.ReadAll(localVarHTTPResponse.Body)
		return successPayload, localVarHTTPResponse, reportError("Status: %v, Body: %s", localVarHTTPResponse.Status, bodyBytes)
	}

	if err = json.NewDecoder(localVarHTTPResponse.Body).Decode(&successPayload); err != nil {
		return successPayload, localVarHTTPResponse, err
	}

	return successPayload, localVarHTTPResponse, err
}

/* PublicApiService
get ticker of specific market
* @param ctx context.Context for authentication, logging, tracing, etc.
@param market unique market id, check /api/v2/markets for available markets
@return Ticker*/
func (a *PublicApiService) GetApiV2TickersMarket(ctx context.Context, market string) (Ticker, *http.Response, error) {
	var (
		localVarHTTPMethod = strings.ToUpper("Get")
		localVarFileName   string
		localVarFileBytes  []byte
		successPayload     Ticker
	)

	// create path and map variables
	localVarPath := a.client.cfg.BasePath + "/api/v2/tickers/{market}"
	localVarPath = strings.Replace(localVarPath, "{"+"market"+"}", fmt.Sprintf("%v", market), -1)

	localVarHeaderParams := make(map[string]string)
	localVarQueryParams := url.Values{}
	localVarFormParams := url.Values{}
	localVarPostBody := make(map[string]interface{})

	// to determine the Content-Type header
	localVarHTTPContentTypes := []string{}

	// set Content-Type header
	localVarHTTPContentType := selectHeaderContentType(localVarHTTPContentTypes)
	if localVarHTTPContentType != "" {
		localVarHeaderParams["Content-Type"] = localVarHTTPContentType
	}

	// to determine the Accept header
	localVarHTTPHeaderAccepts := []string{
		"application/json",
	}

	// set Accept header
	localVarHTTPHeaderAccept := selectHeaderAccept(localVarHTTPHeaderAccepts)
	if localVarHTTPHeaderAccept != "" {
		localVarHeaderParams["Accept"] = localVarHTTPHeaderAccept
	}
	r, err := a.client.prepareRequest(ctx, localVarPath, localVarHTTPMethod, localVarPostBody, localVarHeaderParams, localVarQueryParams, localVarFormParams, localVarFileName, localVarFileBytes)
	if err != nil {
		return successPayload, nil, err
	}

	localVarHTTPResponse, err := a.client.callAPI(r)
	if err != nil || localVarHTTPResponse == nil {
		return successPayload, localVarHTTPResponse, err
	}
	defer localVarHTTPResponse.Body.Close()
	if localVarHTTPResponse.StatusCode >= 300 {
		bodyBytes, _ := ioutil.ReadAll(localVarHTTPResponse.Body)
		return successPayload, localVarHTTPResponse, reportError("Status: %v, Body: %s", localVarHTTPResponse.Status, bodyBytes)
	}

	if err = json.NewDecoder(localVarHTTPResponse.Body).Decode(&successPayload); err != nil {
		return successPayload, localVarHTTPResponse, err
	}

	return successPayload, localVarHTTPResponse, err
}
