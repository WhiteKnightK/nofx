package trader

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// BitgetTrader Bitget交易器
type BitgetTrader struct {
	apiKey     string
	secretKey  string
	passphrase string
	baseURL    string
	client     *http.Client

	// 余额缓存
	cachedBalance     map[string]interface{}
	balanceCacheTime  time.Time
	balanceCacheMutex sync.RWMutex

	// 持仓缓存
	cachedPositions     []map[string]interface{}
	positionsCacheTime  time.Time
	positionsCacheMutex sync.RWMutex

	// 缓存有效期（15秒）
	cacheDuration time.Duration
}

// min 返回两个整数中的较小值
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// NewBitgetTrader 创建Bitget交易器
func NewBitgetTrader(apiKey, secretKey, passphrase string, testnet bool) *BitgetTrader {
	baseURL := "https://api.bitget.com"
	if testnet {
		baseURL = "https://testnet.bitget.com"
	}

	return &BitgetTrader{
		apiKey:        apiKey,
		secretKey:     secretKey,
		passphrase:    passphrase,
		baseURL:       baseURL,
		client:        &http.Client{Timeout: 30 * time.Second},
		cacheDuration: 15 * time.Second,
	}
}

// sign 生成签名
// Bitget签名: Base64(HMAC-SHA256(timestamp + method + requestPath + body, secretKey))
func (t *BitgetTrader) sign(timestamp, method, requestPath, body string) string {
	message := timestamp + strings.ToUpper(method) + requestPath + body
	h := hmac.New(sha256.New, []byte(t.secretKey))
	h.Write([]byte(message))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// request 发送HTTP请求
func (t *BitgetTrader) request(method, endpoint string, params map[string]string, body interface{}) ([]byte, error) {
	// 构建URL和签名路径（需要参数顺序一致）
	var queryString string
	if len(params) > 0 && method == "GET" {
		// 对参数键进行排序，确保每次顺序一致
		keys := make([]string, 0, len(params))
		for k := range params {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		queryParts := make([]string, 0, len(keys))
		for _, k := range keys {
			queryParts = append(queryParts, fmt.Sprintf("%s=%s", k, params[k]))
		}
		queryString = strings.Join(queryParts, "&")
	}

	// 构建URL
	url := t.baseURL + endpoint
	if queryString != "" {
		url += "?" + queryString
	}

	// 构建请求体
	var bodyStr string
	if body != nil {
		bodyBytes, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal body failed: %w", err)
		}
		bodyStr = string(bodyBytes)
	}

	// 创建请求
	var req *http.Request
	var err error
	if bodyStr != "" {
		req, err = http.NewRequest(method, url, strings.NewReader(bodyStr))
	} else {
		req, err = http.NewRequest(method, url, nil)
	}
	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}

	// 获取请求路径（用于签名）
	requestPath := endpoint
	if queryString != "" {
		requestPath += "?" + queryString
	}

	// 生成时间戳（毫秒）
	timestamp := strconv.FormatInt(time.Now().UnixMilli(), 10)

	// 生成签名
	sign := t.sign(timestamp, method, requestPath, bodyStr)

	// 设置请求头
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("ACCESS-KEY", t.apiKey)
	req.Header.Set("ACCESS-SIGN", sign)
	req.Header.Set("ACCESS-TIMESTAMP", timestamp)
	req.Header.Set("ACCESS-PASSPHRASE", t.passphrase)
	req.Header.Set("locale", "zh-CN")

	// 发送请求
	resp, err := t.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response failed: %w", err)
	}

	// 检查HTTP状态码
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http %d: %s", resp.StatusCode, string(respBody))
	}

	// 解析响应检查业务错误码
	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("unmarshal response failed: %w", err)
	}

	code, ok := result["code"].(string)
	if !ok || code != "00000" {
		msg, _ := result["msg"].(string)
		return nil, fmt.Errorf("bitget api error: code=%s, msg=%s", code, msg)
	}

	return respBody, nil
}

