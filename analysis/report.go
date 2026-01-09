package analysis

import (
	"fmt"
	"nofx/market"
	"nofx/mcp"
	"strings"
	"time"
)

// TrendReport 日内趋势分析报告
type TrendReport struct {
	Symbol          string    `json:"symbol"`
	GeneratedAt     time.Time `json:"generated_at"`
	MarkdownContent string    `json:"content"`
	RawPrompt       string    `json:"raw_prompt,omitempty"` // 调试用
}

// AnalysisContext 分析上下文数据
type AnalysisContext struct {
	Symbol       string
	CurrentPrice float64
	
	// 维加斯通道 (1H)
	EMA144_1H float64
	EMA169_1H float64
	
	// 维加斯通道 (4H)
	EMA144_4H float64
	EMA169_4H float64
	
	// RSI
	RSI_1H float64
	RSI_4H float64
	
	// MACD
	MACD_1H float64
	MACD_4H float64
	
	// 价格变化
	PriceChange1H float64
	PriceChange4H float64
	
	// 其他
	FundingRate  float64
	OpenInterest float64
	
	// K线历史 (最近10根)
	RecentPrices_15m []float64
	RecentPrices_1H  []float64
	RecentPrices_4H  []float64
}

// GenerateTrendReport 生成日内趋势分析报告
func GenerateTrendReport(symbol string, mcpClient *mcp.Client) (*TrendReport, error) {
	// 1. 收集分析上下文数据
	ctx, err := collectAnalysisContext(symbol)
	if err != nil {
		return nil, fmt.Errorf("收集分析数据失败: %w", err)
	}
	
	// 2. 构建 AI 分析提示词
	prompt := buildAnalysisPrompt(ctx)
	
	// 3. 调用 AI 生成报告
	systemPrompt := `你是一位精通技术分析的高级加密货币分析师，擅长维加斯通道、多周期共振及价格行为(PA)分析。

请根据提供的市场数据，生成一份专业、详尽且具有可操作性的日内趋势技术分析报告。

核心指令：
1. **去 AI 化**：禁止出现“好的”、“作为一名 AI”、“根据您提供的数据”等开场白，直接输出报告标题并进入正文。
2. **结构分析**：在分析中必须包含对价格结构（如 HH/HL 或 LH/LL）的判断。
3. **维加斯通道**：这是你的核心分析工具，请结合 EMA144/169 给出趋势强度和支撑/阻力判断。
4. **情景预测**：预测未来 24 小时走势时，必须给出具体的情景概率（例如：大概率 60%，小概率 15% 等）。

报告格式要求（使用 Markdown）：
- 使用清晰的层级标题（# ## ###）。
- 重点位和关键结论使用加粗显示。
- 使用引用块（>）或代码块展示关键位。

报告大纲：
# [交易对] 日内趋势技术分析报告
## 1. 当前日内主趋势判断
结论：[明确结论，如：看涨/看跌/高位震荡]
- 维加斯通道深度审计
- 价格结构与 PA 分析 (HH/HL/LH/LL)
- 动量指标(RSI/MACD)佐证

## 2. 多周期趋势一致性分析
- 15m/1h/4h 周期对比
- 周期共振或背离点识别

## 3. 未来 4-24 小时价格走势预测
- 【情景 A】核心推演 (XX% 概率)
- 【情景 B】备选路径 (XX% 概率)
- 关键支撑与阻力位汇总表

请确保分析客观、专业，避免含糊其辞。`

	response, err := mcpClient.CallWithMessages(systemPrompt, prompt)
	if err != nil {
		return nil, fmt.Errorf("AI 生成报告失败: %w", err)
	}
	
	return &TrendReport{
		Symbol:          symbol,
		GeneratedAt:     time.Now(),
		MarkdownContent: response,
		RawPrompt:       prompt,
	}, nil
}

