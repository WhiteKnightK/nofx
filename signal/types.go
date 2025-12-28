package signal

// SignalDecision 从邮件解析出的交易信号结构
type SignalDecision struct {
	SignalID          string          `json:"signal_id"`
	Symbol            string          `json:"symbol"`
	Direction         string          `json:"direction"`
	LeverageRecommend int             `json:"leverage_recommend"`
	Entry             EntryStrategy   `json:"entry"`
	Adds              []AddStrategy   `json:"adds"`
	TakeProfits       []TPStrategy    `json:"take_profits"`
	StopLoss          StopLossStrategy `json:"stop_loss"`
	Hedge             *HedgeStrategy  `json:"hedge,omitempty"`
	RawTextSummary    string          `json:"raw_text_summary"`
	RawContent        string          `json:"raw_content"` // 保存原始邮件全文用于展示
}

type EntryStrategy struct {
	PriceTarget float64 `json:"price_target"`
	RangeLow    float64 `json:"range_low"`
	RangeHigh   float64 `json:"range_high"`
}

type AddStrategy struct {
	Price     float64 `json:"price"`
	Percent   float64 `json:"percent"`
	Condition string  `json:"condition"`
}

type TPStrategy struct {
	Price   float64 `json:"price"`
	Percent float64 `json:"percent"`
}

type StopLossStrategy struct {
	Price         float64         `json:"price"`
	TrailingRules []TrailingRule  `json:"trailing_rules"`
}

type TrailingRule struct {
	TriggerPrice float64 `json:"trigger_price"`
	NewStopLoss  float64 `json:"new_stop_loss"`
}

type HedgeStrategy struct {
	TriggerPrice float64 `json:"trigger_price"`
	SizePercent  float64 `json:"size_percent"`
}


