package api

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net"
	"net/http"
	"nofx/auth"
	"nofx/config"
	"nofx/crypto"
	"nofx/decision"
	"nofx/manager"
	"nofx/trader"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Server HTTP API服务器
type Server struct {
	router        *gin.Engine
	traderManager *manager.TraderManager
	database      *config.Database
	cryptoService *crypto.CryptoService
	port          int
}

// NewServer 创建API服务器
func NewServer(traderManager *manager.TraderManager, database *config.Database, cryptoService *crypto.CryptoService, port int) *Server {
	// 设置为Release模式（减少日志输出）
	gin.SetMode(gin.ReleaseMode)

	router := gin.Default()

	// 启用CORS
	router.Use(corsMiddleware())

	s := &Server{
		router:        router,
		traderManager: traderManager,
		database:      database,
		cryptoService: cryptoService,
		port:          port,
	}

	// 设置路由
	s.setupRoutes()

	return s
}

// corsMiddleware CORS中间件
func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Max-Age", "86400")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusOK)
			return
		}

		c.Next()
	}
}

// setupRoutes 设置路由
func (s *Server) setupRoutes() {
	// API路由组
	api := s.router.Group("/api")
	{
		// 健康检查
		api.Any("/health", s.handleHealth)

		// 管理员登录（管理员模式下使用，公共）
		api.POST("/admin-login", s.handleAdminLogin)

		// 非管理员模式下的公开认证路由
		if !auth.IsAdminMode() {
			// 认证相关路由（无需认证）
			api.POST("/register", s.handleRegister)
			api.POST("/login", s.handleLogin)
			api.POST("/verify-otp", s.handleVerifyOTP)
			api.POST("/complete-registration", s.handleCompleteRegistration)

			// 系统支持的模型和交易所（无需认证）
			api.GET("/supported-models", s.handleGetSupportedModels)
			api.GET("/supported-exchanges", s.handleGetSupportedExchanges)
		}

		// 系统配置（无需认证，用于前端判断是否管理员模式/注册是否开启）
		api.GET("/config", s.handleGetSystemConfig)

		// 加密服务（无需认证）
		api.GET("/crypto/public-key", s.handleGetPublicKey)

		// 系统提示词模板管理（仅在非管理员模式下公开）
		if !auth.IsAdminMode() {
			// 系统提示词模板管理（无需认证）
			api.GET("/prompt-templates", s.handleGetPromptTemplates)
			api.GET("/prompt-templates/:name", s.handleGetPromptTemplate)

			// 公开的竞赛数据（无需认证）
			api.GET("/traders", s.handlePublicTraderList)
			api.GET("/competition", s.handlePublicCompetition)
			api.GET("/top-traders", s.handleTopTraders)
			// 单个交易员收益曲线：需要登录使用，见 protected 组的 /equity-history
			// 批量历史曲线对比：仍然保留为无需认证的公开接口
			api.POST("/equity-history-batch", s.handleEquityHistoryBatch)
			api.GET("/traders/:id/public-config", s.handleGetPublicTraderConfig)
		}

		// 需要认证的路由
		protected := api.Group("/", s.authMiddleware())
		{
			// 注销（加入黑名单）
			protected.POST("/logout", s.handleLogout)

			// 服务器IP查询（需要认证，用于白名单配置）
			protected.GET("/server-ip", s.handleGetServerIP)

			// AI交易员管理
			protected.GET("/my-traders", s.handleTraderList)
			protected.GET("/traders/:id/config", s.handleGetTraderConfig)
			protected.POST("/traders", s.handleCreateTrader)
			protected.PUT("/traders/:id", s.handleUpdateTrader)
			protected.DELETE("/traders/:id", s.handleDeleteTrader)
			protected.POST("/traders/:id/start", s.handleStartTrader)
			protected.POST("/traders/:id/stop", s.handleStopTrader)
			protected.PUT("/traders/:id/prompt", s.handleUpdateTraderPrompt)
			protected.POST("/traders/:id/sync-balance", s.handleSyncBalance)
			protected.GET("/traders/:id/current-balance", s.handleGetCurrentBalance)
			protected.POST("/traders/:id/create-account", s.handleCreateTraderAccount)
			protected.PUT("/traders/:id/account/password", s.handleUpdateTraderAccountPassword)
			protected.GET("/traders/:id/account", s.handleGetTraderAccount)
			protected.DELETE("/traders/:id/account", s.handleDeleteTraderAccount)
			protected.POST("/traders/:id/category", s.handleSetTraderCategory)

			// 分类管理
			protected.GET("/categories", s.handleGetCategories)
			protected.POST("/categories", s.handleCreateCategory)
			protected.PUT("/categories/:id", s.handleUpdateCategory)
			protected.DELETE("/categories/:id", s.handleDeleteCategory)

			// 小组组长管理
			protected.POST("/group-leaders/create", s.handleCreateGroupLeader)
			protected.POST("/group-leaders/create-for-category", s.handleCreateGroupLeaderForCategory)
			protected.GET("/group-leaders", s.handleGetGroupLeaders)
			protected.PUT("/group-leaders/:id/categories", s.handleUpdateGroupLeaderCategories)
			protected.DELETE("/group-leaders/:id", s.handleDeleteGroupLeader)

			// 分类账号管理
			protected.GET("/category-accounts", s.handleGetCategoryAccounts)
			protected.GET("/category-accounts/:id", s.handleGetCategoryAccountInfo)
			protected.PUT("/category-accounts/:id/password", s.handleUpdateCategoryAccountPassword)

			// AI模型配置
			protected.GET("/models", s.handleGetModelConfigs)
			protected.PUT("/models", s.handleUpdateModelConfigs)

			// 交易所配置
			protected.GET("/exchanges", s.handleGetExchangeConfigs)
			protected.PUT("/exchanges", s.handleUpdateExchangeConfigs)

			// 用户信号源配置
			protected.GET("/user/signal-sources", s.handleGetUserSignalSource)
			protected.POST("/user/signal-sources", s.handleSaveUserSignalSource)

			// 用户账户信息
			protected.GET("/user/account", s.handleUserAccount)

			// 指定trader的数据（使用query参数 ?trader_id=xxx）
			protected.GET("/status", s.handleStatus)
			protected.GET("/account", s.handleAccount)
			protected.GET("/positions", s.handlePositions)
			protected.POST("/positions/close", s.handleClosePosition) // 平仓操作
			protected.GET("/decisions", s.handleDecisions)
			protected.GET("/decisions/latest", s.handleLatestDecisions)
			protected.GET("/statistics", s.handleStatistics)
			protected.GET("/equity-history", s.handleEquityHistory) // 需要认证，使用当前登录用户做权限校验
			protected.GET("/performance", s.handlePerformance)
		}
	}
}

// handleHealth 健康检查
func (s *Server) handleHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
		"time":   c.Request.Context().Value("time"),
	})
}

// handleGetPublicKey 获取RSA公钥（用于前端加密敏感数据）
func (s *Server) handleGetPublicKey(c *gin.Context) {
	if s.cryptoService == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Crypto service not initialized",
		})
		return
	}

	publicKeyPEM := s.cryptoService.GetPublicKeyPEM()
	if publicKeyPEM == "" {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get public key",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"public_key": publicKeyPEM,
	})
}

// handleGetSystemConfig 获取系统配置（客户端需要知道的配置）
func (s *Server) handleGetSystemConfig(c *gin.Context) {
	// 获取默认币种
	defaultCoinsStr, _ := s.database.GetSystemConfig("default_coins")
	var defaultCoins []string
	if defaultCoinsStr != "" {
		json.Unmarshal([]byte(defaultCoinsStr), &defaultCoins)
	}
	if len(defaultCoins) == 0 {
		// 使用硬编码的默认币种
		defaultCoins = []string{"BTCUSDT", "ETHUSDT", "SOLUSDT", "BNBUSDT", "XRPUSDT", "DOGEUSDT", "ADAUSDT", "HYPEUSDT"}
	}

	// 获取杠杆配置
	btcEthLeverageStr, _ := s.database.GetSystemConfig("btc_eth_leverage")
	altcoinLeverageStr, _ := s.database.GetSystemConfig("altcoin_leverage")

	btcEthLeverage := 5
	if val, err := strconv.Atoi(btcEthLeverageStr); err == nil && val > 0 {
		btcEthLeverage = val
	}

	altcoinLeverage := 5
	if val, err := strconv.Atoi(altcoinLeverageStr); err == nil && val > 0 {
		altcoinLeverage = val
	}

	// 获取内测模式配置
	betaModeStr, _ := s.database.GetSystemConfig("beta_mode")
	betaMode := betaModeStr == "true"

	c.JSON(http.StatusOK, gin.H{
		"admin_mode":       auth.IsAdminMode(),
		"beta_mode":        betaMode,
		"default_coins":    defaultCoins,
		"btc_eth_leverage": btcEthLeverage,
		"altcoin_leverage": altcoinLeverage,
	})
}

// handleGetServerIP 获取服务器IP地址（用于白名单配置）
func (s *Server) handleGetServerIP(c *gin.Context) {
	// 尝试通过第三方API获取公网IP
	publicIP := getPublicIPFromAPI()

	// 如果第三方API失败，从网络接口获取第一个公网IP
	if publicIP == "" {
		publicIP = getPublicIPFromInterface()
	}

	// 如果还是没有获取到，返回错误
	if publicIP == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无法获取公网IP地址"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"public_ip": publicIP,
		"message":   "请将此IP地址添加到白名单中",
	})
}

// getPublicIPFromAPI 通过第三方API获取公网IP
func getPublicIPFromAPI() string {
	// 尝试多个公网IP查询服务
	services := []string{
		"https://api.ipify.org?format=text",
		"https://icanhazip.com",
		"https://ifconfig.me",
	}

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	for _, service := range services {
		resp, err := client.Get(service)
		if err != nil {
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			body := make([]byte, 128)
			n, err := resp.Body.Read(body)
			if err != nil && err.Error() != "EOF" {
				continue
			}

			ip := strings.TrimSpace(string(body[:n]))
			// 验证是否为有效的IP地址
			if net.ParseIP(ip) != nil {
				return ip
			}
		}
	}

	return ""
}

// getPublicIPFromInterface 从网络接口获取第一个公网IP
func getPublicIPFromInterface() string {
	interfaces, err := net.Interfaces()
	if err != nil {
		return ""
	}

	for _, iface := range interfaces {
		// 跳过未启用的接口和回环接口
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			if ip == nil || ip.IsLoopback() {
				continue
			}

			// 只考虑IPv4地址
			if ip.To4() != nil {
				ipStr := ip.String()
				// 排除私有IP地址范围
				if !isPrivateIP(ip) {
					return ipStr
				}
			}
		}
	}

	return ""
}

// isPrivateIP 判断是否为私有IP地址
func isPrivateIP(ip net.IP) bool {
	// 私有IP地址范围：
	// 10.0.0.0/8
	// 172.16.0.0/12
	// 192.168.0.0/16
	privateRanges := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
	}

	for _, cidr := range privateRanges {
		_, subnet, _ := net.ParseCIDR(cidr)
		if subnet.Contains(ip) {
			return true
		}
	}

	return false
}

// getTraderFromQuery 从query参数获取trader（带权限检查）
func (s *Server) getTraderFromQuery(c *gin.Context) (*manager.TraderManager, string, error) {
	userID := c.GetString("user_id")
	traderID := c.Query("trader_id")

	// 获取用户角色
	user, err := s.database.GetUserByID(userID)
	if err != nil {
		return nil, "", fmt.Errorf("用户不存在")
	}

	role := user.Role
	if role == "" {
		role = "user" // 默认是普通用户
	}

	// 获取用户有权限访问的交易员列表
	var allowedTraders []*config.TraderRecord
	switch role {
	case "admin":
		// 管理员可以访问所有交易员
		allowedTraders, _ = s.database.GetAllTraders()
	case "user":
		// 普通用户：返回自己分类下的所有交易员，或owner_user_id为自己的交易员
		userCategories, _ := s.database.GetUserCategories(userID)
		if len(userCategories) == 0 {
			allowedTraders, _ = s.database.GetTradersByOwnerUserID(userID)
		} else {
			categoryTraders, _ := s.database.GetTradersByCategories(userCategories)
			ownerTraders, _ := s.database.GetTradersByOwnerUserID(userID)
			traderMap := make(map[string]*config.TraderRecord)
			for _, t := range categoryTraders {
				traderMap[t.ID] = t
			}
			for _, t := range ownerTraders {
				if t.Category == "" || contains(userCategories, t.Category) {
					traderMap[t.ID] = t
				}
			}
			allowedTraders = make([]*config.TraderRecord, 0, len(traderMap))
			for _, t := range traderMap {
				allowedTraders = append(allowedTraders, t)
			}
		}
	case "group_leader":
		// 小组组长：返回观测的分类下的交易员
		categories, _ := s.database.GetGroupLeaderCategories(userID)
		allowedTraders, _ = s.database.GetTradersByCategories(categories)
	case "trader_account":
		// 交易员账号：返回自己的交易员
		if user.TraderID != "" {
			traderList, _ := s.database.GetTradersByID(user.TraderID)
			if len(traderList) > 0 {
				allowedTraders = traderList
			}
		}
	default:
		// 向后兼容：默认只返回自己的交易员
		userCategories, _ := s.database.GetUserCategories(userID)
		if len(userCategories) == 0 {
			allowedTraders, _ = s.database.GetTradersByOwnerUserID(userID)
		} else {
			categoryTraders, _ := s.database.GetTradersByCategories(userCategories)
			ownerTraders, _ := s.database.GetTradersByOwnerUserID(userID)
			traderMap := make(map[string]*config.TraderRecord)
			for _, t := range categoryTraders {
				traderMap[t.ID] = t
			}
			for _, t := range ownerTraders {
				if t.Category == "" || contains(userCategories, t.Category) {
					traderMap[t.ID] = t
				}
			}
			allowedTraders = make([]*config.TraderRecord, 0, len(traderMap))
			for _, t := range traderMap {
				allowedTraders = append(allowedTraders, t)
			}
		}
	}

	if len(allowedTraders) == 0 {
		return nil, "", fmt.Errorf("没有可用的trader")
	}

	// 如果指定了trader_id，验证是否有权限访问
	if traderID != "" {
		hasPermission := false
		for _, t := range allowedTraders {
			if t.ID == traderID {
				hasPermission = true
				break
			}
		}
		if !hasPermission {
			return nil, "", fmt.Errorf("无权访问该交易员")
		}
	} else {
		// 如果没有指定trader_id，返回第一个有权限的交易员
		traderID = allowedTraders[0].ID
	}

	// 确保用户的交易员已加载到内存中
	err = s.traderManager.LoadUserTraders(s.database, userID)
	if err != nil {
		log.Printf("⚠️ 加载用户 %s 的交易员失败: %v", userID, err)
	}

	return s.traderManager, traderID, nil
}