// StreamGenerateTrendReport 流式输出趋势报告
func StreamGenerateTrendReport(symbol string, mcpClient *mcp.Client, onChunk func(string)) error {
	ctx, err := collectAnalysisContext(symbol)
	if err != nil {
		return err
	}

	prompt := buildAnalysisPrompt(ctx)
	systemPrompt := `你是一位精通技术分析的高级加密货币分析师，擅长维加斯通道（Vegas Tunnel）、多周期共振及价格行为（Price Action）分析。
请根据提供的市场数据，生成一份极具专业深度且逻辑严密的日内趋势技术分析报告。

核心要求：
1. **去 AI 化**：开头直接输出报告标题，严禁“好的”、“根据数据”等废话。
2. **结构复刻**：必须严格按照下方的【报告大纲】结构输出，保留所有二级和三级标题。
3. **关键位分析**：在“关键价格行为观察”中，必须明确给出具体的“支撑”和“阻力”价格点位。
4. **情景推演**：预测部分必须包含“情景”、“目标”、“触发条件”三个要素，并给出概率。
5. **一致性总结**：必须通过对比 4H（趋势）、1H（波段）、15M（入场）三个周期来得出结论。

报告大纲：

# [交易对] 日内趋势技术分析报告

## 1. 当前日内主趋势判断
**结论：[用一句话定性，如：高位震荡，短期偏强但面临阻力]**

### 维加斯通道深度审计 (1小时图)
- 分析当前价格与 1H 维加斯通道（EMA144/169）的位置关系。
- 此时通道是提供支撑还是阻力？
- 结合价格形态描述当下的多空博弈状态。

### 多周期趋势一致性分析
- **4小时趋势 (主导趋势)**：[判断多/空/震荡] 及理由。
- **1小时趋势 (日内趋势)**：[判断多/空/震荡] 及理由。
- **15分钟趋势 (入场时机)**：[判断多/空/震荡] 及理由。
- **一致性总结**：[如：大周期上涨 > 中周期震荡 > 小周期回调，结论...]

### 关键价格行为观察 (Key PA)
- **支撑**：[具体价格区域]
- **阻力**：[具体价格区域]
- **成交量/动能**：[基于 RSI/MACD 和价格波动幅度，分析当前是放量还是缩量，动能强弱]

## 2. 未来 4-24 小时价格走势预测

### 推演一：[主趋势方向，如：向上突破] (概率：XX%)
- **情景**：[描述价格走势路径]
- **目标**：[第一目标 / 第二目标]
- **触发条件**：[如：1小时收盘站稳 91500 上方]

### 推演二：[次概方向，如：区间震荡 / 回调] (概率：XX%)
- **情景**：[描述价格走势路径]
- **目标**：[下方支撑位]
- **触发条件**：[如：跌破 90000 关键支撑]

### 推演三：[小概率变盘] (概率：XX%)
- **情景**：[描述黑天鹅或反转路径]

## 3. 核心观点总结
[用 2-3 句话总结当前最稳健的交易思路，如：当前处于上升中继，建议关注支撑位的低吸机会，突破前不宜激进追涨。]

请确保语言简练、专业，重点数据加粗显示。`

	return mcpClient.StreamWithMessages(systemPrompt, prompt, onChunk)
}

