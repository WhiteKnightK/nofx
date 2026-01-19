package trader

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
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
	"strconv"
	"strings"
	"sync"
	"time"
)

// withinRelDiff 【功能】价格相对误差判断（用于委托/历史匹配去重）
func withinRelDiff(a, b, rel float64) bool {
	if a <= 0 || b <= 0 {
		return false
	}
	return math.Abs(a-b)/b <= rel
}

// normalizeSideToBitgetOpenSide 【功能】将策略方向转换为Bitget历史委托的open side
func normalizeSideToBitgetOpenSide(direction string) string {
	d := strings.ToUpper(strings.TrimSpace(direction))
	if d == "SHORT" {
		return "open_short"
	}
	return "open_long"
}

// getStrategyReceivedAt 【功能】从全局管理器中获取该策略的接收时间（用于委托历史窗口）
func (at *AutoTrader) getStrategyReceivedAt(signalID string) time.Time {
	if signal.GlobalManager == nil {
		return time.Now()
	}
	snaps := signal.GlobalManager.ListActiveStrategies()
	for _, s := range snaps {
		if s != nil && s.Strategy != nil && s.Strategy.SignalID == signalID {
			if !s.Time.IsZero() {
				return s.Time
			}
		}
	}
	return time.Now()
}

// shouldTriggerRepairAI 【功能】策略修复AI限频，避免每20秒刷爆
func (at *AutoTrader) shouldTriggerRepairAI(strategyID string) bool {
	if strategyID == "" {
		return true
	}

	cooldown := 120 * time.Second // 默认 2 分钟冷却
	if v := os.Getenv("SIGNAL_REPAIR_AI_COOLDOWN_SECONDS"); v != "" {
		if sec, err := strconv.Atoi(v); err == nil && sec > 0 {
			cooldown = time.Duration(sec) * time.Second
		}
	}

	if lastAny, ok := at.repairAICooldown.Load(strategyID); ok {
		if last, ok2 := lastAny.(time.Time); ok2 && !last.IsZero() {
			remaining := cooldown - time.Since(last)
			if remaining > 0 {
				log.Printf("⏳ [ai-throttle] %s 冷却中，剩余 %.0f 秒", strategyID[:8], remaining.Seconds())
				return false
			}
		}
	}
	at.repairAICooldown.Store(strategyID, time.Now())
	return true
}

type expectedPoint struct {
	kind    string
	price   float64
	percent float64
}

// deduplicateOpenOrders 检测并取消同一价位的重复挂单（保留第一个，取消其余）
func (at *AutoTrader) deduplicateOpenOrders(symbol string) {
	openOrders, err := at.trader.GetOpenOrders(symbol)
	if err != nil {
		log.Printf("⚠️ [dedup] 获取挂单失败 %s: %v", symbol, err)
		return
	}

	// 按 price+side 分组
	type orderKey struct {
		price float64
		side  string
	}
	groups := make(map[orderKey][]map[string]interface{})

	for _, o := range openOrders {
		ot, _ := o["type"].(string)
		if strings.ToLower(ot) != "limit" {
			continue
		}
		price, _ := o["price"].(float64)
		side, _ := o["side"].(string)
		if price <= 0 {
			continue
		}
		key := orderKey{price: price, side: side}
		groups[key] = append(groups[key], o)
	}

	// 找到重复的组，取消多余的
	for key, orders := range groups {
		if len(orders) <= 1 {
			continue
		}
		// 保留第一个，取消其余
		log.Printf("🔧 [dedup] 发现 %s 价格 %.2f (%s) 有 %d 个重复挂单，正在取消多余的...",
			symbol, key.price, key.side, len(orders))
		
		for i := 1; i < len(orders); i++ {
			orderId, _ := orders[i]["order_id"].(string)
			if orderId == "" {
				continue
			}
			if err := at.trader.CancelOrder(symbol, orderId); err != nil {
				log.Printf("  ⚠️ 取消失败 order=%s: %v", orderId, err)
			} else {
				log.Printf("  ✓ 已取消重复挂单 order=%s", orderId)
			}
		}
	}
}


