package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"nofx/signal"
	"nofx/market"
)

type StrategyResponse struct {
	Strategy     *signal.SignalDecision `json:"strategy"`
	CurrentPrice float64                `json:"current_price"`
	UpdatedAt    time.Time              `json:"updated_at"`
}

// handleGetActiveStrategies 获取所有活跃全局策略
func (s *Server) handleGetActiveStrategies(c *gin.Context) {
	if signal.GlobalManager == nil {
		c.JSON(http.StatusOK, []interface{}{})
		return
	}

	strategies := signal.GlobalManager.ListActiveStrategies()
	result := make([]StrategyResponse, 0) // 初始化为空切片而不是 nil，确保 JSON 返回 [] 而不是 null

	for _, snap := range strategies {
		if snap != nil && snap.Strategy != nil {
			// 获取当前价格
			currentPrice := 0.0
			marketData, err := market.Get(snap.Strategy.Symbol)
			if err == nil {
				currentPrice = marketData.CurrentPrice
			}

			result = append(result, StrategyResponse{
				Strategy:     snap.Strategy,
				CurrentPrice: currentPrice,
				UpdatedAt:    snap.Time,
			})
		}
	}
	c.JSON(http.StatusOK, result)
}

// handleGetTraderStrategyStatuses 获取交易员的所有策略执行状态
func (s *Server) handleGetTraderStrategyStatuses(c *gin.Context) {
	id := c.Param("id")
	statuses, err := s.database.GetTraderStrategyStatuses(id)
	if err != nil {
		// 出错或无数据都返回空列表，方便前端处理
		c.JSON(http.StatusOK, []interface{}{}) 
		return
	}
	c.JSON(http.StatusOK, statuses)
}

// handleGetParsedSignals 获取所有已解析的信号历史
func (s *Server) handleGetParsedSignals(c *gin.Context) {
	signals, err := s.database.GetAllParsedSignals(100) // 默认返回最近100条
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, signals)
}
