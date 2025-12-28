package signal

import (
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
	Time     time.Time
}

// GlobalManager 全局单例
var GlobalManager *StrategyManager

// InitGlobalManager 初始化全局管理器
func InitGlobalManager(mcpClient *mcp.Client) error {
	// 读取环境变量配置
	gmailUser := os.Getenv("GMAIL_USER")
	if gmailUser == "" { gmailUser = os.Getenv("EMAIL_USER") }
	
	gmailPass := os.Getenv("GMAIL_PASSWORD")
	if gmailPass == "" { gmailPass = os.Getenv("EMAIL_PASSWORD") }

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
		case content := <-sm.gmailMonitor.SignalChan:
			// 解析邮件
			decision, err := sm.parser.Parse(content)
			if err != nil {
				log.Printf("❌ 策略解析失败: %v", err)
				continue
			}
			
			// 更新策略
			sm.UpdateStrategy(decision)
			
		case <-sm.stopChan:
			return
		}
	}
}

func (sm *StrategyManager) UpdateStrategy(newStrat *SignalDecision) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.strategies == nil {
		sm.strategies = make(map[string]*StrategySnapshot)
	}

	if newStrat.SignalID == "" {
		newStrat.SignalID = fmt.Sprintf("anon_%s_%s_%.4f_%d",
			newStrat.Symbol, newStrat.Direction, newStrat.Entry.PriceTarget, time.Now().UnixNano())
	}
	key := newStrat.SignalID

	// 【规则】同一交易对只保留最新策略：如果已有相同 symbol 的其他策略，先移除旧的
	for k, snap := range sm.strategies {
		if snap != nil && snap.Strategy != nil &&
			snap.Strategy.Symbol == newStrat.Symbol && k != key {
			delete(sm.strategies, k)
		}
	}

	// 相同signal_id视为同一策略的更新，直接覆盖快照
	sm.strategies[key] = &StrategySnapshot{
		Strategy: newStrat,
		Time:     time.Now(),
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

	if len(sm.strategies) == 0 {
		return nil
	}

	result := make([]*StrategySnapshot, 0, len(sm.strategies))
	for _, s := range sm.strategies {
		if s != nil && s.Strategy != nil {
			result = append(result, s)
		}
	}

	// 为了轮询顺序稳定，按「收到时间」排序（时间早的在前，时间相同按交易对字母序）
	sort.Slice(result, func(i, j int) bool {
		ti := result[i].Time
		tj := result[j].Time
		if ti.Equal(tj) {
			symI := ""
			symJ := ""
			if result[i].Strategy != nil {
				symI = result[i].Strategy.Symbol
			}
			if result[j].Strategy != nil {
				symJ = result[j].Strategy.Symbol
			}
			return symI < symJ
		}
		return ti.Before(tj)
	})

	return result
}