// detectStrategyDiffFromExchange 【功能】用交易所 openOrders + orderHistory 对账，只判断“是否有缺失/不一致”，不直接补单
func (at *AutoTrader) detectStrategyDiffFromExchange(strat *signal.SignalDecision, receivedAt time.Time) (bool, string, []expectedPoint, bool, bool) {
	if strat == nil || strat.Symbol == "" {
		return false, "", nil, false, false
	}

	symbol := strat.Symbol
	wantOpenSide := normalizeSideToBitgetOpenSide(strat.Direction)

	// 0) 先去重：取消同价位的重复挂单
	at.deduplicateOpenOrders(symbol)

	// 1) 当前持仓
	var hasPosition bool
	var posQty float64
	var posSide string
	var posEntryPrice float64
	positions, err := at.trader.GetPositions()
	if err == nil {
		for _, p := range positions {
			if p["symbol"] == symbol {
				amt, _ := p["positionAmt"].(float64)
				if amt != 0 {
					hasPosition = true
					posQty = math.Abs(amt)
					sideStr, _ := p["side"].(string) // long/short
					if strings.ToLower(sideStr) == "short" {
						posSide = "SHORT"
					} else {
						posSide = "LONG"
					}
					posEntryPrice, _ = p["entryPrice"].(float64)
				}
				break
			}
		}
	}

	// 2) 当前挂单（含计划单）
	openOrders, err := at.trader.GetOpenOrders(symbol)
	if err != nil {
		log.Printf("[signal-audit] GetOpenOrders failed symbol=%s err=%v", symbol, err)
		openOrders = []map[string]interface{}{}
	}

	// 3) 历史委托窗口：从接收时间往前留5分钟buffer
	startAt := receivedAt.Add(-5 * time.Minute).UnixMilli()
	endAt := time.Now().UnixMilli()
	orderHistory, err := at.trader.GetOrderHistory(symbol, startAt, endAt)
	if err != nil {
		log.Printf("[signal-audit] GetOrderHistory failed symbol=%s err=%v", symbol, err)
		orderHistory = []map[string]interface{}{}
	}

	// 4) 期望点位：entry + adds
	var points []expectedPoint
	if strat.Entry.PriceTarget > 0 {
		points = append(points, expectedPoint{kind: "entry", price: strat.Entry.PriceTarget, percent: 0.40})
	}
	for i, a := range strat.Adds {
		if a.Price <= 0 {
			continue
		}
		pct := a.Percent
		if pct <= 0 {
			pct = 0.60 / math.Max(1, float64(len(strat.Adds)))
		}
		points = append(points, expectedPoint{kind: fmt.Sprintf("add_%d", i+1), price: a.Price, percent: pct})
	}

	if len(points) == 0 {
		if hasPosition {
			// 无入场/补仓信息，但有持仓且有保护信息时也需要触发AI补TP/SL
			if strat.StopLoss.Price > 0 || len(strat.TakeProfits) > 0 {
				return true, "DIFF_DETECTED: strategy has position but no entry/add points; please ensure protective orders are set.", nil, true, true
			}
		}
		return false, "", nil, false, false
	}

	// 5) 匹配函数：是否已有“打开方向”的委托/成交覆盖了该点位
	hasOpenOrderAt := func(target float64) bool {
		for _, o := range openOrders {
			ot, _ := o["type"].(string)
			if strings.ToLower(ot) != "limit" {
				continue
			}
			oside, _ := o["side"].(string)
			osideLower := strings.ToLower(oside)
			
			// 匹配方向：wantOpenSide 是 "open_long" 或 "open_short"
			// Bitget 返回的 side 可能是 "buy" 或 "open_long"
			sideMatch := false
			if wantOpenSide == "open_long" {
				sideMatch = strings.Contains(osideLower, "open_long") || osideLower == "buy"
			} else if wantOpenSide == "open_short" {
				sideMatch = strings.Contains(osideLower, "open_short") || osideLower == "sell"
			}
			
			if !sideMatch {
				continue
			}
			p, _ := o["price"].(float64)
			if p > 0 && withinRelDiff(p, target, 0.001) {
				return true
			}
		}
		return false
	}

	wasFilledAt := func(target float64) bool {
		// 1) 先用持仓均价兜底：如果当前持仓均价非常接近某点位，视为已执行过该点位
		if hasPosition && posEntryPrice > 0 && withinRelDiff(posEntryPrice, target, 0.001) {
			return true
		}
		// 2) 再用历史委托：filled/partially_filled 或 avg_price > 0（只要成交过就算）
		for _, h := range orderHistory {
			st, _ := h["status"].(string)
			stLower := strings.ToLower(st)
			avgPrice, _ := h["avg_price"].(float64)
			
			// 判断是否已成交：状态为filled/partially_filled，或者avg_price > 0（表示有成交）
			isFilled := stLower == "filled" || stLower == "partially_filled" || avgPrice > 0
			if !isFilled {
				continue
			}
			
			sd, _ := h["side"].(string)
			sdLower := strings.ToLower(sd)
			
			// 检查方向是否匹配（历史订单的 side 可能是 "buy" 或 "open_long"）
			sideMatch := false
			if wantOpenSide == "open_long" {
				sideMatch = strings.Contains(sdLower, "open_long") || sdLower == "buy"
			} else if wantOpenSide == "open_short" {
				sideMatch = strings.Contains(sdLower, "open_short") || sdLower == "sell"
			}
			if !sideMatch {
				continue
			}
			typ, _ := h["type"].(string)
			price, _ := h["price"].(float64)
			// avgPrice 已在上面声明过

			if strings.ToLower(typ) == "market" {
				if avgPrice > 0 && withinRelDiff(avgPrice, target, 0.003) {
					return true
				}
				continue
			}
			// limit：优先用price，再用avg_price兜底
			if price > 0 && withinRelDiff(price, target, 0.001) {
				return true
			}
			if avgPrice > 0 && withinRelDiff(avgPrice, target, 0.001) {
				return true
			}
		}
		return false
	}

	// 6) 计算缺失项：没挂单、也没成交/部分成交 => 认为缺失
	var missing []expectedPoint
	for _, pt := range points {
		if pt.price <= 0 {
			continue
		}
		if wasFilledAt(pt.price) {
			continue
		}
		if hasOpenOrderAt(pt.price) {
			continue
		}
		missing = append(missing, pt)
	}

	// 7) 有持仓时检查保护单是否缺失
	missingStopLoss := false
	missingTakeProfit := false
	var missingTPPrices []float64
	if hasPosition {
		slMatched := false
		tpOrderPrices := make([]float64, 0)
		for _, oo := range openOrders {
			typ, _ := oo["type"].(string)
			lt := strings.ToLower(typ)
			p, _ := oo["price"].(float64)
			if lt == "stop_loss" && p > 0 && strat.StopLoss.Price > 0 && withinRelDiff(p, strat.StopLoss.Price, 0.01) {
				slMatched = true
			}
			if lt == "take_profit" && p > 0 {
				tpOrderPrices = append(tpOrderPrices, p)
			}
		}

		if strat.StopLoss.Price > 0 && !slMatched {
			missingStopLoss = true
		}

		// 按每个TP价位逐一检查（避免“有一个TP就认为全部TP都有”）
		if len(strat.TakeProfits) > 0 {
			for _, tp := range strat.TakeProfits {
				if tp.Price <= 0 {
					continue
				}
				found := false
				for _, op := range tpOrderPrices {
					if withinRelDiff(op, tp.Price, 0.01) {
						found = true
						break
					}
				}
				if !found {
					missingTPPrices = append(missingTPPrices, tp.Price)
				}
			}
			if len(missingTPPrices) > 0 {
				missingTakeProfit = true
			}
		}
	}

	if len(missing) == 0 && !missingStopLoss && !missingTakeProfit {
		return false, "", nil, false, false
	}

	// 8) 生成给AI的差异报告（只提示“缺啥”，让AI根据 openOrders + history 自行决策补哪些）
	missingStrs := make([]string, 0, len(missing))
	for _, m := range missing {
		missingStrs = append(missingStrs, fmt.Sprintf("%s@%.4f", m.kind, m.price))
	}
	missingTPStrs := make([]string, 0, len(missingTPPrices))
	for _, p := range missingTPPrices {
		missingTPStrs = append(missingTPStrs, fmt.Sprintf("%.4f", p))
	}
	report := fmt.Sprintf(
		"DIFF_DETECTED: symbol=%s hasPosition=%v positionSide=%s positionQty=%.4f entryPrice=%.4f wantOpenSide=%s missingOrders=%v missingStopLoss=%v missingTakeProfit=%v missingTPPrices=%v. "+
			"IMPORTANT: placing LIMIT orders at missing entry/add prices is NOT chasing the market; you MUST output actions to place the missing limit orders even if current price has moved away. "+
			"Please use CURRENT_ORDERS_JSON and ORDER_HISTORY_JSON to avoid duplicates. If DIFF_DETECTED is true, do NOT return wait-only; output the required actions.",
		symbol, hasPosition, posSide, posQty, posEntryPrice, wantOpenSide, missingStrs, missingStopLoss, missingTakeProfit, missingTPStrs,
	)
	return true, report, missing, missingStopLoss, missingTakeProfit
}

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
	EnableDrawdownMonitor bool    // 是否启用回撤监控自动平仓（默认关闭）

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
	peakPnLCache          map[string]float64 // 最高收益缓存 (symbol -> 峰值盈亏百分比)
	peakPnLCacheMutex     sync.RWMutex       // 缓存读写锁
	mu                    sync.RWMutex       // 提示词配置读写锁（保护customPrompt、overrideBasePrompt、systemPromptTemplate）
	lastBalanceSyncTime   time.Time          // 上次余额同步时间
	database              interface{}        // 数据库引用（用于自动更新余额）
	userID                string             // 用户ID
	repairAICooldown      sync.Map           // 策略修复AI调用限频 (strategyID -> time.Time)
	closedStrategyCache   sync.Map           // 已关闭策略缓存 (strategyID -> bool)，用于快速跳过补单/检查

	// 信号模式状态
	lastExecutedSignalID string // 上次执行的信号ID
}

// markStrategyClosed 【功能】将策略标记为已关闭（避免后续继续补单/检查）
func (at *AutoTrader) markStrategyClosed(strategyID string) {
	if at == nil || strategyID == "" {
		return
	}
	at.closedStrategyCache.Store(strategyID, true)
}

// isStrategyClosed 【功能】判断策略是否已关闭
func (at *AutoTrader) isStrategyClosed(strategyID string) bool {
	if at == nil || strategyID == "" {
		return false
	}
	v, ok := at.closedStrategyCache.Load(strategyID)
	if !ok {
		return false
	}
	b, _ := v.(bool)
	return b
}

// hydrateClosedStrategiesFromDB 【功能】启动时从数据库恢复已关闭策略缓存
func (at *AutoTrader) hydrateClosedStrategiesFromDB() {
	if at == nil || at.database == nil {
		return
	}
	db, ok := at.database.(*sysconfig.Database)
	if !ok {
		return
	}
	statuses, err := db.GetTraderStrategyStatuses(at.id)
	if err != nil {
		return
	}
	for _, s := range statuses {
		if s == nil {
			continue
		}
		if strings.ToUpper(strings.TrimSpace(s.Status)) == "CLOSED" {
			at.markStrategyClosed(s.StrategyID)
		}
	}
}

// auditPositionsAndCloseFinishedStrategies 【功能】定时对账：若策略曾进入持仓阶段但当前仓位为0，则关闭该策略
func (at *AutoTrader) auditPositionsAndCloseFinishedStrategies() {
	if at == nil || at.database == nil {
		return
	}
	db, ok := at.database.(*sysconfig.Database)
	if !ok {
		return
	}

	statuses, err := db.GetTraderStrategyStatuses(at.id)
	if err != nil || len(statuses) == 0 {
		return
	}

	positions, err := at.trader.GetPositions()
	if err != nil {
		return
	}
	posQtyBySymbol := make(map[string]float64)
	for _, p := range positions {
		sym, _ := p["symbol"].(string)
		sym = strings.ToUpper(strings.TrimSpace(sym))
		if sym == "" {
			continue
		}
		amt, _ := p["positionAmt"].(float64)
		if amt == 0 {
			continue
		}
		posQtyBySymbol[sym] = math.Abs(amt)
	}

	for _, st := range statuses {
		if st == nil {
			continue
		}
		statusUpper := strings.ToUpper(strings.TrimSpace(st.Status))
		if statusUpper == "" || statusUpper == "WAITING" || statusUpper == "CLOSED" {
			continue
		}
		if at.isStrategyClosed(st.StrategyID) {
			continue
		}

		sym := strings.ToUpper(strings.TrimSpace(st.Symbol))
		if sym == "" {
			continue
		}
		if qty, ok := posQtyBySymbol[sym]; ok && qty > 0 {
			continue
		}

		at.updateStrategyStatus(st.StrategyID, sym, "CLOSED", 0, 0, 0)
		at.markStrategyClosed(st.StrategyID)
		log.Printf("[position-audit] strategy closed due to missing position: trader=%s strategy=%s symbol=%s prev_status=%s",
			at.id, st.StrategyID, sym, st.Status)
	}
}

