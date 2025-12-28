package market

// CalculateRSI 计算相对强弱指数 (Wilder's RSI)
// data: 价格序列 (按时间顺序，最新的在最后)
// period: 周期 (通常为 14)
func CalculateRSI(data []float64, period int) float64 {
	if len(data) < period+1 {
		return 0
	}

	var gains, losses float64

	// 1. 计算初始平均值 (SMA)
	for i := 1; i <= period; i++ {
		diff := data[i] - data[i-1]
		if diff > 0 {
			gains += diff
		} else {
			losses -= diff
		}
	}

	avgGain := gains / float64(period)
	avgLoss := losses / float64(period)

	// 2. 计算后续值的平滑平均 (Wilder's Smoothing)
	for i := period + 1; i < len(data); i++ {
		diff := data[i] - data[i-1]
		var currentGain, currentLoss float64
		if diff > 0 {
			currentGain = diff
		} else {
			currentLoss = -diff
		}

		avgGain = ((avgGain * float64(period-1)) + currentGain) / float64(period)
		avgLoss = ((avgLoss * float64(period-1)) + currentLoss) / float64(period)
	}

	if avgLoss == 0 {
		return 100
	}

	rs := avgGain / avgLoss
	return 100 - (100 / (1 + rs))
}

// CalculateEMA 计算指数移动平均
func CalculateEMA(data []float64, period int) []float64 {
	if len(data) == 0 {
		return nil
	}

	ema := make([]float64, len(data))
	k := 2.0 / float64(period+1)

	// 初始EMA通常使用SMA
	sum := 0.0
	if len(data) < period {
		return nil
	}

	for i := 0; i < period; i++ {
		sum += data[i]
	}
	ema[period-1] = sum / float64(period)

	// 计算后续EMA
	for i := period; i < len(data); i++ {
		ema[i] = (data[i] * k) + (ema[i-1] * (1 - k))
	}

	return ema
}

// CalculateMACD 计算 MACD (12, 26, 9)
// 返回最新的 macdLine, signalLine, histogram
func CalculateMACD(data []float64) (float64, float64, float64) {
	if len(data) < 26 {
		return 0, 0, 0
	}

	ema12 := CalculateEMA(data, 12)
	ema26 := CalculateEMA(data, 26)

	// MACD Line = EMA12 - EMA26
	macdLine := make([]float64, len(data))
	for i := 26; i < len(data); i++ {
		macdLine[i] = ema12[i] - ema26[i]
	}

	// Signal Line = EMA9 of MACD Line
	// 我们只需要计算 macdLine 非零部分的 EMA9
	validMacd := macdLine[26:]
	signalLineVals := CalculateEMA(validMacd, 9)

	if len(signalLineVals) == 0 {
		return 0, 0, 0
	}

	lastIdx := len(data) - 1
	validLastIdx := len(signalLineVals) - 1

	currMacd := macdLine[lastIdx]
	currSignal := signalLineVals[validLastIdx]
	currHist := currMacd - currSignal

	return currMacd, currSignal, currHist
}
