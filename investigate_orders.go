package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Bitget API 配置
var (
	ApiKey    string
	SecretKey string
	Passphrase string
	BaseURL   = "https://api.bitget.com"
)

func main() {
	// 加载环境变量
	_ = godotenv.Load()

	// 尝试从环境变量获取（优先使用 BITGET_ 开头的，如果没有则尝试默认的）
	ApiKey = os.Getenv("BITGET_API_KEY")
	SecretKey = os.Getenv("BITGET_SECRET_KEY") 
	Passphrase = os.Getenv("BITGET_PASSPHRASE")

    if ApiKey == "" {
        // 尝试从默认用户 default_trader 获取 (仅作示例，实际需根据配置)
        // 简单起见，这里假设用户已经配置好 .env 或者直接在这里填入测试 key
        // 为了安全，最好不要硬编码。如果 .env 没有，脚本将失败。
        log.Fatal("❌ 请设置环境变量 BITGET_API_KEY, BITGET_SECRET_KEY, BITGET_PASSPHRASE")
    }

	symbol := "ETHUSDT" // 默认测试 ETHUSDT
	if len(os.Args) > 1 {
		symbol = os.Args[1]
	}

	fmt.Printf("🔍 开始调查 %s 的订单情况...\n", symbol)

	// 1. 查询普通委托 (Pending Orders)
	checkPendingOrders(symbol)

	// 2. 查询各种类型的计划委托 (Plan Orders)
	planTypes := []string{"profit_plan", "loss_plan", "normal_plan", "pos_profit", "pos_loss"}
	for _, pt := range planTypes {
		checkPlanOrders(symbol, pt)
	}
}

func checkPendingOrders(symbol string) {
	fmt.Println("\n--- 普通委托 (orders-pending) ---")
	params := map[string]string{
		"productType": "USDT-FUTURES", // 尝试大写
		"marginCoin":  "USDT",
		"symbol":      symbol,
	}
	doRequest("GET", "/api/v2/mix/order/orders-pending", params)
}

func checkPlanOrders(symbol string, planType string) {
	fmt.Printf("\n--- 计划委托 (orders-plan-pending) type=%s ---\n", planType)
	params := map[string]string{
		"productType": "usdt-futures", // 尝试小写 (根据之前经验)
		"marginCoin":  "USDT",
		"planType":    planType,
		"symbol":      symbol,
	}
	doRequest("GET", "/api/v2/mix/order/orders-plan-pending", params)
}

func doRequest(method, path string, params map[string]string) {
	// 构造 URL 参数
	values := url.Values{}
	for k, v := range params {
		values.Add(k, v)
	}
	queryString := values.Encode()
	fullURL := BaseURL + path
	if queryString != "" {
		fullURL += "?" + queryString
	}

	req, err := http.NewRequest(method, fullURL, nil)
	if err != nil {
		log.Printf("创建请求失败: %v", err)
		return
	}

	// 签名
	timestamp := fmt.Sprintf("%d", time.Now().UnixMilli())
	sign := generateSign(method, path, queryString, timestamp, SecretKey)

	req.Header.Set("ACCESS-KEY", ApiKey)
	req.Header.Set("ACCESS-SIGN", sign)
	req.Header.Set("ACCESS-TIMESTAMP", timestamp)
	req.Header.Set("ACCESS-PASSPHRASE", Passphrase)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("locale", "zh-CN")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("请求失败: %v", err)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	
	// 格式化输出 JSON
	var prettyJSON map[string]interface{}
	if err := json.Unmarshal(body, &prettyJSON); err == nil {
		formatted, _ := json.MarshalIndent(prettyJSON, "", "  ")
		fmt.Printf("Response:\n%s\n", string(formatted))
        
        // 简单统计
        if data, ok := prettyJSON["data"].(map[string]interface{}); ok {
            if list, ok := data["entrustedList"].([]interface{}); ok {
                fmt.Printf("✅ 找到 %d 个订单\n", len(list))
            }
        }
	} else {
		fmt.Printf("Response (Raw): %s\n", string(body))
	}
}

func generateSign(method, requestPath, queryString, timestamp, secretKey string) string {
	var bodyStr string // GET 请求 body 为空
	message := timestamp + strings.ToUpper(method) + requestPath + "?" + queryString + bodyStr
	
	hmac256 := hmac.New(sha256.New, []byte(secretKey))
	hmac256.Write([]byte(message))
	signature := base64.StdEncoding.EncodeToString(hmac256.Sum(nil))
	return signature
}
