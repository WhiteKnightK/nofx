package trader

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	sysconfig "nofx/config"
	"nofx/decision"
	"nofx/logger"
	"nofx/market"
	"nofx/mcp"
	"nofx/pool"
	"nofx/signal"
	"os"
	"strings"
	"sync"
	"time"
)

// AutoTraderConfig 自动交易配置（简化版 - AI全权决策）
type AutoTraderConfig struct {
	// Trader标识
	ID      string // Trader唯一标识（用于日志目录等）
	Name    string // Trader显示名称
	AIModel string // AI模型: "qwen" 或 "deepseek"

	// 交易平台选择
	Exchange string // "binance", "hyperliquid" 或 "aster"

	// 币安API配置
	BinanceAPIKey    string
	BinanceSecretKey string

	// Hyperliquid配置
	HyperliquidPrivateKey string
	HyperliquidWalletAddr string
	HyperliquidTestnet    bool

	// Aster配置
	AsterUser       string // Aster主钱包地址
	AsterSigner     string // Aster API钱包地址
	AsterPrivateKey string // Aster API钱包私钥

	// Bitget配置
	BitgetAPIKey     string // Bitget API Key
	BitgetSecretKey  string // Bitget Secret Key
	BitgetPassphrase string // Bitget API Passphrase
	BitgetTestnet    bool   // 是否使用测试网

	CoinPoolAPIURL string

	// AI配置
	UseQwen     bool
	DeepSeekKey string
	QwenKey     string

	// 自定义AI API配置
	CustomAPIURL    string
	CustomAPIKey    string
	CustomModelName string

	// 扫描配置
	ScanInterval time.Duration // 扫描间隔（建议3分钟）

	// 账户配置
	InitialBalance float64 // 初始金额（用于计算盈亏，需手动设置）

	// 杠杆配置
	BTCETHLeverage  int // BTC和ETH的杠杆倍数
	AltcoinLeverage int // 山寨币的杠杆倍数

	// 风险控制（仅作为提示，AI可自主决定）
	MaxDailyLoss    float64       // 最大日亏损百分比（提示）
	MaxDrawdown     float64       // 最大回撤百分比（提示）
	StopTradingTime time.Duration // 触发风控后暂停时长

	// 仓位模式
	IsCrossMargin bool // true=全仓模式, false=逐仓模式

	// 币种配置
	DefaultCoins []string // 默认币种列表（从数据库获取）
	TradingCoins []string // 实际交易币种列表

	// 系统提示词模板
	SystemPromptTemplate string // 系统提示词模板名称（如 "default", "aggressive"）

	// Gmail配置
	Gmail *sysconfig.GmailConfig
}

// AutoTrader 自动交易器
type AutoTrader struct {
	id                    string // Trader唯一标识
	name                  string // Trader显示名称
	aiModel               string // AI模型名称
	exchange              string // 交易平台名称
	config                AutoTraderConfig
	trader                Trader // 使用Trader接口（支持多平台）
	mcpClient             *mcp.Client
	decisionLogger        *logger.DecisionLogger // 决策日志记录器
	initialBalance        float64
	dailyPnL              float64
	customPrompt          string   // 自定义交易策略prompt
	overrideBasePrompt    bool     // 是否覆盖基础prompt
	systemPromptTemplate  string   // 系统提示词模板名称
	defaultCoins          []string // 默认币种列表（从数据库获取）
	tradingCoins          []string // 实际交易币种列表
	lastResetTime         time.Time
	stopUntil             time.Time
	isRunning             bool
	startTime             time.Time          // 系统启动时间
	callCount             int                // AI调用次数
	positionFirstSeenTime map[string]int64   // 持仓首次出现时间 (symbol_side -> timestamp毫秒)
	stopMonitorCh         chan struct{}      // 用于停止监控goroutine
	monitorWg             sync.WaitGroup     // 用于等待监控goroutine结束
	strategyFixTime       sync.Map           // 策略修复时间记录 (signalID -> time.Time)
	peakPnLCache          map[string]float64 // 最高收益缓存 (symbol -> 峰值盈亏百分比)
	peakPnLCacheMutex     sync.RWMutex       // 缓存读写锁
	mu                    sync.RWMutex       // 提示词配置读写锁（保护customPrompt、overrideBasePrompt、systemPromptTemplate）
	lastBalanceSyncTime   time.Time          // 上次余额同步时间
	database              interface{}        // 数据库引用（用于自动更新余额）
	userID                string             // 用户ID

	// 已应用止盈止损的策略ID (symbol -> strategyID)，用于在策略变更时自动更新委托
	appliedStopStrategy map[string]string

	// 信号模式状态
	lastExecutedSignalID string // 上次执行的信号ID
}

// GetTrader 获取底层交易器接口（用于直接调用交易方法）
func (at *AutoTrader) GetTrader() Trader {
	return at.trader
}

// CloseLong 平多仓（代理方法）
func (at *AutoTrader) CloseLong(symbol string, quantity float64) (map[string]interface{}, error) {
	return at.trader.CloseLong(symbol, quantity)
}

// CloseShort 平空仓（代理方法）
func (at *AutoTrader) CloseShort(symbol string, quantity float64) (map[string]interface{}, error) {
	return at.trader.CloseShort(symbol, quantity)
}

// PlaceLimitOrder 下限价委托开仓单 (代理方法)
func (at *AutoTrader) PlaceLimitOrder(symbol string, side, tradeSide string, quantity float64, price float64, leverage int) (map[string]interface{}, error) {
	at.mu.Lock()
	defer at.mu.Unlock()
	return at.trader.PlaceLimitOrder(symbol, side, tradeSide, quantity, price, leverage)
}

// CancelOrder 取消指定的委托单 (代理方法)
func (at *AutoTrader) CancelOrder(symbol, orderId string) error {
	at.mu.Lock()
	defer at.mu.Unlock()
	return at.trader.CancelOrder(symbol, orderId)
}

// NewAutoTrader 创建自动交易器
func NewAutoTrader(config AutoTraderConfig, database interface{}, userID string) (*AutoTrader, error) {
	// 设置默认值
	if config.ID == "" {
		config.ID = "default_trader"
	}
	if config.Name == "" {
		config.Name = "Default Trader"
	}
	if config.AIModel == "" {
		if config.UseQwen {
			config.AIModel = "qwen"
		} else {
			config.AIModel = "deepseek"
		}
	}

	mcpClient := mcp.New()

	// 初始化AI
	if config.AIModel == "custom" {
		// 使用自定义API
		mcpClient.SetCustomAPI(config.CustomAPIURL, config.CustomAPIKey, config.CustomModelName)
		log.Printf("🤖 [%s] 使用自定义AI API: %s (模型: %s)", config.Name, config.CustomAPIURL, config.CustomModelName)
	} else if config.UseQwen || config.AIModel == "qwen" {
		// 使用Qwen (支持自定义URL和Model)
		mcpClient.SetQwenAPIKey(config.QwenKey, config.CustomAPIURL, config.CustomModelName)
		if config.CustomAPIURL != "" || config.CustomModelName != "" {
			log.Printf("🤖 [%s] 使用阿里云Qwen AI (自定义URL: %s, 模型: %s)", config.Name, config.CustomAPIURL, config.CustomModelName)
		} else {
			log.Printf("🤖 [%s] 使用阿里云Qwen AI", config.Name)
		}
	} else {
		// 默认使用DeepSeek (支持自定义URL和Model)
		mcpClient.SetDeepSeekAPIKey(config.DeepSeekKey, config.CustomAPIURL, config.CustomModelName)
		if config.CustomAPIURL != "" || config.CustomModelName != "" {
			log.Printf("🤖 [%s] 使用DeepSeek AI (自定义URL: %s, 模型: %s)", config.Name, config.CustomAPIURL, config.CustomModelName)
		} else {
			log.Printf("🤖 [%s] 使用DeepSeek AI", config.Name)
		}
	}

	// 初始化币种池API
	if config.CoinPoolAPIURL != "" {
		pool.SetCoinPoolAPI(config.CoinPoolAPIURL)
	}

	// 设置默认交易平台
	if config.Exchange == "" {
		config.Exchange = "binance"
	}

	// 根据配置创建对应的交易器
	var trader Trader
	var err error

	// 记录仓位模式（通用）
	marginModeStr := "全仓"
	if !config.IsCrossMargin {
		marginModeStr = "逐仓"
	}
	log.Printf("📊 [%s] 仓位模式: %s", config.Name, marginModeStr)

	switch config.Exchange {
	case "binance":
		log.Printf("🏦 [%s] 使用币安合约交易", config.Name)
		trader = NewFuturesTrader(config.BinanceAPIKey, config.BinanceSecretKey, userID)
	case "hyperliquid":
		log.Printf("🏦 [%s] 使用Hyperliquid交易", config.Name)
		trader, err = NewHyperliquidTrader(config.HyperliquidPrivateKey, config.HyperliquidWalletAddr, config.HyperliquidTestnet)
		if err != nil {
			return nil, fmt.Errorf("初始化Hyperliquid交易器失败: %w", err)
		}
	case "aster":
		log.Printf("🏦 [%s] 使用Aster交易", config.Name)
		trader, err = NewAsterTrader(config.AsterUser, config.AsterSigner, config.AsterPrivateKey)
		if err != nil {
			return nil, fmt.Errorf("初始化Aster交易器失败: %w", err)
		}
	case "bitget":
		log.Printf("🏦 [%s] 使用Bitget合约交易", config.Name)
		trader = NewBitgetTrader(config.BitgetAPIKey, config.BitgetSecretKey, config.BitgetPassphrase, config.BitgetTestnet)
	default:
		return nil, fmt.Errorf("不支持的交易平台: %s", config.Exchange)
	}

	// 验证初始金额配置
	if config.InitialBalance <= 0 {
		return nil, fmt.Errorf("初始金额必须大于0，请在配置中设置InitialBalance")
	}

	// 初始化决策日志记录器（使用trader ID创建独立目录）
	logDir := fmt.Sprintf("decision_logs/%s", config.ID)
	decisionLogger := logger.NewDecisionLogger(logDir)

	// 设置默认系统提示词模板
	systemPromptTemplate := config.SystemPromptTemplate
	if systemPromptTemplate == "" {
		// feature/partial-close-dynamic-tpsl 分支默认使用 adaptive（支持动态止盈止损）
		systemPromptTemplate = "adaptive"
	}

	return &AutoTrader{
		id:                    config.ID,
		name:                  config.Name,
		aiModel:               config.AIModel,
		exchange:              config.Exchange,
		config:                config,
		trader:                trader,
		mcpClient:             mcpClient,
		decisionLogger:        decisionLogger,
		initialBalance:        config.InitialBalance,
		systemPromptTemplate:  systemPromptTemplate,
		defaultCoins:          config.DefaultCoins,
		tradingCoins:          config.TradingCoins,
		lastResetTime:         time.Now(),
		startTime:             time.Now(),
		callCount:             0,
		isRunning:             false,
		positionFirstSeenTime: make(map[string]int64),
		stopMonitorCh:         make(chan struct{}),
		monitorWg:             sync.WaitGroup{},
		peakPnLCache:          make(map[string]float64),
		peakPnLCacheMutex:     sync.RWMutex{},
		lastBalanceSyncTime:   time.Now(), // 初始化为当前时间
		database:              database,
		userID:                userID,
		appliedStopStrategy:   make(map[string]string),
	}, nil
}

// Run 运行自动交易主循环
func (at *AutoTrader) Run() error {
	at.isRunning = true
	at.stopMonitorCh = make(chan struct{})
	at.startTime = time.Now()

	log.Println("🚀 AI驱动自动交易系统启动")
	log.Printf("💰 初始余额: %.2f USDT", at.initialBalance)

	at.monitorWg.Add(1)
	defer at.monitorWg.Done()

	// 模式选择：如果有 Gmail 配置且启用，或者全局信号管理器已启动，则进入信号模式
	if (at.config.Gmail != nil && at.config.Gmail.Enabled) || signal.GlobalManager != nil {
		log.Println("📧 模式: 信号跟随模式 (Web3团队策略)")
		return at.RunSignalMode()
	}

	// 默认模式：自主决策
	log.Printf("⚙️  扫描间隔: %v", at.config.ScanInterval)
	log.Println("🤖 AI将全权决定杠杆、仓位大小、止损止盈等参数")

	// 循环执行：等待对齐 -> 执行 -> 等待对齐...
	var lastError error
	for at.isRunning {
		// 1. 等待直到下一个整点间隔（+5秒延迟）以获取闭合K线
		// 如果是重试，等待较短时间
		isRetry := (lastError != nil)
		if !at.waitForNextCycle(isRetry) {
			log.Printf("[%s] ⏹ 收到停止信号，退出自动交易主循环", at.name)
			return nil
		}

		// 2. 执行决策周期
		// 如果上次有错误，传入错误信息
		if err := at.runCycle(lastError); err != nil {
			log.Printf("❌ 执行失败: %v", err)
			lastError = err
		} else {
			lastError = nil
		}
	}

	return nil
}

// Stop 停止自动交易
func (at *AutoTrader) Stop() {
	if !at.isRunning {
		return
	}
	at.isRunning = false
	close(at.stopMonitorCh) // 通知监控goroutine停止
	at.monitorWg.Wait()     // 等待监控goroutine结束
	log.Println("⏹ 自动交易系统停止")
}

// waitForNextCycle 等待直到下一个周期
// isRetry: 是否为重试模式（等待时间更短）
// 返回 true 表示时间到了可以继续，返回 false 表示收到停止信号
func (at *AutoTrader) waitForNextCycle(isRetry bool) bool {
	now := time.Now()
	var waitDuration time.Duration
	var targetTime time.Time

	if isRetry {
		// 重试模式：重试间隔（可以配置，这里暂定1分钟）
		waitDuration = 1 * time.Minute
		targetTime = now.Add(waitDuration)
		log.Printf("⏳ [%s] 上轮执行失败，将在 1 分钟后重试... (目标时间: %s)",
			at.name, targetTime.Format("15:04:05"))
	} else {
		// 正常模式：等待下一个整点间隔
		interval := at.config.ScanInterval

		// 计算下一个整点时间
		nextTime := now.Truncate(interval).Add(interval)

		// 添加 5 秒延迟，确保交易所 K 线已生成并固定
		targetTime = nextTime.Add(5 * time.Second)

		// 如果当前时间已经过了 targetTime（极少数情况），则再加一个 interval
		if targetTime.Before(now) {
			targetTime = targetTime.Add(interval)
		}

		waitDuration = targetTime.Sub(now)

		log.Printf("⏳ [%s] 等待对齐 K 线周期: %v 后执行 (目标时间: %s)",
			at.name, waitDuration.Round(time.Second), targetTime.Format("15:04:05"))
	}

	timer := time.NewTimer(waitDuration)
	defer timer.Stop()

	select {
	case <-timer.C:
		return true
	case <-at.stopMonitorCh:
		return false
	}
}

// autoSyncBalanceIfNeeded 自动同步余额（每10分钟检查一次，变化>5%才更新）
func (at *AutoTrader) autoSyncBalanceIfNeeded() {
	// 距离上次同步不足10分钟，跳过
	if time.Since(at.lastBalanceSyncTime) < 10*time.Minute {
		return
	}

	log.Printf("🔄 [%s] 开始自动检查余额变化...", at.name)

	// 查询实际余额
	balanceInfo, err := at.trader.GetBalance()
	if err != nil {
		log.Printf("⚠️ [%s] 查询余额失败: %v", at.name, err)
		at.lastBalanceSyncTime = time.Now() // 即使失败也更新时间，避免频繁重试
		return
	}

	// 提取可用余额
	var actualBalance float64
	if availableBalance, ok := balanceInfo["available_balance"].(float64); ok && availableBalance > 0 {
		actualBalance = availableBalance
	} else if availableBalance, ok := balanceInfo["availableBalance"].(float64); ok && availableBalance > 0 {
		actualBalance = availableBalance
	} else if totalBalance, ok := balanceInfo["balance"].(float64); ok && totalBalance > 0 {
		actualBalance = totalBalance
	} else {
		log.Printf("⚠️ [%s] 无法提取可用余额", at.name)
		at.lastBalanceSyncTime = time.Now()
		return
	}

	oldBalance := at.initialBalance

	// 防止除以零：如果初始余额无效，直接更新为实际余额
	if oldBalance <= 0 {
		log.Printf("⚠️ [%s] 初始余额无效 (%.2f)，直接更新为实际余额 %.2f USDT", at.name, oldBalance, actualBalance)
		at.initialBalance = actualBalance
		if at.database != nil {
			type DatabaseUpdater interface {
				UpdateTraderInitialBalance(userID, id string, newBalance float64) error
			}
			if db, ok := at.database.(DatabaseUpdater); ok {
				if err := db.UpdateTraderInitialBalance(at.userID, at.id, actualBalance); err != nil {
					log.Printf("❌ [%s] 更新数据库失败: %v", at.name, err)
				} else {
					log.Printf("✅ [%s] 已自动同步余额到数据库", at.name)
				}
			} else {
				log.Printf("⚠️ [%s] 数据库类型不支持UpdateTraderInitialBalance接口", at.name)
			}
		} else {
			log.Printf("⚠️ [%s] 数据库引用为空，余额仅在内存中更新", at.name)
		}
		at.lastBalanceSyncTime = time.Now()
		return
	}

	changePercent := ((actualBalance - oldBalance) / oldBalance) * 100

	// 变化超过5%才更新
	if math.Abs(changePercent) > 5.0 {
		log.Printf("🔔 [%s] 检测到余额大幅变化: %.2f → %.2f USDT (%.2f%%)",
			at.name, oldBalance, actualBalance, changePercent)

		// 更新内存中的 initialBalance
		at.initialBalance = actualBalance

		// 更新数据库（需要类型断言）
		if at.database != nil {
			// 这里需要根据实际的数据库类型进行类型断言
			// 由于使用了 interface{}，我们需要在 TraderManager 层面处理更新
			// 或者在这里进行类型检查
			type DatabaseUpdater interface {
				UpdateTraderInitialBalance(userID, id string, newBalance float64) error
			}
			if db, ok := at.database.(DatabaseUpdater); ok {
				err := db.UpdateTraderInitialBalance(at.userID, at.id, actualBalance)
				if err != nil {
					log.Printf("❌ [%s] 更新数据库失败: %v", at.name, err)
				} else {
					log.Printf("✅ [%s] 已自动同步余额到数据库", at.name)
				}
			} else {
				log.Printf("⚠️ [%s] 数据库类型不支持UpdateTraderInitialBalance接口", at.name)
			}
		} else {
			log.Printf("⚠️ [%s] 数据库引用为空，余额仅在内存中更新", at.name)
		}
	} else {
		log.Printf("✓ [%s] 余额变化不大 (%.2f%%)，无需更新", at.name, changePercent)
	}

	at.lastBalanceSyncTime = time.Now()
}

