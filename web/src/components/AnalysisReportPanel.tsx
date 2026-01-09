import React, { useState, useEffect } from 'react';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';

interface AnalysisReportPanelProps {
    symbol: string;
    onClose?: () => void;
}

interface TrendReport {
    symbol: string;
    generated_at: string;
    content: string;
}

const AnalysisReportPanel: React.FC<AnalysisReportPanelProps> = ({ symbol, onClose }) => {
    const [report, setReport] = useState<TrendReport | null>(null);
    const [loading, setLoading] = useState(false);
    const [error, setError] = useState<string | null>(null);

    // 使用相对路径，由 vite 代理或生产环境 nginx 处理

    const fetchReport = async () => {
        setLoading(true);
        setError(null);
        try {
            const response = await fetch(`/api/analysis/report?symbol=${symbol}`);
            if (!response.ok) {
                const data = await response.json();
                throw new Error(data.error || '获取报告失败');
            }
            const data = await response.json();
            setReport(data);
        } catch (err: unknown) {
            setError(err instanceof Error ? err.message : '未知错误');
        } finally {
            setLoading(false);
        }
    };

    useEffect(() => {
        if (symbol) {
            fetchReport();
        }
    }, [symbol]);

    return (
        <div className="analysis-report-panel">
            <div className="panel-header">
                <h2>📊 {symbol} 日内趋势技术分析</h2>
                <div className="header-actions">
                    <button className="refresh-btn" onClick={fetchReport} disabled={loading}>
                        {loading ? '⏳ 生成中...' : '🔄 刷新'}
                    </button>
                    {onClose && (
                        <button className="close-btn" onClick={onClose}>✕</button>
                    )}
                </div>
            </div>

            <div className="panel-content">
                {loading && (
                    <div className="loading-state">
                        <div className="spinner"></div>
                        <p>正在调用 AI 生成分析报告，请稍候...</p>
                    </div>
                )}

                {error && (
                    <div className="error-state">
                        <p>❌ {error}</p>
                        <button onClick={fetchReport}>重试</button>
                    </div>
                )}

                {!loading && !error && report && (
                    <div className="report-content">
                        <div className="report-meta">
                            <span className="generated-time">
                                生成时间: {new Date(report.generated_at).toLocaleString('zh-CN')}
                            </span>
                        </div>
                        <div className="markdown-body">
                            <ReactMarkdown remarkPlugins={[remarkGfm]}>
                                {report.content}
                            </ReactMarkdown>
                        </div>
                    </div>
                )}

                {!loading && !error && !report && (
                    <div className="empty-state">
                        <p>点击「刷新」按钮生成分析报告</p>
                    </div>
                )}
            </div>

            <style>{`
        .analysis-report-panel {
          background: linear-gradient(145deg, #1a1a2e 0%, #16213e 100%);
          border-radius: 16px;
          box-shadow: 0 8px 32px rgba(0, 0, 0, 0.3);
          overflow: hidden;
          max-width: 900px;
          margin: 0 auto;
        }

        .panel-header {
          display: flex;
          justify-content: space-between;
          align-items: center;
          padding: 20px 24px;
          background: linear-gradient(90deg, #0f3460 0%, #16213e 100%);
          border-bottom: 1px solid rgba(255, 255, 255, 0.1);
        }

        .panel-header h2 {
          margin: 0;
          font-size: 1.4rem;
          color: #e94560;
          font-weight: 600;
        }

        .header-actions {
          display: flex;
          gap: 12px;
        }

        .refresh-btn {
          padding: 8px 16px;
          background: linear-gradient(135deg, #e94560 0%, #ff6b6b 100%);
          border: none;
          border-radius: 8px;
          color: white;
          font-weight: 500;
          cursor: pointer;
          transition: all 0.3s ease;
        }

        .refresh-btn:hover:not(:disabled) {
          transform: translateY(-2px);
          box-shadow: 0 4px 12px rgba(233, 69, 96, 0.4);
        }

        .refresh-btn:disabled {
          opacity: 0.6;
          cursor: not-allowed;
        }

        .close-btn {
          width: 36px;
          height: 36px;
          border-radius: 50%;
          background: rgba(255, 255, 255, 0.1);
          border: none;
          color: #aaa;
          font-size: 1.2rem;
          cursor: pointer;
          transition: all 0.2s ease;
        }

        .close-btn:hover {
          background: rgba(233, 69, 96, 0.3);
          color: white;
        }

        .panel-content {
          padding: 24px;
          max-height: 70vh;
          overflow-y: auto;
        }

        .loading-state {
          display: flex;
          flex-direction: column;
          align-items: center;
          gap: 16px;
          padding: 48px;
          color: #aaa;
        }

        .spinner {
          width: 40px;
          height: 40px;
          border: 3px solid rgba(233, 69, 96, 0.3);
          border-top-color: #e94560;
          border-radius: 50%;
          animation: spin 1s linear infinite;
        }

        @keyframes spin {
          to { transform: rotate(360deg); }
        }

        .error-state {
          text-align: center;
          padding: 32px;
          color: #ff6b6b;
        }

        .error-state button {
          margin-top: 16px;
          padding: 8px 24px;
          background: #e94560;
          border: none;
          border-radius: 6px;
          color: white;
          cursor: pointer;
        }

        .report-content {
          color: #e0e0e0;
        }

        .report-meta {
          margin-bottom: 16px;
          padding-bottom: 12px;
          border-bottom: 1px solid rgba(255, 255, 255, 0.1);
        }

        .generated-time {
          font-size: 0.85rem;
          color: #888;
        }

        .markdown-body {
          line-height: 1.8;
        }

        .markdown-body h1 {
          font-size: 1.5rem;
          color: #e94560;
          border-bottom: 2px solid #e94560;
          padding-bottom: 8px;
          margin: 24px 0 16px;
        }

        .markdown-body h2 {
          font-size: 1.3rem;
          color: #ff9f43;
          margin: 20px 0 12px;
        }

        .markdown-body h3 {
          font-size: 1.1rem;
          color: #54a0ff;
          margin: 16px 0 8px;
        }

        .markdown-body p {
          margin: 12px 0;
        }

        .markdown-body strong {
          color: #fff;
        }

        .markdown-body ul, .markdown-body ol {
          padding-left: 24px;
          margin: 12px 0;
        }

        .markdown-body li {
          margin: 8px 0;
        }

        .markdown-body code {
          background: rgba(255, 255, 255, 0.1);
          padding: 2px 6px;
          border-radius: 4px;
          font-family: 'Fira Code', monospace;
        }

        .markdown-body pre {
          background: #0d1117;
          padding: 16px;
          border-radius: 8px;
          overflow-x: auto;
        }

        .markdown-body blockquote {
          border-left: 4px solid #e94560;
          margin: 16px 0;
          padding: 8px 16px;
          background: rgba(233, 69, 96, 0.1);
          border-radius: 0 8px 8px 0;
        }

        .markdown-body hr {
          border: none;
          height: 1px;
          background: linear-gradient(90deg, transparent, rgba(255, 255, 255, 0.2), transparent);
          margin: 24px 0;
        }

        .empty-state {
          text-align: center;
          padding: 48px;
          color: #888;
        }
      `}</style>
        </div>
    );
};

export default AnalysisReportPanel;