// GetBalance 获取账户余额
func (t *BitgetTrader) GetBalance() (map[string]interface{}, error) {
	// 检查缓存
	t.balanceCacheMutex.RLock()
	if t.cachedBalance != nil && time.Since(t.balanceCacheTime) < t.cacheDuration {
		t.balanceCacheMutex.RUnlock()
		log.Printf("✓ 使用缓存的账户余额（缓存时间: %.1f秒前）", time.Since(t.balanceCacheTime).Seconds())
		return t.cachedBalance, nil
	}
	t.balanceCacheMutex.RUnlock()

	log.Printf("🔄 正在调用Bitget API获取账户余额...")

	// 调用API: GET /api/v2/mix/account/accounts
	respBody, err := t.request("GET", "/api/v2/mix/account/accounts", map[string]string{
		"productType": "USDT-FUTURES",
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("get balance failed: %w", err)
	}

	// 解析响应
	var response struct {
		Code string `json:"code"`
		Msg  string `json:"msg"`
		Data []struct {
			MarginCoin        string `json:"marginCoin"`
			Equity            string `json:"equity"`
			Available         string `json:"available"`
			UnrealizedPL      string `json:"unrealizedPL"`
			CrossMaxAvailable string `json:"crossMaxAvailable"`
		} `json:"data"`
	}

	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("parse balance response failed: %w", err)
	}

	// 查找USDT账户
	result := make(map[string]interface{})
	for _, account := range response.Data {
		if account.MarginCoin == "USDT" {
			equity, _ := strconv.ParseFloat(account.Equity, 64)
			available, _ := strconv.ParseFloat(account.Available, 64)
			unrealizedPL, _ := strconv.ParseFloat(account.UnrealizedPL, 64)

			// 调试日志：打印原始API返回值
			log.Printf("📊 Bitget API 原始数据:")
			log.Printf("  - Equity (权益): %s -> %.2f", account.Equity, equity)
			log.Printf("  - Available (可用): %s -> %.2f", account.Available, available)
			log.Printf("  - UnrealizedPL (未实现盈亏): %s -> %.2f", account.UnrealizedPL, unrealizedPL)
			log.Printf("  - CrossMaxAvailable: %s", account.CrossMaxAvailable)

			// 🔧 修复：Bitget的equity字段可能为空，使用available作为总权益
			// 因为没有持仓时，可用余额就是总权益
			totalEquity := equity
			if totalEquity == 0 && available > 0 {
				totalEquity = available
				log.Printf("⚠️ Bitget Equity字段为空，使用Available作为总权益: %.2f", totalEquity)
			}

			result["totalWalletBalance"] = totalEquity
			result["availableBalance"] = available
			result["totalUnrealizedProfit"] = unrealizedPL

			log.Printf("✓ Bitget API返回: 总余额=%.2f, 可用=%.2f, 未实现盈亏=%.2f", totalEquity, available, unrealizedPL)
			break
		}
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("USDT account not found")
	}

	// 更新缓存
	t.balanceCacheMutex.Lock()
	t.cachedBalance = result
	t.balanceCacheTime = time.Now()
	t.balanceCacheMutex.Unlock()

	return result, nil
}

// GetPositions 获取所有持仓
func (t *BitgetTrader) GetPositions() ([]map[string]interface{}, error) {
	// 检查缓存
	t.positionsCacheMutex.RLock()
	if t.cachedPositions != nil && time.Since(t.positionsCacheTime) < t.cacheDuration {
		t.positionsCacheMutex.RUnlock()
		log.Printf("✓ 使用缓存的持仓信息（缓存时间: %.1f秒前）", time.Since(t.positionsCacheTime).Seconds())
		return t.cachedPositions, nil
	}
	t.positionsCacheMutex.RUnlock()

	log.Printf("🔄 正在调用Bitget API获取持仓信息...")

	// 调用API: GET /api/v2/mix/position/all-position
	respBody, err := t.request("GET", "/api/v2/mix/position/all-position", map[string]string{
		"productType": "USDT-FUTURES",
		"marginCoin":  "USDT",
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("get positions failed: %w", err)
	}

	// 解析响应
	var response struct {
		Code string `json:"code"`
		Msg  string `json:"msg"`
		Data []struct {
			Symbol           string `json:"symbol"`
			Total            string `json:"total"`
			Available        string `json:"available"`
			OpenPriceAvg     string `json:"openPriceAvg"`
			MarkPrice        string `json:"markPrice"`
			UnrealizedPL     string `json:"unrealizedPL"`
			Leverage         string `json:"leverage"`
			LiquidationPrice string `json:"liquidationPrice"`
			HoldSide         string `json:"holdSide"`   // long/short
			MarginMode       string `json:"marginMode"` // crossed/isolated
		} `json:"data"`
	}

	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("parse positions response failed: %w", err)
	}

	var result []map[string]interface{}
	for _, pos := range response.Data {
		total, _ := strconv.ParseFloat(pos.Total, 64)
		if total == 0 {
			continue // 跳过无持仓的
		}

		posMap := make(map[string]interface{})
		posMap["symbol"] = pos.Symbol
		posMap["positionAmt"] = total
		if avail, err := strconv.ParseFloat(pos.Available, 64); err == nil {
			posMap["available"] = avail
		} else {
			posMap["available"] = total
		}
		posMap["entryPrice"], _ = strconv.ParseFloat(pos.OpenPriceAvg, 64)
		posMap["markPrice"], _ = strconv.ParseFloat(pos.MarkPrice, 64)
		posMap["unRealizedProfit"], _ = strconv.ParseFloat(pos.UnrealizedPL, 64)
		posMap["leverage"], _ = strconv.ParseFloat(pos.Leverage, 64)
		posMap["liquidationPrice"], _ = strconv.ParseFloat(pos.LiquidationPrice, 64)
		posMap["side"] = pos.HoldSide         // long/short
		posMap["marginMode"] = pos.MarginMode // crossed / isolated

		result = append(result, posMap)
	}

	// 更新缓存
	t.positionsCacheMutex.Lock()
	t.cachedPositions = result
	t.positionsCacheTime = time.Now()
	t.positionsCacheMutex.Unlock()

	return result, nil
}