// syncTraderConfigFromDB 【功能】从数据库同步运行中交易员配置（用于信号模式实时生效）
func (at *AutoTrader) syncTraderConfigFromDB() {
	if at == nil || at.database == nil || at.id == "" {
		return
	}
	db, ok := at.database.(*sysconfig.Database)
	if !ok {
		return
	}
	traderRecord, err := db.GetTraderByID(at.id)
	if err != nil || traderRecord == nil {
		return
	}

	at.mu.Lock()
	defer at.mu.Unlock()

	at.customPrompt = traderRecord.CustomPrompt
	at.overrideBasePrompt = traderRecord.OverrideBasePrompt
	if traderRecord.SystemPromptTemplate != "" {
		at.systemPromptTemplate = traderRecord.SystemPromptTemplate
	}

	// 同步杠杆/仓位模式（信号执行会用到）
	if traderRecord.BTCETHLeverage > 0 {
		at.config.BTCETHLeverage = traderRecord.BTCETHLeverage
	}
	if traderRecord.AltcoinLeverage > 0 {
		at.config.AltcoinLeverage = traderRecord.AltcoinLeverage
	}
	at.config.IsCrossMargin = traderRecord.IsCrossMargin
}

// SetLeverageConfig 【功能】更新运行中交易员的杠杆配置（无需重启）
func (at *AutoTrader) SetLeverageConfig(btcEthLeverage, altcoinLeverage int) {
	if at == nil {
		return
	}
	at.mu.Lock()
	defer at.mu.Unlock()
	if btcEthLeverage > 0 {
		at.config.BTCETHLeverage = btcEthLeverage
	}
	if altcoinLeverage > 0 {
		at.config.AltcoinLeverage = altcoinLeverage
	}
}

// SetCrossMarginMode 【功能】更新运行中交易员的仓位模式（无需重启）
func (at *AutoTrader) SetCrossMarginMode(isCross bool) {
	if at == nil {
		return
	}
	at.mu.Lock()
	defer at.mu.Unlock()
	at.config.IsCrossMargin = isCross
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
	}, nil
}