// AI交易员管理相关结构体
type CreateTraderRequest struct {
	Name                 string  `json:"name" binding:"required"`
	AIModelID            string  `json:"ai_model_id" binding:"required"`
	ExchangeID           string  `json:"exchange_id" binding:"required"`
	InitialBalance       float64 `json:"initial_balance"`
	ScanIntervalMinutes  int     `json:"scan_interval_minutes"`
	BTCETHLeverage       int     `json:"btc_eth_leverage"`
	AltcoinLeverage      int     `json:"altcoin_leverage"`
	TradingSymbols       string  `json:"trading_symbols"`
	CustomPrompt         string  `json:"custom_prompt"`
	OverrideBasePrompt   bool    `json:"override_base_prompt"`
	SystemPromptTemplate string  `json:"system_prompt_template"` // 系统提示词模板名称
	IsCrossMargin        *bool   `json:"is_cross_margin"`        // 指针类型，nil表示使用默认值true
	UseCoinPool          bool    `json:"use_coin_pool"`
	UseOITop             bool    `json:"use_oi_top"`
	Category             string  `json:"category"` // 可选：分类名称（如果提供，必须属于当前用户）
}

type ModelConfig struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Provider     string `json:"provider"`
	Enabled      bool   `json:"enabled"`
	APIKey       string `json:"apiKey,omitempty"`
	CustomAPIURL string `json:"customApiUrl,omitempty"`
}

type ExchangeConfig struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Type      string `json:"type"` // "cex" or "dex"
	Enabled   bool   `json:"enabled"`
	APIKey    string `json:"apiKey,omitempty"`
	SecretKey string `json:"secretKey,omitempty"`
	Testnet   bool   `json:"testnet,omitempty"`
}

type UpdateModelConfigRequest struct {
	Models map[string]struct {
		Enabled         bool   `json:"enabled"`
		APIKey          string `json:"api_key"`
		CustomAPIURL    string `json:"custom_api_url"`
		CustomModelName string `json:"custom_model_name"`
	} `json:"models"`
}

type UpdateExchangeConfigRequest struct {
	Exchanges map[string]struct {
		Enabled               bool   `json:"enabled"`
		APIKey                string `json:"api_key"`
		SecretKey             string `json:"secret_key"`
		Passphrase            string `json:"passphrase"`
		Testnet               bool   `json:"testnet"`
		HyperliquidWalletAddr string `json:"hyperliquid_wallet_addr"`
		AsterUser             string `json:"aster_user"`
		AsterSigner           string `json:"aster_signer"`
		AsterPrivateKey       string `json:"aster_private_key"`
		Provider              string `json:"provider"`
		Label                 string `json:"label"`
	} `json:"exchanges"`
}

// handleCreateTrader 创建新的AI交易员
func (s *Server) handleCreateTrader(c *gin.Context) {
	userID := c.GetString("user_id")
	var req CreateTraderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 校验杠杆值
	if req.BTCETHLeverage < 0 || req.BTCETHLeverage > 50 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "BTC/ETH杠杆必须在1-50倍之间"})
		return
	}
	if req.AltcoinLeverage < 0 || req.AltcoinLeverage > 20 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "山寨币杠杆必须在1-20倍之间"})
		return
	}

	// 校验交易币种格式
	if req.TradingSymbols != "" {
		symbols := strings.Split(req.TradingSymbols, ",")
		for _, symbol := range symbols {
			symbol = strings.TrimSpace(symbol)
			if symbol != "" && !strings.HasSuffix(strings.ToUpper(symbol), "USDT") {
				c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("无效的币种格式: %s，必须以USDT结尾", symbol)})
				return
			}
		}
	}

	// 🔑 关键修复：从交易所配置中获取 provider，用于生成交易员ID
	// 注意：req.ExchangeID 是完整的交易所配置ID（如 bitget_1763638270626）
	// 但生成交易员ID时应该使用 provider（如 bitget）
	exchanges, err := s.database.GetExchanges(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("获取交易所配置失败: %v", err)})
		return
	}

	var exchangeProvider string
	var exchangeCfg *config.ExchangeConfig
	for _, exchange := range exchanges {
		if exchange.ID == req.ExchangeID {
			exchangeCfg = exchange
			exchangeProvider = exchange.Provider
			if exchangeProvider == "" {
				// 如果 provider 为空，从 ID 推断（兼容旧数据）
				if strings.HasPrefix(exchange.ID, "binance") {
					exchangeProvider = "binance"
				} else if strings.HasPrefix(exchange.ID, "hyperliquid") {
					exchangeProvider = "hyperliquid"
				} else if strings.HasPrefix(exchange.ID, "aster") {
					exchangeProvider = "aster"
				} else if strings.HasPrefix(exchange.ID, "bitget") {
					exchangeProvider = "bitget"
				} else {
					exchangeProvider = exchange.ID // Fallback
				}
			}
			break
		}
	}

	if exchangeCfg == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("交易所配置不存在: %s", req.ExchangeID)})
		return
	}

	if !exchangeCfg.Enabled {
		c.JSON(http.StatusBadRequest, gin.H{"error": "交易所未启用"})
		return
	}

	// 🔑 使用 provider 生成交易员ID（而不是完整的 ExchangeID）
	// 格式：{provider}_{AIModelID}_{timestamp}
	traderID := fmt.Sprintf("%s_%s_%d", exchangeProvider, req.AIModelID, time.Now().Unix())
	log.Printf("🔍 [handleCreateTrader] 生成交易员ID: ExchangeID=%s, Provider=%s, TraderID=%s", req.ExchangeID, exchangeProvider, traderID)

	// 设置默认值
	isCrossMargin := true // 默认为全仓模式
	if req.IsCrossMargin != nil {
		isCrossMargin = *req.IsCrossMargin
	}

	// 设置杠杆默认值（从系统配置获取）
	btcEthLeverage := 5
	altcoinLeverage := 5
	if req.BTCETHLeverage > 0 {
		btcEthLeverage = req.BTCETHLeverage
	} else {
		// 从系统配置获取默认值
		if btcEthLeverageStr, _ := s.database.GetSystemConfig("btc_eth_leverage"); btcEthLeverageStr != "" {
			if val, err := strconv.Atoi(btcEthLeverageStr); err == nil && val > 0 {
				btcEthLeverage = val
			}
		}
	}
	if req.AltcoinLeverage > 0 {
		altcoinLeverage = req.AltcoinLeverage
	} else {
		// 从系统配置获取默认值
		if altcoinLeverageStr, _ := s.database.GetSystemConfig("altcoin_leverage"); altcoinLeverageStr != "" {
			if val, err := strconv.Atoi(altcoinLeverageStr); err == nil && val > 0 {
				altcoinLeverage = val
			}
		}
	}

	// 设置系统提示词模板默认值
	systemPromptTemplate := "default"
	if req.SystemPromptTemplate != "" {
		systemPromptTemplate = req.SystemPromptTemplate
	}

	// 设置扫描间隔默认值（移除最小3分钟限制，允许测试用）
	scanIntervalMinutes := req.ScanIntervalMinutes
	if scanIntervalMinutes <= 0 {
		scanIntervalMinutes = 3 // 默认3分钟
	}
	// 注释掉最小3分钟限制，允许设置1分钟用于测试
	// if scanIntervalMinutes < 3 {
	// 	scanIntervalMinutes = 3
	// }

	// ✅ 直接使用用户输入的初始余额，不进行任何自动查询或覆盖
	actualBalance := req.InitialBalance
	log.Printf("✓ 使用用户设置的初始余额: %.2f USDT", actualBalance)

	// 设置分类和所有者用户ID
	category := "" // 默认为空字符串
	if req.Category != "" {
		// 验证分类是否属于当前用户
		categoryObj, err := s.database.GetCategoryByName(req.Category)
		if err != nil || categoryObj == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "分类不存在"})
			return
		}
		if categoryObj.OwnerUserID != userID {
			c.JSON(http.StatusForbidden, gin.H{"error": "只能使用自己的分类"})
			return
		}
		category = req.Category
	}

	// 创建交易员配置（数据库实体）
	trader := &config.TraderRecord{
		ID:                   traderID,
		UserID:               userID,
		OwnerUserID:          userID,   // 设置为当前用户ID
		Category:             category, // 设置分类（如果提供）
		Name:                 req.Name,
		AIModelID:            req.AIModelID,
		ExchangeID:           req.ExchangeID,
		InitialBalance:       actualBalance, // 使用实际查询的余额
		BTCETHLeverage:       btcEthLeverage,
		AltcoinLeverage:      altcoinLeverage,
		TradingSymbols:       req.TradingSymbols,
		UseCoinPool:          req.UseCoinPool,
		UseOITop:             req.UseOITop,
		CustomPrompt:         req.CustomPrompt,
		OverrideBasePrompt:   req.OverrideBasePrompt,
		SystemPromptTemplate: systemPromptTemplate,
		IsCrossMargin:        isCrossMargin,
		ScanIntervalMinutes:  scanIntervalMinutes,
		IsRunning:            false,
	}

	// 保存到数据库
	err = s.database.CreateTrader(trader)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("创建交易员失败: %v", err)})
		return
	}

	// 立即将新交易员加载到TraderManager中
	err = s.traderManager.LoadUserTraders(s.database, userID)
	if err != nil {
		log.Printf("⚠️ 加载用户交易员到内存失败: %v", err)
		// 这里不返回错误，因为交易员已经成功创建到数据库
	}

	log.Printf("✓ 创建交易员成功: %s (模型: %s, 交易所: %s)", req.Name, req.AIModelID, req.ExchangeID)

	c.JSON(http.StatusCreated, gin.H{
		"trader_id":   traderID,
		"trader_name": req.Name,
		"ai_model":    req.AIModelID,
		"is_running":  false,
	})
}

// UpdateTraderRequest 更新交易员请求
type UpdateTraderRequest struct {
	Name                 string  `json:"name" binding:"required"`
	AIModelID            string  `json:"ai_model_id" binding:"required"`
	ExchangeID           string  `json:"exchange_id" binding:"required"`
	InitialBalance       float64 `json:"initial_balance"`
	ScanIntervalMinutes  int     `json:"scan_interval_minutes"`
	BTCETHLeverage       int     `json:"btc_eth_leverage"`
	AltcoinLeverage      int     `json:"altcoin_leverage"`
	TradingSymbols       string  `json:"trading_symbols"`
	CustomPrompt         string  `json:"custom_prompt"`
	OverrideBasePrompt   bool    `json:"override_base_prompt"`
	SystemPromptTemplate string  `json:"system_prompt_template"` // 系统提示词模板名称
	IsCrossMargin        *bool   `json:"is_cross_margin"`
}

// handleUpdateTrader 更新交易员配置
func (s *Server) handleUpdateTrader(c *gin.Context) {
	userID := c.GetString("user_id")
	traderID := c.Param("id")

	var req UpdateTraderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 获取用户角色
	user, err := s.database.GetUserByID(userID)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户不存在"})
		return
	}

	role := user.Role
	if role == "" {
		role = "user" // 默认是普通用户
	}

	// 获取交易员信息
	existingTrader, err := s.database.GetTraderByID(traderID)
	if err != nil || existingTrader == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "交易员不存在"})
		return
	}

	// 权限检查：如果不是admin，验证交易员是否属于当前用户
	if role != "admin" {
		if existingTrader.OwnerUserID != userID {
			c.JSON(http.StatusForbidden, gin.H{"error": "只能修改自己的交易员"})
			return
		}
	}

	// 设置默认值
	isCrossMargin := existingTrader.IsCrossMargin // 保持原值
	if req.IsCrossMargin != nil {
		isCrossMargin = *req.IsCrossMargin
	}

	// 设置杠杆默认值
	btcEthLeverage := req.BTCETHLeverage
	altcoinLeverage := req.AltcoinLeverage
	if btcEthLeverage <= 0 {
		btcEthLeverage = existingTrader.BTCETHLeverage // 保持原值
	}
	if altcoinLeverage <= 0 {
		altcoinLeverage = existingTrader.AltcoinLeverage // 保持原值
	}

	// 设置扫描间隔，允许更新（移除最小3分钟限制，允许测试用）
	scanIntervalMinutes := req.ScanIntervalMinutes
	if scanIntervalMinutes <= 0 {
		scanIntervalMinutes = existingTrader.ScanIntervalMinutes // 保持原值
		// 注释掉最小3分钟限制，允许设置1分钟用于测试
		// } else if scanIntervalMinutes < 3 {
		// 	scanIntervalMinutes = 3
	}

	// 设置系统提示词模板（支持更新）
	systemPromptTemplate := req.SystemPromptTemplate
	if systemPromptTemplate == "" {
		systemPromptTemplate = existingTrader.SystemPromptTemplate // 保持原值
	}

	// 更新交易员配置
	trader := &config.TraderRecord{
		ID:                   traderID,
		UserID:               userID,
		Name:                 req.Name,
		AIModelID:            req.AIModelID,
		ExchangeID:           req.ExchangeID,
		InitialBalance:       req.InitialBalance,
		BTCETHLeverage:       btcEthLeverage,
		AltcoinLeverage:      altcoinLeverage,
		TradingSymbols:       req.TradingSymbols,
		CustomPrompt:         req.CustomPrompt,
		OverrideBasePrompt:   req.OverrideBasePrompt,
		SystemPromptTemplate: systemPromptTemplate, // 🔑 允许更新提示词模板
		IsCrossMargin:        isCrossMargin,
		ScanIntervalMinutes:  scanIntervalMinutes,
		IsRunning:            existingTrader.IsRunning, // 保持原值
	}

	// 更新数据库
	err = s.database.UpdateTrader(trader)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("更新交易员失败: %v", err)})
		return
	}

	// 如果交易员正在运行，更新内存中的配置（立即生效，无需重启）
	if existingTrader.IsRunning {
		runningTrader, err := s.traderManager.GetTrader(traderID)
		if err == nil && runningTrader != nil {
			// 更新系统提示词模板（下次 AI 决策时生效）
			runningTrader.SetSystemPromptTemplate(systemPromptTemplate)
			log.Printf("✓ 已更新运行中交易员的系统提示词模板: %s → %s", existingTrader.SystemPromptTemplate, systemPromptTemplate)
		}
	}

	// 重新加载交易员到内存
	err = s.traderManager.LoadUserTraders(s.database, userID)
	if err != nil {
		log.Printf("⚠️ 重新加载用户交易员到内存失败: %v", err)
	}

	log.Printf("✓ 更新交易员成功: %s (模型: %s, 交易所: %s, 提示词模板: %s)", req.Name, req.AIModelID, req.ExchangeID, systemPromptTemplate)

	c.JSON(http.StatusOK, gin.H{
		"trader_id":   traderID,
		"trader_name": req.Name,
		"ai_model":    req.AIModelID,
		"message":     "交易员更新成功",
	})
}

// handleDeleteTrader 删除交易员
func (s *Server) handleDeleteTrader(c *gin.Context) {
	userID := c.GetString("user_id")
	traderID := c.Param("id")

	// 获取用户角色
	user, err := s.database.GetUserByID(userID)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户不存在"})
		return
	}

	role := user.Role
	if role == "" {
		role = "user" // 默认是普通用户
	}

	// 获取交易员信息
	trader, err := s.database.GetTraderByID(traderID)
	if err != nil || trader == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "交易员不存在"})
		return
	}

	// 权限检查：如果不是admin，验证交易员是否属于当前用户
	if role != "admin" {
		if trader.OwnerUserID != userID {
			c.JSON(http.StatusForbidden, gin.H{"error": "只能删除自己的交易员"})
			return
		}
	}

	// 从数据库删除
	err = s.database.DeleteTrader(userID, traderID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("删除交易员失败: %v", err)})
		return
	}

	// 如果交易员正在运行，先停止它
	if trader, err := s.traderManager.GetTrader(traderID); err == nil {
		status := trader.GetStatus()
		if isRunning, ok := status["is_running"].(bool); ok && isRunning {
			trader.Stop()
			log.Printf("⏹  已停止运行中的交易员: %s", traderID)
		}
	}

	log.Printf("✓ 交易员已删除: %s", traderID)
	c.JSON(http.StatusOK, gin.H{"message": "交易员已删除"})
}

