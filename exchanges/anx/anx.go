package anx

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/champii/gocryptotrader/common"
	"github.com/champii/gocryptotrader/config"
	"github.com/champii/gocryptotrader/exchanges"
)

const (
	ANX_API_URL         = "https://anxpro.com/"
	ANX_API_VERSION     = "3"
	ANX_APIKEY          = "apiKey"
	ANX_DATA_TOKEN      = "dataToken"
	ANX_ORDER_NEW       = "order/new"
	ANX_ORDER_INFO      = "order/info"
	ANX_SEND            = "send"
	ANX_SUBACCOUNT_NEW  = "subaccount/new"
	ANX_RECEIVE_ADDRESS = "receive"
	ANX_CREATE_ADDRESS  = "receive/create"
	ANX_TICKER          = "money/ticker"
)

type ANX struct {
	exchange.ExchangeBase
}

func (a *ANX) SetDefaults() {
	a.Name = "ANX"
	a.Enabled = false
	a.TakerFee = 0.6
	a.MakerFee = 0.3
	a.Verbose = false
	a.Websocket = false
	a.RESTPollingDelay = 10
}

//Setup is run on startup to setup exchange with config values
func (a *ANX) Setup(exch config.ExchangeConfig) {
	if !exch.Enabled {
		a.SetEnabled(false)
	} else {
		a.Enabled = true
		a.AuthenticatedAPISupport = exch.AuthenticatedAPISupport
		a.SetAPIKeys(exch.APIKey, exch.APISecret, "", true)
		a.RESTPollingDelay = exch.RESTPollingDelay
		a.Verbose = exch.Verbose
		a.Websocket = exch.Websocket
		a.BaseCurrencies = common.SplitStrings(exch.BaseCurrencies, ",")
		a.AvailablePairs = common.SplitStrings(exch.AvailablePairs, ",")
		a.EnabledPairs = common.SplitStrings(exch.EnabledPairs, ",")
	}
}

func (a *ANX) GetFee(maker bool) float64 {
	if maker {
		return a.MakerFee
	}
	return a.TakerFee
}

func (a *ANX) GetTicker(currency string) (ANXTicker, error) {
	var ticker ANXTicker
	err := common.SendHTTPGetRequest(fmt.Sprintf("%sapi/2/%s/%s", ANX_API_URL, currency, ANX_TICKER), true, &ticker)
	if err != nil {
		return ANXTicker{}, err
	}
	return ticker, nil
}

func (a *ANX) GetAPIKey(username, password, otp, deviceID string) (string, string, error) {
	request := make(map[string]interface{})
	request["nonce"] = strconv.FormatInt(time.Now().UnixNano(), 10)[0:13]
	request["username"] = username
	request["password"] = password

	if otp != "" {
		request["otp"] = otp
	}

	request["deviceId"] = deviceID

	type APIKeyResponse struct {
		APIKey     string `json:"apiKey"`
		APISecret  string `json:"apiSecret"`
		ResultCode string `json:"resultCode"`
		Timestamp  int64  `json:"timestamp"`
	}
	var response APIKeyResponse

	err := a.SendAuthenticatedHTTPRequest(ANX_APIKEY, request, &response)
	if err != nil {
		return "", "", err
	}

	if response.ResultCode != "OK" {
		return "", "", errors.New("Response code is not OK: " + response.ResultCode)
	}

	return response.APIKey, response.APISecret, nil
}

func (a *ANX) GetDataToken() (string, error) {
	request := make(map[string]interface{})

	type DataTokenResponse struct {
		ResultCode string `json:"resultCode"`
		Timestamp  int64  `json:"timestamp"`
		Token      string `json:"token"`
		UUID       string `json:"uuid"`
	}
	var response DataTokenResponse

	err := a.SendAuthenticatedHTTPRequest(ANX_DATA_TOKEN, request, &response)
	if err != nil {
		return "", err
	}

	if response.ResultCode != "OK" {
		return "", errors.New("Response code is not OK: %s" + response.ResultCode)
	}
	return response.Token, nil
}

func (a *ANX) NewOrder(orderType string, buy bool, tradedCurrency, tradedCurrencyAmount, settlementCurrency, settlementCurrencyAmount, limitPriceSettlement string,
	replace bool, replaceUUID string, replaceIfActive bool) error {
	request := make(map[string]interface{})

	var order ANXOrder
	order.OrderType = orderType
	order.BuyTradedCurrency = buy

	if buy {
		order.TradedCurrencyAmount = tradedCurrencyAmount
	} else {
		order.SettlementCurrencyAmount = settlementCurrencyAmount
	}

	order.TradedCurrency = tradedCurrency
	order.SettlementCurrency = settlementCurrency
	order.LimitPriceInSettlementCurrency = limitPriceSettlement

	if replace {
		order.ReplaceExistingOrderUUID = replaceUUID
		order.ReplaceOnlyIfActive = replaceIfActive
	}

	request["order"] = order

	type OrderResponse struct {
		OrderID    string `json:"orderId"`
		Timestamp  int64  `json:"timestamp"`
		ResultCode string `json:"resultCode"`
	}
	var response OrderResponse

	err := a.SendAuthenticatedHTTPRequest(ANX_ORDER_NEW, request, &response)
	if err != nil {
		return err
	}

	if response.ResultCode != "OK" {
		return errors.New("Response code is not OK: %s" + response.ResultCode)
	}
	return nil
}