// runCycle 运行一个交易周期（使用AI全权决策）
func (at *AutoTrader) runCycle(lastError error) error {
	at.callCount++

	log.Print("\n" + strings.Repeat("=", 70) + "\n")
	log.Printf("⏰ %s - AI决策周期 #%d", time.Now().Format("2006-01-02 15:04:05"), at.callCount)
	log.Println(strings.Repeat("=", 70))

	// 创建决策记录
	record := &logger.DecisionRecord{
		ExecutionLog: []string{},
		Success:      true,
	}

	// 🔄 强制从数据库同步最新配置（确保Prompt实时生效）
	if at.database != nil {
		if db, ok := at.database.(*sysconfig.Database); ok {
			traderRecord, err := db.GetTraderByID(at.id)
			if err == nil && traderRecord != nil {
				at.mu.Lock()
				// 检查是否有变更，如果有变更则打印日志
				if at.customPrompt != traderRecord.CustomPrompt ||
					at.overrideBasePrompt != traderRecord.OverrideBasePrompt ||
					at.systemPromptTemplate != traderRecord.SystemPromptTemplate {
					log.Printf("🔄 [%s] 检测到配置变更，正在同步: 模板=%s, 覆盖基础=%v",
						at.name, traderRecord.SystemPromptTemplate, traderRecord.OverrideBasePrompt)
				}

				at.customPrompt = traderRecord.CustomPrompt
				at.overrideBasePrompt = traderRecord.OverrideBasePrompt
				at.systemPromptTemplate = traderRecord.SystemPromptTemplate
				at.mu.Unlock()
			}
		}
	}

	// 1. 检查是否需要停止交易
	if time.Now().Before(at.stopUntil) {
		remaining := at.stopUntil.Sub(time.Now())
		log.Printf("⏸ 风险控制：暂停交易中，剩余 %.0f 分钟", remaining.Minutes())
		record.Success = false
		record.ErrorMessage = fmt.Sprintf("风险控制暂停中，剩余 %.0f 分钟", remaining.Minutes())
		at.decisionLogger.LogDecision(record)
		return nil
	}

	// 2. 重置日盈亏（每天重置）
	if time.Since(at.lastResetTime) > 24*time.Hour {
		at.dailyPnL = 0
		at.lastResetTime = time.Now()
		log.Println("📅 日盈亏已重置")
	}

	// 3. 自动同步余额功能已禁用
	// 原因：自动同步会覆盖用户手动设置的初始余额，导致盈亏计算错误
	// 例如：用户设置初始余额200，实际余额130（亏70），但自动同步后initialBalance变成130，显示盈利0而不是亏损70
	// 如果需要同步余额，请使用手动同步功能（API: POST /traders/:id/sync-balance）
	// at.autoSyncBalanceIfNeeded()

	// 4. 收集交易上下文
	ctx, err := at.buildTradingContext(lastError)
	if err != nil {
		record.Success = false
		record.ErrorMessage = fmt.Sprintf("构建交易上下文失败: %v", err)
		at.decisionLogger.LogDecision(record)
		return fmt.Errorf("构建交易上下文失败: %w", err)
	}

	// 保存账户状态快照
	record.AccountState = logger.AccountSnapshot{
		TotalBalance:          ctx.Account.TotalEquity,
		AvailableBalance:      ctx.Account.AvailableBalance,
		TotalUnrealizedProfit: ctx.Account.TotalPnL,
		PositionCount:         ctx.Account.PositionCount,
		MarginUsedPct:         ctx.Account.MarginUsedPct,
	}

	// 保存持仓快照
	for _, pos := range ctx.Positions {
		record.Positions = append(record.Positions, logger.PositionSnapshot{
			Symbol:           pos.Symbol,
			Side:             pos.Side,
			PositionAmt:      pos.Quantity,
			EntryPrice:       pos.EntryPrice,
			MarkPrice:        pos.MarkPrice,
			UnrealizedProfit: pos.UnrealizedPnL,
			Leverage:         float64(pos.Leverage),
			LiquidationPrice: pos.LiquidationPrice,
		})
	}

	log.Print(strings.Repeat("=", 70))
	for _, coin := range ctx.CandidateCoins {
		record.CandidateCoins = append(record.CandidateCoins, coin.Symbol)
	}

	log.Printf("📊 账户净值: %.2f USDT | 可用: %.2f USDT | 持仓: %d",
		ctx.Account.TotalEquity, ctx.Account.AvailableBalance, ctx.Account.PositionCount)

	// 5. 读取当前提示词配置（加锁保护）
	at.mu.Lock()
	customPrompt := at.customPrompt
	overrideBasePrompt := at.overrideBasePrompt
	systemPromptTemplate := at.systemPromptTemplate
	at.mu.Unlock()

	// 6. 调用AI获取完整决策
	log.Printf("🤖 正在请求AI分析并决策... [模板: %s, 覆盖基础: %v]", systemPromptTemplate, overrideBasePrompt)
	decision, err := decision.GetFullDecisionWithCustomPrompt(ctx, at.mcpClient, customPrompt, overrideBasePrompt, systemPromptTemplate)

	// 即使有错误，也保存思维链、决策和输入prompt（用于debug）
	if decision != nil {
		record.SystemPrompt = decision.SystemPrompt   // 保存系统提示词
		record.InputPrompt = decision.UserPrompt      // 保存输入提示词
		record.RawAIResponse = decision.RawAIResponse // 保存AI原始响应（未裁剪）
		record.CoTTrace = decision.CoTTrace           // 保存思维链（裁剪后）

		// 🔍 调试：打印字段长度确认数据已保存
		log.Printf("📝 决策记录字段长度: SystemPrompt=%d, InputPrompt=%d, CoTTrace=%d",
			len(record.SystemPrompt), len(record.InputPrompt), len(record.CoTTrace))

		if len(decision.Decisions) > 0 {
			decisionJSON, _ := json.MarshalIndent(decision.Decisions, "", "  ")
			record.DecisionJSON = string(decisionJSON)
		}
	}

	if err != nil {
		record.Success = false
		record.ErrorMessage = fmt.Sprintf("获取AI决策失败: %v", err)

		// 打印系统提示词和AI思维链（即使有错误，也要输出以便调试）
		if decision != nil {
			log.Print("\n" + strings.Repeat("=", 70) + "\n")
			log.Printf("📋 系统提示词 [模板: %s] (错误情况)", at.systemPromptTemplate)
			log.Println(strings.Repeat("=", 70))
			log.Println(decision.SystemPrompt)
			log.Println(strings.Repeat("=", 70))

			if decision.CoTTrace != "" {
				log.Print("\n" + strings.Repeat("-", 70) + "\n")
				log.Println(" AI思维链分析（错误情况）:")
				log.Println(strings.Repeat("-", 70))
				log.Println(decision.CoTTrace)
				log.Println(strings.Repeat("-", 70))
			}
		}

		at.decisionLogger.LogDecision(record)
		return fmt.Errorf("获取AI决策失败: %w", err)
	}

	// 5. 打印系统提示词（用于调试自定义提示词）
	log.Printf("\n" + strings.Repeat("=", 70))
	log.Printf("📋 系统提示词（完整版，包含所有部分）")
	log.Printf("   模板: %s | 自定义提示词: %v | 覆盖基础: %v",
		at.systemPromptTemplate,
		at.customPrompt != "",
		at.overrideBasePrompt)
	log.Println(strings.Repeat("=", 70))
	log.Println(decision.SystemPrompt)
	log.Printf(strings.Repeat("=", 70) + "\n")

	// 6. 打印AI思维链（用于查看AI是否遵循自定义提示词）
	log.Printf("\n" + strings.Repeat("-", 70))
	log.Println("💭 AI思维链分析:")
	log.Println(strings.Repeat("-", 70))
	log.Println(decision.CoTTrace)
	log.Printf(strings.Repeat("-", 70) + "\n")

	// 7. 打印AI决策
	log.Printf("📋 AI决策列表 (%d 个):\n", len(decision.Decisions))
	for i, d := range decision.Decisions {
		log.Printf("  [%d] %s: %s - %s", i+1, d.Symbol, d.Action, d.Reasoning)
		if d.Action == "open_long" || d.Action == "open_short" {
			log.Printf("      杠杆: %dx | 仓位: %.2f USDT | 止损: %.4f | 止盈: %.4f",
				d.Leverage, d.PositionSizeUSD, d.StopLoss, d.TakeProfit)
		}
	}
	log.Println()
	log.Print(strings.Repeat("-", 70))
	// 8. 对决策排序：确保先平仓后开仓（防止仓位叠加超限）
	log.Print(strings.Repeat("-", 70))

	// 8. 对决策排序：确保先平仓后开仓（防止仓位叠加超限）
	sortedDecisions := sortDecisionsByPriority(decision.Decisions)

	log.Println("🔄 执行顺序（已优化）: 先平仓→后开仓")
	for i, d := range sortedDecisions {
		log.Printf("  [%d] %s %s", i+1, d.Symbol, d.Action)
	}
	log.Println()

	// 执行决策并记录结果
	var executionErrors []string
	for _, d := range sortedDecisions {
		actionRecord := logger.DecisionAction{
			Action:    d.Action,
			Symbol:    d.Symbol,
			Quantity:  0,
			Leverage:  d.Leverage,
			Price:     0,
			Timestamp: time.Now(),
			Success:   false,
		}

		if err := at.executeDecisionWithRecord(&d, &actionRecord); err != nil {
			log.Printf("❌ 执行决策失败 (%s %s): %v", d.Symbol, d.Action, err)
			actionRecord.Error = err.Error()
			actionRecord.Success = false
			record.ExecutionLog = append(record.ExecutionLog, fmt.Sprintf("❌ %s %s 失败: %v", d.Symbol, d.Action, err))
			executionErrors = append(executionErrors, fmt.Sprintf("%s %s: %v", d.Symbol, d.Action, err))
		} else {
			actionRecord.Success = true
			record.ExecutionLog = append(record.ExecutionLog, fmt.Sprintf("✓ %s %s 成功", d.Symbol, d.Action))
			// 成功执行后短暂延迟
			time.Sleep(1 * time.Second)
		}

		// 🔍 保存到数据库历史记录，以便前端展示错误
		at.saveDecisionToDB("", &d, &actionRecord, decision.SystemPrompt, decision.UserPrompt, decision.RawAIResponse)

		record.Decisions = append(record.Decisions, actionRecord)
	}

	// 9. 保存决策记录
	if err := at.decisionLogger.LogDecision(record); err != nil {
		log.Printf("⚠ 保存决策记录失败: %v", err)
	}

	// 如果有执行错误，返回错误以触发重试
	if len(executionErrors) > 0 {
		return fmt.Errorf("执行出现错误: %s", strings.Join(executionErrors, "; "))
	}

	return nil
}

// buildTradingContext 构建交易上下文
func (at *AutoTrader) buildTradingContext(lastError error) (*decision.Context, error) {
	// 1. 获取账户信息
	balance, err := at.trader.GetBalance()
	if err != nil {
		return nil, fmt.Errorf("获取账户余额失败: %w", err)
	}

	// 获取账户字段
	totalWalletBalance := 0.0
	totalUnrealizedProfit := 0.0
	availableBalance := 0.0

	if wallet, ok := balance["totalWalletBalance"].(float64); ok {
		totalWalletBalance = wallet
	}
	if unrealized, ok := balance["totalUnrealizedProfit"].(float64); ok {
		totalUnrealizedProfit = unrealized
	}
	if avail, ok := balance["availableBalance"].(float64); ok {
		availableBalance = avail
	}

	// Total Equity = 钱包余额 + 未实现盈亏
	totalEquity := totalWalletBalance + totalUnrealizedProfit

	// 2. 获取持仓信息
	positions, err := at.trader.GetPositions()
	if err != nil {
		return nil, fmt.Errorf("获取持仓失败: %w", err)
	}

	var positionInfos []decision.PositionInfo
	totalMarginUsed := 0.0

	// 当前持仓的key集合（用于清理已平仓的记录）
	currentPositionKeys := make(map[string]bool)

	for _, pos := range positions {
		symbol := pos["symbol"].(string)
		side := pos["side"].(string)
		entryPrice := pos["entryPrice"].(float64)
		markPrice := pos["markPrice"].(float64)
		quantity := pos["positionAmt"].(float64)
		if quantity < 0 {
			quantity = -quantity // 空仓数量为负，转为正数
		}

		// 跳过已平仓的持仓（quantity = 0），防止"幽灵持仓"传递给AI
		if quantity == 0 {
			continue
		}

		unrealizedPnl := pos["unRealizedProfit"].(float64)
		liquidationPrice := pos["liquidationPrice"].(float64)

		// 计算占用保证金（估算）
		leverage := 10 // 默认值，实际应该从持仓信息获取
		if lev, ok := pos["leverage"].(float64); ok {
			leverage = int(lev)
		}
		marginUsed := (quantity * markPrice) / float64(leverage)
		totalMarginUsed += marginUsed

		// 计算盈亏百分比（基于保证金，考虑杠杆）
		pnlPct := calculatePnLPercentage(unrealizedPnl, marginUsed)

		// 跟踪持仓首次出现时间
		posKey := symbol + "_" + side
		currentPositionKeys[posKey] = true
		if _, exists := at.positionFirstSeenTime[posKey]; !exists {
			// 新持仓，记录当前时间
			at.positionFirstSeenTime[posKey] = time.Now().UnixMilli()
		}
		updateTime := at.positionFirstSeenTime[posKey]

		// 获取该持仓的历史最高收益率
		at.peakPnLCacheMutex.RLock()
		peakPnlPct := at.peakPnLCache[symbol]
		at.peakPnLCacheMutex.RUnlock()

		positionInfos = append(positionInfos, decision.PositionInfo{
			Symbol:           symbol,
			Side:             side,
			EntryPrice:       entryPrice,
			MarkPrice:        markPrice,
			Quantity:         quantity,
			Leverage:         leverage,
			UnrealizedPnL:    unrealizedPnl,
			UnrealizedPnLPct: pnlPct,
			PeakPnLPct:       peakPnlPct,
			LiquidationPrice: liquidationPrice,
			MarginUsed:       marginUsed,
			UpdateTime:       updateTime,
		})
	}

	// 清理已平仓的持仓记录
	for key := range at.positionFirstSeenTime {
		if !currentPositionKeys[key] {
			delete(at.positionFirstSeenTime, key)
		}
	}

	// 3. 获取交易员的候选币种池
	candidateCoins, err := at.getCandidateCoins()
	if err != nil {
		return nil, fmt.Errorf("获取候选币种失败: %w", err)
	}

	// 4. 计算总盈亏
	totalPnL := totalEquity - at.initialBalance
	totalPnLPct := 0.0
	if at.initialBalance > 0 {
		totalPnLPct = (totalPnL / at.initialBalance) * 100
	}

	marginUsedPct := 0.0
	if totalEquity > 0 {
		marginUsedPct = (totalMarginUsed / totalEquity) * 100
	}

	// 5. 分析历史表现（最近100个周期，避免长期持仓的交易记录丢失）
	// 假设每3分钟一个周期，100个周期 = 5小时，足够覆盖大部分交易
	performance, err := at.decisionLogger.AnalyzePerformance(100)
	if err != nil {
		log.Printf("⚠️  分析历史表现失败: %v", err)
		// 不影响主流程，继续执行（但设置performance为nil以避免传递错误数据）
		performance = nil
	}

	// 6. 构建上下文
	ctx := &decision.Context{
		CurrentTime:     time.Now().Format("2006-01-02 15:04:05"),
		RuntimeMinutes:  int(time.Since(at.startTime).Minutes()),
		CallCount:       at.callCount,
		BTCETHLeverage:  at.config.BTCETHLeverage,  // 使用配置的杠杆倍数
		AltcoinLeverage: at.config.AltcoinLeverage, // 使用配置的杠杆倍数
		Account: decision.AccountInfo{
			InitialBalance:   at.config.InitialBalance, // 传递初始余额
			TotalEquity:      totalEquity,
			AvailableBalance: availableBalance,
			TotalPnL:         totalPnL,
			TotalPnLPct:      totalPnLPct,
			MarginUsed:       totalMarginUsed,
			MarginUsedPct:    marginUsedPct,
			PositionCount:    len(positionInfos),
		},
		Positions:        positionInfos,
		ActiveStrategies: signal.GlobalManager.GetActiveStrategies(),
		CandidateCoins:   candidateCoins,
		Performance:      performance, // 添加历史表现分析
	}

	if lastError != nil {
		ctx.LastFailureReason = lastError.Error()
	}

	return ctx, nil
}