// collectAnalysisContext 收集多周期分析数据
func collectAnalysisContext(symbol string) (*AnalysisContext, error) {
	symbol = market.Normalize(symbol)
	
	// 获取基础市场数据
	marketData, err := market.Get(symbol)
	if err != nil {
		return nil, err
	}
	
	ctx := &AnalysisContext{
		Symbol:        symbol,
		CurrentPrice:  marketData.CurrentPrice,
		PriceChange1H: marketData.PriceChange1h,
		PriceChange4H: marketData.PriceChange4h,
		FundingRate:   marketData.FundingRate,
	}
	
	// OI
	if marketData.OpenInterest != nil {
		ctx.OpenInterest = marketData.OpenInterest.Latest
	}
	
	// 获取多周期K线并计算维加斯通道 (EMA144 / EMA169)
	klines1H, err := market.WSMonitorCli.GetCurrentKlines(symbol, "1h")
	if err == nil && len(klines1H) > 0 {
		ctx.EMA144_1H = calculateEMA(klines1H, 144)
		ctx.EMA169_1H = calculateEMA(klines1H, 169)
		ctx.RSI_1H = calculateRSI(klines1H, 14)
		ctx.MACD_1H = calculateMACD(klines1H)
		
		// 最近10根收盘价
		start := len(klines1H) - 10
		if start < 0 {
			start = 0
		}
		for i := start; i < len(klines1H); i++ {
			ctx.RecentPrices_1H = append(ctx.RecentPrices_1H, klines1H[i].Close)
		}
	}
	
	klines4H, err := market.WSMonitorCli.GetCurrentKlines(symbol, "4h")
	if err == nil && len(klines4H) > 0 {
		ctx.EMA144_4H = calculateEMA(klines4H, 144)
		ctx.EMA169_4H = calculateEMA(klines4H, 169)
		ctx.RSI_4H = calculateRSI(klines4H, 14)
		ctx.MACD_4H = calculateMACD(klines4H)
		
		start := len(klines4H) - 10
		if start < 0 {
			start = 0
		}
		for i := start; i < len(klines4H); i++ {
			ctx.RecentPrices_4H = append(ctx.RecentPrices_4H, klines4H[i].Close)
		}
	}
	
	// 15分钟周期 (通过5分钟K线模拟)
	klines5m, err := market.WSMonitorCli.GetCurrentKlines(symbol, "5m")
	if err == nil && len(klines5m) > 0 {
		start := len(klines5m) - 30 // 最近30根5分钟 ≈ 10个15分钟周期
		if start < 0 {
			start = 0
		}
		for i := start; i < len(klines5m); i += 3 { // 每3根5分钟取1个点
			ctx.RecentPrices_15m = append(ctx.RecentPrices_15m, klines5m[i].Close)
		}
	}
	
	return ctx, nil
}

// buildAnalysisPrompt 构建分析提示词
func buildAnalysisPrompt(ctx *AnalysisContext) string {
	var sb strings.Builder
	
	sb.WriteString(fmt.Sprintf("# %s 日内趋势技术分析数据\n\n", ctx.Symbol))
	
	sb.WriteString("## 当前状态\n")
	sb.WriteString(fmt.Sprintf("- **当前价格**: %.2f\n", ctx.CurrentPrice))
	sb.WriteString(fmt.Sprintf("- **1小时涨跌幅**: %.2f%%\n", ctx.PriceChange1H))
	sb.WriteString(fmt.Sprintf("- **4小时涨跌幅**: %.2f%%\n", ctx.PriceChange4H))
	sb.WriteString(fmt.Sprintf("- **资金费率**: %.4f%%\n", ctx.FundingRate*100))
	sb.WriteString(fmt.Sprintf("- **持仓量 (OI)**: %.2f\n\n", ctx.OpenInterest))
	
	sb.WriteString("## 维加斯通道 (Vegas Channel)\n")
	sb.WriteString("维加斯通道由 EMA144 和 EMA169 组成，是判断中期趋势的重要指标。\n\n")
	
	sb.WriteString("### 1小时周期\n")
	sb.WriteString(fmt.Sprintf("- EMA144 (1H): %.2f\n", ctx.EMA144_1H))
	sb.WriteString(fmt.Sprintf("- EMA169 (1H): %.2f\n", ctx.EMA169_1H))
	if ctx.CurrentPrice > ctx.EMA144_1H && ctx.CurrentPrice > ctx.EMA169_1H {
		sb.WriteString("- 价格位于维加斯通道**上方**，短期偏多\n\n")
	} else if ctx.CurrentPrice < ctx.EMA144_1H && ctx.CurrentPrice < ctx.EMA169_1H {
		sb.WriteString("- 价格位于维加斯通道**下方**，短期偏空\n\n")
	} else {
		sb.WriteString("- 价格位于维加斯通道**内部**，处于多空争夺区\n\n")
	}
	
	sb.WriteString("### 4小时周期\n")
	sb.WriteString(fmt.Sprintf("- EMA144 (4H): %.2f\n", ctx.EMA144_4H))
	sb.WriteString(fmt.Sprintf("- EMA169 (4H): %.2f\n", ctx.EMA169_4H))
	if ctx.CurrentPrice > ctx.EMA144_4H && ctx.CurrentPrice > ctx.EMA169_4H {
		sb.WriteString("- 价格位于维加斯通道**上方**，中期偏多\n\n")
	} else if ctx.CurrentPrice < ctx.EMA144_4H && ctx.CurrentPrice < ctx.EMA169_4H {
		sb.WriteString("- 价格位于维加斯通道**下方**，中期偏空\n\n")
	} else {
		sb.WriteString("- 价格位于维加斯通道**内部**，中期方向不明\n\n")
	}
	
	sb.WriteString("## 其他技术指标\n")
	sb.WriteString(fmt.Sprintf("- RSI (1H, 14期): %.2f\n", ctx.RSI_1H))
	sb.WriteString(fmt.Sprintf("- RSI (4H, 14期): %.2f\n", ctx.RSI_4H))
	sb.WriteString(fmt.Sprintf("- MACD (1H): %.4f\n", ctx.MACD_1H))
	sb.WriteString(fmt.Sprintf("- MACD (4H): %.4f\n\n", ctx.MACD_4H))
	
	sb.WriteString("## 多周期价格走势\n")
	sb.WriteString(fmt.Sprintf("- 15分钟近期价格: %v\n", formatPrices(ctx.RecentPrices_15m)))
	sb.WriteString(fmt.Sprintf("- 1小时近期价格: %v\n", formatPrices(ctx.RecentPrices_1H)))
	sb.WriteString(fmt.Sprintf("- 4小时近期价格: %v\n\n", formatPrices(ctx.RecentPrices_4H)))
	
	sb.WriteString("---\n\n")
	sb.WriteString("请根据以上数据，生成一份完整的日内趋势技术分析报告。\n")
	
	return sb.String()
}