// handleStartTrader 启动交易员
func (s *Server) handleStartTrader(c *gin.Context) {
	userID := c.GetString("user_id")
	traderID := c.Param("id")

	// 🔍 调试：记录完整的请求信息
	log.Printf("🔍 [handleStartTrader] 请求详情:")
	log.Printf("  - URL路径: %s", c.Request.URL.Path)
	log.Printf("  - 用户ID: %s", userID)
	log.Printf("  - 交易员ID参数: %s", traderID)
	log.Printf("  - 交易员ID长度: %d", len(traderID))

	// 获取用户角色
	user, err := s.database.GetUserByID(userID)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户不存在"})
		return
	}

	role := user.Role
	if role == "" {
		role = "user" // 默认是普通用户
	}

	// 获取交易员信息
	traderRecord, err := s.database.GetTraderByID(traderID)
	if err != nil || traderRecord == nil {
		log.Printf("⚠️ [handleStartTrader] 交易员不存在: ID=%s, 错误=%v", traderID, err)
		// 🔍 调试：列出用户的所有交易员ID
		allTraders, _ := s.database.GetTradersByOwnerUserID(userID)
		log.Printf("🔍 [handleStartTrader] 用户 %s 的所有交易员ID:", userID)
		for _, t := range allTraders {
			log.Printf("  - %s (ExchangeID: %s, AIModelID: %s)", t.ID, t.ExchangeID, t.AIModelID)
		}
		c.JSON(http.StatusNotFound, gin.H{"error": "交易员不存在"})
		return
	}

	log.Printf("✅ [handleStartTrader] 找到交易员: ID=%s, ExchangeID=%s, AIModelID=%s", traderRecord.ID, traderRecord.ExchangeID, traderRecord.AIModelID)

	// 权限检查：如果不是admin，验证交易员是否属于当前用户
	if role != "admin" {
		if traderRecord.OwnerUserID != userID {
			c.JSON(http.StatusForbidden, gin.H{"error": "只能启动自己的交易员"})
			return
		}
	}

	trader, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "交易员不存在"})
		return
	}

	// 检查交易员是否已经在运行
	status := trader.GetStatus()
	if isRunning, ok := status["is_running"].(bool); ok && isRunning {
		c.JSON(http.StatusBadRequest, gin.H{"error": "交易员已在运行中"})
		return
	}

	// 启动交易员
	go func() {
		log.Printf("▶️  启动交易员 %s (%s)", traderID, trader.GetName())
		if err := trader.Run(); err != nil {
			log.Printf("❌ 交易员 %s 运行错误: %v", trader.GetName(), err)
		}
	}()

	// 更新数据库中的运行状态
	err = s.database.UpdateTraderStatus(userID, traderID, true)
	if err != nil {
		log.Printf("⚠️  更新交易员状态失败: %v", err)
	}

	log.Printf("✓ 交易员 %s 已启动", trader.GetName())
	c.JSON(http.StatusOK, gin.H{"message": "交易员已启动"})
}

// handleStopTrader 停止交易员
func (s *Server) handleStopTrader(c *gin.Context) {
	userID := c.GetString("user_id")
	traderID := c.Param("id")

	// 获取用户角色
	user, err := s.database.GetUserByID(userID)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户不存在"})
		return
	}

	role := user.Role
	if role == "" {
		role = "user" // 默认是普通用户
	}

	// 获取交易员信息
	traderRecord, err := s.database.GetTraderByID(traderID)
	if err != nil || traderRecord == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "交易员不存在"})
		return
	}

	// 权限检查：如果不是admin，验证交易员是否属于当前用户
	if role != "admin" {
		if traderRecord.OwnerUserID != userID {
			c.JSON(http.StatusForbidden, gin.H{"error": "只能停止自己的交易员"})
			return
		}
	}

	trader, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "交易员不存在"})
		return
	}

	// 检查交易员是否正在运行
	status := trader.GetStatus()
	if isRunning, ok := status["is_running"].(bool); ok && !isRunning {
		c.JSON(http.StatusBadRequest, gin.H{"error": "交易员已停止"})
		return
	}

	// 停止交易员
	trader.Stop()

	// 更新数据库中的运行状态
	err = s.database.UpdateTraderStatus(userID, traderID, false)
	if err != nil {
		log.Printf("⚠️  更新交易员状态失败: %v", err)
	}

	log.Printf("⏹  交易员 %s 已停止", trader.GetName())
	c.JSON(http.StatusOK, gin.H{"message": "交易员已停止"})
}

// handleUpdateTraderPrompt 更新交易员自定义Prompt
func (s *Server) handleUpdateTraderPrompt(c *gin.Context) {
	traderID := c.Param("id")
	userID := c.GetString("user_id")

	// 获取用户角色
	user, err := s.database.GetUserByID(userID)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户不存在"})
		return
	}

	role := user.Role
	if role == "" {
		role = "user" // 默认是普通用户
	}

	// 获取交易员信息
	traderRecord, err := s.database.GetTraderByID(traderID)
	if err != nil || traderRecord == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "交易员不存在"})
		return
	}

	// 权限检查：如果不是admin，验证交易员是否属于当前用户
	if role != "admin" {
		if traderRecord.OwnerUserID != userID {
			c.JSON(http.StatusForbidden, gin.H{"error": "只能修改自己的交易员"})
			return
		}
	}

	var req struct {
		CustomPrompt       string `json:"custom_prompt"`
		OverrideBasePrompt bool   `json:"override_base_prompt"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 更新数据库
	err = s.database.UpdateTraderCustomPrompt(userID, traderID, req.CustomPrompt, req.OverrideBasePrompt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("更新自定义prompt失败: %v", err)})
		return
	}

	// 如果trader在内存中，更新其custom prompt和override设置
	trader, err := s.traderManager.GetTrader(traderID)
	if err == nil {
		trader.SetCustomPrompt(req.CustomPrompt)
		trader.SetOverrideBasePrompt(req.OverrideBasePrompt)
		log.Printf("✓ 已更新交易员 %s 的自定义prompt (覆盖基础=%v)", trader.GetName(), req.OverrideBasePrompt)
	}

	c.JSON(http.StatusOK, gin.H{"message": "自定义prompt已更新"})
}

// handleSyncBalance 同步交易所余额到initial_balance（选项B：手动同步 + 选项C：智能检测）
func (s *Server) handleSyncBalance(c *gin.Context) {
	userID := c.GetString("user_id")
	traderID := c.Param("id")

	// 获取用户角色
	user, err := s.database.GetUserByID(userID)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户不存在"})
		return
	}

	role := user.Role
	if role == "" {
		role = "user" // 默认是普通用户
	}

	// 获取交易员信息
	traderRecord, err := s.database.GetTraderByID(traderID)
	if err != nil || traderRecord == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "交易员不存在"})
		return
	}

	// 权限检查：如果不是admin，验证交易员是否属于当前用户
	if role != "admin" {
		if traderRecord.OwnerUserID != userID {
			c.JSON(http.StatusForbidden, gin.H{"error": "只能同步自己交易员的余额"})
			return
		}
	}

	log.Printf("🔄 用户 %s 请求同步交易员 %s 的余额", userID, traderID)

	// 从数据库获取交易员配置（包含交易所信息）
	traderConfig, _, exchangeCfg, err := s.database.GetTraderConfig(userID, traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "交易员不存在"})
		return
	}

	if exchangeCfg == nil || !exchangeCfg.Enabled {
		c.JSON(http.StatusBadRequest, gin.H{"error": "交易所未配置或未启用"})
		return
	}

	// 创建临时 trader 查询余额
	var tempTrader trader.Trader
	var createErr error

	switch traderConfig.ExchangeID {
	case "binance":
		tempTrader = trader.NewFuturesTrader(exchangeCfg.APIKey, exchangeCfg.SecretKey, userID)
	case "hyperliquid":
		tempTrader, createErr = trader.NewHyperliquidTrader(
			exchangeCfg.APIKey,
			exchangeCfg.HyperliquidWalletAddr,
			exchangeCfg.Testnet,
		)
	case "aster":
		tempTrader, createErr = trader.NewAsterTrader(
			exchangeCfg.AsterUser,
			exchangeCfg.AsterSigner,
			exchangeCfg.AsterPrivateKey,
		)
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "不支持的交易所类型"})
		return
	}

	if createErr != nil {
		log.Printf("⚠️ 创建临时 trader 失败: %v", createErr)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("连接交易所失败: %v", createErr)})
		return
	}

	// 查询实际余额
	balanceInfo, balanceErr := tempTrader.GetBalance()
	if balanceErr != nil {
		log.Printf("⚠️ 查询交易所余额失败: %v", balanceErr)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("查询余额失败: %v", balanceErr)})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无法获取可用余额"})
		return
	}

	oldBalance := traderConfig.InitialBalance

	// ✅ 选项C：智能检测余额变化
	changePercent := ((actualBalance - oldBalance) / oldBalance) * 100
	changeType := "增加"
	if changePercent < 0 {
		changeType = "减少"
	}

	log.Printf("✓ 查询到交易所实际余额: %.2f USDT (当前配置: %.2f USDT, 变化: %.2f%%)",
		actualBalance, oldBalance, changePercent)

	// 更新数据库中的 initial_balance
	err = s.database.UpdateTraderInitialBalance(userID, traderID, actualBalance)
	if err != nil {
		log.Printf("❌ 更新initial_balance失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新余额失败"})
		return
	}

	// 重新加载交易员到内存
	err = s.traderManager.LoadUserTraders(s.database, userID)
	if err != nil {
		log.Printf("⚠️ 重新加载用户交易员到内存失败: %v", err)
	}

	log.Printf("✅ 已同步余额: %.2f → %.2f USDT (%s %.2f%%)", oldBalance, actualBalance, changeType, changePercent)

	c.JSON(http.StatusOK, gin.H{
		"message":        "余额同步成功",
		"old_balance":    oldBalance,
		"new_balance":    actualBalance,
		"change_percent": changePercent,
		"change_type":    changeType,
	})
}

// handleGetCurrentBalance 获取当前交易所余额（仅用于前端显示，不更新数据库）
func (s *Server) handleGetCurrentBalance(c *gin.Context) {
	userID := c.GetString("user_id")
	traderID := c.Param("id")

	// 获取用户角色
	user, err := s.database.GetUserByID(userID)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户不存在"})
		return
	}

	role := user.Role
	if role == "" {
		role = "user" // 默认是普通用户
	}

	// 获取交易员信息
	traderRecord, err := s.database.GetTraderByID(traderID)
	if err != nil || traderRecord == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "交易员不存在"})
		return
	}

	// 权限检查：如果不是admin，验证交易员是否属于当前用户
	if role != "admin" {
		if traderRecord.OwnerUserID != userID {
			c.JSON(http.StatusForbidden, gin.H{"error": "只能获取自己交易员的余额"})
			return
		}
	}

	log.Printf("🔄 用户 %s 请求获取交易员 %s 当前余额", userID, traderID)

	// 从数据库获取交易员配置（包含交易所信息）
	traderConfig, _, exchangeCfg, err := s.database.GetTraderConfig(userID, traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "交易员不存在"})
		return
	}

	if exchangeCfg == nil || !exchangeCfg.Enabled {
		c.JSON(http.StatusBadRequest, gin.H{"error": "交易所未配置或未启用"})
		return
	}

	// 创建临时 trader 查询余额
	var tempTrader trader.Trader
	var createErr error

	switch traderConfig.ExchangeID {
	case "binance":
		tempTrader = trader.NewFuturesTrader(exchangeCfg.APIKey, exchangeCfg.SecretKey, userID)
	case "hyperliquid":
		tempTrader, createErr = trader.NewHyperliquidTrader(
			exchangeCfg.APIKey,
			exchangeCfg.HyperliquidWalletAddr,
			exchangeCfg.Testnet,
		)
	case "aster":
		tempTrader, createErr = trader.NewAsterTrader(
			exchangeCfg.AsterUser,
			exchangeCfg.AsterSigner,
			exchangeCfg.AsterPrivateKey,
		)
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "不支持的交易所类型"})
		return
	}

	if createErr != nil {
		log.Printf("⚠️ 创建临时 trader 失败: %v", createErr)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("连接交易所失败: %v", createErr)})
		return
	}

	// 查询实际余额
	balanceInfo, balanceErr := tempTrader.GetBalance()
	if balanceErr != nil {
		log.Printf("⚠️ 查询交易所余额失败: %v", balanceErr)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("查询余额失败: %v", balanceErr)})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无法获取可用余额"})
		return
	}

	log.Printf("✓ 查询到交易所当前余额: %.2f USDT", actualBalance)

	// 只返回余额信息，不更新数据库
	c.JSON(http.StatusOK, gin.H{
		"current_balance": actualBalance,
		"exchange_id":     traderConfig.ExchangeID,
	})
}

// handleGetModelConfigs 获取AI模型配置
func (s *Server) handleGetModelConfigs(c *gin.Context) {
	userID := c.GetString("user_id")
	log.Printf("🔍 查询用户 %s 的AI模型配置", userID)
	models, err := s.database.GetAIModels(userID)
	if err != nil {
		log.Printf("❌ 获取AI模型配置失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("获取AI模型配置失败: %v", err)})
		return
	}
	log.Printf("✅ 找到 %d 个AI模型配置", len(models))

	c.JSON(http.StatusOK, models)
}

// handleUpdateModelConfigs 更新AI模型配置
func (s *Server) handleUpdateModelConfigs(c *gin.Context) {
	userID := c.GetString("user_id")
	var req UpdateModelConfigRequest

	// 尝试解析为加密数据
	var encryptedPayload crypto.EncryptedPayload
	if err := c.ShouldBindJSON(&encryptedPayload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Failed to parse request: %v", err)})
		return
	}

	// 检查是否为加密数据（通过检查加密字段是否存在）
	if encryptedPayload.Ciphertext != "" {
		// 解密数据
		if s.cryptoService == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Crypto service not initialized"})
			return
		}

		plaintext, err := s.cryptoService.DecryptSensitiveData(&encryptedPayload)
		if err != nil {
			log.Printf("❌ 解密失败: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Decryption failed: %v", err)})
			return
		}

		// 解析解密后的JSON
		if err := json.Unmarshal([]byte(plaintext), &req); err != nil {
			log.Printf("❌ 解析解密后的JSON失败: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Invalid decrypted JSON: %v", err)})
			return
		}

		log.Printf("✓ 成功解密请求数据，包含 %d 个模型配置", len(req.Models))
	} else {
		// 尝试作为普通JSON解析（理论上不应该到这里，因为前端总是发送加密数据）
		log.Printf("⚠️ 接收到非加密数据，这不应该发生")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Expected encrypted payload"})
		return
	}

	// 更新每个模型的配置
	for modelID, modelData := range req.Models {
		err := s.database.UpdateAIModel(userID, modelID, modelData.Enabled, modelData.APIKey, modelData.CustomAPIURL, modelData.CustomModelName)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("更新模型 %s 失败: %v", modelID, err)})
			return
		}
	}

	// 重新加载该用户的所有交易员，使新配置立即生效
	err := s.traderManager.LoadUserTraders(s.database, userID)
	if err != nil {
		log.Printf("⚠️ 重新加载用户交易员到内存失败: %v", err)
		// 这里不返回错误，因为模型配置已经成功更新到数据库
	}

	log.Printf("✓ AI模型配置已更新: %+v", req.Models)
	c.JSON(http.StatusOK, gin.H{"message": "模型配置已更新"})
}

// handleGetExchangeConfigs 获取交易所配置
func (s *Server) handleGetExchangeConfigs(c *gin.Context) {
	userID := c.GetString("user_id")
	log.Printf("🔍 查询用户 %s 的交易所配置", userID)
	exchanges, err := s.database.GetExchanges(userID)
	if err != nil {
		log.Printf("❌ 获取交易所配置失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("获取交易所配置失败: %v", err)})
		return
	}
	log.Printf("✅ 找到 %d 个交易所配置", len(exchanges))

	c.JSON(http.StatusOK, exchanges)
}

// handleUpdateExchangeConfigs 更新交易所配置
func (s *Server) handleUpdateExchangeConfigs(c *gin.Context) {
	userID := c.GetString("user_id")
	var req UpdateExchangeConfigRequest

	// 尝试解析为加密数据
	var encryptedPayload crypto.EncryptedPayload
	if err := c.ShouldBindJSON(&encryptedPayload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Failed to parse request: %v", err)})
		return
	}

	// 检查是否为加密数据（通过检查加密字段是否存在）
	if encryptedPayload.Ciphertext != "" {
		// 解密数据
		if s.cryptoService == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Crypto service not initialized"})
			return
		}

		plaintext, err := s.cryptoService.DecryptSensitiveData(&encryptedPayload)
		if err != nil {
			log.Printf("❌ 解密失败: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Decryption failed: %v", err)})
			return
		}

		// 解析解密后的JSON
		if err := json.Unmarshal([]byte(plaintext), &req); err != nil {
			log.Printf("❌ 解析解密后的JSON失败: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Invalid decrypted JSON: %v", err)})
			return
		}

		log.Printf("✓ 成功解密请求数据，包含 %d 个交易所配置", len(req.Exchanges))
	} else {
		// 尝试作为普通JSON解析（理论上不应该到这里，因为前端总是发送加密数据）
		log.Printf("⚠️ 接收到非加密数据，这不应该发生")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Expected encrypted payload"})
		return
	}

	// 更新每个交易所的配置
	for exchangeID, exchangeData := range req.Exchanges {
		err := s.database.UpdateExchange(
			userID,
			exchangeID,
			exchangeData.Enabled,
			exchangeData.APIKey,
			exchangeData.SecretKey,
			exchangeData.Passphrase,
			exchangeData.Testnet,
			exchangeData.HyperliquidWalletAddr,
			exchangeData.AsterUser,
			exchangeData.AsterSigner,
			exchangeData.AsterPrivateKey,
			exchangeData.Provider,
			exchangeData.Label,
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("更新交易所 %s 失败: %v", exchangeID, err)})
			return
		}
	}

	// 重新加载该用户的所有交易员，使新配置立即生效
	err := s.traderManager.LoadUserTraders(s.database, userID)
	if err != nil {
		log.Printf("⚠️ 重新加载用户交易员到内存失败: %v", err)
		// 这里不返回错误，因为交易所配置已经成功更新到数据库
	}

	log.Printf("✓ 交易所配置已更新: %+v", req.Exchanges)
	c.JSON(http.StatusOK, gin.H{"message": "交易所配置已更新"})
}

// handleGetUserSignalSource 获取用户信号源配置
func (s *Server) handleGetUserSignalSource(c *gin.Context) {
	userID := c.GetString("user_id")
	source, err := s.database.GetUserSignalSource(userID)
	if err != nil {
		// 如果配置不存在，返回空配置而不是404错误
		c.JSON(http.StatusOK, gin.H{
			"coin_pool_url": "",
			"oi_top_url":    "",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"coin_pool_url": source.CoinPoolURL,
		"oi_top_url":    source.OITopURL,
	})
}

// handleSaveUserSignalSource 保存用户信号源配置
func (s *Server) handleSaveUserSignalSource(c *gin.Context) {
	userID := c.GetString("user_id")
	var req struct {
		CoinPoolURL string `json:"coin_pool_url"`
		OITopURL    string `json:"oi_top_url"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := s.database.CreateUserSignalSource(userID, req.CoinPoolURL, req.OITopURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("保存用户信号源配置失败: %v", err)})
		return
	}

	log.Printf("✓ 用户信号源配置已保存: user=%s, coin_pool=%s, oi_top=%s", userID, req.CoinPoolURL, req.OITopURL)
	c.JSON(http.StatusOK, gin.H{"message": "用户信号源配置已保存"})
}