// OpenLong 开多仓
func (t *BitgetTrader) OpenLong(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	log.Printf("📊 开多仓: %s 数量: %.4f 杠杆: %dx", symbol, quantity, leverage)

	// 格式化数量
	quantityStr, err := t.FormatQuantity(symbol, quantity)
	if err != nil {
		return nil, err
	}

	// POST /api/v2/mix/order/place-order
	body := map[string]interface{}{
		"symbol":      symbol,
		"productType": "USDT-FUTURES",
		"marginMode":  "crossed", // 全仓模式
		"marginCoin":  "USDT",
		"side":        "buy",  // 买入开多
		"tradeSide":   "open", // 开仓
		"orderType":   "market",
		"size":        quantityStr,
	}

	respBody, err := t.request("POST", "/api/v2/mix/order/place-order", nil, body)
	if err != nil {
		return nil, fmt.Errorf("open long failed: %w", err)
	}

	// 解析响应
	var response struct {
		Code string `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			OrderId   string `json:"orderId"`
			ClientOid string `json:"clientOid"`
		} `json:"data"`
	}

	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("parse response failed: %w", err)
	}

	log.Printf("✓ 开多仓成功: %s 订单ID: %s", symbol, response.Data.OrderId)

	result := make(map[string]interface{})
	result["orderId"] = response.Data.OrderId
	result["symbol"] = symbol
	result["status"] = "NEW"

	return result, nil
}

// OpenShort 开空仓
func (t *BitgetTrader) OpenShort(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	log.Printf("📊 开空仓: %s 数量: %.4f 杠杆: %dx", symbol, quantity, leverage)

	quantityStr, err := t.FormatQuantity(symbol, quantity)
	if err != nil {
		return nil, err
	}

	body := map[string]interface{}{
		"symbol":      symbol,
		"productType": "USDT-FUTURES",
		"marginMode":  "crossed",
		"marginCoin":  "USDT",
		"side":        "sell", // 卖出开空
		"tradeSide":   "open", // 开仓
		"orderType":   "market",
		"size":        quantityStr,
	}

	respBody, err := t.request("POST", "/api/v2/mix/order/place-order", nil, body)
	if err != nil {
		return nil, fmt.Errorf("open short failed: %w", err)
	}

	var response struct {
		Code string `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			OrderId   string `json:"orderId"`
			ClientOid string `json:"clientOid"`
		} `json:"data"`
	}

	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("parse response failed: %w", err)
	}

	log.Printf("✓ 开空仓成功: %s 订单ID: %s", symbol, response.Data.OrderId)

	result := make(map[string]interface{})
	result["orderId"] = response.Data.OrderId
	result["symbol"] = symbol
	result["status"] = "NEW"

	return result, nil
}