// formatPrices 格式化价格数组
func formatPrices(prices []float64) string {
	if len(prices) == 0 {
		return "[]"
	}
	strs := make([]string, len(prices))
	for i, p := range prices {
		strs[i] = fmt.Sprintf("%.2f", p)
	}
	return "[" + strings.Join(strs, ", ") + "]"
}

// --- 复用 market 包的指标计算函数 (私有副本) ---

func calculateEMA(klines []market.Kline, period int) float64 {
	if len(klines) < period {
		return 0
	}
	sum := 0.0
	for i := 0; i < period; i++ {
		sum += klines[i].Close
	}
	ema := sum / float64(period)
	multiplier := 2.0 / float64(period+1)
	for i := period; i < len(klines); i++ {
		ema = (klines[i].Close-ema)*multiplier + ema
	}
	return ema
}

func calculateRSI(klines []market.Kline, period int) float64 {
	if len(klines) <= period {
		return 0
	}
	gains, losses := 0.0, 0.0
	for i := 1; i <= period; i++ {
		change := klines[i].Close - klines[i-1].Close
		if change > 0 {
			gains += change
		} else {
			losses += -change
		}
	}
	avgGain := gains / float64(period)
	avgLoss := losses / float64(period)
	for i := period + 1; i < len(klines); i++ {
		change := klines[i].Close - klines[i-1].Close
		if change > 0 {
			avgGain = (avgGain*float64(period-1) + change) / float64(period)
			avgLoss = (avgLoss * float64(period-1)) / float64(period)
		} else {
			avgGain = (avgGain * float64(period-1)) / float64(period)
			avgLoss = (avgLoss*float64(period-1) + (-change)) / float64(period)
		}
	}
	if avgLoss == 0 {
		return 100
	}
	rs := avgGain / avgLoss
	return 100 - (100 / (1 + rs))
}

func calculateMACD(klines []market.Kline) float64 {
	if len(klines) < 26 {
		return 0
	}
	ema12 := calculateEMA(klines, 12)
	ema26 := calculateEMA(klines, 26)
	return ema12 - ema26
}