// handleTraderList trader列表
func (s *Server) handleTraderList(c *gin.Context) {
	userID := c.GetString("user_id")

	// 获取用户角色
	user, err := s.database.GetUserByID(userID)
	if err != nil {
		// 向后兼容：如果获取失败，使用默认行为
		traders, _ := s.database.GetTraders(userID)
		result := make([]map[string]interface{}, 0, len(traders))
		for _, trader := range traders {
			isRunning := trader.IsRunning
			if at, err := s.traderManager.GetTrader(trader.ID); err == nil {
				status := at.GetStatus()
				if running, ok := status["is_running"].(bool); ok {
					isRunning = running
				}
			}
			result = append(result, map[string]interface{}{
				"trader_id":       trader.ID,
				"trader_name":     trader.Name,
				"ai_model":        trader.AIModelID,
				"exchange_id":     trader.ExchangeID,
				"is_running":      isRunning,
				"initial_balance": trader.InitialBalance,
			})
		}
		c.JSON(http.StatusOK, result)
		return
	}

	role := user.Role
	if role == "" {
		role = "user" // 默认是普通用户
	}

	var traders []*config.TraderRecord
	switch role {
	case "admin":
		// 真正的管理员：返回所有交易员（跨用户，特殊角色，一般不使用）
		traders, _ = s.database.GetAllTraders()
	case "user":
		// 普通用户：返回自己分类下的所有交易员，或owner_user_id为自己的交易员
		userCategories, _ := s.database.GetUserCategories(userID)
		if len(userCategories) == 0 {
			// 向后兼容：如果没有分类，返回owner_user_id为该用户的交易员
			traders, _ = s.database.GetTradersByOwnerUserID(userID)
		} else {
			// 返回分类下的交易员，以及owner_user_id为该用户但category为空的交易员
			categoryTraders, _ := s.database.GetTradersByCategories(userCategories)
			ownerTraders, _ := s.database.GetTradersByOwnerUserID(userID)
			// 合并并去重
			traderMap := make(map[string]*config.TraderRecord)
			for _, t := range categoryTraders {
				traderMap[t.ID] = t
			}
			for _, t := range ownerTraders {
				// 只添加category为空或属于用户分类的交易员
				if t.Category == "" || contains(userCategories, t.Category) {
					traderMap[t.ID] = t
				}
			}
			traders = make([]*config.TraderRecord, 0, len(traderMap))
			for _, t := range traderMap {
				traders = append(traders, t)
			}
		}
	case "group_leader":
		// 小组组长：返回观测的分类下的交易员
		categories, _ := s.database.GetGroupLeaderCategories(userID)
		traders, _ = s.database.GetTradersByCategories(categories)
	case "trader_account":
		// 交易员账号：返回自己的交易员
		if user.TraderID != "" {
			traderList, _ := s.database.GetTradersByID(user.TraderID)
			if len(traderList) > 0 {
				traders = traderList
			} else {
				traders = []*config.TraderRecord{}
			}
		} else {
			traders = []*config.TraderRecord{}
		}
	default:
		// 向后兼容：默认只返回自己的交易员（通过owner_user_id或分类）
		userCategories, _ := s.database.GetUserCategories(userID)
		if len(userCategories) == 0 {
			traders, _ = s.database.GetTradersByOwnerUserID(userID)
		} else {
			categoryTraders, _ := s.database.GetTradersByCategories(userCategories)
			ownerTraders, _ := s.database.GetTradersByOwnerUserID(userID)
			log.Printf("[handleGetTraders] User categories: %v, categoryTraders: %d, ownerTraders: %d",
				userCategories, len(categoryTraders), len(ownerTraders))
			traderMap := make(map[string]*config.TraderRecord)
			for _, t := range categoryTraders {
				traderMap[t.ID] = t
				log.Printf("[handleGetTraders] Category trader: ID=%s, Category=%s", t.ID, t.Category)
			}
			for _, t := range ownerTraders {
				if t.Category == "" || contains(userCategories, t.Category) {
					traderMap[t.ID] = t
					log.Printf("[handleGetTraders] Owner trader: ID=%s, Category=%s, Included=%v",
						t.ID, t.Category, t.Category == "" || contains(userCategories, t.Category))
				} else {
					log.Printf("[handleGetTraders] Owner trader excluded: ID=%s, Category=%s", t.ID, t.Category)
				}
			}
			traders = make([]*config.TraderRecord, 0, len(traderMap))
			for _, t := range traderMap {
				traders = append(traders, t)
			}
			log.Printf("[handleGetTraders] Final traders count: %d", len(traders))
		}
	}

	result := make([]map[string]interface{}, 0, len(traders))
	for _, trader := range traders {
		// 获取实时运行状态
		isRunning := trader.IsRunning
		if at, err := s.traderManager.GetTrader(trader.ID); err == nil {
			status := at.GetStatus()
			if running, ok := status["is_running"].(bool); ok {
				isRunning = running
			}
		}

		// 返回完整的 AIModelID（如 "admin_deepseek"），不要截断
		// 前端需要完整 ID 来验证模型是否存在（与 handleGetTraderConfig 保持一致）
		result = append(result, map[string]interface{}{
			"trader_id":       trader.ID,
			"trader_name":     trader.Name,
			"ai_model":        trader.AIModelID, // 使用完整 ID
			"exchange_id":     trader.ExchangeID,
			"is_running":      isRunning,
			"initial_balance": trader.InitialBalance,
			"category":        trader.Category,    // 添加分类字段
			"owner_user_id":   trader.OwnerUserID, // 添加所有者用户ID字段
		})
	}

	c.JSON(http.StatusOK, result)
}

// contains 检查字符串切片是否包含指定字符串
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// handleGetTraderConfig 获取交易员详细配置
func (s *Server) handleGetTraderConfig(c *gin.Context) {
	userID := c.GetString("user_id")
	traderID := c.Param("id")

	if traderID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "交易员ID不能为空"})
		return
	}

	// 获取用户角色
	user, err := s.database.GetUserByID(userID)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户不存在"})
		return
	}

	role := user.Role
	if role == "" {
		role = "user" // 默认是普通用户
	}

	// 获取交易员信息
	trader, err := s.database.GetTraderByID(traderID)
	if err != nil || trader == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "交易员不存在"})
		return
	}

	// 权限检查
	canAccess := false
	switch role {
	case "admin":
		// 管理员可以访问所有交易员
		canAccess = true
	case "user":
		// 普通用户：检查owner_user_id或分类权限
		if trader.OwnerUserID == userID {
			canAccess = true
		} else if trader.Category != "" {
			// 检查分类是否属于该用户
			category, _ := s.database.GetCategoryByName(trader.Category)
			if category != nil && category.OwnerUserID == userID {
				canAccess = true
			}
		}
	case "group_leader":
		// 小组组长：检查交易员是否在管理的分类内
		if trader.Category != "" {
			categories, _ := s.database.GetGroupLeaderCategories(userID)
			for _, cat := range categories {
				if cat == trader.Category {
					canAccess = true
					break
				}
			}
		}
	case "trader_account":
		// 交易员账号：检查是否是自己的交易员
		if user.TraderID == traderID {
			canAccess = true
		}
	}

	if !canAccess {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权访问该交易员"})
		return
	}

	traderConfig, _, _, err := s.database.GetTraderConfig(userID, traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("获取交易员配置失败: %v", err)})
		return
	}

	// 获取实时运行状态
	isRunning := traderConfig.IsRunning
	if at, err := s.traderManager.GetTrader(traderID); err == nil {
		status := at.GetStatus()
		if running, ok := status["is_running"].(bool); ok {
			isRunning = running
		}
	}

	// 返回完整的模型ID，不做转换，保持与前端模型列表一致
	aiModelID := traderConfig.AIModelID

	result := map[string]interface{}{
		"trader_id":              traderConfig.ID,
		"trader_name":            traderConfig.Name,
		"ai_model":               aiModelID,
		"exchange_id":            traderConfig.ExchangeID,
		"system_prompt_template": traderConfig.SystemPromptTemplate,
		"initial_balance":        traderConfig.InitialBalance,
		"scan_interval_minutes":  traderConfig.ScanIntervalMinutes,
		"btc_eth_leverage":       traderConfig.BTCETHLeverage,
		"altcoin_leverage":       traderConfig.AltcoinLeverage,
		"trading_symbols":        traderConfig.TradingSymbols,
		"custom_prompt":          traderConfig.CustomPrompt,
		"override_base_prompt":   traderConfig.OverrideBasePrompt,
		"is_cross_margin":        traderConfig.IsCrossMargin,
		"use_coin_pool":          traderConfig.UseCoinPool,
		"use_oi_top":             traderConfig.UseOITop,
		"is_running":             isRunning,
	}

	c.JSON(http.StatusOK, result)
}