// CloseLong 平多仓（使用 Bitget 官方一键平仓接口）
// 参考文档：https://www.bitget.com/zh-CN/api-doc/contract/trade/Flash-Close-Position
func (t *BitgetTrader) CloseLong(symbol string, quantity float64) (map[string]interface{}, error) {
	log.Printf("📊 平多仓: %s（使用一键市价平仓接口）", symbol)

	// 先强制刷新一次持仓，避免使用旧缓存导致“已平仓仍再次平”的情况
	t.positionsCacheMutex.Lock()
	t.positionsCacheTime = time.Time{}
	t.positionsCacheMutex.Unlock()
	positions, err := t.GetPositions()
	if err == nil {
		hasLong := false
		for _, pos := range positions {
			if pos["symbol"] == symbol && pos["side"] == "long" {
				hasLong = true
				break
			}
		}
		if !hasLong {
			return nil, fmt.Errorf("没有找到 %s 的多仓（可能已在上一笔操作中平掉）", symbol)
		}
	}

	// Bitget 官方一键平仓接口优点：
	// 1. 自动撤销该方向所有挂单（含止盈止损）
	// 2. 自动获取可平数量，无需手动指定 size
	// 3. 参数简洁，避免 marginMode/tradeSide 等复杂参数导致的 22002 错误
	body := map[string]interface{}{
		"symbol":      symbol,
		"productType": "USDT-FUTURES",
		"holdSide":    "long", // 平多仓
	}

	respBody, err := t.request("POST", "/api/v2/mix/order/close-positions", nil, body)
	if err != nil {
		return nil, fmt.Errorf("close long failed: %w", err)
	}

	// 解析返回结果
	var response struct {
		Code string `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			SuccessList []struct {
				OrderId   string `json:"orderId"`
				ClientOid string `json:"clientOid"`
				Symbol    string `json:"symbol"`
			} `json:"successList"`
			FailureList []struct {
				OrderId   string `json:"orderId"`
				ClientOid string `json:"clientOid"`
				Symbol    string `json:"symbol"`
				ErrorMsg  string `json:"errorMsg"`
				ErrorCode string `json:"errorCode"`
			} `json:"failureList"`
		} `json:"data"`
	}

	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("parse response failed: %w", err)
	}

	// 检查是否有失败记录
	if len(response.Data.FailureList) > 0 {
		fail := response.Data.FailureList[0]
		return nil, fmt.Errorf("平仓失败: %s (错误码: %s)", fail.ErrorMsg, fail.ErrorCode)
	}

	// 检查是否有成功记录
	if len(response.Data.SuccessList) == 0 {
		return nil, fmt.Errorf("平仓失败: 无成功记录")
	}

	success := response.Data.SuccessList[0]
	log.Printf("✓ 平多仓成功: %s 订单ID: %s", symbol, success.OrderId)
	// 成功后立即失效本地持仓缓存，确保后续读取到最新状态
	t.positionsCacheMutex.Lock()
	t.positionsCacheTime = time.Time{}
	t.positionsCacheMutex.Unlock()

	result := make(map[string]interface{})
	result["orderId"] = success.OrderId
	result["symbol"] = success.Symbol
	result["status"] = "NEW"

	return result, nil
}

// CloseShort 平空仓（使用 Bitget 官方一键平仓接口）
// 参考文档：https://www.bitget.com/zh-CN/api-doc/contract/trade/Flash-Close-Position
func (t *BitgetTrader) CloseShort(symbol string, quantity float64) (map[string]interface{}, error) {
	log.Printf("📊 平空仓: %s（使用一键市价平仓接口）", symbol)

	// 先强制刷新一次持仓，避免使用旧缓存导致“已平仓仍再次平”的情况
	t.positionsCacheMutex.Lock()
	t.positionsCacheTime = time.Time{}
	t.positionsCacheMutex.Unlock()
	positions, err := t.GetPositions()
	if err == nil {
		hasShort := false
		for _, pos := range positions {
			if pos["symbol"] == symbol && pos["side"] == "short" {
				hasShort = true
				break
			}
		}
		if !hasShort {
			return nil, fmt.Errorf("没有找到 %s 的空仓（可能已在上一笔操作中平掉）", symbol)
		}
	}

	// Bitget 官方一键平仓接口优点：
	// 1. 自动撤销该方向所有挂单（含止盈止损）
	// 2. 自动获取可平数量，无需手动指定 size
	// 3. 参数简洁，避免 marginMode/tradeSide 等复杂参数导致的 22002 错误
	body := map[string]interface{}{
		"symbol":      symbol,
		"productType": "USDT-FUTURES",
		"holdSide":    "short", // 平空仓
	}

	respBody, err := t.request("POST", "/api/v2/mix/order/close-positions", nil, body)
	if err != nil {
		return nil, fmt.Errorf("close short failed: %w", err)
	}

	// 解析返回结果
	var response struct {
		Code string `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			SuccessList []struct {
				OrderId   string `json:"orderId"`
				ClientOid string `json:"clientOid"`
				Symbol    string `json:"symbol"`
			} `json:"successList"`
			FailureList []struct {
				OrderId   string `json:"orderId"`
				ClientOid string `json:"clientOid"`
				Symbol    string `json:"symbol"`
				ErrorMsg  string `json:"errorMsg"`
				ErrorCode string `json:"errorCode"`
			} `json:"failureList"`
		} `json:"data"`
	}

	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("parse response failed: %w", err)
	}

	// 检查是否有失败记录
	if len(response.Data.FailureList) > 0 {
		fail := response.Data.FailureList[0]
		return nil, fmt.Errorf("平仓失败: %s (错误码: %s)", fail.ErrorMsg, fail.ErrorCode)
	}

	// 检查是否有成功记录
	if len(response.Data.SuccessList) == 0 {
		return nil, fmt.Errorf("平仓失败: 无成功记录")
	}

	success := response.Data.SuccessList[0]
	log.Printf("✓ 平空仓成功: %s 订单ID: %s", symbol, success.OrderId)
	// 成功后立即失效本地持仓缓存，确保后续读取到最新状态
	t.positionsCacheMutex.Lock()
	t.positionsCacheTime = time.Time{}
	t.positionsCacheMutex.Unlock()

	result := make(map[string]interface{})
	result["orderId"] = success.OrderId
	result["symbol"] = success.Symbol
	result["status"] = "NEW"

	return result, nil
}