// executeDecisionWithRecord 执行AI决策并记录详细信息
func (at *AutoTrader) executeDecisionWithRecord(decision *decision.Decision, actionRecord *logger.DecisionAction) error {
	switch decision.Action {
	case "open_long":
		return at.executeOpenLongWithRecord(decision, actionRecord)
	case "open_short":
		return at.executeOpenShortWithRecord(decision, actionRecord)
	case "place_long_order":
		return at.executePlaceLimitOrderWithRecord("buy", "open", decision, actionRecord)
	case "place_short_order":
		return at.executePlaceLimitOrderWithRecord("sell", "open", decision, actionRecord)
	case "cancel_order":
		return at.executeCancelOrderWithRecord(decision, actionRecord)
	case "close_long":
		return at.executeCloseLongWithRecord(decision, actionRecord)
	case "close_short":
		return at.executeCloseShortWithRecord(decision, actionRecord)
	case "update_stop_loss":
		return at.executeUpdateStopLossWithRecord(decision, actionRecord)
	case "update_take_profit":
		return at.executeUpdateTakeProfitWithRecord(decision, actionRecord)
	case "partial_close":
		return at.executePartialCloseWithRecord(decision, actionRecord)
	case "set_tp_order":
		return at.executeSetTPOrderWithRecord(decision, actionRecord)
	case "set_sl_order":
		return at.executeSetSLOrderWithRecord(decision, actionRecord)
	case "hold", "wait":
		// 无需执行，仅记录
		return nil
	default:
		return fmt.Errorf("未知的action: %s", decision.Action)
	}
}

// executeOpenLongWithRecord 执行开多仓并记录详细信息
func (at *AutoTrader) executeOpenLongWithRecord(decision *decision.Decision, actionRecord *logger.DecisionAction) error {
	log.Printf("  📈 开多仓: %s", decision.Symbol)

	// ⚠️ 关键：检查是否已有同币种同方向持仓，如果有则拒绝开仓（防止仓位叠加超限）
	positions, err := at.trader.GetPositions()
	if err == nil {
		for _, pos := range positions {
			if pos["symbol"] == decision.Symbol && pos["side"] == "long" {
				return fmt.Errorf("❌ %s 已有多仓，拒绝开仓以防止仓位叠加超限。如需换仓，请先给出 close_long 决策", decision.Symbol)
			}
		}
	}

	// 先获取账户余额，用于后续保证金校验和自动缩小仓位
	balance, err := at.trader.GetBalance()
	if err != nil {
		return fmt.Errorf("获取账户余额失败: %w", err)
	}
	availableBalance := 0.0
	if avail, ok := balance["availableBalance"].(float64); ok {
		availableBalance = avail
	}

	// ⚠️ 自动缩小仓位：防止因 AI 给出的名义金额过大导致保证金不足
	if decision.Leverage <= 0 {
		decision.Leverage = at.config.BTCETHLeverage
	}
	maxPositionSizeUSD := availableBalance * float64(decision.Leverage) * 0.95 // 预留 5% 作为手续费和波动缓冲
	if decision.PositionSizeUSD > maxPositionSizeUSD {
		log.Printf("  ⚠️ 开多金额 %.2f USDT 超过账户可承受上限 %.2f USDT（可用余额 %.2f, 杠杆 %dx），自动缩小仓位",
			decision.PositionSizeUSD, maxPositionSizeUSD, availableBalance, decision.Leverage)
		decision.PositionSizeUSD = maxPositionSizeUSD
	}

	// 获取当前价格
	marketData, err := market.Get(decision.Symbol)
	if err != nil {
		return err
	}

	// 计算数量
	quantity := decision.PositionSizeUSD / marketData.CurrentPrice
	actionRecord.Quantity = quantity
	actionRecord.Price = marketData.CurrentPrice

	// ⚠️ 保证金验证：防止保证金不足错误（code=-2019）
	requiredMargin := decision.PositionSizeUSD / float64(decision.Leverage)

	// 手续费估算（Taker费率 0.04%）
	estimatedFee := decision.PositionSizeUSD * 0.0004
	totalRequired := requiredMargin + estimatedFee

	if totalRequired > availableBalance {
		return fmt.Errorf("❌ 保证金不足: 需要 %.2f USDT（保证金 %.2f + 手续费 %.2f），可用 %.2f USDT",
			totalRequired, requiredMargin, estimatedFee, availableBalance)
	}

	// 设置仓位模式
	if err := at.trader.SetMarginMode(decision.Symbol, at.config.IsCrossMargin); err != nil {
		log.Printf("  ⚠️ 设置仓位模式失败: %v", err)
		// 继续执行，不影响交易
	}

	// 开仓
	order, err := at.trader.OpenLong(decision.Symbol, quantity, decision.Leverage)
	if err != nil {
		return err
	}

	// 记录订单ID
	if orderID, ok := order["orderId"].(int64); ok {
		actionRecord.OrderID = orderID
	}

	log.Printf("  ✓ 开仓成功，订单ID: %v, 数量: %.4f", order["orderId"], quantity)

	// 记录开仓时间
	posKey := decision.Symbol + "_long"
	at.positionFirstSeenTime[posKey] = time.Now().UnixMilli()

	// 设置止损（止盈由 AI 通过 set_tp_order 独立控制，支持分批止盈）
	if decision.StopLoss > 0 {
		if err := at.trader.SetStopLoss(decision.Symbol, "LONG", quantity, decision.StopLoss); err != nil {
			log.Printf("  ⚠ 设置止损失败: %v", err)
		}
	}
	// 注意: 不再自动设置止盈，改由 AI 发送 set_tp_order 决策分批设置

	return nil
}

// executeOpenShortWithRecord 执行开空仓并记录详细信息
func (at *AutoTrader) executeOpenShortWithRecord(decision *decision.Decision, actionRecord *logger.DecisionAction) error {
	log.Printf("  📉 开空仓: %s", decision.Symbol)

	// ⚠️ 关键：检查是否已有同币种同方向持仓，如果有则拒绝开仓（防止仓位叠加超限）
	positions, err := at.trader.GetPositions()
	if err == nil {
		for _, pos := range positions {
			if pos["symbol"] == decision.Symbol && pos["side"] == "short" {
				return fmt.Errorf("❌ %s 已有空仓，拒绝开仓以防止仓位叠加超限。如需换仓，请先给出 close_short 决策", decision.Symbol)
			}
		}
	}

	// 先获取账户余额，用于后续保证金校验和自动缩小仓位
	balance, err := at.trader.GetBalance()
	if err != nil {
		return fmt.Errorf("获取账户余额失败: %w", err)
	}
	availableBalance := 0.0
	if avail, ok := balance["availableBalance"].(float64); ok {
		availableBalance = avail
	}

	// ⚠️ 自动缩小仓位：防止因 AI 给出的名义金额过大导致保证金不足
	if decision.Leverage <= 0 {
		decision.Leverage = at.config.BTCETHLeverage
	}
	maxPositionSizeUSD := availableBalance * float64(decision.Leverage) * 0.95 // 预留 5% 作为手续费和波动缓冲
	if decision.PositionSizeUSD > maxPositionSizeUSD {
		log.Printf("  ⚠️ 开空金额 %.2f USDT 超过账户可承受上限 %.2f USDT（可用余额 %.2f, 杠杆 %dx），自动缩小仓位",
			decision.PositionSizeUSD, maxPositionSizeUSD, availableBalance, decision.Leverage)
		decision.PositionSizeUSD = maxPositionSizeUSD
	}

	// 获取当前价格
	marketData, err := market.Get(decision.Symbol)
	if err != nil {
		return err
	}

	// 计算数量
	quantity := decision.PositionSizeUSD / marketData.CurrentPrice
	actionRecord.Quantity = quantity
	actionRecord.Price = marketData.CurrentPrice

	// ⚠️ 保证金验证：防止保证金不足错误（code=-2019）
	requiredMargin := decision.PositionSizeUSD / float64(decision.Leverage)

	// 手续费估算（Taker费率 0.04%）
	estimatedFee := decision.PositionSizeUSD * 0.0004
	totalRequired := requiredMargin + estimatedFee

	if totalRequired > availableBalance {
		return fmt.Errorf("❌ 保证金不足: 需要 %.2f USDT（保证金 %.2f + 手续费 %.2f），可用 %.2f USDT",
			totalRequired, requiredMargin, estimatedFee, availableBalance)
	}

	// 设置仓位模式
	if err := at.trader.SetMarginMode(decision.Symbol, at.config.IsCrossMargin); err != nil {
		log.Printf("  ⚠️ 设置仓位模式失败: %v", err)
		// 继续执行，不影响交易
	}

	// 开仓
	order, err := at.trader.OpenShort(decision.Symbol, quantity, decision.Leverage)
	if err != nil {
		return err
	}

	// 记录订单ID
	if orderID, ok := order["orderId"].(int64); ok {
		actionRecord.OrderID = orderID
	}

	log.Printf("  ✓ 开仓成功，订单ID: %v, 数量: %.4f", order["orderId"], quantity)

	// 记录开仓时间
	posKey := decision.Symbol + "_short"
	at.positionFirstSeenTime[posKey] = time.Now().UnixMilli()

	// 设置止损（止盈由 AI 通过 set_tp_order 独立控制，支持分批止盈）
	if decision.StopLoss > 0 {
		if err := at.trader.SetStopLoss(decision.Symbol, "SHORT", quantity, decision.StopLoss); err != nil {
			log.Printf("  ⚠ 设置止损失败: %v", err)
		}
	}
	return nil
}

// executePlaceLimitOrderWithRecord 执行限价委托开仓并记录
func (at *AutoTrader) executePlaceLimitOrderWithRecord(side, tradeSide string, d *decision.Decision, actionRecord *logger.DecisionAction) error {
	log.Printf("  📝 限价委托 (%s %s): %s 价格: %.2f", side, tradeSide, d.Symbol, d.Price)

	if d.Price <= 0 {
		return fmt.Errorf("限价委托必须提供有效的价格")
	}

	// 0. 防重复下单检查
	openOrders, err := at.trader.GetOpenOrders(d.Symbol)
	if err == nil {
		for _, order := range openOrders {
			// 检查是否为同方向的普通限价单
			// orderType通常是 limit
			// side通常是 buy/sell 或 open_long/short (取决于交易所实现)
			// 这里做一个宽泛匹配

			oType, _ := order["type"].(string)
			// oSide, _ := order["side"].(string)
			oPrice, _ := order["price"].(float64)

			if strings.ToLower(oType) != "limit" {
				continue
			}

			// 检查方向是否一致
			// 入参 side: "buy" | "sell"
			// 订单 side: bitget="open_long"(buy), "open_short"(sell) ? 需要确认
			// 简单起见，只要价格极度接近且是限价单，就视为重复
			// (同一价格两个方向同时挂单的情况较少，且AI策略通常不会这么做)
			if math.Abs(oPrice-d.Price)/d.Price < 0.001 {
				log.Printf("  ⚠️ 已存在价格为 %.2f 的限价单 (ID: %v)，跳过重复设置", oPrice, order["order_id"])
				return nil
			}
		}
	}

	// 计算数量
	sb := at.initialBalance
	if sb <= 0 {
		sb = 1000
	}
	sizeUSD := d.PositionSizeUSD
	if sizeUSD <= 0 {
		return fmt.Errorf("未提供有效的仓位大小")
	}

	quantity := sizeUSD / d.Price
	actionRecord.Quantity = quantity
	actionRecord.Price = d.Price
	actionRecord.Leverage = d.Leverage

	// 执行下单
	order, err := at.trader.PlaceLimitOrder(d.Symbol, side, tradeSide, quantity, d.Price, d.Leverage)
	if err != nil {
		return err
	}

	log.Printf("  ✓ 限价委托成功，订单信息: %v", order)
	return nil
}

// executeCancelOrderWithRecord 执行撤单并记录
func (at *AutoTrader) executeCancelOrderWithRecord(d *decision.Decision, actionRecord *logger.DecisionAction) error {
	log.Printf("  🗑️ 取消委托: %s (ID: %s)", d.Symbol, d.OrderID)

	if d.OrderID == "" {
		log.Printf("  ℹ️ 未提供订单ID，将取消 %s 的所有挂单", d.Symbol)
		return at.trader.CancelAllOrders(d.Symbol)
	}

	return at.trader.CancelOrder(d.Symbol, d.OrderID)
}

// executeCloseLongWithRecord 执行平多仓并记录详细信息
func (at *AutoTrader) executeCloseLongWithRecord(decision *decision.Decision, actionRecord *logger.DecisionAction) error {
	log.Printf("  🔄 平多仓: %s", decision.Symbol)

	// 获取当前价格
	marketData, err := market.Get(decision.Symbol)
	if err != nil {
		return err
	}
	actionRecord.Price = marketData.CurrentPrice

	// 平仓
	order, err := at.trader.CloseLong(decision.Symbol, 0) // 0 = 全部平仓
	if err != nil {
		return err
	}

	// 记录订单ID
	if orderID, ok := order["orderId"].(int64); ok {
		actionRecord.OrderID = orderID
	}

	log.Printf("  ✓ 平仓成功")
	return nil
}

// executeCloseShortWithRecord 执行平空仓并记录详细信息
func (at *AutoTrader) executeCloseShortWithRecord(decision *decision.Decision, actionRecord *logger.DecisionAction) error {
	log.Printf("  🔄 平空仓: %s", decision.Symbol)

	// 获取当前价格
	marketData, err := market.Get(decision.Symbol)
	if err != nil {
		return err
	}
	actionRecord.Price = marketData.CurrentPrice

	// 平仓
	order, err := at.trader.CloseShort(decision.Symbol, 0) // 0 = 全部平仓
	if err != nil {
		return err
	}

	// 记录订单ID
	if orderID, ok := order["orderId"].(int64); ok {
		actionRecord.OrderID = orderID
	}

	log.Printf("  ✓ 平仓成功")
	return nil
}

// executeUpdateStopLossWithRecord 执行调整止损并记录详细信息
func (at *AutoTrader) executeUpdateStopLossWithRecord(decision *decision.Decision, actionRecord *logger.DecisionAction) error {
	log.Printf("  🎯 调整止损: %s → %.2f", decision.Symbol, decision.NewStopLoss)

	// 获取当前价格
	marketData, err := market.Get(decision.Symbol)
	if err != nil {
		return err
	}
	actionRecord.Price = marketData.CurrentPrice

	// 获取当前持仓
	positions, err := at.trader.GetPositions()
	if err != nil {
		return fmt.Errorf("获取持仓失败: %w", err)
	}

	// 查找目标持仓
	var targetPosition map[string]interface{}
	for _, pos := range positions {
		symbol, _ := pos["symbol"].(string)
		posAmt, _ := pos["positionAmt"].(float64)
		if symbol == decision.Symbol && posAmt != 0 {
			targetPosition = pos
			break
		}
	}

	if targetPosition == nil {
		return fmt.Errorf("持仓不存在: %s", decision.Symbol)
	}

	// 获取持仓方向和数量
	side, _ := targetPosition["side"].(string)
	positionSide := strings.ToUpper(side)
	positionAmt, _ := targetPosition["positionAmt"].(float64)

	// 🔑 关键修复：使用 available（可平数量）而不是 positionAmt（总持仓）
	// 当已有止盈止损单时，available < positionAmt，使用 positionAmt 会导致 43023 "仓位不足" 错误
	available, ok := targetPosition["available"].(float64)
	if !ok || available <= 0 {
		available = positionAmt // 降级到 positionAmt
	}
	log.Printf("  📊 持仓信息: %s %s 总持仓=%.4f 可平=%.4f", decision.Symbol, positionSide, positionAmt, available)

	// 验证新止损价格合理性
	if positionSide == "LONG" && decision.NewStopLoss >= marketData.CurrentPrice {
		return fmt.Errorf("多单止损必须低于当前价格 (当前: %.2f, 新止损: %.2f)", marketData.CurrentPrice, decision.NewStopLoss)
	}
	if positionSide == "SHORT" && decision.NewStopLoss <= marketData.CurrentPrice {
		return fmt.Errorf("空单止损必须高于当前价格 (当前: %.2f, 新止损: %.2f)", marketData.CurrentPrice, decision.NewStopLoss)
	}

	// ⚠️ 防御性检查：检测是否存在双向持仓（不应该出现，但提供保护）
	var hasOppositePosition bool
	oppositeSide := ""
	for _, pos := range positions {
		symbol, _ := pos["symbol"].(string)
		posSide, _ := pos["side"].(string)
		posAmt, _ := pos["positionAmt"].(float64)
		if symbol == decision.Symbol && posAmt != 0 && strings.ToUpper(posSide) != positionSide {
			hasOppositePosition = true
			oppositeSide = strings.ToUpper(posSide)
			break
		}
	}

	if hasOppositePosition {
		log.Printf("  🚨 警告：检测到 %s 存在双向持仓（%s + %s），这违反了策略规则",
			decision.Symbol, positionSide, oppositeSide)
		log.Printf("  🚨 取消止损单将影响两个方向的订单，请检查是否为用户手动操作导致")
		log.Printf("  🚨 建议：手动平掉其中一个方向的持仓，或检查系统是否有BUG")
	}

	// 取消旧的止损单（只删除止损单，不影响止盈单）
	// 注意：如果存在双向持仓，这会删除两个方向的止损单
	if err := at.trader.CancelStopLossOrders(decision.Symbol); err != nil {
		log.Printf("  ⚠ 取消旧止损单失败: %v", err)
		// 不中断执行，继续设置新止损
	}

	// 调用交易所 API 修改止损（使用 available 可平数量）
	quantity := math.Abs(available)
	err = at.trader.SetStopLoss(decision.Symbol, positionSide, quantity, decision.NewStopLoss)
	if err != nil {
		return fmt.Errorf("修改止损失败: %w", err)
	}

	log.Printf("  ✓ 止损已调整: %.2f (当前价格: %.2f)", decision.NewStopLoss, marketData.CurrentPrice)
	return nil
}