// handleStatus 系统状态
func (s *Server) handleStatus(c *gin.Context) {
	_, traderID, err := s.getTraderFromQuery(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	trader, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	status := trader.GetStatus()
	c.JSON(http.StatusOK, status)
}

// handleUserAccount 用户账户信息
func (s *Server) handleUserAccount(c *gin.Context) {
	userID := c.GetString("user_id")
	user, err := s.database.GetUserByID(userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}

	// 构建响应
	response := gin.H{
		"id":    user.ID,
		"email": user.Email,
		"role":  user.Role,
	}

	// 如果是交易员账号，添加trader_id
	if user.Role == "trader_account" && user.TraderID != "" {
		response["trader_id"] = user.TraderID
	}

	// 如果是小组组长，添加categories
	if user.Role == "group_leader" {
		categories, _ := s.database.GetGroupLeaderCategories(userID)
		response["categories"] = categories
	} else {
		response["categories"] = []string{}
	}

	c.JSON(http.StatusOK, response)
}

// handleAccount 账户信息
func (s *Server) handleAccount(c *gin.Context) {
	_, traderID, err := s.getTraderFromQuery(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	trader, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	log.Printf("📊 收到账户信息请求 [%s]", trader.GetName())
	account, err := trader.GetAccountInfo()
	if err != nil {
		log.Printf("❌ 获取账户信息失败 [%s]: %v", trader.GetName(), err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("获取账户信息失败: %v", err),
		})
		return
	}

	log.Printf("✓ 返回账户信息 [%s]: 净值=%.2f, 可用=%.2f, 盈亏=%.2f (%.2f%%)",
		trader.GetName(),
		account["total_equity"],
		account["available_balance"],
		account["total_pnl"],
		account["total_pnl_pct"])
	c.JSON(http.StatusOK, account)
}

// handlePositions 持仓列表
func (s *Server) handlePositions(c *gin.Context) {
	_, traderID, err := s.getTraderFromQuery(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	trader, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	positions, err := trader.GetPositions()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("获取持仓列表失败: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, positions)
}

// handleClosePosition 平仓操作
func (s *Server) handleClosePosition(c *gin.Context) {
	_, traderID, err := s.getTraderFromQuery(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var req struct {
		Symbol   string  `json:"symbol" binding:"required"`   // 交易对，如 BTCUSDT
		Side     string  `json:"side" binding:"required"`     // long 或 short
		Quantity float64 `json:"quantity" binding:"required"` // 平仓数量
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("参数错误: %v", err)})
		return
	}

	trader, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	// 根据持仓方向调用对应的平仓方法
	var result map[string]interface{}
	if req.Side == "long" {
		result, err = trader.CloseLong(req.Symbol, req.Quantity)
	} else if req.Side == "short" {
		result, err = trader.CloseShort(req.Symbol, req.Quantity)
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "side 必须是 long 或 short"})
		return
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("平仓失败: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "平仓成功",
		"result":  result,
	})
}

// handleDecisions 决策日志列表
func (s *Server) handleDecisions(c *gin.Context) {
	_, traderID, err := s.getTraderFromQuery(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	trader, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	// 获取所有历史决策记录（无限制）
	records, err := trader.GetDecisionLogger().GetLatestRecords(10000)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("获取决策日志失败: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, records)
}

// handleLatestDecisions 最新决策日志（最近5条，最新的在前）
func (s *Server) handleLatestDecisions(c *gin.Context) {
	_, traderID, err := s.getTraderFromQuery(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	trader, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	records, err := trader.GetDecisionLogger().GetLatestRecords(5)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("获取决策日志失败: %v", err),
		})
		return
	}

	// 反转数组，让最新的在前面（用于列表显示）
	// GetLatestRecords返回的是从旧到新（用于图表），这里需要从新到旧
	for i, j := 0, len(records)-1; i < j; i, j = i+1, j-1 {
		records[i], records[j] = records[j], records[i]
	}

	c.JSON(http.StatusOK, records)
}

// handleStatistics 统计信息
func (s *Server) handleStatistics(c *gin.Context) {
	_, traderID, err := s.getTraderFromQuery(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	trader, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	stats, err := trader.GetDecisionLogger().GetStatistics()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("获取统计信息失败: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// handleCompetition 竞赛总览（对比所有trader）
func (s *Server) handleCompetition(c *gin.Context) {
	userID := c.GetString("user_id")

	// 确保用户的交易员已加载到内存中
	err := s.traderManager.LoadUserTraders(s.database, userID)
	if err != nil {
		log.Printf("⚠️ 加载用户 %s 的交易员失败: %v", userID, err)
	}

	competition, err := s.traderManager.GetCompetitionData()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("获取竞赛数据失败: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, competition)
}

// handleEquityHistory 收益率历史数据
func (s *Server) handleEquityHistory(c *gin.Context) {
	_, traderID, err := s.getTraderFromQuery(c)
	if err != nil {
		log.Printf("❌ handleEquityHistory: getTraderFromQuery失败 - trader_id=%s, error=%v", c.Query("trader_id"), err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	trader, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	// 获取尽可能多的历史数据（几天的数据）
	// 每3分钟一个周期：10000条 = 约20天的数据
	records, err := trader.GetDecisionLogger().GetLatestRecords(10000)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("获取历史数据失败: %v", err),
		})
		return
	}

	// 构建收益率历史数据点
	type EquityPoint struct {
		Timestamp        string  `json:"timestamp"`
		TotalEquity      float64 `json:"total_equity"`      // 账户净值（wallet + unrealized）
		AvailableBalance float64 `json:"available_balance"` // 可用余额
		TotalPnL         float64 `json:"total_pnl"`         // 总盈亏（相对初始余额）
		TotalPnLPct      float64 `json:"total_pnl_pct"`     // 总盈亏百分比
		PositionCount    int     `json:"position_count"`    // 持仓数量
		MarginUsedPct    float64 `json:"margin_used_pct"`   // 保证金使用率
		CycleNumber      int     `json:"cycle_number"`
	}

	// 从AutoTrader获取初始余额（用于计算盈亏百分比）
	initialBalance := 0.0
	if status := trader.GetStatus(); status != nil {
		if ib, ok := status["initial_balance"].(float64); ok && ib > 0 {
			initialBalance = ib
		}
	}

	// 如果无法从status获取，且有历史记录，则从第一条记录获取
	if initialBalance == 0 && len(records) > 0 {
		// 第一条记录的equity作为初始余额
		initialBalance = records[0].AccountState.TotalBalance
	}

	// 如果还是无法获取，返回错误
	if initialBalance == 0 {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "无法获取初始余额",
		})
		return
	}

	var history []EquityPoint
	for _, record := range records {
		// TotalBalance字段实际存储的是TotalEquity
		totalEquity := record.AccountState.TotalBalance
		// TotalUnrealizedProfit字段实际存储的是TotalPnL（相对初始余额）
		totalPnL := record.AccountState.TotalUnrealizedProfit

		// 计算盈亏百分比
		totalPnLPct := 0.0
		if initialBalance > 0 {
			totalPnLPct = (totalPnL / initialBalance) * 100
		}

		history = append(history, EquityPoint{
			Timestamp:        record.Timestamp.Format("2006-01-02 15:04:05"),
			TotalEquity:      totalEquity,
			AvailableBalance: record.AccountState.AvailableBalance,
			TotalPnL:         totalPnL,
			TotalPnLPct:      totalPnLPct,
			PositionCount:    record.AccountState.PositionCount,
			MarginUsedPct:    record.AccountState.MarginUsedPct,
			CycleNumber:      record.CycleNumber,
		})
	}

	c.JSON(http.StatusOK, history)
}

// handlePerformance AI历史表现分析（用于展示AI学习和反思）
func (s *Server) handlePerformance(c *gin.Context) {
	_, traderID, err := s.getTraderFromQuery(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	trader, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	// 分析最近100个周期的交易表现（避免长期持仓的交易记录丢失）
	// 假设每3分钟一个周期，100个周期 = 5小时，足够覆盖大部分交易
	performance, err := trader.GetDecisionLogger().AnalyzePerformance(100)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("分析历史表现失败: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, performance)
}

// authMiddleware JWT认证中间件
func (s *Server) authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "缺少Authorization头"})
			c.Abort()
			return
		}

		// 检查Bearer token格式
		tokenParts := strings.Split(authHeader, " ")
		if len(tokenParts) != 2 || tokenParts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "无效的Authorization格式"})
			c.Abort()
			return
		}

		tokenString := tokenParts[1]

		// 黑名单检查
		if auth.IsTokenBlacklisted(tokenString) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "token已失效，请重新登录"})
			c.Abort()
			return
		}

		// 验证JWT token
		claims, err := auth.ValidateJWT(tokenString)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "无效的token: " + err.Error()})
			c.Abort()
			return
		}

		// 将用户信息存储到上下文中
		c.Set("user_id", claims.UserID)
		c.Set("email", claims.Email)
		c.Next()
	}
}

// handleAdminLogin 管理员登录（密码仅来自环境变量）
func (s *Server) handleAdminLogin(c *gin.Context) {
	if !auth.IsAdminMode() {
		c.JSON(http.StatusForbidden, gin.H{"error": "仅管理员模式可用"})
		return
	}

	// 简单的IP速率限制（5次/分钟 + 递增退避）
	// 为简化，此处省略复杂实现，可在后续使用中间件或Redis增强

	var req struct {
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Password) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少密码"})
		return
	}
	if !auth.CheckAdminPassword(req.Password) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "密码错误"})
		return
	}

	token, err := auth.GenerateJWT("admin", "admin@localhost")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成token失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"token": token, "user_id": "admin", "email": "admin@localhost"})
}

// handleLogout 将当前token加入黑名单
func (s *Server) handleLogout(c *gin.Context) {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "缺少Authorization头"})
		return
	}
	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || parts[0] != "Bearer" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "无效的Authorization格式"})
		return
	}
	tokenString := parts[1]
	claims, err := auth.ValidateJWT(tokenString)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "无效的token"})
		return
	}
	var exp time.Time
	if claims.ExpiresAt != nil {
		exp = claims.ExpiresAt.Time
	} else {
		exp = time.Now().Add(24 * time.Hour)
	}
	auth.BlacklistToken(tokenString, exp)
	c.JSON(http.StatusOK, gin.H{"message": "已登出"})
}

// handleRegister 处理用户注册请求
func (s *Server) handleRegister(c *gin.Context) {
	// 管理员模式下禁用注册
	if auth.IsAdminMode() {
		c.JSON(http.StatusForbidden, gin.H{"error": "管理员模式下禁用注册"})
		return
	}

	// 若未开启注册，返回403
	allowRegStr, _ := s.database.GetSystemConfig("allow_registration")
	if allowRegStr == "false" {
		c.JSON(http.StatusForbidden, gin.H{"error": "注册已关闭"})
		return
	}

	var req struct {
		Email    string `json:"email" binding:"required,email"`
		Password string `json:"password" binding:"required,min=6"`
		BetaCode string `json:"beta_code"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 检查是否开启了内测模式
	betaModeStr, _ := s.database.GetSystemConfig("beta_mode")
	if betaModeStr == "true" {
		// 内测模式下必须提供有效的内测码
		if req.BetaCode == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "内测期间，注册需要提供内测码"})
			return
		}

		// 验证内测码
		isValid, err := s.database.ValidateBetaCode(req.BetaCode)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "验证内测码失败"})
			return
		}
		if !isValid {
			c.JSON(http.StatusBadRequest, gin.H{"error": "内测码无效或已被使用"})
			return
		}
	}

	// 检查邮箱是否已存在
	_, err := s.database.GetUserByEmail(req.Email)
	if err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "邮箱已被注册"})
		return
	}

	// 生成密码哈希
	passwordHash, err := auth.HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "密码处理失败"})
		return
	}

	// 生成OTP密钥
	otpSecret, err := auth.GenerateOTPSecret()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "OTP密钥生成失败"})
		return
	}

	// 创建用户（未验证OTP状态）
	userID := uuid.New().String()
	user := &config.User{
		ID:           userID,
		Email:        req.Email,
		PasswordHash: passwordHash,
		OTPSecret:    otpSecret,
		OTPVerified:  false,
		Role:         "user", // 注册的用户默认是user角色（只能管理自己的交易员）
	}

	err = s.database.CreateUser(user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建用户失败: " + err.Error()})
		return
	}

	// 如果是内测模式，标记内测码为已使用
	betaModeStr2, _ := s.database.GetSystemConfig("beta_mode")
	if betaModeStr2 == "true" && req.BetaCode != "" {
		err := s.database.UseBetaCode(req.BetaCode, req.Email)
		if err != nil {
			log.Printf("⚠️ 标记内测码为已使用失败: %v", err)
			// 这里不返回错误，因为用户已经创建成功
		} else {
			log.Printf("✓ 内测码 %s 已被用户 %s 使用", req.BetaCode, req.Email)
		}
	}

	// 返回OTP设置信息
	qrCodeURL := auth.GetOTPQRCodeURL(otpSecret, req.Email)
	c.JSON(http.StatusOK, gin.H{
		"user_id":     userID,
		"email":       req.Email,
		"otp_secret":  otpSecret,
		"qr_code_url": qrCodeURL,
		"message":     "请使用Google Authenticator扫描二维码并验证OTP",
	})
}

// handleCompleteRegistration 完成注册（验证OTP）
func (s *Server) handleCompleteRegistration(c *gin.Context) {
	var req struct {
		UserID  string `json:"user_id" binding:"required"`
		OTPCode string `json:"otp_code" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 获取用户信息
	user, err := s.database.GetUserByID(req.UserID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}

	// 验证OTP
	if !auth.VerifyOTP(user.OTPSecret, req.OTPCode) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "OTP验证码错误"})
		return
	}

	// 更新用户OTP验证状态
	err = s.database.UpdateUserOTPVerified(req.UserID, true)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新用户状态失败"})
		return
	}

	// 生成JWT token
	token, err := auth.GenerateJWT(user.ID, user.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成token失败"})
		return
	}

	// 初始化用户的默认模型和交易所配置
	err = s.initUserDefaultConfigs(user.ID)
	if err != nil {
		log.Printf("初始化用户默认配置失败: %v", err)
	}

	c.JSON(http.StatusOK, gin.H{
		"token":   token,
		"user_id": user.ID,
		"email":   user.Email,
		"message": "注册完成",
	})
}

// handleLogin 处理用户登录请求
func (s *Server) handleLogin(c *gin.Context) {
	var req struct {
		Email    string `json:"email" binding:"required,email"`
		Password string `json:"password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 获取用户信息
	user, err := s.database.GetUserByEmail(req.Email)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "邮箱或密码错误"})
		return
	}

	// 验证密码
	if !auth.CheckPassword(req.Password, user.PasswordHash) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "邮箱或密码错误"})
		return
	}

	// 获取用户角色（默认为user，向后兼容）
	role := user.Role
	if role == "" {
		role = "user" // 默认是普通用户
	}

	// 根据角色决定是否需要OTP验证
	if role == "admin" || role == "user" {
		// 管理员或普通用户（注册用户）：需要OTP验证
		if !user.OTPVerified {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":              "账户未完成OTP设置",
				"user_id":            user.ID,
				"requires_otp_setup": true,
			})
			return
		}

		// 返回需要OTP验证的状态
		c.JSON(http.StatusOK, gin.H{
			"user_id":      user.ID,
			"email":        user.Email,
			"message":      "请输入Google Authenticator验证码",
			"requires_otp": true,
		})
		return
	} else {
		// 创建的账号（group_leader 或 trader_account）：不需要OTP，直接登录
		// 这些账号由普通用户创建，不需要OTP验证
		token, err := auth.GenerateJWT(user.ID, user.Email)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "生成token失败"})
			return
		}

		// 为 trader_account 返回 trader_id
		responseData := gin.H{
			"token":   token,
			"user_id": user.ID,
			"email":   user.Email,
			"role":    role,
			"message": "登录成功",
		}

		if role == "trader_account" {
			responseData["trader_id"] = user.TraderID
		}

		c.JSON(http.StatusOK, responseData)
		return
	}
}

// handleVerifyOTP 验证OTP并完成登录
func (s *Server) handleVerifyOTP(c *gin.Context) {
	var req struct {
		UserID  string `json:"user_id" binding:"required"`
		OTPCode string `json:"otp_code" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 获取用户信息
	user, err := s.database.GetUserByID(req.UserID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}

	// 验证OTP
	if !auth.VerifyOTP(user.OTPSecret, req.OTPCode) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "验证码错误"})
		return
	}

	// 生成JWT token
	token, err := auth.GenerateJWT(user.ID, user.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成token失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token":   token,
		"user_id": user.ID,
		"email":   user.Email,
		"message": "登录成功",
	})
}

