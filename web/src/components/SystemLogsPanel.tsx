import React, { useState, useEffect, useRef } from 'react';
import { api } from '../lib/api';
import { LogEntry } from '../types';
import { RefreshCw, Filter, Search, AlertTriangle, Info, AlertCircle } from 'lucide-react';

interface SystemLogsPanelProps {
    visible: boolean;
}

const SystemLogsPanel: React.FC<SystemLogsPanelProps> = ({ visible }) => {
    const [logs, setLogs] = useState<LogEntry[]>([]);
    const [loading, setLoading] = useState(false);
    const [autoRefresh, setAutoRefresh] = useState(true);
    const [filters, setFilters] = useState({
        level: 'ALL',
        module: '',
        keyword: '',
        limit: 200
    });

    const timerRef = useRef<any>(null);

    const fetchLogs = async () => {
        try {
            setLoading(true);
            const res = await api.getLogs({
                limit: filters.limit,
                level: filters.level === 'ALL' ? '' : filters.level,
                module: filters.module,
                keyword: filters.keyword
            });
            setLogs(res.logs);
        } catch (err) {
            console.error('Fetch logs failed', err);
        } finally {
            setLoading(false);
        }
    };

    useEffect(() => {
        if (visible) {
            fetchLogs();
            if (autoRefresh) {
                timerRef.current = setInterval(fetchLogs, 3000);
            }
        }
        return () => {
            if (timerRef.current) clearInterval(timerRef.current);
        };
    }, [visible, autoRefresh, filters]);

    // Pause auto-refresh when user is typing keyword
    const handleKeywordChange = (e: React.ChangeEvent<HTMLInputElement>) => {
        setFilters(prev => ({ ...prev, keyword: e.target.value }));
        setAutoRefresh(false);
    };

    const getLevelColor = (level: string) => {
        switch (level.toUpperCase()) {
            case 'INFO': return 'text-blue-400';
            case 'WARN': return 'text-yellow-400';
            case 'ERROR': return 'text-red-500 font-bold';
            case 'DEBUG': return 'text-gray-400';
            default: return 'text-gray-300';
        }
    };

    const getLevelIcon = (level: string) => {
        switch (level.toUpperCase()) {
            case 'WARN': return <AlertTriangle size={14} />;
            case 'ERROR': return <AlertCircle size={14} />;
            default: return <Info size={14} />;
        }
    };

    const formatTime = (ts: string) => {
        return ts.replace('T', ' ').split('+')[0]; // Simple format
    };

    if (!visible) return null;

    return (
        <div className="bg-[#1E2329] rounded-lg border border-gray-800 flex flex-col h-[600px] animate-fade-in">
            {/* Header / Toolbar */}
            <div className="p-4 border-b border-gray-800 flex flex-wrap gap-4 items-center justify-between">
                <div className="flex items-center gap-2">
                    <h2 className="text-lg font-bold text-gray-200">系统日志</h2>
                    <span className="text-xs text-gray-500 px-2 py-0.5 bg-gray-800 rounded">{logs.length} 条</span>
                </div>

                <div className="flex items-center gap-3">
                    {/* Auto Refresh Toggle */}
                    <button
                        onClick={() => setAutoRefresh(!autoRefresh)}
                        className={`flex items-center gap-1 px-3 py-1.5 rounded text-sm transition-colors ${autoRefresh ? 'bg-green-500/20 text-green-400' : 'bg-gray-700 text-gray-400'
                            }`}
                    >
                        <RefreshCw size={14} className={autoRefresh ? "animate-spin" : ""} />
                        {autoRefresh ? "实时监控中" : "已暂停"}
                    </button>
                </div>
            </div>

            {/* Filters */}
            <div className="p-3 bg-[#161A1E] border-b border-gray-800 flex flex-wrap gap-3 items-center">
                <div className="flex items-center gap-2 bg-[#2B3139] px-2 py-1 rounded border border-gray-700">
                    <Filter size={14} className="text-gray-400" />
                    <select
                        value={filters.level}
                        onChange={(e) => setFilters(prev => ({ ...prev, level: e.target.value }))}
                        className="bg-transparent border-none text-sm text-gray-200 focus:outline-none"
                    >
                        <option value="ALL">全部级别</option>
                        <option value="INFO">INFO</option>
                        <option value="WARN">WARN</option>
                        <option value="ERROR">ERROR</option>
                    </select>
                </div>

                <div className="flex items-center gap-2 bg-[#2B3139] px-2 py-1 rounded border border-gray-700">
                    <span className="text-xs text-gray-400">模块:</span>
                    <input
                        type="text"
                        placeholder="例如: trader"
                        value={filters.module}
                        onChange={(e) => setFilters(prev => ({ ...prev, module: e.target.value }))}
                        className="bg-transparent border-none text-sm text-gray-200 w-24 focus:outline-none"
                    />
                </div>

                <div className="flex-1 flex items-center gap-2 bg-[#2B3139] px-2 py-1 rounded border border-gray-700">
                    <Search size={14} className="text-gray-400" />
                    <input
                        type="text"
                        placeholder="搜索日志内容..."
                        value={filters.keyword}
                        onChange={handleKeywordChange}
                        className="bg-transparent border-none text-sm text-gray-200 w-full focus:outline-none"
                    />
                </div>

                <button
                    onClick={() => fetchLogs()}
                    className="p-1.5 bg-blue-600/20 text-blue-400 rounded hover:bg-blue-600/30"
                    title="立即刷新"
                >
                    <RefreshCw size={16} />
                </button>
            </div>

            {/* Log List */}
            <div className="flex-1 overflow-auto p-2 space-y-1 font-mono text-xs">
                {loading && logs.length === 0 && (
                    <div className="text-center py-10 text-gray-500">加载中...</div>
                )}

                {logs.map((log, idx) => (
                    <div key={idx} className="flex gap-2 p-1.5 hover:bg-gray-800 rounded border border-transparent hover:border-gray-700 transition-colors">
                        <div className="text-gray-500 whitespace-nowrap w-36 shrink-0">{formatTime(log.ts)}</div>

                        <div className={`w-16 shrink-0 font-bold flex items-center gap-1 ${getLevelColor(log.level)}`}>
                            {getLevelIcon(log.level)}
                            {log.level}
                        </div>

                        {log.module && (
                            <div className="text-purple-400 bg-purple-400/10 px-1 rounded h-fit shrink-0">
                                {log.module}
                            </div>
                        )}

                        {log.trader_id && (
                            <div className="text-cyan-400 bg-cyan-400/10 px-1 rounded h-fit shrink-0 max-w-[100px] truncate" title={log.trader_id}>
                                {log.trader_id.split('_')[0]}..
                            </div>
                        )}

                        <div className="text-gray-300 break-all flex-1">
                            {log.msg}
                            {log.stacktrace && (
                                <pre className="mt-1 text-red-400/80 bg-red-900/10 p-2 rounded overflow-x-auto text-[10px]">
                                    {log.stacktrace}
                                </pre>
                            )}
                        </div>
                    </div>
                ))}

                {logs.length === 0 && !loading && (
                    <div className="text-center py-10 text-gray-500">暂无日志</div>
                )}
            </div>
        </div>
    );
};

export default SystemLogsPanel;
