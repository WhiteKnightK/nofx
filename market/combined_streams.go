package market

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type CombinedStreamsClient struct {
	conn        *websocket.Conn
	mu          sync.RWMutex
	subscribers map[string]chan []byte
	reconnect   bool
	done        chan struct{}
	batchSize   int // 每批订阅的流数量
}

func NewCombinedStreamsClient(batchSize int) *CombinedStreamsClient {
	return &CombinedStreamsClient{
		subscribers: make(map[string]chan []byte),
		reconnect:   true,
		done:        make(chan struct{}),
		batchSize:   batchSize,
	}
}

func (c *CombinedStreamsClient) Connect() error {
	// 🔍 调试代理配置
	req, _ := http.NewRequest("GET", "https://fstream.binance.com", nil)
	proxy, err := http.ProxyFromEnvironment(req)
	if err == nil && proxy != nil {
		log.Printf("🔍 [WSClient] 检测到系统代理: %s", proxy.String())
	}

	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
		Proxy:            http.ProxyFromEnvironment, // 支持从环境变量读取代理
	}

	// 组合流使用不同的端点
	conn, _, err := dialer.Dial("wss://fstream.binance.com/stream", nil)
	if err != nil {
		return fmt.Errorf("组合流WebSocket连接失败: %v", err)
	}

	c.mu.Lock()
	c.conn = conn
	c.mu.Unlock()

	log.Println("组合流WebSocket连接成功")
	go c.readMessages()

	return nil
}

// BatchSubscribeKlines 批量订阅K线
func (c *CombinedStreamsClient) BatchSubscribeKlines(symbols []string, interval string) error {
	// 将symbols分批处理
	batches := c.splitIntoBatches(symbols, c.batchSize)

	for i, batch := range batches {
		log.Printf("订阅第 %d 批, 数量: %d", i+1, len(batch))

		streams := make([]string, len(batch))
		for j, symbol := range batch {
			streams[j] = fmt.Sprintf("%s@kline_%s", strings.ToLower(symbol), interval)
		}

		if err := c.subscribeStreams(streams); err != nil {
			return fmt.Errorf("第 %d 批订阅失败: %v", i+1, err)
		}

		// 批次间延迟，避免被限制
		if i < len(batches)-1 {
			time.Sleep(100 * time.Millisecond)
		}
	}

	return nil
}

// splitIntoBatches 将切片分成指定大小的批次
func (c *CombinedStreamsClient) splitIntoBatches(symbols []string, batchSize int) [][]string {
	var batches [][]string

	for i := 0; i < len(symbols); i += batchSize {
		end := i + batchSize
		if end > len(symbols) {
			end = len(symbols)
		}
		batches = append(batches, symbols[i:end])
	}

	return batches
}

// subscribeStreams 订阅多个流
func (c *CombinedStreamsClient) subscribeStreams(streams []string) error {
	subscribeMsg := map[string]interface{}{
		"method": "SUBSCRIBE",
		"params": streams,
		"id":     time.Now().UnixNano(),
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.conn == nil {
		return fmt.Errorf("WebSocket未连接")
	}

	log.Printf("订阅流: %v", streams)
	return c.conn.WriteJSON(subscribeMsg)
}

func (c *CombinedStreamsClient) readMessages() {
	for {
		select {
		case <-c.done:
			return
		default:
			c.mu.RLock()
			conn := c.conn
			c.mu.RUnlock()

			if conn == nil {
				time.Sleep(1 * time.Second)
				continue
			}

			// 设置读取超时，防止僵尸连接
			// Binance 每3分钟发送一次 Ping，如果超过 5 分钟没收到任何数据（包括 Ping），认为连接断开
			_ = conn.SetReadDeadline(time.Now().Add(5 * time.Minute))

			_, message, err := conn.ReadMessage()
			if err != nil {
				log.Printf("读取组合流消息失败: %v", err)
				c.handleReconnect()
				return
			}

			c.handleCombinedMessage(message)
		}
	}
}

func (c *CombinedStreamsClient) handleCombinedMessage(message []byte) {
	var combinedMsg struct {
		Stream string          `json:"stream"`
		Data   json.RawMessage `json:"data"`
	}

	if err := json.Unmarshal(message, &combinedMsg); err != nil {
		log.Printf("解析组合消息失败: %v", err)
		return
	}

	c.mu.RLock()
	ch, exists := c.subscribers[combinedMsg.Stream]
	c.mu.RUnlock()

	if exists {
		select {
		case ch <- combinedMsg.Data:
		default:
			log.Printf("订阅者通道已满: %s", combinedMsg.Stream)
		}
	}
}

func (c *CombinedStreamsClient) AddSubscriber(stream string, bufferSize int) <-chan []byte {
	ch := make(chan []byte, bufferSize)
	c.mu.Lock()
	c.subscribers[stream] = ch
	c.mu.Unlock()
	return ch
}

func (c *CombinedStreamsClient) handleReconnect() {
	if !c.reconnect {
		return
	}

	log.Println("组合流尝试重新连接...")
	time.Sleep(3 * time.Second)

	if err := c.Connect(); err != nil {
		log.Printf("组合流重新连接失败: %v", err)
		go c.handleReconnect()
	}
}

func (c *CombinedStreamsClient) Close() {
	c.reconnect = false
	close(c.done)

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}

	for stream, ch := range c.subscribers {
		close(ch)
		delete(c.subscribers, stream)
	}
}