// SetLeverage 设置杠杆
func (t *BitgetTrader) SetLeverage(symbol string, leverage int) error {
	log.Printf("⚙️ 设置杠杆: %s %dx", symbol, leverage)

	// POST /api/v2/mix/account/set-leverage
	body := map[string]interface{}{
		"symbol":      symbol,
		"productType": "USDT-FUTURES",
		"marginCoin":  "USDT",
		"leverage":    strconv.Itoa(leverage),
		"holdSide":    "long", // 多空共用杠杆
	}

	_, err := t.request("POST", "/api/v2/mix/account/set-leverage", nil, body)
	if err != nil {
		return fmt.Errorf("set leverage failed: %w", err)
	}

	log.Printf("✓ 杠杆设置成功: %s %dx", symbol, leverage)
	return nil
}

// SetMarginMode 设置仓位模式
func (t *BitgetTrader) SetMarginMode(symbol string, isCrossMargin bool) error {
	marginMode := "crossed"
	marginModeStr := "全仓"
	if !isCrossMargin {
		marginMode = "isolated"
		marginModeStr = "逐仓"
	}

	log.Printf("⚙️ 设置仓位模式: %s %s", symbol, marginModeStr)

	// POST /api/v2/mix/account/set-margin-mode
	body := map[string]interface{}{
		"symbol":      symbol,
		"productType": "USDT-FUTURES",
		"marginCoin":  "USDT",
		"marginMode":  marginMode,
	}

	_, err := t.request("POST", "/api/v2/mix/account/set-margin-mode", nil, body)
	if err != nil {
		// 如果错误信息包含"No need to change"，忽略
		if strings.Contains(err.Error(), "No need to change") || strings.Contains(err.Error(), "40772") {
			log.Printf("  ✓ %s 仓位模式已是 %s", symbol, marginModeStr)
			return nil
		}
		return fmt.Errorf("set margin mode failed: %w", err)
	}

	log.Printf("✓ 仓位模式设置成功: %s %s", symbol, marginModeStr)
	return nil
}

