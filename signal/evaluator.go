package signal

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"strings"
	"time"

	"nofx/mcp"
)

type Evaluator struct {
	mcpClient *mcp.Client
	prompt    string
}

type EvaluationResult struct {
	Action       string  `json:"action"`
	Reason       string  `json:"reason"`
	TradePercent float64 `json:"trade_percent"`
}

func NewEvaluator(client *mcp.Client) (*Evaluator, error) {
	content, err := ioutil.ReadFile("prompts/strategy_evaluator.txt")
	if err != nil {
		return nil, fmt.Errorf("读取模板失败: %w", err)
	}
	return &Evaluator{mcpClient: client, prompt: string(content)}, nil
}

func (e *Evaluator) Evaluate(
	strategy *SignalDecision,
	strategyTime time.Time,
	currentPrice float64,
	currentSide string,
	currentQty float64,
	currentPnL float64,
) (*EvaluationResult, error) {
	
	// 序列化策略
	strategyJSON, _ := json.MarshalIndent(strategy, "", "  ")

	// 替换变量
	p := e.prompt
	p = strings.Replace(p, "{{STRATEGY_JSON}}", string(strategyJSON), 1)
	p = strings.Replace(p, "{{SYMBOL}}", strategy.Symbol, 1)
	p = strings.Replace(p, "{{CURRENT_PRICE}}", fmt.Sprintf("%.2f", currentPrice), 1)
	p = strings.Replace(p, "{{STRATEGY_TIME}}", strategyTime.Format("15:04"), 1)
	p = strings.Replace(p, "{{TIME_DIFF}}", fmt.Sprintf("%.0f", time.Since(strategyTime).Minutes()), 1)
	p = strings.Replace(p, "{{CURRENT_SIDE}}", currentSide, 1)
	p = strings.Replace(p, "{{CURRENT_QUANTITY}}", fmt.Sprintf("%.4f", currentQty), 1)
	p = strings.Replace(p, "{{CURRENT_PNL_PCT}}", fmt.Sprintf("%.2f", currentPnL), 1)

	// 调用 AI
	resp, err := e.mcpClient.CallWithMessages("你是一个严谨的交易风控系统。", p)
	if err != nil {
		return nil, err
	}

	// 解析 JSON
	cleanJSON := strings.TrimSpace(resp)
	cleanJSON = strings.TrimPrefix(cleanJSON, "```json")
	cleanJSON = strings.TrimPrefix(cleanJSON, "```")
	cleanJSON = strings.TrimSuffix(cleanJSON, "```")
	cleanJSON = strings.TrimSpace(cleanJSON)
	
	var result EvaluationResult
	if err := json.Unmarshal([]byte(cleanJSON), &result); err != nil {
		log.Printf("解析AI评估结果失败: %s", cleanJSON)
		return nil, err
	}
	
	return &result, nil
}



