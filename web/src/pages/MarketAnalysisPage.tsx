import React, { useState, useEffect, useCallback, useRef } from 'react';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { motion, AnimatePresence } from 'framer-motion';

// 热门交易对列表
const POPULAR_SYMBOLS = [
  'BTCUSDT', 'ETHUSDT', 'SOLUSDT', 'BNBUSDT', 'XRPUSDT',
  'DOGEUSDT', 'ADAUSDT', 'AVAXUSDT', 'LINKUSDT', 'DOTUSDT'
];

interface MarketData {
  symbol: string;
  price: number;
  priceChange24h: number;
  high24h: number;
  low24h: number;
  volume24h: number;
}

interface TrendReport {
  symbol: string;
  generated_at: string;
  content: string;
  isStreaming?: boolean;
}

// 全局报告缓存（组件卸载后依然保留，刷新页面后重置）
let globalReportsCache: Record<string, TrendReport> = {};

const MarketAnalysisPage: React.FC = () => {
  const [selectedSymbol, setSelectedSymbol] = useState('BTCUSDT');
  const [marketData, setMarketData] = useState<MarketData | null>(null);
  const [reportsCache, setReportsCache] = useState<Record<string, TrendReport>>(() => globalReportsCache);
  const [loading, setLoading] = useState(false);
  const [marketLoading, setMarketLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [timeframe, setTimeframe] = useState<'1h' | '4h' | '1d' | '1w'>('4h');
  const reportEndRef = useRef<HTMLDivElement>(null);

  // 同步本地缓存到全局缓存
  useEffect(() => {
    globalReportsCache = reportsCache;
  }, [reportsCache]);

  // 从币安获取市场数据
  const fetchMarketData = useCallback(async (symbol: string) => {
    try {
      const response = await fetch(
        `https://api.binance.com/api/v3/ticker/24hr?symbol=${symbol}`
      );
      if (!response.ok) throw new Error('获取行情失败');
      const data = await response.json();

      setMarketData({
        symbol: data.symbol,
        price: parseFloat(data.lastPrice),
        priceChange24h: parseFloat(data.priceChangePercent),
        high24h: parseFloat(data.highPrice),
        low24h: parseFloat(data.lowPrice),
        volume24h: parseFloat(data.quoteVolume),
      });
    } catch (err) {
      console.error('获取市场数据失败:', err);
    }
  }, []);

  // 生成分析报告 (流式)
  const generateReport = async () => {
    setLoading(true);
    setError(null);

    const newReport: TrendReport = {
      symbol: selectedSymbol,
      generated_at: new Date().toISOString(),
      content: '',
      isStreaming: true
    };

    setReportsCache(prev => ({
      ...prev,
      [selectedSymbol]: newReport
    }));

    try {
      const response = await fetch(`/api/analysis/report/stream?symbol=${selectedSymbol}`);
      if (!response.ok) {
        const errorData = await response.json().catch(() => ({}));
        throw new Error(errorData.error || `网络请求失败 (状态码: ${response.status})`);
      }

      const reader = response.body?.getReader();
      if (!reader) throw new Error('无法初始化流读取器');

      const decoder = new TextDecoder();
      let accumulatedContent = '';
      let leftover = '';

      while (true) {
        const { done, value } = await reader.read();
        if (done) break;

        const chunk = decoder.decode(value, { stream: true });
        const lines = (leftover + chunk).split('\n');
        leftover = lines.pop() || '';

        for (const line of lines) {
          const trimmedLine = line.trim();
          if (trimmedLine.startsWith('data:')) {
            const jsonStr = trimmedLine.slice(5).trim();
            if (!jsonStr) continue;

            try {
              const data = JSON.parse(jsonStr);
              if (data.content) {
                accumulatedContent += data.content;
                setReportsCache(prev => ({
                  ...prev,
                  [selectedSymbol]: {
                    ...prev[selectedSymbol],
                    content: accumulatedContent
                  }
                }));
              } else if (data.status === 'done') {
                continue;
              } else if (data.error) {
                throw new Error(data.error);
              }
            } catch (e) {
              console.error('解析流数据失败:', e, jsonStr);
            }
          }
        }

        if (reportEndRef.current) {
          reportEndRef.current.scrollIntoView({ behavior: 'smooth' });
        }
      }

      setReportsCache(prev => ({
        ...prev,
        [selectedSymbol]: {
          ...prev[selectedSymbol],
          isStreaming: false
        }
      }));
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : '分析中断');
    } finally {
      setLoading(false);
    }
  };

  // 初始加载
  useEffect(() => {
    setMarketLoading(true);
    setError(null);
    fetchMarketData(selectedSymbol).finally(() => setMarketLoading(false));

    const interval = setInterval(() => fetchMarketData(selectedSymbol), 3000);
    return () => clearInterval(interval);
  }, [selectedSymbol, fetchMarketData]);

  const formatPrice = (price: number) => {
    if (price >= 1000) return price.toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 });
    if (price >= 1) return price.toFixed(4);
    return price.toFixed(6);
  };

  const formatVolume = (volume: number) => {
    if (volume >= 1e9) return `${(volume / 1e9).toFixed(2)}B`;
    if (volume >= 1e6) return `${(volume / 1e6).toFixed(2)}M`;
    if (volume >= 1e3) return `${(volume / 1e3).toFixed(2)}K`;
    return volume.toFixed(2);
  };

  return (
    <div className="market-analysis-container">
      <div className="market-analysis-main">
        {/* 顶部行情概览 */}
        <header className="analysis-header glass-card">
          <div className="symbol-info-wrapper">
            <div className="symbol-dropdown-container">
              <select
                value={selectedSymbol}
                onChange={(e) => setSelectedSymbol(e.target.value)}
                className="symbol-select-premium"
              >
                {POPULAR_SYMBOLS.map(sym => (
                  <option key={sym} value={sym}>{sym.replace('USDT', ' / USDT')}</option>
                ))}
              </select>
            </div>

            <AnimatePresence mode="wait">
              <motion.div
                key={marketData?.price}
                initial={{ opacity: 0, y: 10 }}
                animate={{ opacity: 1, y: 0 }}
                className="live-price-block"
              >
                <span className="live-price">
                  {marketLoading ? '---' : marketData ? formatPrice(marketData.price) : '--'}
                </span>
                <span className={`live-change ${marketData && marketData.priceChange24h >= 0 ? 'up' : 'down'}`}>
                  {marketData ? `${marketData.priceChange24h >= 0 ? '+' : ''}${marketData.priceChange24h.toFixed(2)}%` : '--'}
                </span>
              </motion.div>
            </AnimatePresence>
          </div>

          <div className="header-stats hidden-mobile">
            <div className="header-stat-item">
              <label>24h 最高</label>
              <span>{marketData ? formatPrice(marketData.high24h) : '--'}</span>
            </div>
            <div className="header-stat-item">
              <label>24h 最低</label>
              <span>{marketData ? formatPrice(marketData.low24h) : '--'}</span>
            </div>
            <div className="header-stat-item">
              <label>24h 成交额</label>
              <span>{marketData ? formatVolume(marketData.volume24h) : '--'}</span>
            </div>
          </div>
        </header>

        <div className="analysis-content-grid">
          {/* 主图表区域 */}
          <section className="chart-section glass-card">
            <div className="chart-header">
              <div className="timeframe-tabs">
                {(['1h', '4h', '1d', '1w'] as const).map(tf => (
                  <button
                    key={tf}
                    className={`tf-tab ${timeframe === tf ? 'active' : ''}`}
                    onClick={() => setTimeframe(tf)}
                  >
                    {tf === '1h' ? '1小时' : tf === '4h' ? '4小时' : tf === '1d' ? '日线' : '周线'}
                  </button>
                ))}
              </div>
              <div className="chart-title">
                TradingView 实时图表终端
              </div>
            </div>
            <div className="chart-iframe-wrapper">
              <iframe
                src={`https://s.tradingview.com/widgetembed/?symbol=BINANCE:${selectedSymbol}&interval=${timeframe === '1h' ? '60' : timeframe === '4h' ? '240' : timeframe === '1d' ? 'D' : 'W'}&theme=dark&style=1&locale=zh_CN&hide_legend=1&hide_side_toolbar=0&allow_symbol_change=0&save_image=0&hide_volume=1`}
                style={{ width: '100%', height: '100%', border: 'none' }}
                title="TradingView Chart"
              />
            </div>
          </section>

          {/* 右侧动作与信息面板 */}
          <aside className="action-sidebar">
            {/* 快速选择列表 */}
            <div className="quick-select-card glass-card">
              <h4>🔥 热门交易对</h4>
              <div className="quick-symbols-grid">
                {POPULAR_SYMBOLS.map(sym => (
                  <button
                    key={sym}
                    className={`symbol-chip ${selectedSymbol === sym ? 'active' : ''}`}
                    onClick={() => setSelectedSymbol(sym)}
                  >
                    {sym.replace('USDT', '')}
                  </button>
                ))}
              </div>
            </div>

            {/* 智能分析控制卡片 */}
            <div className="ai-control-card glass-card">
              <h3>深度行情研判</h3>
              <p>针对当前盘面进行多维度技术审计，包含维加斯通道强度、价格行为形态及多周期共振预测。</p>

              <button
                className={`ai-trigger-btn ${loading ? 'loading' : ''}`}
                onClick={generateReport}
                disabled={loading}
              >
                {loading ? (
                  <div className="loader-container">
                    <div className="pulse-loader"></div>
                    <span>深度分析中...</span>
                  </div>
                ) : (
                  <>
                    生成技术分析报告
                  </>
                )}
              </button>

              <div className="ai-features-list">
                <div className="feature-tag">Vegas 审计</div>
                <div className="feature-tag">PA 价格行为</div>
                <div className="feature-tag">多周期共振</div>
              </div>
            </div>
          </aside>
        </div>

        {/* AI 报告展示区 */}
        <AnimatePresence mode="wait">
          {(reportsCache[selectedSymbol] || error) && (
            <motion.div
              key={selectedSymbol + (error ? '-error' : '')}
              initial={{ opacity: 0, y: 10 }}
              animate={{ opacity: 1, y: 0 }}
              exit={{ opacity: 0, scale: 0.98 }}
              className="analysis-report-section glass-card"
            >
              {error ? (
                <div className="report-error">
                  <span className="error-icon">⚠️</span>
                  <div className="error-text">
                    <h5>分析服务暂时不可用</h5>
                    <p>{error}</p>
                    <button onClick={generateReport} className="retry-btn">重新尝试</button>
                  </div>
                </div>
              ) : (
                <div className="report-success">
                  <div className="report-header-premium">
                    <div className="report-title-group">
                      <h2>{selectedSymbol} 技术分析报告</h2>
                    </div>
                    <div className="report-timestamp">
                      生成时间: {new Date(reportsCache[selectedSymbol].generated_at).toLocaleString('zh-CN')}
                    </div>
                  </div>

                  <div className="markdown-container-premium">
                    <ReactMarkdown remarkPlugins={[remarkGfm]}>
                      {reportsCache[selectedSymbol].content}
                    </ReactMarkdown>
                    {reportsCache[selectedSymbol].isStreaming && <span className="streaming-cursor">|</span>}
                  </div>
                </div>
              )}
              <div ref={reportEndRef}></div>
            </motion.div>
          )}
        </AnimatePresence>
      </div>

      <style>{`
        :root {
          --brand-yellow: #F0B90B;
          --brand-green: #0ECB81;
          --brand-red: #F6465D;
          --glass-bg: #1e2329;
          --glass-border: #363c44;
          --text-primary: #EAECEF;
          --text-secondary: #848E9C;
        }

        .market-analysis-main {
          min-height: 100vh;
          background: #0B0E11;
          padding: 24px;
          display: flex;
          flex-direction: column;
          gap: 16px;
        }

        .glass-card {
          background: var(--glass-bg);
          border: 1px solid var(--glass-border);
          border-radius: 8px;
        }

        /* 头部样式 */
        .analysis-header {
          padding: 20px 32px;
          display: flex;
          justify-content: space-between;
          align-items: center;
        }

        .symbol-info-wrapper {
          display: flex;
          align-items: center;
          gap: 32px;
        }

        .symbol-select-premium {
          appearance: none;
          background: rgba(255, 255, 255, 0.05);
          border: 1px solid var(--glass-border);
          padding: 12px 24px;
          border-radius: 12px;
          color: var(--text-primary);
          font-size: 1.5rem;
          font-weight: 700;
          cursor: pointer;
          min-width: 220px;
          outline: none;
          transition: border-color 0.3s;
        }
        .symbol-select-premium:focus {
          border-color: var(--brand-yellow);
        }

        .live-price-block {
          display: flex;
          flex-direction: column;
          gap: 2px;
        }
        .live-price {
          font-size: 2rem;
          font-weight: 800;
          letter-spacing: -1px;
        }
        .live-change {
          font-size: 1rem;
          font-weight: 600;
        }
        .live-change.up { color: var(--brand-green); }
        .live-change.down { color: var(--brand-red); }

        .header-stats {
          display: flex;
          gap: 48px;
        }
        .header-stat-item {
          display: flex;
          flex-direction: column;
          gap: 4px;
        }
        .header-stat-item label {
          font-size: 0.75rem;
          color: var(--text-secondary);
          text-transform: uppercase;
        }
        .header-stat-item span {
          font-size: 1rem;
          font-weight: 600;
        }

        /* 内容布局 */
        .analysis-content-grid {
          display: grid;
          grid-template-columns: 1fr 320px;
          gap: 20px;
        }

        .chart-section {
          padding: 0;
          display: flex;
          flex-direction: column;
          overflow: hidden;
          min-height: 600px;
        }
        .chart-header {
          padding: 16px 24px;
          display: flex;
          justify-content: space-between;
          align-items: center;
          border-bottom: 1px solid var(--glass-border);
        }
        .timeframe-tabs {
          display: flex;
          background: rgba(255, 255, 255, 0.05);
          padding: 4px;
          border-radius: 10px;
        }
        .tf-tab {
          padding: 6px 16px;
          border: none;
          background: transparent;
          color: var(--text-secondary);
          font-size: 0.85rem;
          font-weight: 600;
          cursor: pointer;
          border-radius: 8px;
          transition: all 0.2s;
        }
        .tf-tab.active {
          background: var(--brand-yellow);
          color: #000;
        }
        .chart-title {
          font-size: 0.85rem;
          font-weight: 500;
          color: var(--text-secondary);
        }
        .chart-iframe-wrapper {
          flex: 1;
        }

        .action-sidebar {
          display: flex;
          flex-direction: column;
          gap: 20px;
        }

        .quick-select-card {
          padding: 20px;
        }
        .quick-select-card h4 {
          font-size: 0.9rem;
          color: var(--text-secondary);
          margin-bottom: 16px;
          text-transform: uppercase;
        }
        .quick-symbols-grid {
          display: grid;
          grid-template-columns: 1fr 1fr;
          gap: 10px;
        }
        .symbol-chip {
          padding: 10px;
          background: rgba(255, 255, 255, 0.03);
          border: 1px solid var(--glass-border);
          border-radius: 10px;
          color: var(--text-primary);
          font-size: 0.9rem;
          font-weight: 600;
          cursor: pointer;
          transition: all 0.2s;
        }
        .symbol-chip:hover {
          background: rgba(255, 255, 255, 0.08);
          border-color: rgba(255, 255, 255, 0.2);
        }
        .symbol-chip.active {
          border-color: var(--brand-yellow);
          color: var(--brand-yellow);
          background: rgba(240, 185, 11, 0.05);
        }

        .ai-control-card {
          padding: 24px;
          display: flex;
          flex-direction: column;
          gap: 16px;
        }
        .ai-control-card h3 {
          font-size: 1.2rem;
          font-weight: 700;
          color: var(--text-primary);
        }
        .ai-control-card p {
          font-size: 0.9rem;
          color: var(--text-secondary);
          line-height: 1.5;
        }

        .ai-trigger-btn {
          padding: 12px;
          background: var(--brand-yellow);
          border: none;
          border-radius: 4px;
          color: #1e2329;
          font-size: 0.95rem;
          font-weight: 600;
          cursor: pointer;
          transition: opacity 0.2s;
        }
        .ai-trigger-btn:hover:not(:disabled) {
          opacity: 0.9;
        }
        .ai-trigger-btn.loading {
          background: #474d57;
          color: #848e9c;
        }

        .ai-features-list {
          display: flex;
          flex-wrap: wrap;
          gap: 8px;
          margin-top: 12px;
        }
        .feature-tag {
          font-size: 0.75rem;
          color: var(--text-secondary);
          background: rgba(255, 255, 255, 0.05);
          padding: 4px 12px;
          border-radius: 20px;
          border: 1px solid var(--glass-border);
        }

        /* 报告区域样式 */
        .analysis-report-section {
          padding: 40px;
          margin-top: 20px;
        }

        .report-header-premium h2 {
          font-size: 1.5rem;
          font-weight: 700;
          color: var(--text-primary);
        }
        .report-timestamp {
          font-size: 0.9rem;
          color: var(--text-secondary);
        }

        .markdown-container-premium {
          color: #FFF;
          line-height: 2;
          font-size: 1.15rem;
          font-weight: 400;
          letter-spacing: 0.02em;
        }
        .markdown-container-premium h1 {
          font-size: 1.8rem;
          color: #FFF;
          margin: 32px 0 20px;
          font-weight: 800;
        }
        .markdown-container-premium h2 {
          font-size: 1.4rem;
          color: #FFF;
          margin: 32px 0 16px;
          padding-bottom: 8px;
          border-bottom: 1px solid var(--glass-border);
          font-weight: 700;
        }
        .markdown-container-premium h3 {
          font-size: 1.15rem;
          color: var(--text-primary);
          margin: 24px 0 12px;
          font-weight: 700;
        }
        .markdown-container-premium blockquote {
          background: rgba(255, 255, 255, 0.02);
          border-left: 3px solid var(--brand-yellow);
          margin: 24px 0;
          padding: 16px 20px;
          border-radius: 4px;
        }
        .markdown-container-premium strong {
          color: var(--brand-yellow);
          font-weight: 700;
        }
        .streaming-cursor {
          display: inline-block;
          width: 8px;
          height: 18px;
          background: var(--brand-yellow);
          margin-left: 4px;
          animation: blink 0.8s step-end infinite;
          vertical-align: middle;
        }
        @keyframes blink {
          50% { opacity: 0; }
        }
        .markdown-container-premium pre {
          background: rgba(0, 0, 0, 0.4);
          border: 1px solid var(--glass-border);
          border-radius: 12px;
          padding: 24px;
          margin: 32px 0;
          overflow-x: auto;
          font-family: 'Fira Code', 'Roboto Mono', monospace;
          font-size: 0.95rem;
          color: #E2E8F0;
        }
        .markdown-container-premium table {
          width: 100%;
          border-collapse: collapse;
          margin: 32px 0;
          background: rgba(255, 255, 255, 0.02);
          border-radius: 12px;
          overflow: hidden;
        }
        .markdown-container-premium th, .markdown-container-premium td {
          padding: 16px;
          text-align: left;
          border-bottom: 1px solid var(--glass-border);
        }
        .markdown-container-premium th {
          background: rgba(255, 255, 255, 0.05);
          color: var(--brand-yellow);
          font-weight: 700;
          text-transform: uppercase;
          font-size: 0.85rem;
        }
        .markdown-container-premium ul, .markdown-container-premium ol {
          padding-left: 24px;
          margin: 16px 0;
        }
        .markdown-container-premium li {
          margin-bottom: 12px;
        }

        .report-footer {
          margin-top: 48px;
          padding-top: 24px;
          border-top: 1px solid var(--glass-border);
          text-align: center;
          font-size: 0.85rem;
          color: var(--brand-red);
          opacity: 0.8;
        }

        .report-error {
          display: flex;
          align-items: flex-start;
          gap: 20px;
          padding: 24px;
        }
        .error-icon {
          font-size: 2.5rem;
        }
        .error-text h5 {
          font-size: 1.2rem;
          color: var(--brand-red);
          margin: 0 0 8px;
        }
        .error-text p {
          color: var(--text-secondary);
          margin-bottom: 16px;
        }
        .retry-btn {
          padding: 8px 24px;
          background: var(--brand-red);
          border: none;
          border-radius: 8px;
          color: #FFF;
          font-weight: 600;
          cursor: pointer;
        }

        .loader-container {
          display: flex;
          align-items: center;
          gap: 12px;
        }
        .pulse-loader {
          width: 12px;
          height: 12px;
          background: #FFF;
          border-radius: 50%;
          animation: pulse 1s infinite alternate;
        }
        @keyframes pulse {
          from { opacity: 1; transform: scale(1); }
          to { opacity: 0.3; transform: scale(0.6); }
        }

        /* 响应式适配 */
        @media (max-width: 1024px) {
          .analysis-content-grid {
            grid-template-columns: 1fr;
          }
          .action-sidebar {
            flex-direction: row;
          }
          .quick-select-card, .ai-control-card {
            flex: 1;
          }
        }

        @media (max-width: 768px) {
          .analysis-header {
            flex-direction: column;
            gap: 20px;
            align-items: flex-start;
          }
          .hidden-mobile { display: none; }
          .action-sidebar {
            flex-direction: column;
          }
          .market-analysis-container {
            padding: 12px;
          }
          .symbol-select-premium {
            min-width: 100%;
            font-size: 1.2rem;
          }
          .live-price { font-size: 1.8rem; }
          .analysis-report-section { padding: 20px; }
          .report-header-premium h2 { font-size: 1.5rem; }
        }
      `}</style>
    </div>
  );
};

export default MarketAnalysisPage;