// GetMarketPrice 获取市场价格
func (t *BitgetTrader) GetMarketPrice(symbol string) (float64, error) {
	// GET /api/v2/mix/market/ticker
	respBody, err := t.request("GET", "/api/v2/mix/market/ticker", map[string]string{
		"symbol":      symbol,
		"productType": "USDT-FUTURES",
	}, nil)
	if err != nil {
		return 0, fmt.Errorf("get market price failed: %w", err)
	}

	var response struct {
		Code string `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			LastPr string `json:"lastPr"`
		} `json:"data"`
	}

	if err := json.Unmarshal(respBody, &response); err != nil {
		return 0, fmt.Errorf("parse response failed: %w", err)
	}

	price, err := strconv.ParseFloat(response.Data.LastPr, 64)
	if err != nil {
		return 0, fmt.Errorf("parse price failed: %w", err)
	}

	return price, nil
}

// SetStopLoss 设置止损单
func (t *BitgetTrader) SetStopLoss(symbol string, positionSide string, quantity, stopPrice float64) error {
	log.Printf("  🛡️ 设置止损: %s %s 数量: %.4f 止损价: %.4f", symbol, positionSide, quantity, stopPrice)

	quantityStr, err := t.FormatQuantity(symbol, quantity)
	if err != nil {
		return err
	}

	// 确定持仓方向
	var holdSide string
	if positionSide == "LONG" {
		holdSide = "long"
	} else {
		holdSide = "short"
	}

	// POST /api/v2/mix/order/place-tpsl-order (专用止盈止损接口)
	body := map[string]interface{}{
		"marginCoin":   "USDT",
		"productType":  "usdt-futures", // ⚠️ 小写！官方文档要求
		"symbol":       symbol,
		"planType":     "loss_plan", // 止损计划
		"triggerPrice": fmt.Sprintf("%.8f", stopPrice),
		"triggerType":  "mark_price", // 标记价格触发
		"executePrice": "0",          // 0=市价执行
		"holdSide":     holdSide,
		"size":         quantityStr,
	}

	_, err = t.request("POST", "/api/v2/mix/order/place-tpsl-order", nil, body)
	if err != nil {
		return fmt.Errorf("set stop loss failed: %w", err)
	}

	log.Printf("  ✓ 止损设置成功: %.4f", stopPrice)
	return nil
}

// SetTakeProfit 设置止盈单
func (t *BitgetTrader) SetTakeProfit(symbol string, positionSide string, quantity, takeProfitPrice float64) error {
	log.Printf("  💰 设置止盈: %s %s 数量: %.4f 止盈价: %.4f", symbol, positionSide, quantity, takeProfitPrice)

	quantityStr, err := t.FormatQuantity(symbol, quantity)
	if err != nil {
		return err
	}

	// 确定持仓方向
	var holdSide string
	if positionSide == "LONG" {
		holdSide = "long"
	} else {
		holdSide = "short"
	}

	// POST /api/v2/mix/order/place-tpsl-order (专用止盈止损接口)
	body := map[string]interface{}{
		"marginCoin":   "USDT",
		"productType":  "usdt-futures", // ⚠️ 小写！官方文档要求
		"symbol":       symbol,
		"planType":     "profit_plan", // 止盈计划
		"triggerPrice": fmt.Sprintf("%.8f", takeProfitPrice),
		"triggerType":  "mark_price", // 标记价格触发
		"executePrice": "0",          // 0=市价执行
		"holdSide":     holdSide,
		"size":         quantityStr,
	}

	_, err = t.request("POST", "/api/v2/mix/order/place-tpsl-order", nil, body)
	if err != nil {
		return fmt.Errorf("set take profit failed: %w", err)
	}

	log.Printf("  ✓ 止盈设置成功: %.4f", takeProfitPrice)
	return nil
}

// CancelStopLossOrders 仅取消止损单（使用 Bitget 计划委托撤单接口）
func (t *BitgetTrader) CancelStopLossOrders(symbol string) error {
	log.Printf("  🗑️ 取消止损单: %s", symbol)
	// 使用 cancel-plan-order 接口，planType="loss_plan" 表示止损单
	return t.cancelPlanOrders(symbol, "loss_plan")
}

// CancelTakeProfitOrders 仅取消止盈单（使用 Bitget 计划委托撤单接口）
func (t *BitgetTrader) CancelTakeProfitOrders(symbol string) error {
	log.Printf("  🗑️ 取消止盈单: %s", symbol)
	// 使用 cancel-plan-order 接口，planType="profit_plan" 表示止盈单
	return t.cancelPlanOrders(symbol, "profit_plan")
}

// cancelPlanOrders 取消指定类型的计划委托单（内部方法）
// planType: "loss_plan"（止损）| "profit_plan"（止盈）| "normal_plan"（普通计划委托）| "pos_loss"（仓位止损）| "pos_profit"（仓位止盈）| "moving_plan"（移动止盈止损）
func (t *BitgetTrader) cancelPlanOrders(symbol string, planType string) error {
	// POST /api/v2/mix/order/cancel-plan-order
	// 参考文档：https://www.bitget.com/zh-CN/api-doc/contract/plan/Cancel-Plan-Order
	body := map[string]interface{}{
		"symbol":      symbol,
		"productType": "USDT-FUTURES",
		"marginCoin":  "USDT",   // 🔑 必填参数：保证金币种（必须大写）
		"planType":    planType, // loss_plan=止损, profit_plan=止盈
	}

	_, err := t.request("POST", "/api/v2/mix/order/cancel-plan-order", nil, body)
	if err != nil {
		// 如果返回 "暂无委托可撤销"（43025 或 22001），不视为错误
		if strings.Contains(err.Error(), "43025") || strings.Contains(err.Error(), "22001") {
			log.Printf("  ℹ️  %s 没有 %s 类型的计划单需要取消", symbol, planType)
			return nil
		}
		return fmt.Errorf("cancel plan orders failed: %w", err)
	}

	log.Printf("  ✓ 已取消 %s 的 %s 计划单", symbol, planType)
	return nil
}

// CancelAllOrders 取消该币种的所有限价/市价委托单（不含计划单）
func (t *BitgetTrader) CancelAllOrders(symbol string) error {
	// POST /api/v2/mix/order/cancel-all-orders
	body := map[string]interface{}{
		"symbol":      symbol,
		"productType": "USDT-FUTURES",
		"marginCoin":  "USDT",
	}

	_, err := t.request("POST", "/api/v2/mix/order/cancel-all-orders", nil, body)
	if err != nil {
		// 如果返回 "暂无委托可撤销"，不视为错误
		if strings.Contains(err.Error(), "22001") {
			log.Printf("  ℹ️  %s 没有普通委托单需要取消", symbol)
			return nil
		}
		return fmt.Errorf("cancel all orders failed: %w", err)
	}

	log.Printf("  ✓ 已取消 %s 的所有普通委托单", symbol)
	return nil
}

// CancelStopOrders 取消该币种的止盈/止损计划单
func (t *BitgetTrader) CancelStopOrders(symbol string) error {
	// 同时取消止损和止盈单
	errLoss := t.CancelStopLossOrders(symbol)
	errProfit := t.CancelTakeProfitOrders(symbol)

	// 只要有一个成功就视为成功
	if errLoss != nil && errProfit != nil {
		return fmt.Errorf("cancel stop orders failed: loss=%v, profit=%v", errLoss, errProfit)
	}
	return nil
}

// FormatQuantity 格式化数量到正确的精度
func (t *BitgetTrader) FormatQuantity(symbol string, quantity float64) (string, error) {
	// GET /api/v2/mix/market/contracts
	respBody, err := t.request("GET", "/api/v2/mix/market/contracts", map[string]string{
		"symbol":      symbol,
		"productType": "USDT-FUTURES",
	}, nil)
	if err != nil {
		// 如果获取失败，使用默认精度
		log.Printf("⚠️ 获取交易规则失败，使用默认精度: %v", err)
		return fmt.Sprintf("%.4f", quantity), nil
	}

	var response struct {
		Code string `json:"code"`
		Msg  string `json:"msg"`
		Data []struct {
			Symbol         string `json:"symbol"`
			SizeMultiplier string `json:"sizeMultiplier"` // 数量精度
			MinTradeNum    string `json:"minTradeNum"`    // 最小下单数量
		} `json:"data"`
	}

	if err := json.Unmarshal(respBody, &response); err != nil {
		log.Printf("⚠️ 解析交易规则失败，使用默认精度: %v", err)
		return fmt.Sprintf("%.4f", quantity), nil
	}

	if len(response.Data) == 0 {
		log.Printf("⚠️ 未找到 %s 的交易规则，使用默认精度", symbol)
		return fmt.Sprintf("%.4f", quantity), nil
	}

	// 计算精度
	sizeMultiplier := response.Data[0].SizeMultiplier
	precision := 0
	if strings.Contains(sizeMultiplier, ".") {
		parts := strings.Split(sizeMultiplier, ".")
		precision = len(parts[1])
	}

	format := fmt.Sprintf("%%.%df", precision)
	return fmt.Sprintf(format, quantity), nil
}

// GetOpenOrders 获取当前未成交的委托单（含止盈止损计划单）
// 返回格式统一为：type (limit/market/stop_loss/take_profit), price, quantity, side, status
func (t *BitgetTrader) GetOpenOrders(symbol string) ([]map[string]interface{}, error) {
	result := []map[string]interface{}{}

	// 1. 获取普通委托单（限价/市价）
	// GET /api/v2/mix/order/orders-pending
	pendingParams := map[string]string{
		"productType": "USDT-FUTURES",
		"marginCoin":  "USDT", // 必填：保证金币种
	}
	// symbol 允许为空，为空时查询账号下所有该品种的未成交委托
	if symbol != "" {
		pendingParams["symbol"] = symbol
	}
	pendingBody, err := t.request("GET", "/api/v2/mix/order/orders-pending", pendingParams, nil)

	if err != nil {
		log.Printf("⚠️ [委托查询] 获取普通委托单失败 symbol=%s err=%v", symbol, err)
	} else {
		var pendingResp struct {
			Code string `json:"code"`
			Msg  string `json:"msg"`
			Data struct {
				EntrustedList []struct {
					OrderId    string `json:"orderId"`
					ClientOid  string `json:"clientOid"`
					Symbol     string `json:"symbol"`
					Size       string `json:"size"`
					FilledSize string `json:"filledSize"`
					Price      string `json:"price"`
					OrderType  string `json:"orderType"` // limit, market
					Side       string `json:"side"`      // open_long, open_short, close_long, close_short
					Status     string `json:"status"`    // live, partially_filled
					CTime      string `json:"cTime"`     // 创建时间(毫秒时间戳)
					PriceAvg   string `json:"priceAvg"`  // 成交均价
				} `json:"entrustedList"`
			} `json:"data"`
		}

		if err := json.Unmarshal(pendingBody, &pendingResp); err != nil {
			log.Printf("⚠️ [委托查询] 解析普通委托响应失败 symbol=%s err=%v body=%s", symbol, err, string(pendingBody))
		} else if pendingResp.Code != "00000" {
			log.Printf("⚠️ [委托查询] Bitget返回错误 symbol=%s code=%s msg=%s", symbol, pendingResp.Code, pendingResp.Msg)
		} else {
			log.Printf("✓ [委托查询] 普通委托: %s 找到 %d 个", symbol, len(pendingResp.Data.EntrustedList))
			for _, order := range pendingResp.Data.EntrustedList {
				price, _ := strconv.ParseFloat(order.Price, 64)
				quantity, _ := strconv.ParseFloat(order.Size, 64)
				filledSize, _ := strconv.ParseFloat(order.FilledSize, 64)
				avgPrice, _ := strconv.ParseFloat(order.PriceAvg, 64)

				result = append(result, map[string]interface{}{
					"order_id":       order.OrderId,
					"symbol":         order.Symbol,
					"type":           order.OrderType,
					"price":          price,
					"quantity":       quantity,
					"filled_size":    filledSize,
					"avg_price":      avgPrice,
					"side":           order.Side,
					"status":         order.Status,
					"created_at":     order.CTime,
					"client_oid":     order.ClientOid,
					"order_category": "normal",
				})
			}
		}
	}

	// 2. 获取计划委托单（止盈/止损）
	// 由于Bitget要求planType必填，且不支持一次查询所有，我们需要分别查询 "profit_plan" (止盈) 和 "loss_plan" (止损)
	planTypes := []string{"profit_plan", "loss_plan"}

	for _, pType := range planTypes {
		planParams := map[string]string{
			"productType": "USDT-FUTURES",
			"marginCoin":  "USDT",
			"planType":    pType, // 分别查询
		}
		// symbol 允许为空，为空时查询该planType下所有交易对
		if symbol != "" {
			planParams["symbol"] = symbol
		}

		planBody, err := t.request("GET", "/api/v2/mix/order/orders-plan-pending", planParams, nil)

		if err != nil {
			// 忽略部分错误，继续查询下一个
			log.Printf("⚠️ [委托查询] 获取计划委托(%s)失败 symbol=%s err=%v", pType, symbol, err)
			continue
		}

		var planResp struct {
			Code string `json:"code"`
			Msg  string `json:"msg"`
			Data struct {
				EntrustedList []struct {
					OrderId      string `json:"orderId"`
					Symbol       string `json:"symbol"`
					PlanType     string `json:"planType"`
					TriggerPrice string `json:"triggerPrice"`
					Size         string `json:"size"`
					HoldSide     string `json:"holdSide"`
					Status       string `json:"status"`
					CTime        string `json:"cTime"`
				} `json:"entrustedList"`
			} `json:"data"`
		}

		if err := json.Unmarshal(planBody, &planResp); err != nil {
			log.Printf("⚠️ [委托查询] 解析计划委托(%s)响应失败: %v", pType, err)
			continue
		}

		if planResp.Code != "00000" {
			// Code 43025 = 暂无数据，忽略
			if planResp.Code != "43025" {
				log.Printf("⚠️ [委托查询] Bitget返回错误(%s) code=%s msg=%s", pType, planResp.Code, planResp.Msg)
			}
			continue
		}

		log.Printf("✓ [委托查询] 计划委托(%s): %s 找到 %d 个", pType, symbol, len(planResp.Data.EntrustedList))

		for _, plan := range planResp.Data.EntrustedList {
			triggerPrice, _ := strconv.ParseFloat(plan.TriggerPrice, 64)
			quantity, _ := strconv.ParseFloat(plan.Size, 64)

			var orderType string
			if plan.PlanType == "profit_plan" {
				orderType = "take_profit"
			} else if plan.PlanType == "loss_plan" {
				orderType = "stop_loss"
			} else {
				orderType = plan.PlanType
			}

			result = append(result, map[string]interface{}{
				"order_id":   plan.OrderId,
				"symbol":     plan.Symbol,
				"type":       orderType,
				"price":      triggerPrice,
				"quantity":   quantity,
				"side":       plan.HoldSide,
				"status":     plan.Status,
				"created_at": plan.CTime,
				// 增加计划单特有标识
				"order_category": "plan",
				"plan_type":      plan.PlanType,
			})
		}
	}

	log.Printf("✓ 获取委托单成功: %s 共 %d 个", symbol, len(result))
	return result, nil
}