// executeUpdateTakeProfitWithRecord 执行调整止盈并记录详细信息
func (at *AutoTrader) executeUpdateTakeProfitWithRecord(decision *decision.Decision, actionRecord *logger.DecisionAction) error {
	log.Printf("  🎯 调整止盈: %s → %.2f", decision.Symbol, decision.NewTakeProfit)

	// 获取当前价格
	marketData, err := market.Get(decision.Symbol)
	if err != nil {
		return err
	}
	actionRecord.Price = marketData.CurrentPrice

	// 获取当前持仓
	positions, err := at.trader.GetPositions()
	if err != nil {
		return fmt.Errorf("获取持仓失败: %w", err)
	}

	// 查找目标持仓
	var targetPosition map[string]interface{}
	for _, pos := range positions {
		symbol, _ := pos["symbol"].(string)
		posAmt, _ := pos["positionAmt"].(float64)
		if symbol == decision.Symbol && posAmt != 0 {
			targetPosition = pos
			break
		}
	}

	if targetPosition == nil {
		return fmt.Errorf("持仓不存在: %s", decision.Symbol)
	}

	// 获取持仓方向和数量
	side, _ := targetPosition["side"].(string)
	positionSide := strings.ToUpper(side)
	positionAmt, _ := targetPosition["positionAmt"].(float64)

	// 🔑 关键修复：使用 available（可平数量）而不是 positionAmt（总持仓）
	// 当已有止盈止损单时，available < positionAmt，使用 positionAmt 会导致 43023 "仓位不足" 错误
	available, ok := targetPosition["available"].(float64)
	if !ok || available <= 0 {
		available = positionAmt // 降级到 positionAmt
	}
	log.Printf("  📊 持仓信息: %s %s 总持仓=%.4f 可平=%.4f", decision.Symbol, positionSide, positionAmt, available)

	// 验证新止盈价格合理性
	if positionSide == "LONG" && decision.NewTakeProfit <= marketData.CurrentPrice {
		return fmt.Errorf("多单止盈必须高于当前价格 (当前: %.2f, 新止盈: %.2f)", marketData.CurrentPrice, decision.NewTakeProfit)
	}
	if positionSide == "SHORT" && decision.NewTakeProfit >= marketData.CurrentPrice {
		return fmt.Errorf("空单止盈必须低于当前价格 (当前: %.2f, 新止盈: %.2f)", marketData.CurrentPrice, decision.NewTakeProfit)
	}

	// ⚠️ 防御性检查：检测是否存在双向持仓（不应该出现，但提供保护）
	var hasOppositePosition bool
	oppositeSide := ""
	for _, pos := range positions {
		symbol, _ := pos["symbol"].(string)
		posSide, _ := pos["side"].(string)
		posAmt, _ := pos["positionAmt"].(float64)
		if symbol == decision.Symbol && posAmt != 0 && strings.ToUpper(posSide) != positionSide {
			hasOppositePosition = true
			oppositeSide = strings.ToUpper(posSide)
			break
		}
	}

	if hasOppositePosition {
		log.Printf("  🚨 警告：检测到 %s 存在双向持仓（%s + %s），这违反了策略规则",
			decision.Symbol, positionSide, oppositeSide)
		log.Printf("  🚨 取消止盈单将影响两个方向的订单，请检查是否为用户手动操作导致")
		log.Printf("  🚨 建议：手动平掉其中一个方向的持仓，或检查系统是否有BUG")
	}

	// 取消旧的止盈单（只删除止盈单，不影响止损单）
	// 注意：如果存在双向持仓，这会删除两个方向的止盈单
	if err := at.trader.CancelTakeProfitOrders(decision.Symbol); err != nil {
		log.Printf("  ⚠ 取消旧止盈单失败: %v", err)
		// 不中断执行，继续设置新止盈
	}

	// 调用交易所 API 修改止盈（使用 available 可平数量）
	quantity := math.Abs(available)
	err = at.trader.SetTakeProfit(decision.Symbol, positionSide, quantity, decision.NewTakeProfit)
	if err != nil {
		return fmt.Errorf("修改止盈失败: %w", err)
	}

	log.Printf("  ✓ 止盈已调整: %.2f (当前价格: %.2f)", decision.NewTakeProfit, marketData.CurrentPrice)
	return nil
}

// executePartialCloseWithRecord 执行部分平仓并记录详细信息
func (at *AutoTrader) executePartialCloseWithRecord(decision *decision.Decision, actionRecord *logger.DecisionAction) error {
	log.Printf("  📊 部分平仓: %s %.1f%%", decision.Symbol, decision.ClosePercentage)

	// 验证百分比范围
	if decision.ClosePercentage <= 0 || decision.ClosePercentage > 100 {
		return fmt.Errorf("平仓百分比必须在 0-100 之间，当前: %.1f", decision.ClosePercentage)
	}

	// 获取当前价格
	marketData, err := market.Get(decision.Symbol)
	if err != nil {
		return err
	}
	actionRecord.Price = marketData.CurrentPrice

	// 获取当前持仓
	positions, err := at.trader.GetPositions()
	if err != nil {
		return fmt.Errorf("获取持仓失败: %w", err)
	}

	// 查找目标持仓
	var targetPosition map[string]interface{}
	for _, pos := range positions {
		symbol, _ := pos["symbol"].(string)
		posAmt, _ := pos["positionAmt"].(float64)
		if symbol == decision.Symbol && posAmt != 0 {
			targetPosition = pos
			break
		}
	}

	if targetPosition == nil {
		return fmt.Errorf("持仓不存在: %s", decision.Symbol)
	}

	// 获取持仓方向和数量
	side, _ := targetPosition["side"].(string)
	positionSide := strings.ToUpper(side)
	positionAmt, _ := targetPosition["positionAmt"].(float64)

	// 计算平仓数量
	totalQuantity := math.Abs(positionAmt)
	closeQuantity := totalQuantity * (decision.ClosePercentage / 100.0)
	actionRecord.Quantity = closeQuantity

	// 执行平仓
	var order map[string]interface{}
	if positionSide == "LONG" {
		order, err = at.trader.CloseLong(decision.Symbol, closeQuantity)
	} else {
		order, err = at.trader.CloseShort(decision.Symbol, closeQuantity)
	}

	if err != nil {
		return fmt.Errorf("部分平仓失败: %w", err)
	}

	// 记录订单ID
	if orderID, ok := order["orderId"].(int64); ok {
		actionRecord.OrderID = orderID
	}

	remainingQuantity := totalQuantity - closeQuantity
	log.Printf("  ✓ 部分平仓成功: 平仓 %.4f (%.1f%%), 剩余 %.4f",
		closeQuantity, decision.ClosePercentage, remainingQuantity)

	return nil
}

// executeSetTPOrderWithRecord 设置止盈委托单（挂单，达到触发价后自动平仓）
func (at *AutoTrader) executeSetTPOrderWithRecord(decision *decision.Decision, actionRecord *logger.DecisionAction) error {
	log.Printf("  📈 设置止盈单: %s @ %.2f (平仓 %.0f%%)", decision.Symbol, decision.TpTriggerPrice, decision.TpClosePercentage)

	// 获取当前价格
	marketData, err := market.Get(decision.Symbol)
	if err != nil {
		return err
	}
	actionRecord.Price = marketData.CurrentPrice

	// 0. 防重复下单检查：检查是否已存在相同价格的止盈单
	openOrders, err := at.trader.GetOpenOrders(decision.Symbol)
	if err == nil {
		for _, order := range openOrders {
			// 检查是否为止盈单 (type包含 profit 或 planType=profit_plan)
			orderType, _ := order["type"].(string)
			planType, _ := order["planType"].(string)

			isTP := strings.Contains(strings.ToLower(orderType), "profit") ||
				strings.Contains(strings.ToLower(planType), "profit")

			if isTP {
				// 获取触发价格
				var triggerPrice float64
				if tp, ok := order["triggerPrice"].(float64); ok {
					triggerPrice = tp
				} else if p, ok := order["price"].(float64); ok { // 部分接口可能放在 price
					triggerPrice = p
				}

				// 如果价格接近 (1%以内)，则认为是重复单
				if math.Abs(triggerPrice-decision.TpTriggerPrice)/decision.TpTriggerPrice < 0.01 {
					log.Printf("  ⚠️ 已存在价格为 %.2f 的止盈单 (ID: %v)，跳过重复设置", triggerPrice, order["order_id"])
					return nil
				}
			}
		}
	}

	// 获取当前持仓
	positions, err := at.trader.GetPositions()
	if err != nil {
		return fmt.Errorf("获取持仓失败: %w", err)
	}

	// 查找目标持仓
	var targetPosition map[string]interface{}
	for _, pos := range positions {
		symbol, _ := pos["symbol"].(string)
		posAmt, _ := pos["positionAmt"].(float64)
		if symbol == decision.Symbol && posAmt != 0 {
			targetPosition = pos
			break
		}
	}

	if targetPosition == nil {
		return fmt.Errorf("持仓不存在: %s", decision.Symbol)
	}

	// 获取持仓方向和数量
	side, _ := targetPosition["side"].(string)
	positionSide := strings.ToUpper(side)
	positionAmt, _ := targetPosition["positionAmt"].(float64)
	available, ok := targetPosition["available"].(float64)
	if !ok || available <= 0 {
		available = positionAmt
	}

	// 验证止盈触发价格
	if decision.TpTriggerPrice <= 0 {
		return fmt.Errorf("止盈触发价格无效: %.2f", decision.TpTriggerPrice)
	}

	// 验证方向与止盈价的关系
	if positionSide == "LONG" && decision.TpTriggerPrice <= marketData.CurrentPrice {
		return fmt.Errorf("多单止盈价必须高于当前价格 (当前: %.2f, 止盈: %.2f)", marketData.CurrentPrice, decision.TpTriggerPrice)
	}
	if positionSide == "SHORT" && decision.TpTriggerPrice >= marketData.CurrentPrice {
		return fmt.Errorf("空单止盈价必须低于当前价格 (当前: %.2f, 止盈: %.2f)", marketData.CurrentPrice, decision.TpTriggerPrice)
	}

	// 计算止盈数量
	closePercent := decision.TpClosePercentage
	if closePercent <= 0 || closePercent > 100 {
		closePercent = 100 // 默认全部止盈
	}
	quantity := math.Abs(available) * (closePercent / 100)

	// 检查最小交易量（需要 Trader 接口支持 GetMinTradeNum 方法）
	if minChecker, ok := at.trader.(interface{ GetMinTradeNum(string) (float64, error) }); ok {
		minNum, _ := minChecker.GetMinTradeNum(decision.Symbol)
		if quantity < minNum {
			// 如果计算数量低于最小值，检查是否能使用最小值
			if math.Abs(available) >= minNum {
				log.Printf("  ⚠️ 止盈数量 %.6f 低于最小值 %.6f，自动调整为最小值", quantity, minNum)
				quantity = minNum
			} else {
				// 可用数量本身就不足，跳过该止盈单（记录警告）
				log.Printf("  ⚠️ 可用数量 %.6f 低于最小交易量 %.6f，无法设置止盈单", available, minNum)
				return fmt.Errorf("仓位太小无法分批止盈，可用: %.6f, 最小: %.6f", available, minNum)
			}
		}
	}

	actionRecord.Quantity = quantity

	// 设置止盈委托
	err = at.trader.SetTakeProfit(decision.Symbol, positionSide, quantity, decision.TpTriggerPrice)
	if err != nil {
		return fmt.Errorf("设置止盈单失败: %w", err)
	}

	log.Printf("  ✓ 止盈单已设置: %s @ %.2f 平仓 %.4f (%.0f%%)", decision.Symbol, decision.TpTriggerPrice, quantity, closePercent)
	return nil
}

// executeSetSLOrderWithRecord 设置止损委托单（挂单，达到触发价后自动平仓）
func (at *AutoTrader) executeSetSLOrderWithRecord(decision *decision.Decision, actionRecord *logger.DecisionAction) error {
	log.Printf("  📉 设置止损单: %s @ %.2f", decision.Symbol, decision.SlTriggerPrice)

	// 获取当前价格
	marketData, err := market.Get(decision.Symbol)
	if err != nil {
		return err
	}
	actionRecord.Price = marketData.CurrentPrice

	// 获取当前持仓
	positions, err := at.trader.GetPositions()
	if err != nil {
		return fmt.Errorf("获取持仓失败: %w", err)
	}

	// 查找目标持仓
	var targetPosition map[string]interface{}
	for _, pos := range positions {
		symbol, _ := pos["symbol"].(string)
		posAmt, _ := pos["positionAmt"].(float64)
		if symbol == decision.Symbol && posAmt != 0 {
			targetPosition = pos
			break
		}
	}

	if targetPosition == nil {
		return fmt.Errorf("持仓不存在: %s", decision.Symbol)
	}

	// 获取持仓方向和数量
	side, _ := targetPosition["side"].(string)
	positionSide := strings.ToUpper(side)
	positionAmt, _ := targetPosition["positionAmt"].(float64)
	available, ok := targetPosition["available"].(float64)
	if !ok || available <= 0 {
		available = positionAmt
	}

	// 验证止损触发价格
	if decision.SlTriggerPrice <= 0 {
		return fmt.Errorf("止损触发价格无效: %.2f", decision.SlTriggerPrice)
	}

	// 验证方向与止损价的关系
	if positionSide == "LONG" && decision.SlTriggerPrice >= marketData.CurrentPrice {
		return fmt.Errorf("多单止损价必须低于当前价格 (当前: %.2f, 止损: %.2f)", marketData.CurrentPrice, decision.SlTriggerPrice)
	}
	if positionSide == "SHORT" && decision.SlTriggerPrice <= marketData.CurrentPrice {
		return fmt.Errorf("空单止损价必须高于当前价格 (当前: %.2f, 止损: %.2f)", marketData.CurrentPrice, decision.SlTriggerPrice)
	}

	// 止损全仓
	quantity := math.Abs(available)
	actionRecord.Quantity = quantity

	// ⚠️ 先取消已有的止损单，防止重复叠加
	if canceler, ok := at.trader.(interface{ CancelStopLossOrders(string) error }); ok {
		log.Printf("  🗑️ 正在取消已有止损单...")
		if err := canceler.CancelStopLossOrders(decision.Symbol); err != nil {
			log.Printf("  ⚠️ 取消旧止损单失败（可能不存在）: %v", err)
			// 不影响继续设置新止损
		}
	}

	// 设置止损委托
	err = at.trader.SetStopLoss(decision.Symbol, positionSide, quantity, decision.SlTriggerPrice)
	if err != nil {
		return fmt.Errorf("设置止损单失败: %w", err)
	}

	log.Printf("  ✓ 止损单已设置: %s @ %.2f 平仓 %.4f", decision.Symbol, decision.SlTriggerPrice, quantity)
	return nil
}

// GetID 获取trader ID
func (at *AutoTrader) GetID() string {
	return at.id
}

// GetName 获取trader名称
func (at *AutoTrader) GetName() string {
	return at.name
}

// GetAIModel 获取AI模型
func (at *AutoTrader) GetAIModel() string {
	return at.aiModel
}

// GetExchange 获取交易所
func (at *AutoTrader) GetExchange() string {
	return at.exchange
}

// SetCustomPrompt 设置自定义交易策略prompt
func (at *AutoTrader) SetCustomPrompt(prompt string) {
	at.mu.Lock()
	defer at.mu.Unlock()
	at.customPrompt = prompt
	log.Printf("🔄 [%s] 自定义提示词已更新", at.name)
}

// SetOverrideBasePrompt 设置是否覆盖基础prompt
func (at *AutoTrader) SetOverrideBasePrompt(override bool) {
	at.mu.Lock()
	defer at.mu.Unlock()
	at.overrideBasePrompt = override
	log.Printf("🔄 [%s] 覆盖基础提示词设置已更新: %v", at.name, override)
}

// SetSystemPromptTemplate 设置系统提示词模板
func (at *AutoTrader) SetSystemPromptTemplate(templateName string) {
	at.mu.Lock()
	defer at.mu.Unlock()
	at.systemPromptTemplate = templateName
	log.Printf("🔄 [%s] 系统提示词模板已更新: %s", at.name, templateName)
}

// GetSystemPromptTemplate 获取当前系统提示词模板名称
func (at *AutoTrader) GetSystemPromptTemplate() string {
	return at.systemPromptTemplate
}

// GetDecisionLogger 获取决策日志记录器
func (at *AutoTrader) GetDecisionLogger() *logger.DecisionLogger {
	return at.decisionLogger
}