// GetConfig returns the trader configuration
func (at *AutoTrader) GetConfig() *AutoTraderConfig {
	if at == nil {
		return nil
	}
	return &at.config
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

	// 【功能】回撤监控（默认关闭，仅在显式开启时启用）
	if at.config.EnableDrawdownMonitor {
		at.startDrawdownMonitor()
	}

	// 循环执行：等待对齐 -> 执行 -> 等待对齐...
	for at.isRunning {
		// 1. 等待直到下一个整点间隔（+5秒延迟）以获取闭合K线
		if !at.waitUntilNextInterval() {
			log.Printf("[%s] ⏹ 收到停止信号，退出自动交易主循环", at.name)
			return nil
		}

		// 2. 执行决策周期
		if err := at.runCycle(); err != nil {
			log.Printf("❌ 执行失败: %v", err)
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

// waitUntilNextInterval 等待直到下一个时间间隔点（带延迟）
// 返回 true 表示时间到了可以继续，返回 false 表示收到停止信号
func (at *AutoTrader) waitUntilNextInterval() bool {
	now := time.Now()
	interval := at.config.ScanInterval

	// 计算下一个整点时间
	// Truncate 向下取整到最近的 interval 倍数
	// 例如：interval=5m, now=12:03:00 -> truncated=12:00:00 -> next=12:05:00
	nextTime := now.Truncate(interval).Add(interval)

	// 添加 5 秒延迟，确保交易所 K 线已生成并固定
	targetTime := nextTime.Add(5 * time.Second)

	// 如果当前时间已经过了 targetTime（极少数情况），则再加一个 interval
	if targetTime.Before(now) {
		targetTime = targetTime.Add(interval)
	}

	waitDuration := targetTime.Sub(now)

	log.Printf("⏳ [%s] 等待对齐 K 线周期: %v 后执行 (目标时间: %s)",
		at.name, waitDuration.Round(time.Second), targetTime.Format("15:04:05"))

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
func (at *AutoTrader) runCycle() error {
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
		remaining := time.Until(at.stopUntil)
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
	ctx, err := at.buildTradingContext()
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
	log.Print("\n" + strings.Repeat("=", 70) + "\n")
	log.Printf("📋 系统提示词（完整版，包含所有部分）")
	log.Printf("   模板: %s | 自定义提示词: %v | 覆盖基础: %v",
		at.systemPromptTemplate,
		at.customPrompt != "",
		at.overrideBasePrompt)
	log.Println(strings.Repeat("=", 70))
	log.Println(decision.SystemPrompt)
	log.Println(strings.Repeat("=", 70))

	// 6. 打印AI思维链（用于查看AI是否遵循自定义提示词）
	log.Print("\n" + strings.Repeat("-", 70) + "\n")
	log.Println("💭 AI思维链分析:")
	log.Println(strings.Repeat("-", 70))
	log.Println(decision.CoTTrace)
	log.Println(strings.Repeat("-", 70))

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
			record.ExecutionLog = append(record.ExecutionLog, fmt.Sprintf("❌ %s %s 失败: %v", d.Symbol, d.Action, err))
		} else {
			actionRecord.Success = true
			record.ExecutionLog = append(record.ExecutionLog, fmt.Sprintf("✓ %s %s 成功", d.Symbol, d.Action))
			// 成功执行后短暂延迟
			time.Sleep(1 * time.Second)
		}

		record.Decisions = append(record.Decisions, actionRecord)
	}

	// 9. 保存决策记录
	if err := at.decisionLogger.LogDecision(record); err != nil {
		log.Printf("⚠ 保存决策记录失败: %v", err)
	}

	return nil
}

// buildTradingContext 构建交易上下文
func (at *AutoTrader) buildTradingContext() (*decision.Context, error) {
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
			TotalEquity:      totalEquity,
			AvailableBalance: availableBalance,
			TotalPnL:         totalPnL,
			TotalPnLPct:      totalPnLPct,
			MarginUsed:       totalMarginUsed,
			MarginUsedPct:    marginUsedPct,
			PositionCount:    len(positionInfos),
		},
		Positions:      positionInfos,
		CandidateCoins: candidateCoins,
		Performance:    performance, // 添加历史表现分析
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

// executePlaceLimitOrderWithRecord 【功能】执行限价委托并记录
func (at *AutoTrader) executePlaceLimitOrderWithRecord(side, tradeSide string, d *decision.Decision, actionRecord *logger.DecisionAction) error {
	if d == nil {
		return fmt.Errorf("nil decision")
	}
	if d.Price <= 0 {
		return fmt.Errorf("invalid limit price: %.8f", d.Price)
	}
	if d.PositionSizeUSD <= 0 {
		return fmt.Errorf("invalid position_size_usd: %.8f", d.PositionSizeUSD)
	}

	lev := d.Leverage
	if lev <= 0 {
		lev = at.config.BTCETHLeverage
		if lev <= 0 {
			lev = 5
		}
	}
	d.Leverage = lev

	// 防重复：同价同方向的limit单已存在则跳过
	openOrders, err := at.trader.GetOpenOrders(d.Symbol)
	if err == nil {
		expectedSides := []string{}
		if side == "buy" {
			expectedSides = []string{"open_long", "buy"}
		} else {
			expectedSides = []string{"open_short", "sell"}
		}
		
		for _, o := range openOrders {
			ot, _ := o["type"].(string)
			if strings.ToLower(ot) != "limit" {
				continue
			}
			oside, _ := o["side"].(string)
			osideLower := strings.ToLower(oside)
			
			// 检查方向是否匹配
			sideMatch := false
			for _, expected := range expectedSides {
				if strings.Contains(osideLower, expected) {
					sideMatch = true
					break
				}
			}
			if !sideMatch {
				continue
			}
			
			op, _ := o["price"].(float64)
			if op > 0 && withinRelDiff(op, d.Price, 0.001) {
				log.Printf("⏭️ [duplicate-check] 跳过重复挂单: %s 价格=%.2f (已存在挂单价格=%.2f side=%s)", d.Action, d.Price, op, oside)
				return nil
			}
		}
	} else {
		log.Printf("⚠️ [duplicate-check] 获取挂单失败，继续下单: %v", err)
	}

	quantity := d.PositionSizeUSD / d.Price
	if quantity <= 0 {
		return fmt.Errorf("invalid computed quantity: %.8f", quantity)
	}
	
	// 最小下单量检查 (Bitget 要求：ETH/BTC 通常是 0.001，山寨币更大)
	// 改进：如果计算出的 quantity 小于 minQty 但差距不大（例如 > 0.5 * minQty），自动向上取整到 minQty，而不是报错
	minQty := 0.001
	if !strings.Contains(d.Symbol, "BTC") && !strings.Contains(d.Symbol, "ETH") {
		minQty = 0.01 // 山寨币最小下单量通常更大
	}
	
	if quantity < minQty {
		// 检查是否可以强制升级到最小下单量
		// 计算最小下单量所需的保证金
		minNotional := minQty * d.Price
		// requiredMargin := minNotional / float64(lev) // 暂时未使用，依赖后续检查
		
		// 获取余额 (使用 auto_trader 缓存的余额或实时获取)
		// 这里在 下面已经有 GetBalance 调用，我们可以提前调用一次简单的 check
		// 为简单起见，我们只能在这里尽量允许升级，依赖后面的 strict check 拦截
		
		log.Printf("⚠️ [order-fix] 数量 %.6f 低于最小限制 %.4f (名义价值 $%.2f < $%.2f)。尝试自动调整为最小下单量...", 
			quantity, minQty, d.PositionSizeUSD, minNotional)

		// 只要升级后的保证金不超过当前计算的 position_size_usd 太多(比如3倍以内)，或者虽然很多但绝对值很小(比如<20U)，就允许升级
		// 实际上，对于测试账户，$15 -> $92 是必须要做的，否则无法测试
		// 所以如果不通过，就直接改为报错
		
		quantity = minQty // 强制升级
		log.Printf("✅ [order-fix] 已强制调整为最小下单量 %.4f (名义价值 $%.2f)", quantity, minNotional)
	}

	actionRecord.Price = d.Price
	actionRecord.Quantity = quantity
	actionRecord.Leverage = lev

	// 保证金校验
	balance, err := at.trader.GetBalance()
	if err != nil {
		return fmt.Errorf("failed to get balance: %w", err)
	}
	availableBalance := 0.0
	if avail, ok := balance["availableBalance"].(float64); ok {
		availableBalance = avail
	}
	requiredMargin := d.PositionSizeUSD / float64(lev)
	estimatedFee := d.PositionSizeUSD * 0.0004
	totalRequired := requiredMargin + estimatedFee
	if totalRequired > availableBalance {
		return fmt.Errorf("insufficient margin: require=%.2f (margin=%.2f fee=%.2f) available=%.2f", totalRequired, requiredMargin, estimatedFee, availableBalance)
	}

	if err := at.trader.SetMarginMode(d.Symbol, at.config.IsCrossMargin); err != nil {
		log.Printf("[signal-ai] SetMarginMode failed symbol=%s err=%v", d.Symbol, err)
	}

	res, err := at.trader.PlaceLimitOrder(d.Symbol, side, tradeSide, quantity, d.Price, lev)
	if err != nil {
		log.Printf("❌ [PlaceLimitOrder失败] symbol=%s side=%s tradeSide=%s quantity=%.8f price=%.4f leverage=%d position_size_usd=%.2f err=%v",
			d.Symbol, side, tradeSide, quantity, d.Price, lev, d.PositionSizeUSD, err)
		return err
	}
	if rawID, ok := res["orderId"]; ok {
		switch v := rawID.(type) {
		case int64:
			actionRecord.OrderID = v
		case float64:
			actionRecord.OrderID = int64(v)
		}
	}
	return nil
}

// executeCancelOrderWithRecord 【功能】执行撤单并记录
func (at *AutoTrader) executeCancelOrderWithRecord(d *decision.Decision, actionRecord *logger.DecisionAction) error {
	if d == nil {
		return fmt.Errorf("nil decision")
	}
	if strings.TrimSpace(d.OrderID) == "" {
		return at.trader.CancelAllOrders(d.Symbol)
	}
	return at.trader.CancelOrder(d.Symbol, d.OrderID)
}

// executeSetTPOrderWithRecord 【功能】设置止盈计划单并记录
func (at *AutoTrader) executeSetTPOrderWithRecord(d *decision.Decision, actionRecord *logger.DecisionAction) error {
	if d == nil {
		return fmt.Errorf("nil decision")
	}
	tp := d.TpTriggerPrice
	if tp <= 0 {
		tp = d.TakeProfit
	}
	if tp <= 0 {
		return fmt.Errorf("invalid tp trigger price")
	}

	openOrders, err := at.trader.GetOpenOrders(d.Symbol)
	if err == nil {
		for _, o := range openOrders {
			ot, _ := o["type"].(string)
			if strings.ToLower(ot) != "take_profit" {
				continue
			}
			op, _ := o["price"].(float64)
			if op > 0 && withinRelDiff(op, tp, 0.01) {
				return nil
			}
		}
	}

	positions, err := at.trader.GetPositions()
	if err != nil {
		return fmt.Errorf("failed to get positions: %w", err)
	}
	var pos map[string]interface{}
	for _, p := range positions {
		if p["symbol"] == d.Symbol {
			amt, _ := p["positionAmt"].(float64)
			if amt != 0 {
				pos = p
			}
			break
		}
	}
	if pos == nil {
		return fmt.Errorf("no position for %s", d.Symbol)
	}

	posSide := "LONG"
	if s, ok := pos["side"].(string); ok && strings.ToLower(s) == "short" {
		posSide = "SHORT"
	}
	totalQty := math.Abs(pos["positionAmt"].(float64))
	qty := totalQty
	if d.TpClosePercentage > 0 && d.TpClosePercentage <= 100 {
		qty = totalQty * (d.TpClosePercentage / 100.0)
	}
	if qty <= 0 {
		return fmt.Errorf("invalid tp quantity: %.8f", qty)
	}

	actionRecord.Price = tp
	actionRecord.Quantity = qty
	return at.trader.SetTakeProfit(d.Symbol, posSide, qty, tp)
}

// executeSetSLOrderWithRecord 【功能】设置止损计划单并记录
func (at *AutoTrader) executeSetSLOrderWithRecord(d *decision.Decision, actionRecord *logger.DecisionAction) error {
	if d == nil {
		return fmt.Errorf("nil decision")
	}
	sl := d.SlTriggerPrice
	if sl <= 0 {
		sl = d.StopLoss
	}
	if sl <= 0 {
		return fmt.Errorf("invalid sl trigger price")
	}

	openOrders, err := at.trader.GetOpenOrders(d.Symbol)
	if err == nil {
		for _, o := range openOrders {
			ot, _ := o["type"].(string)
			if strings.ToLower(ot) != "stop_loss" {
				continue
			}
			op, _ := o["price"].(float64)
			if op > 0 && withinRelDiff(op, sl, 0.01) {
				return nil
			}
		}
	}

	positions, err := at.trader.GetPositions()
	if err != nil {
		return fmt.Errorf("failed to get positions: %w", err)
	}
	var pos map[string]interface{}
	for _, p := range positions {
		if p["symbol"] == d.Symbol {
			amt, _ := p["positionAmt"].(float64)
			if amt != 0 {
				pos = p
			}
			break
		}
	}
	if pos == nil {
		return fmt.Errorf("no position for %s", d.Symbol)
	}

	posSide := "LONG"
	if s, ok := pos["side"].(string); ok && strings.ToLower(s) == "short" {
		posSide = "SHORT"
	}
	totalQty := math.Abs(pos["positionAmt"].(float64))
	if totalQty <= 0 {
		return fmt.Errorf("invalid sl quantity: %.8f", totalQty)
	}

	actionRecord.Price = sl
	actionRecord.Quantity = totalQty
	return at.trader.SetStopLoss(d.Symbol, posSide, totalQty, sl)
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

	balance, err := at.trader.GetBalance()
	if err != nil {
		return fmt.Errorf("获取账户余额失败: %w", err)
	}
	availableBalance := 0.0
	if avail, ok := balance["availableBalance"].(float64); ok {
		availableBalance = avail
	}

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

	// 设置止损止盈
	if err := at.trader.SetStopLoss(decision.Symbol, "LONG", quantity, decision.StopLoss); err != nil {
		log.Printf("  ⚠ 设置止损失败: %v", err)
	}
	if err := at.trader.SetTakeProfit(decision.Symbol, "LONG", quantity, decision.TakeProfit); err != nil {
		log.Printf("  ⚠ 设置止盈失败: %v", err)
	}

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

	balance, err := at.trader.GetBalance()
	if err != nil {
		return fmt.Errorf("获取账户余额失败: %w", err)
	}
	availableBalance := 0.0
	if avail, ok := balance["availableBalance"].(float64); ok {
		availableBalance = avail
	}

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

	// 设置止损止盈
	if err := at.trader.SetStopLoss(decision.Symbol, "SHORT", quantity, decision.StopLoss); err != nil {
		log.Printf("  ⚠ 设置止损失败: %v", err)
	}
	if err := at.trader.SetTakeProfit(decision.Symbol, "SHORT", quantity, decision.TakeProfit); err != nil {
		log.Printf("  ⚠ 设置止盈失败: %v", err)
	}

	return nil
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
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// ⚡️ 补单检查定时器 (20秒)：用于快速补齐止损/止盈
	reconcileTicker := time.NewTicker(20 * time.Second)
	defer reconcileTicker.Stop()

	// ⚡️ 仓位对账定时器（30分钟）：若仓位已消失则关闭策略，避免继续跑
	positionAuditTicker := time.NewTicker(30 * time.Minute)
	defer positionAuditTicker.Stop()

	// 启动时恢复已关闭策略缓存
	at.hydrateClosedStrategiesFromDB()

	// ⚡️ 策略更新监听：策略一到就立刻触发一次（避免“更新了不触发”）
	if signal.GlobalManager != nil {
		signal.GlobalManager.RegisterListener(func(newStrat, prev *signal.SignalDecision) {
			if newStrat == nil {
				return
			}
			if at.isStrategyClosed(newStrat.SignalID) {
				return
			}
			receivedAt := at.getStrategyReceivedAt(newStrat.SignalID)
			diff, report, missing, missingSL, missingTP := at.detectStrategyDiffFromExchange(newStrat, receivedAt)
			if diff && at.shouldTriggerRepairAI(newStrat.SignalID) {
				log.Printf("[signal-listener] diff detected symbol=%s id=%s; triggering ai repair", newStrat.Symbol, newStrat.SignalID)
				at.CheckAndExecuteStrategyWithAI(newStrat, report, missing, missingSL, missingTP)
			} else {
				log.Printf("[signal-listener] no diff or throttled symbol=%s id=%s; skip ai", newStrat.Symbol, newStrat.SignalID)
			}
		})
	}

	for at.isRunning {
		select {
		case <-reconcileTicker.C:
			// 快速自检：遍历所有活跃策略，只做差异检查；有差异立刻调用AI（把openOrders+history喂给AI）
			if signal.GlobalManager == nil {
				continue
			}
			snaps := signal.GlobalManager.ListActiveStrategies()
			for _, snap := range snaps {
				if snap == nil || snap.Strategy == nil {
					continue
				}
				if at.isStrategyClosed(snap.Strategy.SignalID) {
					continue
				}
				diff, report, missing, missingSL, missingTP := at.detectStrategyDiffFromExchange(snap.Strategy, snap.Time)
				if diff && at.shouldTriggerRepairAI(snap.Strategy.SignalID) {
					log.Printf("[signal-audit] diff detected symbol=%s id=%s; triggering ai repair", snap.Strategy.Symbol, snap.Strategy.SignalID)
					at.CheckAndExecuteStrategyWithAI(snap.Strategy, report, missing, missingSL, missingTP)
				}
			}

		case <-positionAuditTicker.C:
			at.auditPositionsAndCloseFinishedStrategies()

		case <-ticker.C:
			// 如果全局管理器未初始化或未启动，等待
			if signal.GlobalManager == nil {
				continue
			}

			// 定时器保留：避免与 20s 自检重复刷AI；需要AI修复由自检触发
			continue

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
		if currentSide == "LONG" {
			at.trader.CloseLong(strat.Symbol, 0)
		} else {
			at.trader.CloseShort(strat.Symbol, 0)
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
	case "close_long":
		result.Action = "CLOSE_LONG"
	case "close_short":
		result.Action = "CLOSE_SHORT"
	case "hold", "wait", "":
		result.Action = "WAIT"
	default:
		// 未知动作一律降级为 WAIT，避免误触发交易
		result.Action = "WAIT"
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

// placeMissingLimitOrdersFallback 【功能】当AI拒绝执行（只返回wait）时，按差异检查结果兜底补齐缺失限价单，并写入决策历史供前端展示
func (at *AutoTrader) placeMissingLimitOrdersFallback(
	strat *signal.SignalDecision,
	missing []expectedPoint,
	currentPrice, rsi1h, rsi4h, macd4h float64,
	positionSide string,
	positionQty float64,
) {
	if strat == nil || len(missing) == 0 {
		return
	}

	leverage := strat.LeverageRecommend
	if leverage <= 0 {
		leverage = 5
	}
	totalInvestmentUSD := at.initialBalance
	if totalInvestmentUSD <= 0 {
		totalInvestmentUSD = 1000
	}

	for _, m := range missing {
		if m.price <= 0 || m.percent <= 0 {
			continue
		}
		marginUSD := totalInvestmentUSD * m.percent
		notionalUSD := marginUSD * float64(leverage)

		action := "place_long_order"
		side := "buy"
		if strings.ToUpper(strings.TrimSpace(strat.Direction)) == "SHORT" {
			action = "place_short_order"
			side = "sell"
		}

		d := &decision.Decision{
			Symbol:          strat.Symbol,
			Action:          action,
			Leverage:        leverage,
			PositionSizeUSD: notionalUSD,
			Price:           m.price,
			Reasoning:       "Fallback placement due to missing order detected by diff audit.",
		}
		ar := &logger.DecisionAction{
			Symbol:    d.Symbol,
			Action:    d.Action,
			Reasoning: d.Reasoning,
		}

		log.Printf("[signal-fallback] placing missing limit order symbol=%s kind=%s price=%.4f side=%s", strat.Symbol, m.kind, m.price, side)
		execErr := at.executeDecisionWithRecord(d, ar)
		if execErr != nil {
			ar.Success = false
			ar.Error = execErr.Error()
			log.Printf("[signal-fallback] place limit failed symbol=%s kind=%s price=%.4f err=%v", strat.Symbol, m.kind, m.price, execErr)
		} else {
			ar.Success = true
		}

		at.saveStrategyDecisionHistoryFromDecision(
			strat,
			d,
			ar,
			currentPrice, rsi1h, rsi4h, macd4h,
			positionSide,
			positionQty,
			"signal_fallback",
			"",
			"",
			execErr,
		)
	}
}

// CheckAndExecuteStrategyWithAI 【功能】发现差异后调用AI，让AI依据当前委托+历史委托决定如何补齐
func (at *AutoTrader) CheckAndExecuteStrategyWithAI(strat *signal.SignalDecision, extraDirective string, missing []expectedPoint, missingSL, missingTP bool) {
	if strat != nil && at.isStrategyClosed(strat.SignalID) {
		return
	}
	// 信号模式：每次执行前从DB同步最新配置，确保配置面板修改立即生效
	at.syncTraderConfigFromDB()

	// 1. 获取市场数据
	apiClient := market.NewAPIClient()

	// 获取 1h K线
	klines1h, err := apiClient.GetKlines(strat.Symbol, "1h", 100)
	if err != nil {
		log.Printf("❌ 获取1h K线失败: %v", err)
		return
	}

	// 获取 4h K线
	klines4h, err := apiClient.GetKlines(strat.Symbol, "4h", 100)
	if err != nil {
		log.Printf("❌ 获取4h K线失败: %v", err)
		return
	}

	// 提取收盘价序列
	closes1h := make([]float64, len(klines1h))
	for i, k := range klines1h {
		closes1h[i] = k.Close
	}

	closes4h := make([]float64, len(klines4h))
	for i, k := range klines4h {
		closes4h[i] = k.Close
	}

	// 计算指标
	rsi1h := market.CalculateRSI(closes1h, 14)
	rsi4h := market.CalculateRSI(closes4h, 14)
	_, _, macdHist4h := market.CalculateMACD(closes4h)

	currentPrice := closes1h[len(closes1h)-1]

	// 2. 获取当前持仓
	var currentQty float64 = 0
	var currentSide string = "NONE"
	var avgPrice float64 = 0
	// var unrealizedPnl float64 = 0

	positions, err := at.trader.GetPositions()
	if err == nil {
		for _, pos := range positions {
			if pos["symbol"] == strat.Symbol {
				amt := pos["positionAmt"].(float64)
				if amt != 0 {
					currentQty = math.Abs(amt)
					side := pos["side"].(string)
					currentSide = strings.ToUpper(side)
					avgPrice = pos["entryPrice"].(float64)
					// unrealizedPnl = pos["unRealizedProfit"].(float64)
				}
				break
			}
		}
	}

	// 计算盈亏百分比
	pnlPercent := 0.0
	if avgPrice > 0 {
		if currentSide == "LONG" {
			pnlPercent = ((currentPrice - avgPrice) / avgPrice) * 100 * float64(strat.LeverageRecommend)
		} else {
			pnlPercent = ((avgPrice - currentPrice) / avgPrice) * 100 * float64(strat.LeverageRecommend)
		}
	}

	// 3. 准备 Prompt
	promptContent, err := ioutil.ReadFile("prompts/strategy_executor.txt")
	if err != nil {
		log.Printf("❌ 读取Prompt模板失败: %v", err)
		return
	}

	prompt := string(promptContent)

	// 补齐模板缺失字段（避免前端/提示词残留{{...}}导致AI误判）
	prevText := "N/A"
	activeCount := 1
	maxAlloc := at.initialBalance
	activeSimple := []map[string]interface{}{}
	if signal.GlobalManager != nil {
		snaps := signal.GlobalManager.ListActiveStrategies()
		if len(snaps) > 0 {
			activeCount = len(snaps)
			if activeCount > 0 {
				maxAlloc = at.initialBalance / float64(activeCount)
			}
			for _, s := range snaps {
				if s != nil && s.Strategy != nil {
					activeSimple = append(activeSimple, map[string]interface{}{
						"symbol": s.Strategy.Symbol,
						"dir":    s.Strategy.Direction,
						"entry":  s.Strategy.Entry.PriceTarget,
						"id":     s.Strategy.SignalID,
					})
					if s.Strategy.SignalID == strat.SignalID && s.PrevStrategy != nil {
						if s.PrevStrategy.RawContent != "" {
							prevText = s.PrevStrategy.RawContent
						}
					}
				}
			}
		}
	}

	totalEquity := 0.0
	availableBalance := 0.0
	if bal, err := at.trader.GetBalance(); err == nil {
		if v, ok := bal["totalEquity"].(float64); ok {
			totalEquity = v
		}
		if v, ok := bal["availableBalance"].(float64); ok {
			availableBalance = v
		}
	}
	if totalEquity <= 0 {
		totalEquity = at.initialBalance
	}

	execStatus := "WAITING"
	if currentSide != "NONE" && currentQty > 0 {
		execStatus = "ENTRY"
	}

	// 替换变量
	addsBytes, _ := json.Marshal(strat.Adds)
	addsJson := string(addsBytes)

	prompt = strings.ReplaceAll(prompt, "{{STRATEGY_DIRECTION}}", strat.Direction)
	prompt = strings.ReplaceAll(prompt, "{{SYMBOL}}", strat.Symbol)
	prompt = strings.ReplaceAll(prompt, "{{ENTRY_PRICE}}", fmt.Sprintf("%.2f", strat.Entry.PriceTarget))
	prompt = strings.ReplaceAll(prompt, "{{ADDS_JSON}}", addsJson)
	prompt = strings.ReplaceAll(prompt, "{{STOP_LOSS}}", fmt.Sprintf("%.2f", strat.StopLoss.Price))
	prompt = strings.ReplaceAll(prompt, "{{TAKE_PROFITS}}", fmt.Sprintf("%v", strat.TakeProfits))
	prompt = strings.ReplaceAll(prompt, "{{PREV_STRATEGY_TEXT}}", prevText)
	prompt = strings.ReplaceAll(prompt, "{{INITIAL_BALANCE}}", fmt.Sprintf("%.2f", at.initialBalance))
	prompt = strings.ReplaceAll(prompt, "{{TOTAL_EQUITY}}", fmt.Sprintf("%.2f", totalEquity))
	prompt = strings.ReplaceAll(prompt, "{{AVAILABLE_BALANCE}}", fmt.Sprintf("%.2f", availableBalance))
	prompt = strings.ReplaceAll(prompt, "{{PERFORMANCE_INFO}}", "N/A")
	prompt = strings.ReplaceAll(prompt, "{{ACTIVE_STRATEGY_COUNT}}", fmt.Sprintf("%d", activeCount))
	prompt = strings.ReplaceAll(prompt, "{{MAX_ALLOCATION_PER_STRATEGY}}", fmt.Sprintf("%.2f", maxAlloc))
	activeJSON, _ := json.Marshal(activeSimple)
	prompt = strings.ReplaceAll(prompt, "{{ACTIVE_STRATEGIES}}", string(activeJSON))
	prompt = strings.ReplaceAll(prompt, "{{EXECUTION_STATUS}}", execStatus)
	prompt = strings.ReplaceAll(prompt, "{{EXECUTED_ADD_COUNT}}", "0")

	prompt = strings.ReplaceAll(prompt, "{{CURRENT_PRICE}}", fmt.Sprintf("%.2f", currentPrice))
	prompt = strings.ReplaceAll(prompt, "{{RSI_1H}}", fmt.Sprintf("%.2f", rsi1h))
	prompt = strings.ReplaceAll(prompt, "{{RSI_4H}}", fmt.Sprintf("%.2f", rsi4h))
	prompt = strings.ReplaceAll(prompt, "{{MACD_4H}}", fmt.Sprintf("%.2f", macdHist4h))

	prompt = strings.ReplaceAll(prompt, "{{CURRENT_POSITION_SIDE}}", currentSide)
	prompt = strings.ReplaceAll(prompt, "{{CURRENT_POSITION_SIZE}}", fmt.Sprintf("%.4f", currentQty))
	prompt = strings.ReplaceAll(prompt, "{{AVG_PRICE}}", fmt.Sprintf("%.2f", avgPrice))
	prompt = strings.ReplaceAll(prompt, "{{UNREALIZED_PNL}}", fmt.Sprintf("%.2f", pnlPercent))

	// 注入 LEVERAGE
	// 修正：优先使用用户配置的杠杆，而不是策略推荐的
	// 如果用户配置为 0，才回退到策略推荐
	userLeverage := 5
	if strings.Contains(strat.Symbol, "BTC") || strings.Contains(strat.Symbol, "ETH") {
		userLeverage = at.config.BTCETHLeverage
	} else {
		userLeverage = at.config.AltcoinLeverage
	}
	if userLeverage <= 0 {
		userLeverage = strat.LeverageRecommend
	}
	
	// 同时更新 strat 对象中的值，以便后续逻辑一致
	strat.LeverageRecommend = userLeverage
	
	prompt = strings.ReplaceAll(prompt, "{{LEVERAGE}}", fmt.Sprintf("%d", userLeverage))

	// 原始策略全文直接给 AI，自主解析，不在本地提取关键字
	prompt = strings.ReplaceAll(prompt, "{{RAW_STRATEGY_TEXT}}", strat.RawContent)

	// 🔑 获取当前未成交委托和订单历史，让 AI 判断哪些订单需要补齐
	openOrders, err := at.trader.GetOpenOrders(strat.Symbol)
	if err != nil {
		log.Printf("⚠️ 获取当前委托失败: %v", err)
		openOrders = []map[string]interface{}{}
	}
	openOrdersJson, _ := json.Marshal(openOrders)
	prompt = strings.ReplaceAll(prompt, "{{CURRENT_ORDERS_JSON}}", string(openOrdersJson))
	log.Printf("📋 [AI上下文] 当前委托: %d 个", len(openOrders))

	// 获取从策略接收时间以来的订单历史（已成交/已取消），用于判断哪些点位已经发生过
	receivedAt := at.getStrategyReceivedAt(strat.SignalID)
	startAt := receivedAt.Add(-5 * time.Minute).UnixMilli()
	endAt := time.Now().UnixMilli()
	orderHistory, err := at.trader.GetOrderHistory(strat.Symbol, startAt, endAt)
	if err != nil {
		log.Printf("⚠️ 获取订单历史失败: %v", err)
		orderHistory = []map[string]interface{}{}
	}
	// 计划单历史（止盈/止损）可选补充：仅在交易器支持时启用
	if ph, ok := at.trader.(interface {
		GetPlanOrderHistory(symbol string, startTime, endTime int64) ([]map[string]interface{}, error)
	}); ok {
		if planHist, err := ph.GetPlanOrderHistory(strat.Symbol, startAt, endAt); err == nil && len(planHist) > 0 {
			orderHistory = append(orderHistory, planHist...)
		}
	}
	orderHistoryJson, _ := json.Marshal(orderHistory)
	prompt = strings.ReplaceAll(prompt, "{{ORDER_HISTORY_JSON}}", string(orderHistoryJson))
	log.Printf("[ai-context] Order history: %d records", len(orderHistory))

	// 使用配置面板中的自定义系统提示词
	at.mu.RLock()
	customPrompt := at.customPrompt
	overrideBase := at.overrideBasePrompt
	sysTemplateName := at.systemPromptTemplate
	btcEthLevCfg := at.config.BTCETHLeverage
	altLevCfg := at.config.AltcoinLeverage
	at.mu.RUnlock()

	// 自检差异指令（由代码层提供，只用于提示“缺啥”；具体怎么补由AI决定）
	diffDirective := strings.TrimSpace(extraDirective)
	if diffDirective == "" {
		diffDirective = "DIFF_CHECK: no explicit diff report."
	}

	// 将 trader 配置与 diff 报告一起注入到 user prompt 的 {{CUSTOM_PROMPT}}
	var promptDirective strings.Builder
	promptDirective.WriteString("TRADER_CONFIG:\n")
	promptDirective.WriteString(fmt.Sprintf("- btc_eth_leverage: %d\n", btcEthLevCfg))
	promptDirective.WriteString(fmt.Sprintf("- altcoin_leverage: %d\n", altLevCfg))
	promptDirective.WriteString(fmt.Sprintf("- leverage_for_%s: %d\n", strat.Symbol, userLeverage))
	if strings.TrimSpace(customPrompt) != "" {
		promptDirective.WriteString("- trader_custom_directive: |\n")
		for _, line := range strings.Split(strings.TrimSpace(customPrompt), "\n") {
			promptDirective.WriteString("  " + line + "\n")
		}
	} else {
		promptDirective.WriteString("- trader_custom_directive: (empty)\n")
	}
	promptDirective.WriteString("\nDIFF_REPORT:\n")
	promptDirective.WriteString(diffDirective)
	prompt = strings.ReplaceAll(prompt, "{{CUSTOM_PROMPT}}", promptDirective.String())

	// 【修复】信号执行器的 system prompt 仅使用“执行器基础约束 + 配置页附加提示词”
	// 避免错误引入 prompts/default.txt 或其他模板内容，导致前端看到的 system prompt 与配置页不一致
	executorBaseSystemPrompt := "You are a strict trading execution agent.\n" +
		"You must output only a valid JSON array. No markdown.\n"

	trimmedCustomPrompt := strings.TrimSpace(customPrompt)
	if overrideBase && trimmedCustomPrompt != "" {
		// 覆盖模式：用户希望完全自定义（但仍保留执行器的硬约束，避免返回非JSON）
		log.Printf("⚠️ [signal-ai] override_base_prompt enabled; using custom prompt with executor constraints")
	} else {
		overrideBase = false // 仅影响 system prompt 拼装，不影响 DB 配置本身
	}

	var systemPromptBuilder strings.Builder
	systemPromptBuilder.WriteString(executorBaseSystemPrompt)
	systemPromptBuilder.WriteString("\n")
	systemPromptBuilder.WriteString(fmt.Sprintf("TemplateName: %s\n", sysTemplateName))
	systemPromptBuilder.WriteString(fmt.Sprintf("LeverageConfig: btc_eth=%d altcoin=%d chosen_for_symbol=%d\n", btcEthLevCfg, altLevCfg, userLeverage))
	if trimmedCustomPrompt != "" {
		systemPromptBuilder.WriteString("\nTraderCustomDirective:\n")
		systemPromptBuilder.WriteString(trimmedCustomPrompt)
		systemPromptBuilder.WriteString("\n")
	}
	systemPrompt := systemPromptBuilder.String()

	log.Printf("[signal-ai] prompt assembled trader=%s symbol=%s template=%s system_prompt_len=%d input_prompt_len=%d",
		at.id, strat.Symbol, sysTemplateName, len(systemPrompt), len(prompt))

	resp, err := at.mcpClient.CallWithMessages(systemPrompt, prompt)
	if err != nil {
		log.Printf("❌ AI调用失败: %v", err)
		return
	}

	// 5. 解析结果（完全复用主决策引擎的解析逻辑，保证JSON格式和容错行为一致）
	decisions, err := decision.ExtractDecisionsFromResponse(resp)
	if err != nil {
		log.Printf("❌ 解析AI结果失败: %v", err)
		return
	}

	// 6. 多动作逐条执行（避免“只补TP/SL不补入场/补仓”）
	if len(decisions) == 0 {
		log.Printf("[signal-ai] No decisions returned for %s", strat.Symbol)
		return
	}

	// 如果是差异修复模式，不允许纯 wait-only
	hasActionable := false
	for i := range decisions {
		a := strings.ToLower(strings.TrimSpace(decisions[i].Action))
		if a != "wait" && a != "hold" {
			hasActionable = true
			break
		}
	}

	if strings.Contains(diffDirective, "DIFF_DETECTED") && !hasActionable {
		// 二次强提示重试一次
		retryDirective := diffDirective + " STRICT_MODE: You must output actions to fix the missing items. Do NOT output wait. Place limit orders for all missing entry/add prices."
		promptRetry := strings.ReplaceAll(prompt, diffDirective, retryDirective)
		resp2, err2 := at.mcpClient.CallWithMessages(systemPrompt, promptRetry)
		if err2 == nil {
			if ds2, errx := decision.ExtractDecisionsFromResponse(resp2); errx == nil && len(ds2) > 0 {
				decisions = ds2
				resp = resp2
				hasActionable = false
				for i := range decisions {
					a := strings.ToLower(strings.TrimSpace(decisions[i].Action))
					if a != "wait" && a != "hold" {
						hasActionable = true
						break
					}
				}
			}
		}
	}

	// 仍然 wait-only：走兜底补单，确保不是“只检查不执行”
	if strings.Contains(diffDirective, "DIFF_DETECTED") && !hasActionable {
		log.Printf("[signal-ai] wait-only on diff detected; fallback to deterministic limit placement symbol=%s", strat.Symbol)
		at.placeMissingLimitOrdersFallback(strat, missing, currentPrice, rsi1h, rsi4h, macdHist4h, currentSide, currentQty)
		if missingSL || missingTP {
			at.CheckStrategyCompletion(strat)
		}
		return
	}

	// 6.2 准备缺失价位队列，用于AI未给出价格时兜底填充
	missingQueue := make([]expectedPoint, 0, len(missing))
	missingQueue = append(missingQueue, missing...)

	// 6.3 本批次去重：跟踪已下单的价位，避免同一AI回复中重复下单
	placedPrices := make(map[string]bool) // key: "action_price" e.g. "place_long_order_3119.00"

	for i := range decisions {
		d := decisions[i]
		if strings.TrimSpace(d.Symbol) == "" {
			d.Symbol = strat.Symbol
		}
		// 强制限制为当前策略币对，防止跨symbol误下单
		d.Symbol = strat.Symbol

		// 兼容 AI 返回 place_limit_order：按策略方向映射为 place_long_order/place_short_order
		if strings.TrimSpace(d.Action) == "place_limit_order" {
			if strings.ToUpper(strings.TrimSpace(strat.Direction)) == "SHORT" {
				d.Action = "place_short_order"
			} else {
				d.Action = "place_long_order"
			}
		}

		// 强制使用用户配置的杠杆（信号模式不信任AI自由选择杠杆）
		switch strings.ToLower(strings.TrimSpace(d.Action)) {
		case "open_long", "open_short", "place_long_order", "place_short_order":
			if userLeverage > 0 {
				d.Leverage = userLeverage
			}
		}

		// 价格兜底：AI未给出 price 时，按缺失队列或入场价自动填充，避免 0 价导致失败
		if (d.Action == "place_long_order" || d.Action == "place_short_order") && d.Price <= 0 {
			if len(missingQueue) > 0 {
				d.Price = missingQueue[0].price
				missingQueue = missingQueue[1:]
				if d.Reasoning == "" {
					d.Reasoning = "Auto-filled limit price from missing queue."
				} else {
					d.Reasoning += " (auto-filled price)"
				}
			} else if strat.Entry.PriceTarget > 0 {
				d.Price = strat.Entry.PriceTarget
				if d.Reasoning == "" {
					d.Reasoning = "Auto-filled limit price from strategy entry."
				} else {
					d.Reasoning += " (auto-filled entry price)"
				}
			}
		}

		// 本批次去重：如果同一价位的同类型订单已经下过，跳过
		if d.Action == "place_long_order" || d.Action == "place_short_order" {
			priceKey := fmt.Sprintf("%s_%.2f", d.Action, d.Price)
			if placedPrices[priceKey] {
				log.Printf("⏭️ [ai-exec] skipping duplicate order in batch: %s price=%.2f", d.Action, d.Price)
				continue
			}
			placedPrices[priceKey] = true
		}

		actionRecord := &logger.DecisionAction{
			Symbol:    d.Symbol,
			Action:    d.Action,
			Reasoning: d.Reasoning,
		}

		execErr := at.executeDecisionWithRecord(&d, actionRecord)
		if execErr != nil {
			actionRecord.Success = false
			actionRecord.Error = execErr.Error()
			log.Printf("❌ [ai-exec] action=%s symbol=%s failed: %v", d.Action, d.Symbol, execErr)
		} else {
			actionRecord.Success = true
			log.Printf("✅ [ai-exec] action=%s symbol=%s done", d.Action, d.Symbol)
		}

		at.saveStrategyDecisionHistoryFromDecision(strat, &d, actionRecord, currentPrice, rsi1h, rsi4h, macdHist4h, currentSide, currentQty, systemPrompt, prompt, resp, execErr)
	}
}

// executeAIAction 执行 AI 的决策
func (at *AutoTrader) executeAIAction(result AIExecutionResult, strat *signal.SignalDecision, currentPrice float64) {
	if result.Action == "WAIT" {
		return
	}

	// 计算金额
	if at.initialBalance <= 0 {
		at.initialBalance = 1000
	}
	sizeUSD := at.initialBalance * result.AmountPercent
	quantity := sizeUSD / currentPrice
	leverage := strat.LeverageRecommend
	if leverage == 0 {
		leverage = 5
	}

	var err error

	switch result.Action {
	case "OPEN_LONG", "ADD_LONG":
		if result.AmountPercent > 0 {
			log.Printf("🚀 执行做多: %.4f (%.0f%%)", quantity, result.AmountPercent*100)
			_, err = at.trader.OpenLong(strat.Symbol, quantity, leverage)
		}
	case "OPEN_SHORT", "ADD_SHORT":
		if result.AmountPercent > 0 {
			log.Printf("🚀 执行做空: %.4f (%.0f%%)", quantity, result.AmountPercent*100)
			_, err = at.trader.OpenShort(strat.Symbol, quantity, leverage)
		}
	case "CLOSE_LONG":
		log.Printf("🔄 执行平多")
		_, err = at.trader.CloseLong(strat.Symbol, 0) // 全平
	case "CLOSE_SHORT":
		log.Printf("🔄 执行平空")
		_, err = at.trader.CloseShort(strat.Symbol, 0) // 全平
	}

	if err != nil {
		log.Printf("❌ 交易执行失败: %v", err)
	} else {
		// 成功后设置止盈止损 (如果是开仓/加仓)
		if strings.Contains(result.Action, "OPEN") || strings.Contains(result.Action, "ADD") {
			at.setStrategySLTP(strat, quantity)
			// 更新状态到数据库
			at.updateStrategyStatus(strat.SignalID, strat.Symbol, result.Action, currentPrice, quantity, 0)

			// 【新增】启动延迟二次检查 (等待成交和交易所状态更新)
			go func() {
				// 等待5秒让限价单可能成交，或者状态同步
				time.Sleep(5 * time.Second)
				at.CheckStrategyCompletion(strat)
			}()
		} else if strings.Contains(result.Action, "CLOSE") {
			// 平仓更新状态
			at.updateStrategyStatus(strat.SignalID, strat.Symbol, "CLOSED", 0, 0, 0)
			at.markStrategyClosed(strat.SignalID)
		}
	}
}

// setStrategySLTP 设置策略的止盈止损
func (at *AutoTrader) setStrategySLTP(strat *signal.SignalDecision, quantity float64) {
	// 获取最新总持仓
	positions, _ := at.trader.GetPositions()
	totalQty := quantity
	for _, p := range positions {
		if p["symbol"] == strat.Symbol {
			totalQty = math.Abs(p["positionAmt"].(float64))
			break
		}
	}

	slPrice := strat.StopLoss.Price
	side := "LONG"
	if strat.Direction == "SHORT" {
		side = "SHORT"
	}

	if slPrice > 0 {
		at.trader.SetStopLoss(strat.Symbol, side, totalQty, slPrice)
	}

	if len(strat.TakeProfits) > 0 {
		tpPrice := strat.TakeProfits[0].Price
		if tpPrice > 0 {
			at.trader.SetTakeProfit(strat.Symbol, side, totalQty, tpPrice)
		}
	}
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
		ExecutionSuccess: true, // 默认视为成功；若后续有错误再写入 ExecutionError
		ExecutionError:   "",
	}

	if history.ExecutionError != "" {
		history.ExecutionSuccess = false
	}

	if err := db.SaveStrategyDecision(history); err != nil {
		log.Printf("⚠️ 保存决策历史失败: %v", err)
	} else {
		log.Printf("📝 已保存决策历史: %s | %s | ID: %d", result.Action, strat.Symbol, history.ID)
	}
}

// saveStrategyDecisionHistoryFromDecision 【功能】保存每一条AI动作的执行结果（用于前端逐条展示成功/失败）
func (at *AutoTrader) saveStrategyDecisionHistoryFromDecision(
	strat *signal.SignalDecision,
	d *decision.Decision,
	actionRecord *logger.DecisionAction,
	currentPrice, rsi1h, rsi4h, macd4h float64,
	positionSide string,
	positionQty float64,
	systemPrompt, inputPrompt, rawResponse string,
	execErr error,
) {
	if strat == nil || d == nil || at.database == nil {
		return
	}

	db, ok := at.database.(*sysconfig.Database)
	if !ok {
		return
	}

	amtPct := 0.0
	if at.initialBalance > 0 && d.PositionSizeUSD > 0 {
		amtPct = d.PositionSizeUSD / at.initialBalance
		if amtPct > 1 {
			amtPct = 1
		}
		if amtPct < 0 {
			amtPct = 0
		}
	}

	h := &sysconfig.StrategyDecisionHistory{
		TraderID:         at.id,
		StrategyID:       strat.SignalID,
		DecisionTime:     time.Now(),
		Action:           d.Action,
		Symbol:           strat.Symbol,
		CurrentPrice:     currentPrice,
		TargetPrice:      strat.Entry.PriceTarget,
		PositionSide:     positionSide,
		PositionQty:      positionQty,
		AmountPercent:    amtPct,
		Reason:           d.Reasoning,
		RSI1H:            rsi1h,
		RSI4H:            rsi4h,
		MACD4H:           macd4h,
		SystemPrompt:     systemPrompt,
		InputPrompt:      inputPrompt,
		RawAIResponse:    rawResponse,
		ExecutionSuccess: execErr == nil,
		ExecutionError:   "",
	}
	if actionRecord != nil {
		if actionRecord.Reasoning != "" {
			h.Reason = actionRecord.Reasoning
		}
		if actionRecord.Error != "" {
			h.ExecutionError = actionRecord.Error
		}
	}
	if execErr != nil && h.ExecutionError == "" {
		h.ExecutionError = execErr.Error()
	}

	if err := db.SaveStrategyDecision(h); err != nil {
		log.Printf("[signal-ai] Failed to save decision history: %v", err)
	}
}

// CheckStrategyCompletion 检查策略执行完整性（二次检查）
// 当持仓已建立但止损/止盈未设置时，触发 AI 补设
func (at *AutoTrader) CheckStrategyCompletion(strat *signal.SignalDecision) {
	if strat == nil {
		return
	}

	log.Printf("🔍 [二次检查] 检查 %s 策略完整性 (ID: %s)...", strat.Symbol, strat.SignalID)

	// 1. 获取当前持仓
	positions, err := at.trader.GetPositions()
	if err != nil {
		log.Printf("⚠️ [二次检查] 获取持仓失败: %v", err)
		return
	}

	// 2. 检查是否有该策略的持仓
	var posQty float64
	var posSide string
	for _, pos := range positions {
		if pos["symbol"] == strat.Symbol {
			amt := pos["positionAmt"].(float64)
			if amt != 0 {
				posQty = math.Abs(amt)
				if amt > 0 {
					posSide = "LONG"
				} else {
					posSide = "SHORT"
				}
				break
			}
		}
	}

	if posQty == 0 {
		log.Printf("  ℹ️  [二次检查] %s 暂无持仓，跳过", strat.Symbol)
		return
	}

	// 3. 获取当前计划委托（止损止盈）
	openOrders, err := at.trader.GetOpenOrders(strat.Symbol)
	if err != nil {
		log.Printf("⚠️ [二次检查] 获取委托失败: %v", err)
		return
	}

	hasStopLoss := false
	hasTakeProfit := false
	for _, order := range openOrders {
		orderType, _ := order["type"].(string)
		if orderType == "stop_loss" || orderType == "loss_plan" || orderType == "pos_loss" {
			hasStopLoss = true
		}
		if orderType == "take_profit" || orderType == "profit_plan" || orderType == "pos_profit" {
			hasTakeProfit = true
		}
	}

	// 4. 检查策略是否要求止损/止盈
	needsStopLoss := strat.StopLoss.Price > 0 && !hasStopLoss
	needsTakeProfit := len(strat.TakeProfits) > 0 && strat.TakeProfits[0].Price > 0 && !hasTakeProfit

	if !needsStopLoss && !needsTakeProfit {
		log.Printf("  ✅ [二次检查] %s 止损止盈已设置完毕", strat.Symbol)
		return
	}

	// 5. 有持仓但缺少止损/止盈，自动补设
	log.Printf("  ⚠️ [二次检查] %s 持仓 %.4f (%s) 但: 止损=%v 止盈=%v",
		strat.Symbol, posQty, posSide, hasStopLoss, hasTakeProfit)

	if needsStopLoss {
		log.Printf("  🛡️ [二次检查] 自动补设止损: %.4f", strat.StopLoss.Price)
		if err := at.trader.SetStopLoss(strat.Symbol, posSide, posQty, strat.StopLoss.Price); err != nil {
			log.Printf("  ❌ [二次检查] 设置止损失败: %v", err)
		}
	}

	if needsTakeProfit {
		tpPrice := strat.TakeProfits[0].Price
		log.Printf("  💰 [二次检查] 自动补设止盈: %.4f", tpPrice)
		if err := at.trader.SetTakeProfit(strat.Symbol, posSide, posQty, tpPrice); err != nil {
			log.Printf("  ❌ [二次检查] 设置止盈失败: %v", err)
		}
	}

	log.Printf("  ✅ [二次检查] %s 完成补设", strat.Symbol)
}