// handleResetPassword 重置密码（通过邮箱 + OTP 验证）
func (s *Server) handleResetPassword(c *gin.Context) {
	var req struct {
		Email       string `json:"email" binding:"required,email"`
		NewPassword string `json:"new_password" binding:"required,min=6"`
		OTPCode     string `json:"otp_code" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 查询用户
	user, err := s.database.GetUserByEmail(req.Email)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "邮箱不存在"})
		return
	}

	// 验证 OTP
	if !auth.VerifyOTP(user.OTPSecret, req.OTPCode) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Google Authenticator 验证码错误"})
		return
	}

	// 生成新密码哈希
	newPasswordHash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "密码处理失败"})
		return
	}

	// 更新密码
	err = s.database.UpdateUserPassword(user.ID, newPasswordHash)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "密码更新失败"})
		return
	}

	log.Printf("✓ 用户 %s 密码已重置", user.Email)
	c.JSON(http.StatusOK, gin.H{"message": "密码重置成功，请使用新密码登录"})
}

// initUserDefaultConfigs 为新用户初始化默认的模型和交易所配置
func (s *Server) initUserDefaultConfigs(userID string) error {
	// 注释掉自动创建默认配置，让用户手动添加
	// 这样新用户注册后不会自动有配置项
	log.Printf("用户 %s 注册完成，等待手动配置AI模型和交易所", userID)
	return nil
}

// handleGetSupportedModels 获取系统支持的AI模型列表
func (s *Server) handleGetSupportedModels(c *gin.Context) {
	// 返回系统支持的AI模型（从default用户获取）
	models, err := s.database.GetAIModels("default")
	if err != nil {
		log.Printf("❌ 获取支持的AI模型失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取支持的AI模型失败"})
		return
	}

	c.JSON(http.StatusOK, models)
}

// handleGetSupportedExchanges 获取系统支持的交易所列表
func (s *Server) handleGetSupportedExchanges(c *gin.Context) {
	// 返回系统支持的交易所（从default用户获取）
	exchanges, err := s.database.GetExchanges("default")
	if err != nil {
		log.Printf("❌ 获取支持的交易所失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取支持的交易所失败"})
		return
	}

	c.JSON(http.StatusOK, exchanges)
}

// Start 启动服务器
func (s *Server) Start() error {
	addr := fmt.Sprintf(":%d", s.port)
	log.Printf("🌐 API服务器启动在 http://localhost%s", addr)
	log.Printf("📊 API文档:")
	log.Printf("  • GET  /api/health           - 健康检查")
	log.Printf("  • GET  /api/traders          - 公开的AI交易员排行榜前50名（无需认证）")
	log.Printf("  • GET  /api/competition      - 公开的竞赛数据（无需认证）")
	log.Printf("  • GET  /api/top-traders      - 前5名交易员数据（无需认证，表现对比用）")
	log.Printf("  • GET  /api/equity-history?trader_id=xxx - 公开的收益率历史数据（无需认证，竞赛用）")
	log.Printf("  • GET  /api/equity-history-batch?trader_ids=a,b,c - 批量获取历史数据（无需认证，表现对比优化）")
	log.Printf("  • GET  /api/traders/:id/public-config - 公开的交易员配置（无需认证，不含敏感信息）")
	log.Printf("  • POST /api/traders          - 创建新的AI交易员")
	log.Printf("  • DELETE /api/traders/:id    - 删除AI交易员")
	log.Printf("  • POST /api/traders/:id/start - 启动AI交易员")
	log.Printf("  • POST /api/traders/:id/stop  - 停止AI交易员")
	log.Printf("  • GET  /api/models           - 获取AI模型配置")
	log.Printf("  • PUT  /api/models           - 更新AI模型配置")
	log.Printf("  • GET  /api/exchanges        - 获取交易所配置")
	log.Printf("  • PUT  /api/exchanges        - 更新交易所配置")
	log.Printf("  • GET  /api/status?trader_id=xxx     - 指定trader的系统状态")
	log.Printf("  • GET  /api/account?trader_id=xxx    - 指定trader的账户信息")
	log.Printf("  • GET  /api/positions?trader_id=xxx  - 指定trader的持仓列表")
	log.Printf("  • GET  /api/decisions?trader_id=xxx  - 指定trader的决策日志")
	log.Printf("  • GET  /api/decisions/latest?trader_id=xxx - 指定trader的最新决策")
	log.Printf("  • GET  /api/statistics?trader_id=xxx - 指定trader的统计信息")
	log.Printf("  • GET  /api/performance?trader_id=xxx - 指定trader的AI学习表现分析")
	log.Println()

	return s.router.Run(addr)
}

// handleGetPromptTemplates 获取所有系统提示词模板列表
func (s *Server) handleGetPromptTemplates(c *gin.Context) {
	// 导入 decision 包
	templates := decision.GetAllPromptTemplates()

	// 转换为响应格式
	response := make([]map[string]interface{}, 0, len(templates))
	for _, tmpl := range templates {
		response = append(response, map[string]interface{}{
			"name": tmpl.Name,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"templates": response,
	})
}

// handleGetPromptTemplate 获取指定名称的提示词模板内容
func (s *Server) handleGetPromptTemplate(c *gin.Context) {
	templateName := c.Param("name")

	template, err := decision.GetPromptTemplate(templateName)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("模板不存在: %s", templateName)})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"name":    template.Name,
		"content": template.Content,
	})
}

// handlePublicTraderList 获取公开的交易员列表（无需认证）
func (s *Server) handlePublicTraderList(c *gin.Context) {
	// 从所有用户获取交易员信息
	competition, err := s.traderManager.GetCompetitionData()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("获取交易员列表失败: %v", err),
		})
		return
	}

	// 获取traders数组
	tradersData, exists := competition["traders"]
	if !exists {
		c.JSON(http.StatusOK, []map[string]interface{}{})
		return
	}

	traders, ok := tradersData.([]map[string]interface{})
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "交易员数据格式错误",
		})
		return
	}

	// 返回交易员基本信息，过滤敏感信息
	result := make([]map[string]interface{}, 0, len(traders))
	for _, trader := range traders {
		result = append(result, map[string]interface{}{
			"trader_id":       trader["trader_id"],
			"trader_name":     trader["trader_name"],
			"ai_model":        trader["ai_model"],
			"exchange":        trader["exchange"],
			"is_running":      trader["is_running"],
			"total_equity":    trader["total_equity"],
			"total_pnl":       trader["total_pnl"],
			"total_pnl_pct":   trader["total_pnl_pct"],
			"position_count":  trader["position_count"],
			"margin_used_pct": trader["margin_used_pct"],
		})
	}

	c.JSON(http.StatusOK, result)
}

// handlePublicCompetition 获取公开的竞赛数据（无需认证）
func (s *Server) handlePublicCompetition(c *gin.Context) {
	competition, err := s.traderManager.GetCompetitionData()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("获取竞赛数据失败: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, competition)
}

// handleTopTraders 获取前5名交易员数据（无需认证，用于表现对比）
func (s *Server) handleTopTraders(c *gin.Context) {
	topTraders, err := s.traderManager.GetTopTradersData()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("获取前10名交易员数据失败: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, topTraders)
}

// handleEquityHistoryBatch 批量获取多个交易员的收益率历史数据（无需认证，用于表现对比）
func (s *Server) handleEquityHistoryBatch(c *gin.Context) {
	var requestBody struct {
		TraderIDs []string `json:"trader_ids"`
	}

	// 尝试解析POST请求的JSON body
	if err := c.ShouldBindJSON(&requestBody); err != nil {
		// 如果JSON解析失败，尝试从query参数获取（兼容GET请求）
		traderIDsParam := c.Query("trader_ids")
		if traderIDsParam == "" {
			// 如果没有指定trader_ids，则返回前5名的历史数据
			topTraders, err := s.traderManager.GetTopTradersData()
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": fmt.Sprintf("获取前5名交易员失败: %v", err),
				})
				return
			}

			traders, ok := topTraders["traders"].([]map[string]interface{})
			if !ok {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "交易员数据格式错误"})
				return
			}

			// 提取trader IDs
			traderIDs := make([]string, 0, len(traders))
			for _, trader := range traders {
				if traderID, ok := trader["trader_id"].(string); ok {
					traderIDs = append(traderIDs, traderID)
				}
			}

			result := s.getEquityHistoryForTraders(traderIDs)
			c.JSON(http.StatusOK, result)
			return
		}

		// 解析逗号分隔的trader IDs
		requestBody.TraderIDs = strings.Split(traderIDsParam, ",")
		for i := range requestBody.TraderIDs {
			requestBody.TraderIDs[i] = strings.TrimSpace(requestBody.TraderIDs[i])
		}
	}

	// 限制最多20个交易员，防止请求过大
	if len(requestBody.TraderIDs) > 20 {
		requestBody.TraderIDs = requestBody.TraderIDs[:20]
	}

	result := s.getEquityHistoryForTraders(requestBody.TraderIDs)
	c.JSON(http.StatusOK, result)
}

// getEquityHistoryForTraders 获取多个交易员的历史数据
func (s *Server) getEquityHistoryForTraders(traderIDs []string) map[string]interface{} {
	result := make(map[string]interface{})
	histories := make(map[string]interface{})
	errors := make(map[string]string)

	for _, traderID := range traderIDs {
		if traderID == "" {
			continue
		}

		trader, err := s.traderManager.GetTrader(traderID)
		if err != nil {
			errors[traderID] = "交易员不存在"
			continue
		}

		// 获取历史数据（用于对比展示，限制数据量）
		records, err := trader.GetDecisionLogger().GetLatestRecords(500)
		if err != nil {
			errors[traderID] = fmt.Sprintf("获取历史数据失败: %v", err)
			continue
		}

		// 构建收益率历史数据
		history := make([]map[string]interface{}, 0, len(records))
		for _, record := range records {
			// 计算总权益（余额+未实现盈亏）
			totalEquity := record.AccountState.TotalBalance + record.AccountState.TotalUnrealizedProfit

			history = append(history, map[string]interface{}{
				"timestamp":    record.Timestamp,
				"total_equity": totalEquity,
				"total_pnl":    record.AccountState.TotalUnrealizedProfit,
				"balance":      record.AccountState.TotalBalance,
			})
		}

		histories[traderID] = history
	}

	result["histories"] = histories
	result["count"] = len(histories)
	if len(errors) > 0 {
		result["errors"] = errors
	}

	return result
}

// handleGetPublicTraderConfig 获取公开的交易员配置信息（无需认证，不包含敏感信息）
func (s *Server) handleGetPublicTraderConfig(c *gin.Context) {
	traderID := c.Param("id")
	if traderID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "交易员ID不能为空"})
		return
	}

	trader, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "交易员不存在"})
		return
	}

	// 获取交易员的状态信息
	status := trader.GetStatus()

	// 只返回公开的配置信息，不包含API密钥等敏感数据
	result := map[string]interface{}{
		"trader_id":   trader.GetID(),
		"trader_name": trader.GetName(),
		"ai_model":    trader.GetAIModel(),
		"exchange":    trader.GetExchange(),
		"is_running":  status["is_running"],
		"ai_provider": status["ai_provider"],
		"start_time":  status["start_time"],
	}

	c.JSON(http.StatusOK, result)
}

// generateRandomEmail 生成随机邮箱
func generateRandomEmail() string {
	randomStr := uuid.New().String()[:8]
	return fmt.Sprintf("trader_%s@nofx.local", randomStr)
}

// generateRandomPassword 生成随机密码
func generateRandomPassword(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}

// handleCreateTraderAccount 创建交易员账号
func (s *Server) handleCreateTraderAccount(c *gin.Context) {
	userID := c.GetString("user_id")
	traderID := c.Param("id")

	// 检查用户角色（必须是admin或user）
	user, err := s.database.GetUserByID(userID)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户不存在"})
		return
	}

	if user.Role != "admin" && user.Role != "user" {
		c.JSON(http.StatusForbidden, gin.H{"error": "权限不足"})
		return
	}

	// 验证交易员是否存在
	trader, err := s.database.GetTraderByID(traderID)
	if err != nil || trader == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "交易员不存在"})
		return
	}

	// 如果不是admin，验证交易员是否属于当前用户
	if user.Role != "admin" && trader.OwnerUserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "只能为自己的交易员创建账号"})
		return
	}

	// 检查交易员是否已有账号
	if trader.TraderAccountID != "" {
		// 检查账号是否仍然存在
		account, _ := s.database.GetUserByID(trader.TraderAccountID)
		if account != nil {
			c.JSON(http.StatusConflict, gin.H{
				"error":         "交易员已有账号",
				"account_id":    trader.TraderAccountID,
				"account_email": account.Email,
			})
			return
		}
		// 如果账号不存在，清除关联，允许重新创建
		s.database.UpdateTraderAccountID(traderID, "")
	}

	var req struct {
		GenerateRandomEmail    bool   `json:"generate_random_email"`
		GenerateRandomPassword bool   `json:"generate_random_password"`
		Email                  string `json:"email"`
		Password               string `json:"password"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 验证必填字段
	if !req.GenerateRandomEmail && req.Email == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "账号未选择随机生成时，必须提供email"})
		return
	}
	if !req.GenerateRandomPassword && req.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "密码未选择随机生成时，必须提供password"})
		return
	}

	// 根据四种组合模式生成账号信息
	var accountEmail, accountPassword string

	// 1. 账号处理：随机生成或使用输入的
	if req.GenerateRandomEmail {
		// 随机生成邮箱，检查是否已存在
		maxRetries := 10
		for i := 0; i < maxRetries; i++ {
			accountEmail = generateRandomEmail()
			existing, _ := s.database.GetUserByEmail(accountEmail)
			if existing == nil {
				break // 邮箱可用
			}
		}
		if accountEmail == "" {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "无法生成唯一邮箱，请重试"})
			return
		}
	} else {
		accountEmail = req.Email
		// 检查邮箱是否已存在
		existing, _ := s.database.GetUserByEmail(accountEmail)
		if existing != nil {
			c.JSON(http.StatusConflict, gin.H{"error": "邮箱已存在"})
			return
		}
	}

	// 2. 密码处理：随机生成或使用输入的
	if req.GenerateRandomPassword {
		accountPassword = generateRandomPassword(12) // 12位随机密码
	} else {
		accountPassword = req.Password
	}

	// 创建用户（trader_account角色）
	passwordHash, err := auth.HashPassword(accountPassword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "密码处理失败"})
		return
	}

	newUserID := uuid.New().String()
	newUser := &config.User{
		ID:           newUserID,
		Email:        accountEmail,
		PasswordHash: passwordHash,
		Role:         "trader_account",
		TraderID:     traderID,
		Category:     trader.Category, // 自动继承交易员的分类
		OTPSecret:    "",              // 不需要OTP
		OTPVerified:  true,            // 直接设置为已验证（跳过OTP）
	}

	err = s.database.CreateUser(newUser)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建用户失败: " + err.Error()})
		return
	}

	// 更新交易员的trader_account_id
	err = s.database.UpdateTraderAccountID(traderID, newUserID)
	if err != nil {
		log.Printf("⚠️ 更新交易员账号ID失败: %v", err)
	}

	c.JSON(http.StatusOK, gin.H{
		"user_id":   newUserID,
		"email":     accountEmail,
		"password":  accountPassword, // 返回密码（仅此一次）
		"role":      "trader_account",
		"trader_id": traderID,
	})
}

