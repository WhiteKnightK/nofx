package signal

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"sort"
	"strings"
	"sync"
	"time"

	"nofx/config"
	"nofx/mcp"
	"nofx/signal/gmail"
)

type StrategyManager struct {
	mu sync.RWMutex

	strategies map[string]*StrategySnapshot

	listeners []StrategyListener

	notifySuppressUntil time.Time
	maxActiveAge        time.Duration
	maxAutoExecuteAge   time.Duration

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

// StrategyListener 策略更新监听器
type StrategyListener func(newStrat, prev *SignalDecision)

// GetActiveStrategies 获取所有活跃策略快照
func (sm *StrategyManager) GetActiveStrategies() []*StrategySnapshot {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	var results []*StrategySnapshot
	for _, snapshot := range sm.strategies {
		results = append(results, snapshot)
	}
	return results
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
		listeners:    make([]StrategyListener, 0),
		maxActiveAge:  24 * time.Hour,
		maxAutoExecuteAge: 12 * time.Hour,
		gmailMonitor: monitor,
		parser:       parser,
		stopChan:     make(chan struct{}),
	}

	// 可配置：ACTIVE_STRATEGY_MAX_AGE_HOURS (默认 24h)
	if v := os.Getenv("ACTIVE_STRATEGY_MAX_AGE_HOURS"); v != "" {
		if hours, err := strconv.Atoi(v); err == nil && hours > 0 {
			GlobalManager.maxActiveAge = time.Duration(hours) * time.Hour
		}
	}

	// 可配置：SIGNAL_AUTO_EXEC_MAX_AGE_HOURS (默认 12h)
	if v := os.Getenv("SIGNAL_AUTO_EXEC_MAX_AGE_HOURS"); v != "" {
		if hours, err := strconv.Atoi(v); err == nil && hours > 0 {
			GlobalManager.maxAutoExecuteAge = time.Duration(hours) * time.Hour
		}
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

	// 启动 warmup：抑制启动阶段（历史回放）触发 AI
	sm.mu.Lock()
	sm.notifySuppressUntil = time.Now().Add(20 * time.Second)
	sm.mu.Unlock()

	// 启动恢复：从数据库恢复每个 symbol 最新策略到内存活跃池（用于前端展示/自检补单）
	sm.restoreLatestStrategiesFromDB(500)

	// 启动 Gmail 监听
	sm.gmailMonitor.Start()

	// 启动处理循环
	go sm.loop()

	// warmup 结束后：对当前每个 symbol 的“最新策略”触发一次监听（仅一次）
	go func() {
		time.Sleep(21 * time.Second)
		sm.notifyAllLatest("warmup_complete")
	}()
}

func (sm *StrategyManager) Stop() {
	sm.isRunning = false
	sm.gmailMonitor.Stop()
	close(sm.stopChan)
}

// restoreLatestStrategiesFromDB 从数据库恢复活跃策略快照（每个 symbol 仅保留最新一条）
func (sm *StrategyManager) restoreLatestStrategiesFromDB(limit int) {
	if config.GlobalDB == nil {
		return
	}

	signals, err := config.GlobalDB.GetAllParsedSignals(limit)
	if err != nil || len(signals) == 0 {
		return
	}

	type found struct {
		latest     *SignalDecision
		latestTime time.Time
		prev       *SignalDecision
	}

	bySymbol := make(map[string]*found)

	for _, ps := range signals {
		if ps.Symbol == "" {
			continue
		}

		receivedAt := ps.ReceivedAt
		if receivedAt.IsZero() {
			continue
		}

		sm.mu.RLock()
		maxAge := sm.maxActiveAge
		sm.mu.RUnlock()
		if maxAge > 0 && time.Since(receivedAt) > maxAge {
			continue
		}

		var d SignalDecision
		if ps.ContentJSON != "" {
			if err := json.Unmarshal([]byte(ps.ContentJSON), &d); err != nil {
				continue
			}
		}

		if d.SignalID == "" {
			d.SignalID = ps.SignalID
		}
		if d.Symbol == "" {
			d.Symbol = ps.Symbol
		}
		if d.Direction == "" {
			d.Direction = ps.Direction
		}
		if d.RawContent == "" {
			d.RawContent = ps.RawContent
		}

		symbol := strings.ToUpper(d.Symbol)
		d.Symbol = symbol

		if bySymbol[symbol] == nil {
			dd := d
			bySymbol[symbol] = &found{latest: &dd, latestTime: receivedAt}
			continue
		}
		if bySymbol[symbol].prev == nil {
			dd := d
			bySymbol[symbol].prev = &dd
		}
	}

	if len(bySymbol) == 0 {
		return
	}

	sm.mu.Lock()
	if sm.strategies == nil {
		sm.strategies = make(map[string]*StrategySnapshot)
	}
	for sym, f := range bySymbol {
		if f == nil || f.latest == nil {
			continue
		}
		sm.strategies[sym] = &StrategySnapshot{
			Strategy:     f.latest,
			PrevStrategy: f.prev,
			Time:         f.latestTime,
		}
	}
	sm.mu.Unlock()

	log.Printf("ℹ️ Restored active strategies from DB: %d", len(bySymbol))
}

func (sm *StrategyManager) isExpired(receivedAt time.Time) bool {
	if receivedAt.IsZero() {
		return false
	}
	sm.mu.RLock()
	maxAge := sm.maxActiveAge
	sm.mu.RUnlock()
	if maxAge <= 0 {
		return false
	}
	return time.Since(receivedAt) > maxAge
}

func (sm *StrategyManager) shouldAutoExecute(receivedAt time.Time) bool {
	if receivedAt.IsZero() {
		return false
	}

	sm.mu.RLock()
	suppressUntil := sm.notifySuppressUntil
	maxExecAge := sm.maxAutoExecuteAge
	sm.mu.RUnlock()

	if !suppressUntil.IsZero() && time.Now().Before(suppressUntil) {
		return false
	}
	if maxExecAge > 0 && time.Since(receivedAt) > maxExecAge {
		return false
	}
	return true
}

// notifyAllLatest 对每个 symbol 的最新策略触发一次监听（用于启动 warmup 结束后补齐）
func (sm *StrategyManager) notifyAllLatest(reason string) {
	sm.mu.RLock()
	strategies := make([]*StrategySnapshot, 0, len(sm.strategies))
	for _, s := range sm.strategies {
		if s != nil && s.Strategy != nil {
			strategies = append(strategies, s)
		}
	}
	listenersCopy := append([]StrategyListener(nil), sm.listeners...)
	sm.mu.RUnlock()

	if len(strategies) == 0 || len(listenersCopy) == 0 {
		return
	}

	for _, snap := range strategies {
		if snap == nil || snap.Strategy == nil {
			continue
		}
		if sm.isExpired(snap.Time) || !sm.shouldAutoExecute(snap.Time) {
			continue
		}
		for _, l := range listenersCopy {
			if l == nil {
				continue
			}
			go func(fn StrategyListener, s *StrategySnapshot) {
				defer func() {
					if r := recover(); r != nil {
						log.Printf("⚠️ Strategy listener panic: %v", r)
					}
				}()
				fn(s.Strategy, s.PrevStrategy)
			}(l, snap)
		}
	}

	log.Printf("ℹ️ Strategy listeners notified for latest snapshots (reason=%s, count=%d)", reason, len(strategies))
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

// RegisterListener 注册策略更新监听器
func (sm *StrategyManager) RegisterListener(listener StrategyListener) {
	if listener == nil {
		return
	}

	sm.mu.Lock()
	sm.listeners = append(sm.listeners, listener)
	suppressUntil := sm.notifySuppressUntil
	sm.mu.Unlock()

	// 若 warmup 已结束，为新注册的监听器补发一次“最新策略”
	if suppressUntil.IsZero() || time.Now().After(suppressUntil) {
		go sm.notifyAllLatest("listener_registered")
	}
}

func (sm *StrategyManager) UpdateStrategy(newStrat *SignalDecision, receivedAt time.Time) {
	// 1. 规范化时间与 SignalID，并更新内存快照
	sm.mu.Lock()

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
			sm.mu.Unlock()
			return
		}

		// 如果时间相同且 SignalID 相同，视为重复处理，直接忽略
		if receivedAt.Equal(existing.Time) && existing.Strategy.SignalID == newStrat.SignalID {
			sm.mu.Unlock()
			return
		}

		prev = existing.Strategy
	}

	// 过期策略不进入活跃池，避免旧策略触发 AI/审计
	if sm.maxActiveAge > 0 && time.Since(receivedAt) > sm.maxActiveAge {
		sm.mu.Unlock()

		// 仍然持久化到数据库用于历史追溯
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

		log.Printf("ℹ️ Ignored stale strategy (symbol=%s id=%s receivedAt=%s)", newStrat.Symbol, newStrat.SignalID, receivedAt.Format(time.RFC3339))
		return
	}

	// 同一交易对无论有多少封新邮件，这里都会覆盖为“最新一封”
	sm.strategies[key] = &StrategySnapshot{
		Strategy:     newStrat,
		PrevStrategy: prev,
		Time:         receivedAt,
	}

	var listenersCopy []StrategyListener
	if len(sm.listeners) > 0 {
		listenersCopy = append([]StrategyListener(nil), sm.listeners...)
	}

	sm.mu.Unlock()

	// 2. 持久化到数据库
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

	// 3. 通知所有监听器（仅当满足 warmup/新鲜度要求）
	if sm.shouldAutoExecute(receivedAt) {
		for _, l := range listenersCopy {
			if l == nil {
				continue
			}
			go func(fn StrategyListener) {
				defer func() {
					if r := recover(); r != nil {
						log.Printf("⚠️ Strategy listener panic: %v", r)
					}
				}()
				fn(newStrat, prev)
			}(l)
		}
	} else {
		log.Printf("ℹ️ Skipped notifying listeners (warmup/stale) for strategy id=%s symbol=%s", newStrat.SignalID, newStrat.Symbol)
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