// GetStatus 获取系统状态（用于API）
func (at *AutoTrader) GetStatus() map[string]interface{} {
	aiProvider := "DeepSeek"
	if at.config.UseQwen {
		aiProvider = "Qwen"
	}

	return map[string]interface{}{
		"trader_id":       at.id,
		"trader_name":     at.name,
		"ai_model":        at.aiModel,
		"exchange":        at.exchange,
		"is_running":      at.isRunning,
		"start_time":      at.startTime.Format(time.RFC3339),
		"runtime_minutes": int(time.Since(at.startTime).Minutes()),
		"call_count":      at.callCount,
		"initial_balance": at.initialBalance,
		"scan_interval":   at.config.ScanInterval.String(),
		"stop_until":      at.stopUntil.Format(time.RFC3339),
		"last_reset_time": at.lastResetTime.Format(time.RFC3339),
		"ai_provider":     aiProvider,
	}
}

// GetAccountInfo 获取账户信息（用于API）
func (at *AutoTrader) GetAccountInfo() (map[string]interface{}, error) {
	balance, err := at.trader.GetBalance()
	if err != nil {
		return nil, fmt.Errorf("获取余额失败: %w", err)
	}

	// 获取账户字段
	totalWalletBalance := 0.0
	totalUnrealizedProfit := 0.0
	availableBalance := 0.0

	if wallet, ok := balance["totalWalletBalance"].(float64); ok {
		totalWalletBalance = wallet
	}
	if unrealized, ok := balance["totalUnrealizedProfit"].(float64); ok {
		totalUnrealizedProfit = unrealized
	}
	if avail, ok := balance["availableBalance"].(float64); ok {
		availableBalance = avail
	}

	// Total Equity = 钱包余额 + 未实现盈亏
	totalEquity := totalWalletBalance + totalUnrealizedProfit

	// 记录初始余额状态（用于调试）
	log.Printf("🔍 [%s] GetAccountInfo - 当前initial_balance: %.2f, total_equity: %.2f", at.name, at.initialBalance, totalEquity)

	// 获取持仓计算总保证金
	positions, err := at.trader.GetPositions()
	if err != nil {
		return nil, fmt.Errorf("获取持仓失败: %w", err)
	}

	totalMarginUsed := 0.0
	totalUnrealizedPnL := 0.0
	for _, pos := range positions {
		markPrice := pos["markPrice"].(float64)
		quantity := pos["positionAmt"].(float64)
		if quantity < 0 {
			quantity = -quantity
		}
		unrealizedPnl := pos["unRealizedProfit"].(float64)
		totalUnrealizedPnL += unrealizedPnl

		leverage := 10
		if lev, ok := pos["leverage"].(float64); ok {
			leverage = int(lev)
		}
		marginUsed := (quantity * markPrice) / float64(leverage)
		totalMarginUsed += marginUsed
	}

	totalPnL := totalEquity - at.initialBalance
	totalPnLPct := 0.0
	if at.initialBalance > 0 {
		totalPnLPct = (totalPnL / at.initialBalance) * 100
	}

	marginUsedPct := 0.0
	if totalEquity > 0 {
		marginUsedPct = (totalMarginUsed / totalEquity) * 100
	}

	return map[string]interface{}{
		// 核心字段
		"total_equity":      totalEquity,           // 账户净值 = wallet + unrealized
		"wallet_balance":    totalWalletBalance,    // 钱包余额（不含未实现盈亏）
		"unrealized_profit": totalUnrealizedProfit, // 未实现盈亏（从API）
		"available_balance": availableBalance,      // 可用余额

		// 盈亏统计
		"total_pnl":            totalPnL,           // 总盈亏 = equity - initial
		"total_pnl_pct":        totalPnLPct,        // 总盈亏百分比
		"total_unrealized_pnl": totalUnrealizedPnL, // 未实现盈亏（从持仓计算）
		"initial_balance":      at.initialBalance,  // 初始余额
		"daily_pnl":            at.dailyPnL,        // 日盈亏

		// 持仓信息
		"position_count":  len(positions),  // 持仓数量
		"margin_used":     totalMarginUsed, // 保证金占用
		"margin_used_pct": marginUsedPct,   // 保证金使用率
	}, nil
}

// GetPositions 获取持仓列表（用于API）
func (at *AutoTrader) GetPositions() ([]map[string]interface{}, error) {
	positions, err := at.trader.GetPositions()
	if err != nil {
		return nil, fmt.Errorf("获取持仓失败: %w", err)
	}

	var result []map[string]interface{}
	for _, pos := range positions {
		symbol := pos["symbol"].(string)
		side := pos["side"].(string)
		entryPrice := pos["entryPrice"].(float64)
		markPrice := pos["markPrice"].(float64)
		quantity := pos["positionAmt"].(float64)
		if quantity < 0 {
			quantity = -quantity
		}
		unrealizedPnl := pos["unRealizedProfit"].(float64)
		liquidationPrice := pos["liquidationPrice"].(float64)

		leverage := 10
		if lev, ok := pos["leverage"].(float64); ok {
			leverage = int(lev)
		}

		// 计算占用保证金
		marginUsed := (quantity * markPrice) / float64(leverage)

		// 计算盈亏百分比（基于保证金）
		pnlPct := calculatePnLPercentage(unrealizedPnl, marginUsed)

		result = append(result, map[string]interface{}{
			"symbol":             symbol,
			"side":               side,
			"entry_price":        entryPrice,
			"mark_price":         markPrice,
			"quantity":           quantity,
			"leverage":           leverage,
			"unrealized_pnl":     unrealizedPnl,
			"unrealized_pnl_pct": pnlPct,
			"liquidation_price":  liquidationPrice,
			"margin_used":        marginUsed,
		})
	}

	return result, nil
}

// calculatePnLPercentage 计算盈亏百分比（基于保证金，自动考虑杠杆）
// 收益率 = 未实现盈亏 / 保证金 × 100%
func calculatePnLPercentage(unrealizedPnl, marginUsed float64) float64 {
	if marginUsed > 0 {
		return (unrealizedPnl / marginUsed) * 100
	}
	return 0.0
}

