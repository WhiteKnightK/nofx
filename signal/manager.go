package signal

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sort"
	"sync"
	"time"

	"nofx/config"
	"nofx/mcp"
	"nofx/signal/gmail"
)

type StrategyManager struct {
	mu sync.RWMutex

	strategies map[string]*StrategySnapshot

	gmailMonitor *gmail.Monitor
	parser       *Parser
	isRunning    bool
	stopChan     chan struct{}
}

// StrategySnapshot 策略快照（用于多策略轮询）
type StrategySnapshot struct {
	Strategy *SignalDecision
	// PrevStrategy 记录同一交易对上一次生效的策略（用于提示AI了解策略变更前后差异）
	PrevStrategy *SignalDecision
	Time         time.Time
}

// GlobalManager 全局单例
var GlobalManager *StrategyManager

// InitGlobalManager 初始化全局管理器
func InitGlobalManager(mcpClient *mcp.Client) error {
	// 读取环境变量配置
	gmailUser := os.Getenv("GMAIL_USER")
	if gmailUser == "" {
		gmailUser = os.Getenv("EMAIL_USER")
	}

	gmailPass := os.Getenv("GMAIL_PASSWORD")
	if gmailPass == "" {
		gmailPass = os.Getenv("EMAIL_PASSWORD")
	}

	if gmailUser == "" || gmailPass == "" {
		log.Println("⚠️ 未配置 GMAIL_USER/PASSWORD，信号模式将不可用")
		return nil
	}

	// 构造配置
	cfg := &config.GmailConfig{
		Enabled:  true,
		User:     gmailUser,
		Password: gmailPass,
		Host:     "imap.gmail.com",
		Port:     993,
	}

	monitor := gmail.NewMonitor(cfg)
	parser, err := NewParser(mcpClient)
	if err != nil {
		return err
	}

	GlobalManager = &StrategyManager{
		strategies:   make(map[string]*StrategySnapshot),
		gmailMonitor: monitor,
		parser:       parser,
		stopChan:     make(chan struct{}),
	}

	return nil
}

// Start 启动管理器
func (sm *StrategyManager) Start() {
	if sm.isRunning {
		return
	}
	sm.isRunning = true
	log.Println("🧠 全局策略管理器已启动")

	// 启动 Gmail 监听
	sm.gmailMonitor.Start()

	// 启动处理循环
	go sm.loop()
}

func (sm *StrategyManager) Stop() {
	sm.isRunning = false
	sm.gmailMonitor.Stop()
	close(sm.stopChan)
}

func (sm *StrategyManager) loop() {
	for {
		select {
		case email := <-sm.gmailMonitor.SignalChan:
			// 【优化】使用 Goroutine 并行解析多封邮件，避免串行排队导致处理慢
			go func(e *gmail.Email) {
				// 解析邮件
				if e == nil || e.Body == "" {
					return
				}

				decision, err := sm.parser.Parse(e.Body)
				if err != nil {
					log.Printf("❌ 策略解析失败: %v", err)
					return
				}

				// 使用邮件指纹作为 SignalID，实现持久化去重
				if e.MessageID != "" {
					decision.SignalID = e.MessageID
				}

				// 更新策略（使用邮件原始时间作为策略时间轴的基准）
				sm.UpdateStrategy(decision, e.Date)
			}(email)

		case <-sm.stopChan:
			return
		}
	}
}