func (a *ANX) OrderInfo(orderID string) (ANXOrderResponse, error) {
	request := make(map[string]interface{})
	request["orderId"] = orderID

	type OrderInfoResponse struct {
		Order      ANXOrderResponse `json:"order"`
		ResultCode string           `json:"resultCode"`
		Timestamp  int64            `json:"timestamp"`
	}
	var response OrderInfoResponse

	err := a.SendAuthenticatedHTTPRequest(ANX_ORDER_INFO, request, &response)

	if err != nil {
		return ANXOrderResponse{}, err
	}

	if response.ResultCode != "OK" {
		log.Printf("Response code is not OK: %s\n", response.ResultCode)
		return ANXOrderResponse{}, errors.New(response.ResultCode)
	}
	return response.Order, nil
}

func (a *ANX) Send(currency, address, otp, amount string) (string, error) {
	request := make(map[string]interface{})
	request["ccy"] = currency
	request["amount"] = amount
	request["address"] = address

	if otp != "" {
		request["otp"] = otp
	}

	type SendResponse struct {
		TransactionID string `json:"transactionId"`
		ResultCode    string `json:"resultCode"`
		Timestamp     int64  `json:"timestamp"`
	}
	var response SendResponse

	err := a.SendAuthenticatedHTTPRequest(ANX_SEND, request, &response)

	if err != nil {
		return "", err
	}

	if response.ResultCode != "OK" {
		log.Printf("Response code is not OK: %s\n", response.ResultCode)
		return "", errors.New(response.ResultCode)
	}
	return response.TransactionID, nil
}

func (a *ANX) CreateNewSubAccount(currency, name string) (string, error) {
	request := make(map[string]interface{})
	request["ccy"] = currency
	request["customRef"] = name

	type SubaccountResponse struct {
		SubAccount string `json:"subAccount"`
		ResultCode string `json:"resultCode"`
		Timestamp  int64  `json:"timestamp"`
	}
	var response SubaccountResponse

	err := a.SendAuthenticatedHTTPRequest(ANX_SUBACCOUNT_NEW, request, &response)

	if err != nil {
		return "", err
	}

	if response.ResultCode != "OK" {
		log.Printf("Response code is not OK: %s\n", response.ResultCode)
		return "", errors.New(response.ResultCode)
	}
	return response.SubAccount, nil
}

func (a *ANX) GetDepositAddress(currency, name string, new bool) (string, error) {
	request := make(map[string]interface{})
	request["ccy"] = currency

	if name != "" {
		request["subAccount"] = name
	}

	type AddressResponse struct {
		Address    string `json:"address"`
		SubAccount string `json:"subAccount"`
		ResultCode string `json:"resultCode"`
		Timestamp  int64  `json:"timestamp"`
	}
	var response AddressResponse

	path := ANX_RECEIVE_ADDRESS
	if new {
		path = ANX_CREATE_ADDRESS
	}

	err := a.SendAuthenticatedHTTPRequest(path, request, &response)

	if err != nil {
		return "", err
	}

	if response.ResultCode != "OK" {
		log.Printf("Response code is not OK: %s\n", response.ResultCode)
		return "", errors.New(response.ResultCode)
	}

	return response.Address, nil
}

func (a *ANX) SendAuthenticatedHTTPRequest(path string, params map[string]interface{}, result interface{}) error {
	request := make(map[string]interface{})
	request["nonce"] = strconv.FormatInt(time.Now().UnixNano(), 10)[0:13]
	path = fmt.Sprintf("api/%s/%s", ANX_API_VERSION, path)

	if params != nil {
		for key, value := range params {
			request[key] = value
		}
	}

	PayloadJson, err := common.JSONEncode(request)

	if err != nil {
		return errors.New("SendAuthenticatedHTTPRequest: Unable to JSON request")
	}

	if a.Verbose {
		log.Printf("Request JSON: %s\n", PayloadJson)
	}

	hmac := common.GetHMAC(common.HASH_SHA512, []byte(path+string("\x00")+string(PayloadJson)), []byte(a.APISecret))
	headers := make(map[string]string)
	headers["Rest-Key"] = a.APIKey
	headers["Rest-Sign"] = common.Base64Encode([]byte(hmac))
	headers["Content-Type"] = "application/json"

	resp, err := common.SendHTTPRequest("POST", ANX_API_URL+path, headers, bytes.NewBuffer(PayloadJson))

	if a.Verbose {
		log.Printf("Recieved raw: \n%s\n", resp)
	}

	err = common.JSONDecode([]byte(resp), &result)

	if err != nil {
		return errors.New("Unable to JSON Unmarshal response.")
	}

	return nil
}