// sortDecisionsByPriority 对决策排序：先平仓，再开仓，最后hold/wait
// 这样可以避免换仓时仓位叠加超限
func sortDecisionsByPriority(decisions []decision.Decision) []decision.Decision {
	if len(decisions) <= 1 {
		return decisions
	}

	// 定义优先级
	getActionPriority := func(action string) int {
		switch action {
		case "close_long", "close_short", "partial_close":
			return 1 // 最高优先级：先平仓（包括部分平仓）
		case "update_stop_loss", "update_take_profit":
			return 2 // 调整持仓止盈止损
		case "open_long", "open_short":
			return 3 // 次优先级：后开仓
		case "hold", "wait":
			return 4 // 最低优先级：观望
		default:
			return 999 // 未知动作放最后
		}
	}

	// 复制决策列表
	sorted := make([]decision.Decision, len(decisions))
	copy(sorted, decisions)

	// 按优先级排序
	for i := 0; i < len(sorted)-1; i++ {
		for j := i + 1; j < len(sorted); j++ {
			if getActionPriority(sorted[i].Action) > getActionPriority(sorted[j].Action) {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	return sorted
}

// getCandidateCoins 获取交易员的候选币种列表
func (at *AutoTrader) getCandidateCoins() ([]decision.CandidateCoin, error) {
	if len(at.tradingCoins) == 0 {
		// 使用数据库配置的默认币种列表
		var candidateCoins []decision.CandidateCoin

		if len(at.defaultCoins) > 0 {
			// 使用数据库中配置的默认币种
			for _, coin := range at.defaultCoins {
				symbol := normalizeSymbol(coin)
				candidateCoins = append(candidateCoins, decision.CandidateCoin{
					Symbol:  symbol,
					Sources: []string{"default"}, // 标记为数据库默认币种
				})
			}
			log.Printf("📋 [%s] 使用数据库默认币种: %d个币种 %v",
				at.name, len(candidateCoins), at.defaultCoins)
			return candidateCoins, nil
		} else {
			// 如果数据库中没有配置默认币种，则使用AI500+OI Top作为fallback
			const ai500Limit = 20 // AI500取前20个评分最高的币种

			mergedPool, err := pool.GetMergedCoinPool(ai500Limit)
			if err != nil {
				return nil, fmt.Errorf("获取合并币种池失败: %w", err)
			}

			// 构建候选币种列表（包含来源信息）
			for _, symbol := range mergedPool.AllSymbols {
				sources := mergedPool.SymbolSources[symbol]
				candidateCoins = append(candidateCoins, decision.CandidateCoin{
					Symbol:  symbol,
					Sources: sources, // "ai500" 和/或 "oi_top"
				})
			}

			log.Printf("📋 [%s] 数据库无默认币种配置，使用AI500+OI Top: AI500前%d + OI_Top20 = 总计%d个候选币种",
				at.name, ai500Limit, len(candidateCoins))
			return candidateCoins, nil
		}
	} else {
		// 使用自定义币种列表
		var candidateCoins []decision.CandidateCoin
		for _, coin := range at.tradingCoins {
			// 确保币种格式正确（转为大写USDT交易对）
			symbol := normalizeSymbol(coin)
			candidateCoins = append(candidateCoins, decision.CandidateCoin{
				Symbol:  symbol,
				Sources: []string{"custom"}, // 标记为自定义来源
			})
		}

		log.Printf("📋 [%s] 使用自定义币种: %d个币种 %v",
			at.name, len(candidateCoins), at.tradingCoins)
		return candidateCoins, nil
	}
}

// normalizeSymbol 标准化币种符号（确保以USDT结尾）
func normalizeSymbol(symbol string) string {
	// 转为大写
	symbol = strings.ToUpper(strings.TrimSpace(symbol))

	// 确保以USDT结尾
	if !strings.HasSuffix(symbol, "USDT") {
		symbol = symbol + "USDT"
	}

	return symbol
}

// 启动回撤监控
func (at *AutoTrader) startDrawdownMonitor() {
	at.monitorWg.Add(1)
	go func() {
		defer at.monitorWg.Done()

		ticker := time.NewTicker(1 * time.Minute) // 每分钟检查一次
		defer ticker.Stop()

		log.Println("📊 启动持仓回撤监控（每分钟检查一次）")

		for {
			select {
			case <-ticker.C:
				at.checkPositionDrawdown()
			case <-at.stopMonitorCh:
				log.Println("⏹ 停止持仓回撤监控")
				return
			}
		}
	}()
}

// 检查持仓回撤情况
func (at *AutoTrader) checkPositionDrawdown() {
	// 获取当前持仓
	positions, err := at.trader.GetPositions()
	if err != nil {
		log.Printf("❌ 回撤监控：获取持仓失败: %v", err)
		return
	}

	for _, pos := range positions {
		symbol := pos["symbol"].(string)
		side := pos["side"].(string)
		entryPrice := pos["entryPrice"].(float64)
		markPrice := pos["markPrice"].(float64)
		quantity := pos["positionAmt"].(float64)
		if quantity < 0 {
			quantity = -quantity // 空仓数量为负，转为正数
		}

		// 计算当前盈亏百分比
		leverage := 10 // 默认值
		if lev, ok := pos["leverage"].(float64); ok {
			leverage = int(lev)
		}

		var currentPnLPct float64
		if side == "long" {
			currentPnLPct = ((markPrice - entryPrice) / entryPrice) * float64(leverage) * 100
		} else {
			currentPnLPct = ((entryPrice - markPrice) / entryPrice) * float64(leverage) * 100
		}

		// 构造持仓唯一标识（区分多空）
		posKey := symbol + "_" + side

		// 获取该持仓的历史最高收益
		at.peakPnLCacheMutex.RLock()
		peakPnLPct, exists := at.peakPnLCache[posKey]
		at.peakPnLCacheMutex.RUnlock()

		if !exists {
			// 如果没有历史最高记录，使用当前盈亏作为初始值
			peakPnLPct = currentPnLPct
			at.UpdatePeakPnL(symbol, side, currentPnLPct)
		} else {
			// 更新峰值缓存
			at.UpdatePeakPnL(symbol, side, currentPnLPct)
		}

		// 计算回撤（从最高点下跌的幅度）
		var drawdownPct float64
		if peakPnLPct > 0 && currentPnLPct < peakPnLPct {
			drawdownPct = ((peakPnLPct - currentPnLPct) / peakPnLPct) * 100
		}

		// 检查平仓条件：收益大于5%且回撤超过40%
		if currentPnLPct > 5.0 && drawdownPct >= 40.0 {
			log.Printf("🚨 触发回撤平仓条件: %s %s | 当前收益: %.2f%% | 最高收益: %.2f%% | 回撤: %.2f%%",
				symbol, side, currentPnLPct, peakPnLPct, drawdownPct)

			// 执行平仓
			if err := at.emergencyClosePosition(symbol, side); err != nil {
				log.Printf("❌ 回撤平仓失败 (%s %s): %v", symbol, side, err)
			} else {
				log.Printf("✅ 回撤平仓成功: %s %s", symbol, side)
				// 平仓后清理该持仓的缓存
				at.ClearPeakPnLCache(symbol, side)

				// 【重要】记录此次回撤平仓到数据库（状态 + 决策历史），避免“平仓但不记账”
				if db, ok := at.database.(*sysconfig.Database); ok {
					// 1. 找到该交易对最近使用的策略ID（同一币对我们只保留最新策略）
					strategyID, err := db.GetLatestStrategyIDBySymbol(at.id, symbol)
					if err != nil {
						log.Printf("⚠️ 回撤平仓: 获取最新策略ID失败 (%s): %v", symbol, err)
					} else if strategyID != "" {
						// 2. 更新策略状态为 CLOSED（使用当前入场价作为参考）
						at.updateStrategyStatus(strategyID, symbol, "CLOSED", entryPrice, quantity, 0)

						// 3. 追加一条决策历史，标记为紧急回撤平仓，方便前端筛选 & 追踪
						action := "EMERGENCY_CLOSE"
						if side == "long" {
							action = "EMERGENCY_CLOSE_LONG"
						} else if side == "short" {
							action = "EMERGENCY_CLOSE_SHORT"
						}

						h := &sysconfig.StrategyDecisionHistory{
							TraderID:         at.id,
							StrategyID:       strategyID,
							DecisionTime:     time.Now(),
							Action:           action,
							Symbol:           symbol,
							CurrentPrice:     markPrice,
							TargetPrice:      entryPrice,
							PositionSide:     strings.ToUpper(side),
							PositionQty:      quantity,
							AmountPercent:    0,
							Reason:           fmt.Sprintf("Triggered drawdown close: currentPnL=%.2f%% peakPnL=%.2f%% drawdown=%.2f%%", currentPnLPct, peakPnLPct, drawdownPct),
							RSI1H:            0,
							RSI4H:            0,
							MACD4H:           0,
							SystemPrompt:     "drawdown_monitor",
							InputPrompt:      "",
							RawAIResponse:    "",
							ExecutionSuccess: true,
							ExecutionError:   "",
						}

						if err := db.SaveStrategyDecision(h); err != nil {
							log.Printf("⚠️ 回撤平仓: 保存决策历史失败 (%s): %v", symbol, err)
						}
					}
				}
			}
		} else if currentPnLPct > 5.0 {
			// 记录接近平仓条件的情况（用于调试）
			log.Printf("📊 回撤监控: %s %s | 收益: %.2f%% | 最高: %.2f%% | 回撤: %.2f%%",
				symbol, side, currentPnLPct, peakPnLPct, drawdownPct)
		}
	}
}

// 紧急平仓函数
func (at *AutoTrader) emergencyClosePosition(symbol, side string) error {
	switch side {
	case "long":
		order, err := at.trader.CloseLong(symbol, 0) // 0 = 全部平仓
		if err != nil {
			return err
		}
		log.Printf("✅ 紧急平多仓成功，订单ID: %v", order["orderId"])
	case "short":
		order, err := at.trader.CloseShort(symbol, 0) // 0 = 全部平仓
		if err != nil {
			return err
		}
		log.Printf("✅ 紧急平空仓成功，订单ID: %v", order["orderId"])
	default:
		return fmt.Errorf("未知的持仓方向: %s", side)
	}

	return nil
}

// GetPeakPnLCache 获取最高收益缓存
func (at *AutoTrader) GetPeakPnLCache() map[string]float64 {
	at.peakPnLCacheMutex.RLock()
	defer at.peakPnLCacheMutex.RUnlock()

	// 返回缓存的副本
	cache := make(map[string]float64)
	for k, v := range at.peakPnLCache {
		cache[k] = v
	}
	return cache
}

// UpdatePeakPnL 更新最高收益缓存
func (at *AutoTrader) UpdatePeakPnL(symbol, side string, currentPnLPct float64) {
	at.peakPnLCacheMutex.Lock()
	defer at.peakPnLCacheMutex.Unlock()

	posKey := symbol + "_" + side
	if peak, exists := at.peakPnLCache[posKey]; exists {
		// 更新峰值（如果是多头，取较大值；如果是空头，currentPnLPct为负，也要比较）
		if currentPnLPct > peak {
			at.peakPnLCache[posKey] = currentPnLPct
		}
	} else {
		// 首次记录
		at.peakPnLCache[posKey] = currentPnLPct
	}
}

// ClearPeakPnLCache 清除指定持仓的峰值缓存
func (at *AutoTrader) ClearPeakPnLCache(symbol, side string) {
	at.peakPnLCacheMutex.Lock()
	defer at.peakPnLCacheMutex.Unlock()

	posKey := symbol + "_" + side
	delete(at.peakPnLCache, posKey)
}

// RunSignalMode 运行信号跟随模式 (全局共享策略)
func (at *AutoTrader) RunSignalMode() error {
	log.Println("✅ 信号模式已启动，正在等待全局策略...")

	// ⚡️ 智能监听模式：使用配置的扫描频率
	interval := at.config.ScanInterval
	if interval <= 0 {
		interval = 1 * time.Minute
	}
	log.Printf("⏳ 信号模式扫描频率: %v", interval)

	// ⚡️ 启动时立即执行一次分析，不再等待第一个 ticker
	go func() {
		log.Println("⚡️ 正在进行启动后的首次策略分析...")
		// 如果全局管理器未初始化或未启动，等待一小会儿
		for i := 0; i < 10; i++ {
			if signal.GlobalManager != nil {
				break
			}
			time.Sleep(1 * time.Second)
		}

		if signal.GlobalManager != nil {
			strategies := signal.GlobalManager.ListActiveStrategies()
			for _, snap := range strategies {
				if snap != nil && snap.Strategy != nil {
					at.CheckAndExecuteStrategyWithAI(snap.Strategy, snap.PrevStrategy)
				}
			}
		}
	}()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// 🏥 健康检查定期任务 (默认30分钟)
	healthCheckInterval := 30 * time.Minute
	healthTicker := time.NewTicker(healthCheckInterval)
	defer healthTicker.Stop()

	for at.isRunning {
		select {
		case <-ticker.C:
			// ... (原有策略扫描逻辑) ...
			if at.database != nil {
				if db, ok := at.database.(sysconfig.DatabaseInterface); ok {
					traderRecord, err := db.GetTraderByID(at.id)
					if err == nil && traderRecord != nil {
						at.mu.Lock()
						at.customPrompt = traderRecord.CustomPrompt
						at.config.InitialBalance = traderRecord.InitialBalance
						at.initialBalance = traderRecord.InitialBalance
						at.mu.Unlock()
					}
				}
			}

			if signal.GlobalManager == nil {
				continue
			}

			strategies := signal.GlobalManager.ListActiveStrategies()
			for _, snap := range strategies {
				if snap != nil && snap.Strategy != nil {
					at.CheckAndExecuteStrategyWithAI(snap.Strategy, snap.PrevStrategy)
				}
			}

		case <-healthTicker.C:
			log.Println("🔍 [定期自检] 正在执行 30 分钟策略健康审计...")
			at.RunPeriodicHealthAudit()

		case <-at.stopMonitorCh:
			log.Println("⏹ 退出信号模式")
			return nil
		}
	}
	return nil
}

// CheckAndExecuteStrategy 检查当前状态并执行策略
func (at *AutoTrader) CheckAndExecuteStrategy(strat *signal.SignalDecision) {
	// 1. 获取行情
	marketData, err := market.Get(strat.Symbol)
	if err != nil {
		log.Printf("❌ 获取行情失败: %v", err)
		return
	}

	// 2. 获取持仓
	var currentQty float64 = 0
	var currentSide string = "NONE"

	positions, err := at.trader.GetPositions()
	if err == nil {
		for _, pos := range positions {
			if pos["symbol"] == strat.Symbol {
				amt := pos["positionAmt"].(float64)
				if amt != 0 {
					currentQty = math.Abs(amt)
					side := pos["side"].(string)
					currentSide = strings.ToUpper(side)
				}
				break
			}
		}
	}

	targetSide := strings.ToUpper(strat.Direction)

	// 3. 执行逻辑

	// A. 如果持有反向仓位 -> 平仓
	if currentSide != "NONE" && currentSide != targetSide {
		log.Printf("🔄 [信号执行] 发现反向持仓 (%s)，正在平仓...", currentSide)

		var closeErr error
		if currentSide == "LONG" {
			_, closeErr = at.trader.CloseLong(strat.Symbol, 0)
		} else {
			_, closeErr = at.trader.CloseShort(strat.Symbol, 0)
		}

		// 【修复】记录反向平仓到数据库，避免"平仓但不记录"的问题
		if db, ok := at.database.(*sysconfig.Database); ok {
			action := "SIGNAL_REVERSE_CLOSE_LONG"
			if currentSide == "SHORT" {
				action = "SIGNAL_REVERSE_CLOSE_SHORT"
			}

			reason := fmt.Sprintf("信号模式：检测到反向持仓(%s)，策略要求方向(%s)，执行平仓",
				currentSide, targetSide)
			if closeErr != nil {
				reason += fmt.Sprintf(" [执行失败: %v]", closeErr)
			}

			history := &sysconfig.StrategyDecisionHistory{
				TraderID:         at.id,
				StrategyID:       strat.SignalID,
				DecisionTime:     time.Now(),
				Action:           action,
				Symbol:           strat.Symbol,
				CurrentPrice:     marketData.CurrentPrice,
				TargetPrice:      strat.Entry.PriceTarget,
				PositionSide:     currentSide,
				PositionQty:      currentQty,
				AmountPercent:    1.0, // 100% 平仓
				Reason:           reason,
				ExecutionSuccess: closeErr == nil,
				ExecutionError:   "",
			}
			if closeErr != nil {
				history.ExecutionError = closeErr.Error()
			}

			if err := db.SaveStrategyDecision(history); err != nil {
				log.Printf("⚠️ 保存反向平仓决策历史失败: %v", err)
			} else {
				log.Printf("📝 已记录反向平仓决策: %s | %s", action, strat.Symbol)
			}
		}
		return
	}

	// B. 计算期望仓位比例
	// 基础仓位 (底仓)
	expectedPercent := 0.2

	// 加上所有已触发的补仓点
	for _, add := range strat.Adds {
		triggered := false
		if targetSide == "LONG" && marketData.CurrentPrice <= add.Price {
			triggered = true
		}
		if targetSide == "SHORT" && marketData.CurrentPrice >= add.Price {
			triggered = true
		}

		if triggered {
			p := add.Percent
			if p == 0 {
				p = 0.1
			} // 默认补 10%
			expectedPercent += p
		}
	}

	// C. 检查是否需要开仓/补仓
	currentSizeUSD := currentQty * marketData.CurrentPrice
	// 避免除以0
	if at.initialBalance <= 0 {
		at.initialBalance = 1000
	} // 兜底
	currentPercent := currentSizeUSD / at.initialBalance

	// 如果当前仓位明显小于期望 (差距 > 5%)
	if currentPercent < (expectedPercent - 0.05) {
		diffPercent := expectedPercent - currentPercent
		action := "ADD"
		if currentSide == "NONE" {
			action = "ENTRY"
		}

		log.Printf("🤖 [策略执行] 目标仓位 %.0f%% | 当前 %.0f%% | 动作: %s (+%.0f%%)",
			expectedPercent*100, currentPercent*100, action, diffPercent*100)

		at.executeSignalTrade(strat, action, diffPercent, marketData.CurrentPrice)
	}
}

// executeSignalTrade 执行信号交易
func (at *AutoTrader) executeSignalTrade(strat *signal.SignalDecision, actionType string, percent float64, currentPrice float64) {
	if percent <= 0 {
		return
	}

	// 计算下单金额
	sizeUSD := at.initialBalance * percent
	quantity := sizeUSD / currentPrice
	leverage := strat.LeverageRecommend
	if leverage == 0 {
		leverage = 5
	}

	// 确定方向
	isShort := strings.ToUpper(strat.Direction) == "SHORT"

	log.Printf("🚀 执行 %s: %s 数量: %.4f 杠杆: %d", actionType, strat.Symbol, quantity, leverage)

	var err error
	if isShort {
		_, err = at.trader.OpenShort(strat.Symbol, quantity, leverage)
	} else {
		_, err = at.trader.OpenLong(strat.Symbol, quantity, leverage)
	}

	if err != nil {
		log.Printf("❌ 下单失败: %v", err)
		return
	}

	// 设置止盈止损
	slPrice := strat.StopLoss.Price
	if len(strat.TakeProfits) > 0 {
		tpPrice := strat.TakeProfits[0].Price
		side := "LONG"
		if isShort {
			side = "SHORT"
		}

		// 重新获取总持仓以设置总SL/TP
		positions, _ := at.trader.GetPositions()
		totalQty := quantity
		for _, p := range positions {
			if p["symbol"] == strat.Symbol {
				totalQty = math.Abs(p["positionAmt"].(float64))
				break
			}
		}

		if slPrice > 0 {
			at.trader.SetStopLoss(strat.Symbol, side, totalQty, slPrice)
		}
		if tpPrice > 0 {
			at.trader.SetTakeProfit(strat.Symbol, side, totalQty, tpPrice)
		}
	}
}

// AIExecutionResult AI 执行结果结构
type AIExecutionResult struct {
	Action        string  `json:"action"`
	AmountPercent float64 `json:"amount_percent"`
	Reason        string  `json:"reason"`
}

// convertDecisionToExecution 将通用 Decision 结构转换为单币种执行结果
// 【功能】把老的 Decision JSON 结构适配为当前执行模块使用的结果格式
func convertDecisionToExecution(decisions []decision.Decision, symbol string, initialBalance float64) AIExecutionResult {
	// 默认结果：安全等待
	result := AIExecutionResult{
		Action:        "WAIT",
		AmountPercent: 0,
		Reason:        "AI 未返回有效决策，进入安全等待",
	}

	if len(decisions) == 0 {
		return result
	}

	// 优先匹配当前交易对，其次 ALL 或第一个
	var chosen *decision.Decision
	for i := range decisions {
		d := &decisions[i]
		if strings.EqualFold(d.Symbol, symbol) {
			chosen = d
			break
		}
	}
	if chosen == nil {
		for i := range decisions {
			d := &decisions[i]
			if strings.EqualFold(d.Symbol, "ALL") || d.Symbol == "" {
				chosen = d
				break
			}
		}
	}
	if chosen == nil {
		chosen = &decisions[0]
	}

	actionLower := strings.ToLower(chosen.Action)
	switch actionLower {
	case "open_long":
		result.Action = "OPEN_LONG"
	case "open_short":
		result.Action = "OPEN_SHORT"
	case "add_long":
		result.Action = "ADD_LONG"
	case "add_short":
		result.Action = "ADD_SHORT"
	case "close_long":
		result.Action = "CLOSE_LONG"
	case "close_short":
		result.Action = "CLOSE_SHORT"
	case "update_stop_loss":
		result.Action = "UPDATE_STOP_LOSS"
	case "update_take_profit":
		result.Action = "UPDATE_TAKE_PROFIT"
	case "partial_close":
		result.Action = "PARTIAL_CLOSE"
	case "set_tp_order":
		result.Action = "SET_TP_ORDER"
	case "set_sl_order":
		result.Action = "SET_SL_ORDER"
	case "hold", "wait", "":
		result.Action = "WAIT"
	default:
		// 未知动作保留原始名称（大写），不再降级为 WAIT
		result.Action = strings.ToUpper(chosen.Action)
	}

	// 计算资金占比：使用 position_size_usd / initialBalance
	if chosen.PositionSizeUSD > 0 && initialBalance > 0 {
		amt := math.Min(chosen.PositionSizeUSD, initialBalance)
		pct := amt / initialBalance
		if pct > 1 {
			pct = 1
		}
		if pct < 0 {
			pct = 0
		}
		result.AmountPercent = pct
	}

	if chosen.Reasoning != "" {
		result.Reason = chosen.Reasoning
	} else {
		result.Reason = "基于策略与当前市场状态的综合判断"
	}

	return result
}

// ExecuteSingleStrategyWithAI 为单个策略执行 AI 辅助决策
// 使用专用的 strategy_executor.txt 提示词模板，严格遵循策略原文
func (at *AutoTrader) ExecuteSingleStrategyWithAI(strat *signal.SignalDecision) error {
	log.Printf("🤖 [AI执行] 开始处理策略 %s (%s)...", strat.SignalID, strat.Symbol)

	// 1. 读取专用提示词模板
	promptPath := "prompts/strategy_executor.txt"
	promptContent, err := os.ReadFile(promptPath)
	if err != nil {
		return fmt.Errorf("读取提示词模板失败: %v", err)
	}
	promptTemplate := string(promptContent)

	// 0. 自动清理重复订单 (System Self-Correction)
	// 在获取数据前，先尝试清理明显的重复挂单
	at.cleanupDuplicateOrders(strat.Symbol)

	// 2. 构建交易上下文 (Context)
	// BuildContext通常会获取市场数据填充到 MarketDataMap
	ctx, err := at.buildTradingContext(nil)
	if err != nil {
		return fmt.Errorf("构建交易上下文失败: %v", err)
	}

	// 3. 获取特定于该策略的数据
	// 3.1 市场数据
	var currentPrice float64
	var rsi1h, rsi4h, macd4h string = "null", "null", "null"

	// 尝试从 Context 的 MarketDataMap 获取数据
	if ctx.MarketDataMap != nil {
		if data, ok := ctx.MarketDataMap[strat.Symbol]; ok {
			currentPrice = data.CurrentPrice
			// TODO: 如果 market.Data 有指标字段，也可以在这里获取
		}
	}

	// 如果 Context 里没拿到，尝试直接从 Trader 获取Ticker (兜底)
	if currentPrice == 0 {
		if price, err := at.trader.GetMarketPrice(strat.Symbol); err == nil {
			currentPrice = price
		}
	}

	// 3.2 持仓数据
	var posSide string = "NONE"
	var posSize float64 = 0
	var avgPrice float64 = 0
	var unrealizedPnL float64 = 0

	for _, p := range ctx.Positions {
		if p.Symbol == strat.Symbol {
			posSide = p.Side
			posSize = p.Quantity    // Corresponds to decision.PositionInfo.Quantity
			avgPrice = p.EntryPrice // Corresponds to decision.PositionInfo.EntryPrice
			unrealizedPnL = p.UnrealizedPnL
			break
		}
	}

	// 3.3 委托数据 (当前所有未成交挂单)
	// 需要从交易所获取最新的 Open Orders
	openOrders, err := at.trader.GetOpenOrders(strat.Symbol)
	var currentOrdersJSON string = "[]"
	if err == nil {
		ordersBytes, _ := json.Marshal(openOrders)
		currentOrdersJSON = string(ordersBytes)
	} else {
		log.Printf("⚠️ 获取 %s 挂单失败: %v", strat.Symbol, err)
	}

	// 3.3.1 阶段识别：根据持仓与当前委托，决定本次 AI 可以做哪些事
	hasPosition := posSide != "NONE" && posSize > 0
	hasLimitEntryOrders := false
	hasPlanOrders := false

	for _, o := range openOrders {
		oType, _ := o["type"].(string)
		orderCategory, _ := o["order_category"].(string)

		lt := strings.ToLower(oType)
		// 视为入场/补仓限价单：普通 limit 且非计划单
		if lt == "limit" && (orderCategory == "" || orderCategory == "normal") {
			hasLimitEntryOrders = true
		}

		// 视为止盈/止损计划单
		if lt == "take_profit" || lt == "stop_loss" || orderCategory == "plan" {
			hasPlanOrders = true
		}
	}

	// 阶段：
	// ENTRY_PLACEMENT = 只负责挂入场/补仓单
	// SLTP_PLACEMENT  = 已有持仓，只负责挂止盈止损
	// DONE            = 已有持仓且已挂止盈止损，本策略不再需要 AI 干预
	stage := "ENTRY_PLACEMENT"
	if hasPosition {
		if hasPlanOrders {
			stage = "DONE"
		} else {
			stage = "SLTP_PLACEMENT"
		}
	}

	// 如果已经有入场/补仓挂单但尚未持仓：只等成交，不再重复调用 AI（防止重复挂单）
	if !hasPosition && hasLimitEntryOrders {
		log.Printf("⏭️ [AI执行] 策略 %s (%s) 已存在入场/补仓挂单，跳过本次 AI 决策以防重复挂单", strat.SignalID, strat.Symbol)
		return nil
	}

	// 如果已经有持仓且已设置止盈/止损：认为该策略已完全交给交易所托管，不再调用 AI
	if hasPosition && hasPlanOrders {
		log.Printf("⏭️ [AI执行] 策略 %s (%s) 已有持仓且已设置止盈/止损，跳过本次 AI 决策", strat.SignalID, strat.Symbol)
		return nil
	}

	// 3.4 资金分配计算
	activeStratCount := len(ctx.ActiveStrategies)
	if activeStratCount == 0 {
		activeStratCount = 1
	} // 避免除零，至少为1
	maxAllocation := ctx.Account.TotalEquity / float64(activeStratCount)

	// 3.5 活跃策略列表 (用于 Prompt 展示)
	// ctx.ActiveStrategies 是 []*signal.StrategySnapshot
	// 我们需要提取其中的 Symbol 和 PriceTarget 信息
	type SimpleStrat struct {
		Symbol string  `json:"symbol"`
		Price  float64 `json:"price"` // 入场价
		Dir    string  `json:"dir"`
	}
	var simpleActiveStrats []SimpleStrat
	for _, s := range ctx.ActiveStrategies {
		if s != nil && s.Strategy != nil {
			simpleActiveStrats = append(simpleActiveStrats, SimpleStrat{
				Symbol: s.Strategy.Symbol,
				Price:  s.Strategy.Entry.PriceTarget,
				Dir:    s.Strategy.Direction,
			})
		}
	}

	// 3.6 根据阶段推导执行进度状态（用于提示词）
	executionStatus := "WAITING"
	switch stage {
	case "ENTRY_PLACEMENT":
		if hasPosition {
			executionStatus = "ENTRY"
		} else if hasLimitEntryOrders {
			executionStatus = "ENTRY_PENDING"
		} else {
			executionStatus = "WAITING"
		}
	case "SLTP_PLACEMENT":
		executionStatus = "ENTRY"
	case "DONE":
		executionStatus = "CLOSED"
	default:
		executionStatus = "WAITING"
	}
	// 目前暂不细分已执行补仓次数，先统一为 0
	executedAddCount := "0"

	// 4. 替换模板变量
	replacer := strings.NewReplacer(
		"{{STRATEGY_DIRECTION}}", strat.Direction,
		"{{SYMBOL}}", strat.Symbol,
		"{{ENTRY_PRICE}}", fmt.Sprintf("%.4f - %.4f", strat.Entry.RangeLow, strat.Entry.RangeHigh),
		"{{ADDS_JSON}}", toJSON(strat.Adds),
		"{{STOP_LOSS}}", toJSON(strat.StopLoss),
		"{{TAKE_PROFITS}}", toJSON(strat.TakeProfits),
		"{{RAW_STRATEGY_TEXT}}", strat.RawContent,
		"{{PREV_STRATEGY_TEXT}}", "无", // 暂不支持旧策略对比
		"{{INITIAL_BALANCE}}", fmt.Sprintf("%.2f", ctx.Account.InitialBalance),
		"{{TOTAL_EQUITY}}", fmt.Sprintf("%.2f", ctx.Account.TotalEquity),
		"{{AVAILABLE_BALANCE}}", fmt.Sprintf("%.2f", ctx.Account.AvailableBalance),
		"{{PERFORMANCE_INFO}}", "暂无",
		"{{ACTIVE_STRATEGY_COUNT}}", fmt.Sprintf("%d", activeStratCount),
		"{{MAX_ALLOCATION_PER_STRATEGY}}", fmt.Sprintf("%.2f", maxAllocation),
		"{{ACTIVE_STRATEGIES}}", toJSON(simpleActiveStrats),
		"{{CURRENT_PRICE}}", fmt.Sprintf("%.4f", currentPrice),
		"{{RSI_1H}}", rsi1h,
		"{{RSI_4H}}", rsi4h,
		"{{MACD_4H}}", macd4h,
		"{{CURRENT_POSITION_SIDE}}", posSide,
		"{{CURRENT_POSITION_SIZE}}", fmt.Sprintf("%.4f", posSize),
		"{{AVG_PRICE}}", fmt.Sprintf("%.4f", avgPrice),
		"{{UNREALIZED_PNL}}", fmt.Sprintf("%.2f", unrealizedPnL),
		"{{EXECUTION_STATUS}}", executionStatus,
		"{{EXECUTED_ADD_COUNT}}", executedAddCount,
		"{{CURRENT_ORDERS_JSON}}", currentOrdersJSON,
		"{{CUSTOM_PROMPT}}", at.customPrompt, // 如果有额外指令
	)

	finalPrompt := replacer.Replace(promptTemplate)

	// DEBUG: 打印最终发送给 AI 的 Prompt，方便排查变量替换问题
	log.Printf("🔍 [DEBUG] 最终提示词内容:\n%s\n--------------------------------------------------", finalPrompt)

	// 5. 调用 AI
	// 使用 GetFullDecisionWithCustomPrompt 但传入我们要的 finalPrompt 作为 systemPrompt (或 userPrompt)
	// 这里我们利用 customPrompt 参数传递完整的 prompt，systemPromptTemplate 设为 "raw" 或简单透传

	log.Printf("📤 发送提示词 (长度 %d) 给 AI...", len(finalPrompt))

	// 我们可以复用 decision.Engine 的能力，但需要绕过默认模板
	// 简单起见，我们把 finalPrompt 作为 `customPrompt` 传入，并设置 `overrideBase` 为 true
	// 这样 decision engine 会优先使用它
	decisionResult, err := decision.GetFullDecisionWithCustomPrompt(ctx, at.mcpClient, finalPrompt, true, "raw")

	if err != nil {
		return fmt.Errorf("AI 决策请求失败: %v", err)
	}

	// 6. 执行决策 & 保存记录
	if decisionResult != nil && len(decisionResult.Decisions) > 0 {
		log.Printf("📥 [AI执行] 收到 %d 条指令", len(decisionResult.Decisions))

		for _, d := range decisionResult.Decisions {
			// 强制覆盖 Symbol (防止 AI 生成错误的币种)
			d.Symbol = strat.Symbol

			// 6.1 在 ENTRY 阶段，将 open_long/open_short 语义映射为 place_long/short_order（限价入场）
			action := d.Action
			if stage == "ENTRY_PLACEMENT" {
				switch action {
				case "open_long":
					action = "place_long_order"
				case "open_short":
					action = "place_short_order"
				}
			}
			d.Action = action

			// 6.2 根据阶段过滤允许的动作，防止 AI 做越权操作
			allowed := false
			switch stage {
			case "ENTRY_PLACEMENT":
				// 仅允许挂入场/补仓委托 + 撤单/等待
				switch d.Action {
				case "place_long_order", "place_short_order", "cancel_order", "wait", "hold":
					allowed = true
				}
			case "SLTP_PLACEMENT":
				// 仅允许设置 / 更新 止盈止损 + 撤单/等待
				switch d.Action {
				case "set_tp_order", "set_sl_order", "update_stop_loss", "update_take_profit", "cancel_order", "wait", "hold":
					allowed = true
				}
			default:
				// DONE 或未知阶段：仅允许 wait/hold，其他全部忽略
				if d.Action == "wait" || d.Action == "hold" {
					allowed = true
				}
			}

			if !allowed {
				log.Printf("  ⚠️ [AI执行] 当前阶段(%s)不允许执行操作 %s，已忽略 (symbol=%s)", stage, d.Action, d.Symbol)
				continue
			}

			record := &logger.DecisionAction{
				Symbol:    d.Symbol,
				Action:    d.Action,
				Reasoning: d.Reasoning,
			}

			log.Printf("  👉 执行: %s %s (数量: %.4f, 价格: %.2f)", d.Action, d.Symbol, d.PositionSizeUSD, d.Price)

			if err := at.executeDecisionWithRecord(&d, record); err != nil {
				log.Printf("  ❌ 执行失败: %v", err)
			} else {
				log.Printf("  ✅ 执行成功")
			}

			// 6.3 保存到数据库
			// 对于信号模式，我们将完整 Prompt 放在 input_prompt，raw_ai_response 直接保存模型原始输出
			at.saveDecisionToDB(strat.SignalID, &d, record, "strategy_executor", finalPrompt, decisionResult.RawAIResponse)
		}
	} else {
		log.Println("💤 [AI执行] AI 决定暂不操作 (Wait)")
	}

	return nil
}

// 辅助函数：转JSON
func toJSON(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// CheckAndExecuteStrategyWithAI 使用AI进行策略执行判断 (旧接口适配)
func (at *AutoTrader) CheckAndExecuteStrategyWithAI(strat *signal.SignalDecision, prev *signal.SignalDecision) {
	// 调用新的专用执行函数
	if err := at.ExecuteSingleStrategyWithAI(strat); err != nil {
		log.Printf("❌ 策略 %s AI执行出错: %v", strat.SignalID, err)
	}
}

// =========================================================================================
// 新增：一次性委托下单模式相关方法
// =========================================================================================

// isStrategyOrdersPlaced 检查策略是否已经下过委托单
// 返回 true 如果数据库中存在该策略的任何订单记录（无论状态）
// 这防止策略被重复下单
func (at *AutoTrader) isStrategyOrdersPlaced(strategyID string) bool {
	if at.database == nil {
		return false
	}
	db, ok := at.database.(sysconfig.DatabaseInterface)
	if !ok {
		return false
	}

	orders, err := db.GetStrategyOrders(at.id, strategyID)
	if err != nil {
		log.Printf("⚠️ 查询策略订单失败: %v", err)
		return false
	}

	// 关键修复：只要有任何订单记录就认为已下过单
	// 不管状态是 new、filled 还是 cancelled
	// 这防止 syncOrderStatus 标记订单后重复下单
	if len(orders) > 0 {
		log.Printf("✓ 策略 %s 已有 %d 个订单记录，跳过重复下单", strategyID[:16], len(orders))
		return true
	}
	return false
}

// markStrategyOrdersPlaced (不需要单独标记，通过CreateStrategyOrder记录即可)

// PlaceStrategyOrders 根据策略下所有入场点位的限价委托单
func (at *AutoTrader) PlaceStrategyOrders(strat *signal.SignalDecision) error {
	// 防止同一策略重复下单：如果数据库里已经有该策略的订单记录，直接跳过
	if at.isStrategyOrdersPlaced(strat.SignalID) {
		log.Printf("✓ 策略 %s 已存在订单记录，跳过重复一次性委托", strat.SignalID)
		return nil
	}

	log.Printf("🚀 [策略委托] 开始为策略 %s 下单...", strat.SignalID)

	// 0. 关键修复：预先清理该币种的所有旧挂单
	// 这防止了之前失败或部分成功的策略尝试占用了保证金，导致"余额不足"错误
	log.Printf("🧹 正在清理 %s 的旧挂单以释放资金...", strat.Symbol)
	if err := at.trader.CancelAllOrders(strat.Symbol); err != nil {
		log.Printf("  ⚠️ 清理挂单警告 (可忽略): %v", err)
	} else {
		// 如果成功取消了订单，等待一秒让交易所后端处理资金释放
		time.Sleep(1 * time.Second)
	}

	// 1. 获取基本信息
	side := "buy"
	if strings.ToUpper(strat.Direction) == "SHORT" {
		side = "sell"
	}

	leverage := strat.LeverageRecommend
	// 根据币种选择正确的杠杆配置
	symbol := strings.ToUpper(strat.Symbol)
	isMajorCoin := strings.Contains(symbol, "BTC") || strings.Contains(symbol, "ETH")

	var configLeverage int
	if isMajorCoin {
		configLeverage = at.config.BTCETHLeverage
	} else {
		configLeverage = at.config.AltcoinLeverage
	}

	// 优先使用交易员配置的杠杆（用户在面板设置的），否则使用信号推荐，最后兜底
	if configLeverage > 0 && configLeverage != 5 {
		// 用户明确配置了非默认杠杆，优先使用
		leverage = configLeverage
		log.Printf("📊 使用交易员配置杠杆: %dx (覆盖信号推荐 %dx)", leverage, strat.LeverageRecommend)
	} else if leverage == 0 {
		leverage = configLeverage // 使用默认杠杆
	}

	// 2. 收集所有入场点位 (Entry + Adds)
	type OrderPoint struct {
		Price   float64
		Percent float64
		Type    string // "entry", "add_1", "add_2"...
	}

	var points []OrderPoint
	// 主入场点 (默认 40%)
	entryPrice := strat.Entry.PriceTarget
	points = append(points, OrderPoint{Price: entryPrice, Percent: 0.4, Type: "entry"})

	// 补仓点 (如果有2个补仓，各30%；如果有1个，60%；如果没有，主入场100%)
	totalAddPercent := 0.6
	if len(strat.Adds) == 0 {
		points[0].Percent = 1.0
	} else {
		perAddPercent := totalAddPercent / float64(len(strat.Adds))
		for i, add := range strat.Adds {
			points = append(points, OrderPoint{
				Price:   add.Price,
				Percent: perAddPercent,
				Type:    fmt.Sprintf("add_%d", i+1),
			})
		}
	}

	// 3. 计算总投入金额 (基于配置的初始余额，但受限于实际可用余额)
	totalInvestmentUSD := at.initialBalance
	if totalInvestmentUSD <= 0 {
		totalInvestmentUSD = 1000 // 兜底
	}

	// 【新增】资金均衡分配：总资金 / 活跃策略数
	// 防止某一个策略"抢光"所有资金，导致其他策略无法下单
	if sm := signal.GlobalManager; sm != nil {
		activeStrategies := sm.GetActiveStrategies()
		if len(activeStrategies) > 1 {
			maxAllocationPerStrategy := at.initialBalance / float64(len(activeStrategies))
			if totalInvestmentUSD > maxAllocationPerStrategy {
				log.Printf("💡 资金均衡分配: 检测到 %d 个活跃策略，每个最多分配 %.2f USDT (原: %.2f)",
					len(activeStrategies), maxAllocationPerStrategy, totalInvestmentUSD)
				totalInvestmentUSD = maxAllocationPerStrategy
			}
		}
	}

	// 获取实际可用余额进行双重校验
	if balance, err := at.trader.GetBalance(); err == nil {
		var availableBalance float64
		if avail, ok := balance["availableBalance"].(float64); ok {
			availableBalance = avail
		} else if avail, ok := balance["available_balance"].(float64); ok {
			availableBalance = avail
		}

		// 如果获取到了有效余额
		if availableBalance > 0 {
			// ⚠️ 极其保守的资金管理：只使用 80% 的可用余额
			// 防止因"逐仓/全仓"模式计算差异、未结盈亏或其他隐性占用导致下单失败 (Code 40762)
			safeLimit := availableBalance * 0.80

			if totalInvestmentUSD > safeLimit {
				log.Printf("⚠️ 配置的投入金额 %.2f USD 超过账户安全限额 %.2f USD (余额 %.2f * 80%%)，自动下调",
					totalInvestmentUSD, safeLimit, availableBalance)
				totalInvestmentUSD = safeLimit
			}
		}
	}

	log.Printf("💰 总投入金额: %.2f USDT, 杠杆: %dx", totalInvestmentUSD, leverage)

	// 4. 逐个下委托单
	db, hasDB := at.database.(sysconfig.DatabaseInterface)

	for _, point := range points {
		// 计算该点位的下单数量 (名义价值 / 价格)
		// 注意：Bitget的 quantity 是 币的数量
		amountUSD := totalInvestmentUSD * point.Percent
		quantity := (amountUSD * float64(leverage)) / point.Price

		log.Printf("📝 委托点位 [%s]: 价格 %.4f, 占比 %.0f%%, 金额 %.2f",
			point.Type, point.Price, point.Percent*100, amountUSD)

		// 执行限价下单
		res, err := at.PlaceLimitOrder(strat.Symbol, side, "open", quantity, point.Price, leverage)
		if err != nil {
			log.Printf("❌ 下单失败 [%s]: %v", point.Type, err)

			// 【新增】记录失败到数据库，供前端显示
			errMsg := fmt.Sprintf("下单失败 [%s]: %v", point.Type, err)
			// 特殊处理余额不足
			if strings.Contains(err.Error(), "40762") || strings.Contains(err.Error(), "余额") {
				errMsg = "❌ 余额不足 (Code 40762)"
			}
			at.logOrderExecutionToDB(strat, "ENTRY_ORDER_FAIL", false, errMsg)

			continue // 继续尝试下一个点位
		}

		// 记录到数据库
		if hasDB {
			orderId := fmt.Sprintf("%v", res["orderId"]) // 确保是字符串
			strategyOrder := &sysconfig.StrategyOrder{
				TraderID:   at.id,
				StrategyID: strat.SignalID,
				Symbol:     strat.Symbol,
				OrderID:    orderId,
				OrderType:  point.Type,
				Side:       side,
				Price:      point.Price,
				Quantity:   quantity,
				Leverage:   leverage,
				Status:     "new",
			}
			if err := db.CreateStrategyOrder(strategyOrder); err != nil {
				log.Printf("⚠️ 记录订单失败: %v", err)
			}
			log.Printf("✅ 委托成功: %s (ID: %s)", point.Type, orderId)

			// 【新增】记录成功到数据库
			at.logOrderExecutionToDB(strat, "ENTRY_ORDER_SUCCESS", true, fmt.Sprintf("委托成功 %s: %.4f", point.Type, point.Price))
		}
	}

	// 5. 设置止盈止损 (使用 Plan Order)
	// 【修改】不再立即设置止盈止损，改为在 syncOrderStatus 中检测到持仓后设置
	// 原因：限价单未成交时，直接设置止盈止损会导致 "仓位不足" 或 "找不到持仓" 错误
	// 逻辑移至 syncOrderStatus

	return nil
}

// logOrderExecutionToDB 记录订单执行结果到数据库（用于前端显示）
func (at *AutoTrader) logOrderExecutionToDB(strat *signal.SignalDecision, action string, success bool, errInfo string) {
	if at.database == nil {
		return
	}
	db, ok := at.database.(sysconfig.DatabaseInterface)
	if !ok {
		return
	}

	// 构造一个模拟的 Decision 记录
	// 注意：这里我们插入一条新的记录，用于前端Timeline显示
	// Action 建议使用 "EXECUTION_REPORT" 或具体动作

	// 替代方案：使用 LogExecutionEvent
	if err := db.LogExecutionEvent(at.id, strat.SignalID, action, strat.Symbol, "订单执行反馈", success, errInfo); err != nil {
		log.Printf("⚠️ 记录执行日志失败: %v", err)
	}
}

// syncOrderStatus 同步策略订单状态，确保交易所委托单与数据库一致
// 返回 needsRecovery: 如果关键订单丢失且无持仓，返回 true
func (at *AutoTrader) syncOrderStatus(strat *signal.SignalDecision) bool {
	db, ok := at.database.(sysconfig.DatabaseInterface)
	if !ok {
		return false
	}

	// 1. 从数据库获取该策略的所有委托单
	dbOrders, err := db.GetStrategyOrders(at.id, strat.SignalID)
	if err != nil || len(dbOrders) == 0 {
		return false
	}

	// 2. 从交易所获取当前挂单快照
	openOrders, err := at.trader.GetOpenOrders(strat.Symbol)
	if err != nil {
		log.Printf("⚠️ 同步订单失败 (API错误): %v", err)
		return false
	}

	// 3. 交叉比对
	openOrderIDs := make(map[string]bool)
	for _, oo := range openOrders {
		if id, ok := oo["orderId"].(string); ok {
			openOrderIDs[id] = true
		}
	}

	missingOrders := 0
	for _, dbo := range dbOrders {
		if dbo.Status == "new" || dbo.Status == "partially_filled" {
			if !openOrderIDs[dbo.OrderID] {
				// 订单在交易所找不到了，可能已成交或被手动取消
				log.Printf("🔍 订单 %s (%s) 在交易所不可见", dbo.OrderID, dbo.OrderType)

				// 检查持仓情况 (需要最新的持仓信息)
				hasPosition := false
				positions, _ := at.trader.GetPositions() // 这里忽略错误，尽量尝试获取
				for _, pos := range positions {
					if pos["symbol"] == strat.Symbol && pos["positionAmt"].(float64) != 0 {
						hasPosition = true
						break
					}
				}

				if dbo.OrderType == "entry" {
					if hasPosition {
						// 有持仓，说明 Entry 单肯定成交了
						log.Printf("✅ 检测到持仓，标记 Entry 订单 %s 为 filled", dbo.OrderID)
						db.UpdateStrategyOrderStatus(dbo.ID, "filled")
					} else {
						// 无持仓，说明 Entry 单被取消了 (或者过期)
						log.Printf("❌ 无持仓，标记 Entry 订单 %s 为 cancelled", dbo.OrderID)
						db.UpdateStrategyOrderStatus(dbo.ID, "cancelled")
					}
				} else {
					// 对于 add (补仓) 单或其他类型
					// 如果在交易所找不到了，不管有没有持仓，都认为是Cancelled (因为如果成交了会有持仓变化，但很难精确对应)
					// 更稳妥的做法是：如果找不到了，就当做 cancelled，让 AI 在下一轮决定是否要重新挂单
					log.Printf("🗑️ 补仓/其他订单 %s 丢失，标记为 cancelled", dbo.OrderID)
					db.UpdateStrategyOrderStatus(dbo.ID, "cancelled")
				}
				missingOrders++
			}
		}
	}

	if missingOrders > 0 {
		log.Printf("📊 策略 %s 有 %d 个委托单已不在挂单列表", strat.SignalID, missingOrders)
	}

	// 3.5 关键检查：如果主要挂单丢失且无持仓，标记需要恢复
	needsRecovery := false
	if missingOrders > 0 {
		// 检查是否还有持仓
		hasPosition := false
		positions, err := at.trader.GetPositions()
		if err == nil {
			for _, pos := range positions {
				if pos["symbol"] == strat.Symbol {
					amt := pos["positionAmt"].(float64)
					if amt != 0 {
						hasPosition = true
					}
					break
				}
			}
		}

		if !hasPosition {
			// 如果没有持仓，且有订单丢失，说明 limit order 被取消了或者过期了，需要 AI 补单
			needsRecovery = true
		}
	}

	// 4. 【新增】检查持仓并补充止盈止损
	// 只有当有持仓时，才去设置 TP/SL
	positions, err := at.trader.GetPositions()
	if err == nil {
		var currentPos map[string]interface{}
		for _, pos := range positions {
			if pos["symbol"] == strat.Symbol {
				amt := pos["positionAmt"].(float64)
				if amt != 0 {
					currentPos = pos
				}
				break
			}
		}

		if currentPos != nil {
			// 有持仓，检查是否有 TP/SL 计划单
			hasTPSL := false
			for _, oo := range openOrders {
				// 获取 planType (需要 GetOpenOrders 返回 planType)
				if pt, ok := oo["plan_type"].(string); ok {
					if pt == "loss_plan" || pt == "profit_plan" || pt == "pos_loss" || pt == "pos_profit" {
						hasTPSL = true
						break
					}
				}
				// 兼容旧的 type 字段判断
				if t, ok := oo["type"].(string); ok {
					if t == "stop_loss" || t == "take_profit" {
						hasTPSL = true
						break
					}
				}
			}

			if !hasTPSL {
				log.Printf("🛡️ 检测到策略 %s 有持仓但未设置止盈止损，正在补充...", strat.Symbol)

				amt := math.Abs(currentPos["positionAmt"].(float64))
				side := currentPos["side"].(string) // "long" or "short"

				// 转换 side 格式
				posSide := "LONG"
				if strings.ToLower(side) == "short" {
					posSide = "SHORT"
				}

				// 设置止损
				if strat.StopLoss.Price > 0 {
					if err := at.trader.SetStopLoss(strat.Symbol, posSide, amt, strat.StopLoss.Price); err != nil {
						log.Printf("⚠️ 补充止损失败: %v", err)
					} else {
						log.Printf("✅ 补充止损成功: %.4f", strat.StopLoss.Price)
					}
				}

				// 设置止盈 (取第一个)
				if len(strat.TakeProfits) > 0 {
					tpPrice := strat.TakeProfits[0].Price
					if err := at.trader.SetTakeProfit(strat.Symbol, posSide, amt, tpPrice); err != nil {
						log.Printf("⚠️ 补充止盈失败: %v", err)
					} else {
						log.Printf("✅ 补充止盈成功: %.4f", tpPrice)
					}
				}
			}
		}
	}
	return needsRecovery
}

// RunPeriodicHealthAudit 执行全球自检，处理僵尸策略和订单丢失
func (at *AutoTrader) RunPeriodicHealthAudit() {
	if signal.GlobalManager == nil {
		return
	}

	strategies := signal.GlobalManager.ListActiveStrategies()
	if len(strategies) == 0 {
		return
	}

	// 1. 代码层：基础同步
	for _, snap := range strategies {
		if snap == nil || snap.Strategy == nil {
			continue
		}
		log.Printf("🏥 [代码自检] 正在确认委托状态: %s (%s)", snap.Strategy.Symbol, snap.Strategy.SignalID)
		at.syncOrderStatus(snap.Strategy)
	}

	// 2. AI 层：智能审计 (每小时整点或 30 分时执行)
	min := time.Now().Minute()
	if min >= 0 && min <= 5 || min >= 30 && min <= 35 {
		log.Println("🧠 [AI审计] 启动全局智能审计周期...")
		if err := at.RunSmartAuditCycle(); err != nil {
			log.Printf("❌ AI 审计失败: %v", err)
		}
	}
}

// RunSmartAuditCycle 运行一个完整的 AI 审计周期，审查当前订单与持仓是否符合全局策略
func (at *AutoTrader) RunSmartAuditCycle() error {
	// 1. 收集完整上下文 (包含账户、持仓、活跃策略)
	ctx, err := at.buildTradingContext(nil)
	if err != nil {
		return fmt.Errorf("构建审计上下文失败: %w", err)
	}

	// 2. 设置特殊的审计提示词
	at.mu.RLock()
	originalPrompt := at.customPrompt
	template := at.systemPromptTemplate
	override := at.overrideBasePrompt
	at.mu.RUnlock()

	auditPrompt := "【重点指令：定期自检与审计】\n" +
		"当前正在执行定期系统审计。你现在的主要任务不是寻找新机会开仓，而是【检查已有委托与持仓】。\n" +
		"请审阅 CURRENT_ORDERS_JSON 中的委托是否与 ACTIVE_STRATEGIES (活跃策略) 的目标价格、补仓计划、止盈止损一致。\n" +
		"1. 如果发现主要委托单（如入场或补仓挂单）丢失，请使用 place_xxx_order 补齐。\n" +
		"2. 如果发现持仓已变动但止盈止损未更新，请使用 update_xxx_xxx 或 set_xxx_order。 \n" +
		"3. 如果发现某个策略已到达止损或清仓点但系统未动作，请执行平仓命令。\n" +
		"4. **禁止**由于害怕错过而进行计划外的市价开仓。"

	combinedPrompt := auditPrompt
	if originalPrompt != "" {
		combinedPrompt = originalPrompt + "\n\n" + auditPrompt
	}

	// 3. 调用 AI 决策
	log.Printf("🤖 正在请求 AI 审计 %d 个活跃策略...", len(ctx.ActiveStrategies))
	fullDecision, err := decision.GetFullDecisionWithCustomPrompt(ctx, at.mcpClient, combinedPrompt, override, template)
	if err != nil {
		return err
	}

	// 4. 执行 AI 审计决策
	if fullDecision != nil && len(fullDecision.Decisions) > 0 {
		log.Printf("📥 AI 返回了 %d 条审计决策", len(fullDecision.Decisions))

		// 获取数据库接口用于保存决策
		db, hasDB := at.database.(sysconfig.DatabaseInterface)

		for _, d := range fullDecision.Decisions {
			record := &logger.DecisionAction{
				Symbol:    d.Symbol,
				Action:    d.Action,
				Reasoning: d.Reasoning,
			}

			execSuccess := true
			execError := ""

			if err := at.executeDecisionWithRecord(&d, record); err != nil {
				log.Printf("  ⚠️ 执行审计决策失败 (%s %s): %v", d.Symbol, d.Action, err)
				execSuccess = false
				execError = err.Error()
			} else {
				log.Printf("  ✅ 执行审计决策成功: %s %s", d.Symbol, d.Action)
			}

			// 【关键】保存决策到数据库，供前端显示
			if hasDB {
				if err := db.LogExecutionEvent(at.id, "", d.Action, d.Symbol, d.Reasoning, execSuccess, execError); err != nil {
					log.Printf("  ⚠️ 保存决策记录失败: %v", err)
				}
			}
		}
	} else {
		log.Println("💤 AI 审计完成：一切正常，无建议变动")
	}

	return nil
}

// updateStrategyStatus 更新策略执行状态到数据库
func (at *AutoTrader) updateStrategyStatus(stratID, symbol, status string, entryPrice, quantity, realizedPnL float64) {
	if at.database == nil {
		return
	}

	if db, ok := at.database.(*sysconfig.Database); ok {
		s := &sysconfig.TraderStrategyStatus{
			TraderID:    at.id,
			StrategyID:  stratID,
			Symbol:      symbol,
			Status:      status,
			EntryPrice:  entryPrice,
			Quantity:    quantity,
			RealizedPnL: realizedPnL,
		}
		if err := db.UpdateTraderStrategyStatus(s); err != nil {
			log.Printf("⚠️ 更新策略状态失败: %v", err)
		}
	}
}

// statusToStep 将状态字符串转换为执行步骤编号
func statusToStep(status string) int {
	switch status {
	case "ENTRY":
		return 1
	case "ADD_1":
		return 2
	case "ADD_2":
		return 3
	default:
		return 0
	}
}

// stepToStatus 将执行步骤编号转换为状态字符串
func stepToStatus(step int) string {
	switch step {
	case 1:
		return "ENTRY"
	case 2:
		return "ADD_1"
	case 3:
		return "ADD_2"
	default:
		return "WAITING"
	}
}

// cleanupDuplicateOrders 清理重复委托单
// 逻辑：获取当前所有委托，按 (symbol, type, price, side) 分组
// 如果同一组内有多个订单，保留最新的一个，取消其余的
func (at *AutoTrader) cleanupDuplicateOrders(symbol string) {
	if symbol == "" {
		return
	}

	orders, err := at.trader.GetOpenOrders(symbol)
	if err != nil {
		log.Printf("⚠️ [清理重复] 获取委托失败: %v", err)
		return
	}

	if len(orders) < 2 {
		return
	}

	// 分组键: type_side_price (例如: limit_buy_3000.5)
	// 对于止盈止损，使用 triggerPrice
	type OrderKey struct {
		Type  string
		Side  string
		Price string
	}

	groups := make(map[OrderKey][]string) // key -> [orderID]

	for _, order := range orders {
		oType, _ := order["type"].(string)
		oSide, _ := order["side"].(string) // holdSide or side

		// 统一 Limit 和 Plan 的价格获取方式
		var price float64
		if p, ok := order["price"].(float64); ok && p > 0 {
			price = p
		} else if tp, ok := order["triggerPrice"].(float64); ok && tp > 0 {
			price = tp
		}

		// 格式化价格，保留2位小数作为指纹，忽略微小差异
		priceKey := fmt.Sprintf("%.2f", price)

		// 针对止盈止损单，类型可能会有多种变体 (profit_plan, take_profit等)，统一归类
		normType := oType // 默认为 limit 或其他

		// PlanType 检查
		if pType, ok := order["planType"].(string); ok && pType != "" {
			if strings.Contains(strings.ToLower(pType), "profit") {
				normType = "take_profit"
			} else if strings.Contains(strings.ToLower(pType), "loss") {
				normType = "stop_loss"
			}
		} else if strings.Contains(strings.ToLower(oType), "limit") {
			normType = "limit"
		}

		key := OrderKey{
			Type:  normType,
			Side:  oSide,
			Price: priceKey,
		}

		orderID, _ := order["order_id"].(string)
		if orderID != "" {
			groups[key] = append(groups[key], orderID)
		}
	}

	// 检查每组，如果超过1个，则取消多余的
	for key, ids := range groups {
		if len(ids) > 1 {
			log.Printf("🧹 [清理重复] 发现 %d 个重复订单 (Type:%s Price:%s Side:%s)，保留最新的", len(ids), key.Type, key.Price, key.Side)

			// 保留最后一个 (假设通常是最新的，虽然不一定，但随机保留一个也可)
			for i := 0; i < len(ids)-1; i++ {
				toCancel := ids[i]

				// 区分撤单逻辑
				if key.Type == "limit" {
					log.Printf("  🗑️ 自动撤销重复限价单: %s", toCancel)
					at.trader.CancelOrder(symbol, toCancel)
				} else {
					// 尝试调用 BitgetTrader 的 CancelPlanOrder 方法
					if bt, ok := at.trader.(*BitgetTrader); ok {
						// 需要找到该订单的 original planType
						var planType string
						for _, o := range orders {
							if oid, _ := o["order_id"].(string); oid == toCancel {
								if pt, ok := o["plan_type"].(string); ok {
									planType = pt
								}
								break
							}
						}

						if planType != "" {
							log.Printf("  🗑️ 自动撤销重复计划单: %s (Type: %s)", toCancel, planType)
							if err := bt.CancelPlanOrder(symbol, toCancel, planType); err != nil {
								log.Printf("  ❌ 撤销失败: %v", err)
							}
						} else {
							log.Printf("  ⚠️ 无法确定计划单类型，跳过撤销: %s", toCancel)
						}
					} else {
						log.Printf("  ⚠️ 当前 Trader 不支持按ID撤销计划单: %s", toCancel)
					}
				}
			}
		}
	}
}

// saveDecisionToDB 将单个 AI 决策执行结果保存到数据库历史记录
func (at *AutoTrader) saveDecisionToDB(strategyID string, d *decision.Decision, result *logger.DecisionAction, systemPrompt, inputPrompt, rawResponse string) {
	if at.database == nil {
		return
	}

	db, ok := at.database.(*sysconfig.Database)
	if !ok {
		return
	}

	// 如果没有传入策略ID，尝试获取该币种当前关联的策略ID
	if strategyID == "" {
		at.mu.RLock()
		if id, ok := at.appliedStopStrategy[d.Symbol]; ok {
			strategyID = id
		}
		at.mu.RUnlock()
	}

	// 如果仍然没有关联策略ID，且是开仓动作，生成一个临时的唯一ID（标记为AI独立决策）
	if strategyID == "" && (strings.Contains(d.Action, "open")) {
		strategyID = fmt.Sprintf("ai_periodic_%s_%d", d.Symbol, time.Now().Unix())
	}

	history := &sysconfig.StrategyDecisionHistory{
		TraderID:         at.id,
		StrategyID:       strategyID,
		DecisionTime:     time.Now(),
		Action:           d.Action,
		Symbol:           d.Symbol,
		CurrentPrice:     result.Price,
		TargetPrice:      0,  // 周期性决策通常没有固定目标价，使用当前价或0
		PositionSide:     "", // 将在 SaveStrategyDecision 内部逻辑或后续更新中完善
		PositionQty:      result.Quantity,
		AmountPercent:    0,
		Reason:           d.Reasoning,
		SystemPrompt:     systemPrompt,
		InputPrompt:      inputPrompt,
		RawAIResponse:    rawResponse,
		ExecutionSuccess: result.Success,
		ExecutionError:   result.Error,
	}

	if err := db.SaveStrategyDecision(history); err != nil {
		log.Printf("⚠️ 保存周期性决策历史失败: %v", err)
	}
}

// saveStrategyDecisionHistory 保存策略决策历史
func (at *AutoTrader) saveStrategyDecisionHistory(strat *signal.SignalDecision, result *AIExecutionResult, currentPrice, rsi1h, rsi4h, macd4h float64, positionSide string, positionQty float64, systemPrompt, inputPrompt, rawResponse string) {
	if at.database == nil {
		return
	}

	db, ok := at.database.(*sysconfig.Database)
	if !ok {
		return
	}

	history := &sysconfig.StrategyDecisionHistory{
		TraderID:         at.id,
		StrategyID:       strat.SignalID,
		DecisionTime:     time.Now(),
		Action:           result.Action,
		Symbol:           strat.Symbol,
		CurrentPrice:     currentPrice,
		TargetPrice:      strat.Entry.PriceTarget,
		PositionSide:     positionSide,
		PositionQty:      positionQty,
		AmountPercent:    result.AmountPercent,
		Reason:           result.Reason,
		RSI1H:            rsi1h,
		RSI4H:            rsi4h,
		MACD4H:           macd4h,
		SystemPrompt:     systemPrompt,
		InputPrompt:      inputPrompt,
		RawAIResponse:    rawResponse,
		ExecutionSuccess: false, // 默认false,执行后会更新
		ExecutionError:   "",
	}

	if err := db.SaveStrategyDecision(history); err != nil {
		log.Printf("⚠️ 保存决策历史失败: %v", err)
	} else {
		log.Printf("📝 已保存决策历史: %s | %s | ID: %d", result.Action, strat.Symbol, history.ID)
	}
}