func (sm *StrategyManager) UpdateStrategy(newStrat *SignalDecision, receivedAt time.Time) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// 如果未提供邮件时间，使用当前时间兜底
	if receivedAt.IsZero() {
		receivedAt = time.Now()
	}

	if sm.strategies == nil {
		sm.strategies = make(map[string]*StrategySnapshot)
	}

	if newStrat.SignalID == "" {
		// 兜底：如果解析器没有提供 ID（理论上现在不会发生），则生成一个基于内容的 ID
		newStrat.SignalID = fmt.Sprintf("sig_%s_%s_%d",
			newStrat.Symbol, newStrat.Direction, receivedAt.Unix())
	}

	// 关键：内存中的 active 策略池按「交易对」维度去重
	// - map 的 key 使用 symbol，保证同一交易对始终只有一条最新策略
	// - PrevStrategy 用于记录上一次策略版本，便于 AI 对比前后差异
	key := newStrat.Symbol

	var prev *SignalDecision
	if existing, ok := sm.strategies[key]; ok && existing != nil && existing.Strategy != nil {
		// 如果新邮件时间比当前记录还旧，则忽略（防止 IMAP 回溯时老邮件覆盖新邮件）
		if receivedAt.Before(existing.Time) {
			log.Printf("⏭ [全局] 收到较旧策略，忽略: %s %s @ %.2f (new %s < existing %s)",
				newStrat.Direction, newStrat.Symbol, newStrat.Entry.PriceTarget,
				receivedAt.Format(time.RFC3339), existing.Time.Format(time.RFC3339))
			return
		}

		// 如果时间相同且 SignalID 相同，视为重复处理，直接忽略
		if receivedAt.Equal(existing.Time) && existing.Strategy.SignalID == newStrat.SignalID {
			return
		}

		prev = existing.Strategy
	}

	// 同一交易对无论有多少封新邮件，这里都会覆盖为“最新一封”
	sm.strategies[key] = &StrategySnapshot{
		Strategy:     newStrat,
		PrevStrategy: prev,
		Time:         receivedAt,
	}

	// 【新增】持久化到数据库
	if config.GlobalDB != nil {
		contentJSON, _ := json.Marshal(newStrat)
		err := config.GlobalDB.SaveParsedSignal(&config.ParsedSignal{
			SignalID:    newStrat.SignalID,
			Symbol:      newStrat.Symbol,
			Direction:   newStrat.Direction,
			ReceivedAt:  receivedAt,
			ContentJSON: string(contentJSON),
			RawContent:  newStrat.RawContent,
		})
		if err != nil {
			log.Printf("⚠️ 持久化策略信号失败: %v", err)
		}
	}

	log.Printf("📢 [全局] 策略已更新: %s %s @ %.2f (ID: %s)",
		newStrat.Direction, newStrat.Symbol, newStrat.Entry.PriceTarget, newStrat.SignalID)
}

func (sm *StrategyManager) GetActiveStrategy() (*SignalDecision, time.Time) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	var latest *StrategySnapshot
	for _, s := range sm.strategies {
		if latest == nil || s.Time.After(latest.Time) {
			latest = s
		}
	}
	if latest == nil {
		return nil, time.Time{}
	}
	return latest.Strategy, latest.Time
}

// ListActiveStrategies 返回当前所有活跃策略快照
func (sm *StrategyManager) ListActiveStrategies() []*StrategySnapshot {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	result := make([]*StrategySnapshot, 0)
	if len(sm.strategies) == 0 {
		return result
	}
	for _, s := range sm.strategies {
		if s != nil && s.Strategy != nil {
			result = append(result, s)
		}
	}

	// 为了轮询顺序与邮件时间一致，这里按「邮件接收时间」排序（旧 -> 新）
	// 说明：
	// - Time 字段现在使用邮件原始接收时间（Envelope.Date），不会因为重复轮询而抖动
	// - 同一时间的多条策略，再按 Symbol 做字母序兜底，保证顺序稳定可预期
	sort.Slice(result, func(i, j int) bool {
		// 先按时间从旧到新
		if !result[i].Time.Equal(result[j].Time) {
			return result[i].Time.Before(result[j].Time)
		}

		// 时间相同再按 symbol 字母序兜底
		symI := ""
		symJ := ""
		if result[i].Strategy != nil {
			symI = result[i].Strategy.Symbol
		}
		if result[j].Strategy != nil {
			symJ = result[j].Strategy.Symbol
		}
		return symI < symJ
	})

	return result
}