// handleUpdateTraderAccountPassword 更新交易员账号密码
func (s *Server) handleUpdateTraderAccountPassword(c *gin.Context) {
	userID := c.GetString("user_id")
	traderID := c.Param("id")

	// 检查用户角色（必须是admin或user）
	user, err := s.database.GetUserByID(userID)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户不存在"})
		return
	}

	if user.Role != "admin" && user.Role != "user" {
		c.JSON(http.StatusForbidden, gin.H{"error": "权限不足"})
		return
	}

	// 验证交易员是否存在
	trader, err := s.database.GetTraderByID(traderID)
	if err != nil || trader == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "交易员不存在"})
		return
	}

	// 如果不是admin，验证交易员是否属于当前用户
	// trader_account用户只能修改自己的账号密码
	if user.Role == "trader_account" {
		if trader.TraderAccountID != userID {
			c.JSON(http.StatusForbidden, gin.H{"error": "只能修改自己的账号密码"})
			return
		}
	} else if user.Role != "admin" && trader.OwnerUserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "只能修改自己交易员的账号密码"})
		return
	}

	// 检查交易员是否有账号
	if trader.TraderAccountID == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "交易员还没有账号"})
		return
	}

	var req struct {
		Password string `json:"password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "密码不能为空"})
		return
	}

	// 更新密码
	passwordHash, err := auth.HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "密码处理失败"})
		return
	}

	err = s.database.UpdateUserPassword(trader.TraderAccountID, passwordHash)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "密码更新失败: " + err.Error()})
		return
	}

	log.Printf("✓ 交易员 %s 的账号密码已更新", traderID)
	c.JSON(http.StatusOK, gin.H{
		"message":  "密码更新成功",
		"password": req.Password, // 返回新密码（前端需要保存）
	})
}

// handleCreateGroupLeader 创建小组组长账号
func (s *Server) handleCreateGroupLeader(c *gin.Context) {
	userID := c.GetString("user_id")

	// 检查用户角色（必须是admin或user）
	user, err := s.database.GetUserByID(userID)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户不存在"})
		return
	}

	if user.Role != "admin" && user.Role != "user" {
		c.JSON(http.StatusForbidden, gin.H{"error": "权限不足"})
		return
	}

	var req struct {
		GenerateRandomEmail    bool     `json:"generate_random_email"`
		GenerateRandomPassword bool     `json:"generate_random_password"`
		Email                  string   `json:"email"`
		Password               string   `json:"password"`
		Categories             []string `json:"categories" binding:"required"` // 必填：可以观测的分类列表
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 验证必填字段
	if !req.GenerateRandomEmail && req.Email == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "账号未选择随机生成时，必须提供email"})
		return
	}
	if !req.GenerateRandomPassword && req.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "密码未选择随机生成时，必须提供password"})
		return
	}

	// 如果不是admin，验证分类是否属于当前用户
	if user.Role != "admin" {
		userCategories, _ := s.database.GetUserCategories(userID)
		for _, cat := range req.Categories {
			if !contains(userCategories, cat) {
				c.JSON(http.StatusForbidden, gin.H{"error": "只能为自己的分类创建小组组长"})
				return
			}
		}
	}

	// 根据四种组合模式生成账号信息
	var accountEmail, accountPassword string

	// 1. 账号处理：随机生成或使用输入的
	if req.GenerateRandomEmail {
		// 随机生成邮箱，检查是否已存在
		maxRetries := 10
		for i := 0; i < maxRetries; i++ {
			accountEmail = generateRandomEmail()
			existing, _ := s.database.GetUserByEmail(accountEmail)
			if existing == nil {
				break // 邮箱可用
			}
		}
		if accountEmail == "" {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "无法生成唯一邮箱，请重试"})
			return
		}
	} else {
		accountEmail = req.Email
		// 检查邮箱是否已存在
		existing, _ := s.database.GetUserByEmail(accountEmail)
		if existing != nil {
			c.JSON(http.StatusConflict, gin.H{"error": "邮箱已存在"})
			return
		}
	}

	// 2. 密码处理：随机生成或使用输入的
	if req.GenerateRandomPassword {
		accountPassword = generateRandomPassword(12) // 12位随机密码
	} else {
		accountPassword = req.Password
	}

	// 创建用户（group_leader角色）
	passwordHash, err := auth.HashPassword(accountPassword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "密码处理失败"})
		return
	}

	newUserID := uuid.New().String()
	newUser := &config.User{
		ID:           newUserID,
		Email:        accountEmail,
		PasswordHash: passwordHash,
		Role:         "group_leader",
		OTPSecret:    "",   // 不需要OTP
		OTPVerified:  true, // 直接设置为已验证（跳过OTP）
	}

	err = s.database.CreateUser(newUser)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建用户失败: " + err.Error()})
		return
	}

	// 关联分类（group_leader_categories表）
	// 注意：owner_user_id必须设置为创建者的用户ID，确保数据隔离
	for _, cat := range req.Categories {
		err := s.database.InsertGroupLeaderCategory(newUserID, cat, userID) // 第三个参数是owner_user_id
		if err != nil {
			// 如果关联失败，回滚用户创建
			s.database.DeleteUser(newUserID)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "创建小组组长失败"})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"user_id":    newUserID,
		"email":      accountEmail,
		"password":   accountPassword, // 返回密码（仅此一次）
		"role":       "group_leader",
		"categories": req.Categories,
	})
}

// CreateGroupLeaderForCategoryRequest 为特定分类创建小组组长账号的请求
type CreateGroupLeaderForCategoryRequest struct {
	GenerateRandomEmail    *bool  `json:"generate_random_email,omitempty"`
	GenerateRandomPassword *bool  `json:"generate_random_password,omitempty"`
	Email                  string `json:"email,omitempty"`
	Password               string `json:"password,omitempty"`
	Category               string `json:"category"` // 单个分类
}

// handleCreateGroupLeaderForCategory 为特定分类创建小组组长账号
func (s *Server) handleCreateGroupLeaderForCategory(c *gin.Context) {
	userID := c.GetString("user_id")

	var req CreateGroupLeaderForCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求参数"})
		return
	}

	// 验证分类是否存在且属于当前用户
	cat, err := s.database.GetCategoryByName(req.Category)
	if err != nil || cat == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "分类不存在"})
		return
	}
	if cat.OwnerUserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "只能为自己的分类创建小组组长"})
		return
	}

	// 生成账号和密码
	accountEmail := req.Email
	accountPassword := req.Password

	if req.GenerateRandomEmail == nil || *req.GenerateRandomEmail {
		// 生成随机邮箱
		randomSuffix := fmt.Sprintf("%d", time.Now().UnixNano()%1000000)
		accountEmail = fmt.Sprintf("groupleader_%s@random.local", randomSuffix)
	}

	if req.GenerateRandomPassword == nil || *req.GenerateRandomPassword {
		// 生成随机密码
		const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*"
		passwordBytes := make([]byte, 12)
		for i := range passwordBytes {
			passwordBytes[i] = charset[rand.Intn(len(charset))]
		}
		accountPassword = string(passwordBytes)
	}

	// 创建用户（group_leader角色）
	passwordHash, err := auth.HashPassword(accountPassword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "密码处理失败"})
		return
	}

	newUserID := uuid.New().String()
	newUser := &config.User{
		ID:           newUserID,
		Email:        accountEmail,
		PasswordHash: passwordHash,
		Role:         "group_leader",
		OTPSecret:    "",   // 不需要OTP
		OTPVerified:  true, // 直接设置为已验证（跳过OTP）
	}

	err = s.database.CreateUser(newUser)
	if err != nil {
		log.Printf("创建用户失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建账号失败"})
		return
	}

	// 关联分类（group_leader_categories表）
	err = s.database.InsertGroupLeaderCategory(newUserID, req.Category, userID)
	if err != nil {
		log.Printf("关联分类失败: %v", err)
		// 清理已创建的用户
		s.database.DeleteUser(newUserID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建小组组长失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user_id":    newUserID,
		"email":      accountEmail,
		"password":   accountPassword, // 返回密码（仅此一次）
		"role":       "group_leader",
		"categories": []string{req.Category},
	})
}

// handleGetTraderAccount 获取交易员的账号信息
func (s *Server) handleGetTraderAccount(c *gin.Context) {
	userID := c.GetString("user_id")
	traderID := c.Param("id")

	// 检查用户角色（必须是admin、user或trader_account）
	user, err := s.database.GetUserByID(userID)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户不存在"})
		return
	}

	if user.Role != "admin" && user.Role != "user" && user.Role != "trader_account" {
		c.JSON(http.StatusForbidden, gin.H{"error": "权限不足"})
		return
	}

	// 验证交易员是否存在
	trader, err := s.database.GetTraderByID(traderID)
	if err != nil || trader == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "交易员不存在"})
		return
	}

	// 如果不是admin，验证交易员是否属于当前用户
	// trader_account用户只能查看自己的账号信息
	if user.Role == "trader_account" {
		if trader.TraderAccountID != userID {
			c.JSON(http.StatusForbidden, gin.H{"error": "只能查看自己的账号信息"})
			return
		}
	} else if user.Role != "admin" && trader.OwnerUserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "只能查看自己交易员的账号信息"})
		return
	}

	// 检查交易员是否有账号
	if trader.TraderAccountID == "" {
		c.JSON(http.StatusOK, gin.H{"account": nil})
		return
	}

	// 获取账号信息
	account, err := s.database.GetUserByID(trader.TraderAccountID)
	if err != nil || account == nil {
		// 账号不存在，清除关联
		s.database.UpdateTraderAccountID(traderID, "")
		c.JSON(http.StatusOK, gin.H{"account": nil})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"account": gin.H{
			"user_id":    account.ID,
			"email":      account.Email,
			"created_at": account.CreatedAt,
		},
	})
}

// handleDeleteTraderAccount 删除交易员的账号
func (s *Server) handleDeleteTraderAccount(c *gin.Context) {
	userID := c.GetString("user_id")
	traderID := c.Param("id")

	// 检查用户角色（必须是admin或user）
	user, err := s.database.GetUserByID(userID)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户不存在"})
		return
	}

	if user.Role != "admin" && user.Role != "user" {
		c.JSON(http.StatusForbidden, gin.H{"error": "权限不足"})
		return
	}

	// 验证交易员是否存在
	trader, err := s.database.GetTraderByID(traderID)
	if err != nil || trader == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "交易员不存在"})
		return
	}

	// 如果不是admin，验证交易员是否属于当前用户
	if user.Role != "admin" && trader.OwnerUserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "只能删除自己交易员的账号"})
		return
	}

	// 检查交易员是否有账号
	if trader.TraderAccountID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "交易员没有账号"})
		return
	}

	// 删除账号
	err = s.database.DeleteUser(trader.TraderAccountID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除账号失败: " + err.Error()})
		return
	}

	// 清除交易员的账号关联
	err = s.database.UpdateTraderAccountID(traderID, "")
	if err != nil {
		log.Printf("⚠️ 清除交易员账号关联失败: %v", err)
	}

	c.JSON(http.StatusOK, gin.H{"message": "账号已删除"})
}

// handleGetGroupLeaders 获取小组组长列表
func (s *Server) handleGetGroupLeaders(c *gin.Context) {
	userID := c.GetString("user_id")

	// 获取用户角色
	user, err := s.database.GetUserByID(userID)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户不存在"})
		return
	}

	role := user.Role
	if role == "" {
		role = "user" // 默认是普通用户
	}

	// 获取所有小组组长
	allUsers, err := s.database.GetAllUsers()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取用户列表失败"})
		return
	}

	var leaders []gin.H
	for _, uid := range allUsers {
		u, err := s.database.GetUserByID(uid)
		if err != nil || u.Role != "group_leader" {
			continue
		}

		// 如果不是admin，只返回当前用户创建的小组组长
		if role != "admin" {
			// 检查该小组组长是否由当前用户创建（通过group_leader_categories表的owner_user_id）
			categories, _ := s.database.GetGroupLeaderCategories(uid)
			isCreatedByUser := false
			for _, cat := range categories {
				category, _ := s.database.GetCategoryByName(cat)
				if category != nil && category.OwnerUserID == userID {
					isCreatedByUser = true
					break
				}
			}
			if !isCreatedByUser {
				continue
			}
		}

		// 获取该小组组长管理的分类
		categories, _ := s.database.GetGroupLeaderCategories(uid)

		// 统计该小组组长可以查看的交易员数量
		traders, _ := s.database.GetTradersByCategories(categories)
		traderCount := len(traders)

		leaders = append(leaders, gin.H{
			"id":           u.ID,
			"email":        u.Email,
			"role":         u.Role,
			"categories":   categories,
			"trader_count": traderCount,
			"created_at":   u.CreatedAt,
		})
	}

	c.JSON(http.StatusOK, gin.H{"leaders": leaders})
}

// handleUpdateGroupLeaderCategories 更新小组组长的分类
func (s *Server) handleUpdateGroupLeaderCategories(c *gin.Context) {
	userID := c.GetString("user_id")
	groupLeaderID := c.Param("id")

	// 检查用户角色（必须是admin或user）
	user, err := s.database.GetUserByID(userID)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户不存在"})
		return
	}

	if user.Role != "admin" && user.Role != "user" {
		c.JSON(http.StatusForbidden, gin.H{"error": "权限不足"})
		return
	}

	// 验证小组组长是否存在
	groupLeader, err := s.database.GetUserByID(groupLeaderID)
	if err != nil || groupLeader == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "小组组长不存在"})
		return
	}

	if groupLeader.Role != "group_leader" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "该用户不是小组组长"})
		return
	}

	// 如果不是admin，验证小组组长是否由当前用户创建
	if user.Role != "admin" {
		existingCategories, _ := s.database.GetGroupLeaderCategories(groupLeaderID)
		isCreatedByUser := false
		for _, cat := range existingCategories {
			category, _ := s.database.GetCategoryByName(cat)
			if category != nil && category.OwnerUserID == userID {
				isCreatedByUser = true
				break
			}
		}
		if !isCreatedByUser {
			c.JSON(http.StatusForbidden, gin.H{"error": "只能更新自己创建的小组组长的分类"})
			return
		}
	}

	var req struct {
		Categories []string `json:"categories" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 如果不是admin，验证分类是否属于当前用户
	if user.Role != "admin" {
		userCategories, _ := s.database.GetUserCategories(userID)
		for _, cat := range req.Categories {
			if !contains(userCategories, cat) {
				c.JSON(http.StatusForbidden, gin.H{"error": "只能使用自己的分类"})
				return
			}
		}
	}

	// 删除现有的分类关联
	err = s.database.DeleteGroupLeaderCategories(groupLeaderID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新分类失败: " + err.Error()})
		return
	}

	// 添加新的分类关联
	for _, cat := range req.Categories {
		ownerUserID := userID
		if user.Role == "admin" {
			// Admin可以设置任何分类，需要找到分类的所有者
			category, _ := s.database.GetCategoryByName(cat)
			if category != nil {
				ownerUserID = category.OwnerUserID
			}
		}
		err := s.database.InsertGroupLeaderCategory(groupLeaderID, cat, ownerUserID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "更新分类失败: " + err.Error()})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message":    "分类已更新",
		"categories": req.Categories,
	})
}

// handleDeleteGroupLeader 删除小组组长账号
func (s *Server) handleDeleteGroupLeader(c *gin.Context) {
	userID := c.GetString("user_id")
	groupLeaderID := c.Param("id")

	// 检查用户角色（必须是admin或user）
	user, err := s.database.GetUserByID(userID)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户不存在"})
		return
	}

	if user.Role != "admin" && user.Role != "user" {
		c.JSON(http.StatusForbidden, gin.H{"error": "权限不足"})
		return
	}

	// 验证小组组长是否存在
	groupLeader, err := s.database.GetUserByID(groupLeaderID)
	if err != nil || groupLeader == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "小组组长不存在"})
		return
	}

	if groupLeader.Role != "group_leader" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "该用户不是小组组长"})
		return
	}

	// 如果不是admin，验证小组组长是否由当前用户创建
	if user.Role != "admin" {
		categories, _ := s.database.GetGroupLeaderCategories(groupLeaderID)
		isCreatedByUser := false
		for _, cat := range categories {
			category, _ := s.database.GetCategoryByName(cat)
			if category != nil && category.OwnerUserID == userID {
				isCreatedByUser = true
				break
			}
		}
		if !isCreatedByUser {
			c.JSON(http.StatusForbidden, gin.H{"error": "只能删除自己创建的小组组长"})
			return
		}
	}

	// 删除账号（会自动删除group_leader_categories中的关联，通过FOREIGN KEY CASCADE）
	err = s.database.DeleteUser(groupLeaderID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除小组组长失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "小组组长已删除"})
}

// handleGetCategories 获取分类列表
func (s *Server) handleGetCategories(c *gin.Context) {
	userID := c.GetString("user_id")

	// 获取用户角色
	user, err := s.database.GetUserByID(userID)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户不存在"})
		return
	}

	var categories []*config.Category
	if user.Role == "admin" {
		// Admin可以查看所有分类（特殊角色，一般不使用）
		// 这里简化处理，只返回当前用户的分类
		categories, _ = s.database.GetCategoriesByOwner(userID)
	} else {
		// User只能查看自己创建的分类
		categories, _ = s.database.GetCategoriesByOwner(userID)
	}

	c.JSON(http.StatusOK, gin.H{"categories": categories})
}

// handleCreateCategory 创建分类
func (s *Server) handleCreateCategory(c *gin.Context) {
	userID := c.GetString("user_id")

	// 检查用户角色（必须是admin或user）
	user, err := s.database.GetUserByID(userID)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户不存在"})
		return
	}

	if user.Role != "admin" && user.Role != "user" {
		c.JSON(http.StatusForbidden, gin.H{"error": "权限不足"})
		return
	}

	var req struct {
		Name        string `json:"name" binding:"required"`
		Description string `json:"description"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 检查分类名称是否已存在（同一用户下）
	existing, _ := s.database.GetCategoryByNameAndOwner(req.Name, userID)
	if existing != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "分类名称已存在"})
		return
	}

	category, err := s.database.CreateCategory(userID, req.Name, req.Description)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建分类失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, category)
}

// handleUpdateCategory 更新分类信息
func (s *Server) handleUpdateCategory(c *gin.Context) {
	userID := c.GetString("user_id")
	categoryIDStr := c.Param("id")
	categoryID, err := strconv.Atoi(categoryIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的分类ID"})
		return
	}

	// 获取用户角色
	user, err := s.database.GetUserByID(userID)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户不存在"})
		return
	}

	// 获取分类信息
	category, err := s.database.GetCategoryByID(categoryID)
	if err != nil || category == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "分类不存在"})
		return
	}

	// 权限检查：如果不是admin，验证分类是否属于当前用户
	if user.Role != "admin" && category.OwnerUserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "只能修改自己的分类"})
		return
	}

	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 如果修改了名称，检查新名称是否已存在（同一用户下）
	if req.Name != "" && req.Name != category.Name {
		existing, _ := s.database.GetCategoryByNameAndOwner(req.Name, userID)
		if existing != nil && existing.ID != categoryID {
			c.JSON(http.StatusConflict, gin.H{"error": "分类名称已存在"})
			return
		}
	}

	// 更新分类
	err = s.database.UpdateCategory(categoryID, req.Name, req.Description)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新分类失败: " + err.Error()})
		return
	}

	// 返回更新后的分类信息
	updatedCategory, _ := s.database.GetCategoryByID(categoryID)
	c.JSON(http.StatusOK, updatedCategory)
}

// handleDeleteCategory 删除分类
func (s *Server) handleDeleteCategory(c *gin.Context) {
	userID := c.GetString("user_id")
	categoryIDStr := c.Param("id")
	categoryID, err := strconv.Atoi(categoryIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的分类ID"})
		return
	}

	// 获取用户角色
	user, err := s.database.GetUserByID(userID)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户不存在"})
		return
	}

	// 获取分类信息
	category, err := s.database.GetCategoryByID(categoryID)
	if err != nil || category == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "分类不存在"})
		return
	}

	// 权限检查：如果不是admin，验证分类是否属于当前用户
	if user.Role != "admin" && category.OwnerUserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "只能删除自己的分类"})
		return
	}

	categoryName := category.Name

	// 1. 将该分类下的所有交易员的category设为空字符串
	err = s.database.UpdateTradersCategoryToEmpty(categoryName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新交易员分类失败"})
		return
	}

	// 2. 删除分类（会自动删除group_leader_categories中的关联，通过FOREIGN KEY CASCADE）
	err = s.database.DeleteCategory(categoryID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除分类失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":       "分类删除成功",
		"category_name": categoryName,
	})
}

// handleSetTraderCategory 设置交易员分类
func (s *Server) handleSetTraderCategory(c *gin.Context) {
	userID := c.GetString("user_id")
	traderID := c.Param("id")

	// 检查用户角色（必须是admin或user）
	user, err := s.database.GetUserByID(userID)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户不存在"})
		return
	}

	if user.Role != "admin" && user.Role != "user" {
		c.JSON(http.StatusForbidden, gin.H{"error": "权限不足"})
		return
	}

	var req struct {
		Category string `json:"category"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 验证交易员是否存在
	trader, err := s.database.GetTraderByID(traderID)
	if err != nil || trader == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "交易员不存在"})
		return
	}

	// 如果不是admin，验证交易员是否属于当前用户
	if user.Role != "admin" && trader.OwnerUserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "只能设置自己交易员的分类"})
		return
	}

	// 如果提供了分类，验证分类是否属于当前用户
	if req.Category != "" {
		if user.Role != "admin" {
			category, err := s.database.GetCategoryByName(req.Category)
			if err != nil || category == nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "分类不存在"})
				return
			}
			if category.OwnerUserID != userID {
				c.JSON(http.StatusForbidden, gin.H{"error": "只能使用自己的分类"})
				return
			}
		}
	}

	// 更新交易员分类
	err = s.database.UpdateTraderCategory(traderID, req.Category)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新交易员分类失败: " + err.Error()})
		return
	}

	// 验证更新是否成功
	updatedTrader, err := s.database.GetTraderByID(traderID)
	if err == nil && updatedTrader != nil {
		log.Printf("[handleSetTraderCategory] Updated trader: ID=%s, Category=%s, OwnerUserID=%s",
			updatedTrader.ID, updatedTrader.Category, updatedTrader.OwnerUserID)
	}

	c.JSON(http.StatusOK, gin.H{"message": "交易员分类已更新"})
}

// handleGetCategoryAccounts 获取分类账号列表
// 新逻辑：先获取分类下的交易员，然后通过 trader_account_id 找到对应的账号
func (s *Server) handleGetCategoryAccounts(c *gin.Context) {
	userID := c.GetString("user_id")

	// 获取用户角色
	user, err := s.database.GetUserByID(userID)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户不存在"})
		return
	}

	role := user.Role
	if role == "" {
		role = "user" // 默认是普通用户
	}

	var accounts []gin.H
	var visibleCategories []string

	// 根据用户角色确定可见的分类
	if role == "admin" {
		// 管理员可以看到所有分类
		allCategories, _ := s.database.GetAllCategories()
		for _, cat := range allCategories {
			visibleCategories = append(visibleCategories, cat.Name)
		}
	} else if role == "group_leader" {
		// 小组组长可以看到自己管理的分类
		visibleCategories, _ = s.database.GetGroupLeaderCategories(userID)
	} else if role == "user" {
		// 普通用户可以看到自己创建的分类
		allCategories, _ := s.database.GetAllCategories()
		for _, cat := range allCategories {
			if cat != nil && cat.OwnerUserID == userID {
				visibleCategories = append(visibleCategories, cat.Name)
			}
		}
	}

	// 1. 先获取所有可见分类下的交易员（traders）
	traders, _ := s.database.GetTradersByCategories(visibleCategories)
	log.Printf("📊 找到 %d 个交易员，分类: %v", len(traders), visibleCategories)

	// 2. 通过每个 trader 的 trader_account_id 找到对应的账号
	traderAccountMap := make(map[string]*config.TraderRecord) // trader_account_id -> trader
	for _, trader := range traders {
		if trader.TraderAccountID != "" {
			traderAccountMap[trader.TraderAccountID] = trader
		}
	}

	// 3. 获取这些账号的用户信息
	for accountID, trader := range traderAccountMap {
		accountUser, err := s.database.GetUserByID(accountID)
		if err != nil || accountUser == nil {
			log.Printf("⚠️ 找不到账号: account_id=%s, trader_id=%s", accountID, trader.ID)
			continue
		}

		accounts = append(accounts, gin.H{
			"id":         accountUser.ID,
			"email":      accountUser.Email,
			"role":       "trader_account", // 明确标记为交易员账号
			"category":   trader.Category,
			"trader_id":  trader.ID,
			"created_at": accountUser.CreatedAt,
		})
		log.Printf("✅ 找到交易员账号: email=%s, trader_id=%s, category=%s", accountUser.Email, trader.ID, trader.Category)
	}

	// 4. 获取小组组长账号（通过 group_leader_categories 表）
	for _, categoryName := range visibleCategories {
		// 查找管理这个分类的小组组长
		allUsers, _ := s.database.GetAllUsers()
		for _, uid := range allUsers {
			u, err := s.database.GetUserByID(uid)
			if err != nil || u.Role != "group_leader" {
				continue
			}

			// 检查这个小组组长是否管理当前分类
			categories, _ := s.database.GetGroupLeaderCategories(uid)
			for _, cat := range categories {
				if cat == categoryName {
					// 检查权限
					if role != "admin" {
						if role == "user" {
							// 普通用户只能看到自己创建的分类下的小组组长
							categoryObj, _ := s.database.GetCategoryByName(categoryName)
							if categoryObj == nil || categoryObj.OwnerUserID != userID {
								continue
							}
						}
						// group_leader 可以看到管理相同分类的其他小组组长
					}

					accounts = append(accounts, gin.H{
						"id":         u.ID,
						"email":      u.Email,
						"role":       "group_leader",
						"category":   categoryName,
						"trader_id":  nil,
						"created_at": u.CreatedAt,
					})
					log.Printf("✅ 找到小组组长账号: email=%s, category=%s", u.Email, categoryName)
					break
				}
			}
		}
	}

	log.Printf("📊 返回账号列表，共 %d 个账号", len(accounts))
	c.JSON(http.StatusOK, accounts)
}

// handleGetCategoryAccountInfo 获取分类账号信息
func (s *Server) handleGetCategoryAccountInfo(c *gin.Context) {
	userID := c.GetString("user_id")
	accountID := c.Param("id")

	// 获取用户角色
	user, err := s.database.GetUserByID(userID)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户不存在"})
		return
	}

	role := user.Role
	if role == "" {
		role = "user"
	}

	// 获取账号信息
	account, err := s.database.GetUserByID(accountID)
	if err != nil || account == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "账号不存在"})
		return
	}

	if account.Role != "group_leader" && account.Role != "trader_account" {
		c.JSON(http.StatusNotFound, gin.H{"error": "账号不存在"})
		return
	}

	// 权限检查：如果不是admin，验证账号是否属于当前用户
	if role != "admin" {
		hasPermission := false
		if account.Role == "group_leader" {
			categories, _ := s.database.GetGroupLeaderCategories(accountID)
			for _, cat := range categories {
				category, _ := s.database.GetCategoryByName(cat)
				if category != nil && category.OwnerUserID == userID {
					hasPermission = true
					break
				}
			}
		} else if account.Role == "trader_account" {
			trader, _ := s.database.GetTraderByID(account.Email)
			if trader != nil && trader.OwnerUserID == userID {
				hasPermission = true
			}
		}
		if !hasPermission {
			c.JSON(http.StatusForbidden, gin.H{"error": "无权查看此账号"})
			return
		}
	}

	// 获取密码（如果是第一次获取，需要解密存储的密码）
	password := ""
	if account.PasswordHash != "" {
		// 密码是明文存储的（不安全，但在用户要求下这样做）
		password = account.PasswordHash
	}

	c.JSON(http.StatusOK, gin.H{
		"id":       account.ID,
		"email":    account.Email,
		"role":     account.Role,
		"password": password,
	})
}

// handleUpdateCategoryAccountPassword 更新分类账号密码
func (s *Server) handleUpdateCategoryAccountPassword(c *gin.Context) {
	userID := c.GetString("user_id")
	accountID := c.Param("id")

	// 获取用户角色
	user, err := s.database.GetUserByID(userID)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户不存在"})
		return
	}

	role := user.Role
	if role == "" {
		role = "user"
	}

	// 获取账号信息
	account, err := s.database.GetUserByID(accountID)
	if err != nil || account == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "账号不存在"})
		return
	}

	if account.Role != "group_leader" && account.Role != "trader_account" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的账号类型"})
		return
	}

	// 权限检查：如果不是admin，验证账号是否属于当前用户
	if role != "admin" {
		hasPermission := false
		if account.Role == "group_leader" {
			categories, _ := s.database.GetGroupLeaderCategories(accountID)
			for _, cat := range categories {
				category, _ := s.database.GetCategoryByName(cat)
				if category != nil && category.OwnerUserID == userID {
					hasPermission = true
					break
				}
			}
		} else if account.Role == "trader_account" {
			trader, _ := s.database.GetTraderByID(account.Email)
			if trader != nil && trader.OwnerUserID == userID {
				hasPermission = true
			}
		}
		if !hasPermission {
			c.JSON(http.StatusForbidden, gin.H{"error": "无权修改此账号"})
			return
		}
	}

	var req struct {
		Password string `json:"password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 哈希新密码
	hashedPassword, err := auth.HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "密码处理失败"})
		return
	}

	// 更新密码
	err = s.database.UpdateUserPassword(accountID, hashedPassword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新密码失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "密码已更新"})
}
