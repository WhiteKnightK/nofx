import { useState } from 'react';
import useSWR from 'swr';
import { api } from '../lib/api';
import { StrategyDecisionHistory as HistoryType } from '../types';
import { Clock, AlertCircle, CheckCircle, XCircle, ChevronDown, ChevronUp, Filter } from 'lucide-react';

interface StrategyDecisionHistoryProps {
    traderId: string;
}

// 动作类型分类
const ACTION_CATEGORIES = {
    all: '全部',
    open: '开仓类',
    close: '平仓类',
    sltp: '止盈止损',
    wait: '观望'
};

// 判断动作属于哪个分类
const getActionCategory = (action: string): string => {
    const upperAction = action.toUpperCase();
    if (upperAction.includes('OPEN') || upperAction.includes('ADD')) return 'open';
    if (upperAction.includes('CLOSE') || upperAction.includes('PARTIAL')) return 'close';
    if (upperAction.includes('STOP') || upperAction.includes('TP') || upperAction.includes('SL') || upperAction.includes('PROFIT') || upperAction.includes('LOSS')) return 'sltp';
    if (upperAction === 'WAIT' || upperAction === 'HOLD') return 'wait';
    return 'all';
};

export function StrategyDecisionHistory({ traderId }: StrategyDecisionHistoryProps) {
    const [limit] = useState(50);
    const [actionFilter, setActionFilter] = useState<string>('all');
    const { data, error } = useSWR(
        traderId ? `strategy-decisions-${traderId}-${limit}` : null,
        () => api.getStrategyDecisions(traderId, 'latest', limit),
        { refreshInterval: 5000 }
    );

    const [expandedId, setExpandedId] = useState<number | null>(null);

    if (error) return <div className="text-red-500 text-sm p-4">加载决策历史失败</div>;
    if (!data) return <div className="text-gray-500 text-sm p-4">加载中...</div>;

    const allDecisions: HistoryType[] = data.decisions || [];

    // 根据筛选条件过滤
    const decisions = actionFilter === 'all'
        ? allDecisions
        : allDecisions.filter(d => getActionCategory(d.action) === actionFilter);

    if (allDecisions.length === 0) {
        return (
            <div className="bg-[#1E2329] rounded-xl border border-[#2B3139] p-6 text-center">
                <div className="text-gray-500 text-sm">暂无策略决策记录</div>
            </div>
        );
    }

    const getActionColor = (action: string) => {
        if (action.includes('LONG')) return 'text-green-500 bg-green-500/10 border-green-500/20';
        if (action.includes('SHORT')) return 'text-red-500 bg-red-500/10 border-red-500/20';
        if (action === 'WAIT') return 'text-yellow-500 bg-yellow-500/10 border-yellow-500/20';
        if (action.includes('STOP') || action.includes('TP') || action.includes('SL')) return 'text-blue-500 bg-blue-500/10 border-blue-500/20';
        return 'text-gray-400 bg-gray-500/10 border-gray-500/20';
    };

    const formatTime = (timeStr: string) => {
        return new Date(timeStr).toLocaleString('zh-CN', {
            month: '2-digit',
            day: '2-digit',
            hour: '2-digit',
            minute: '2-digit',
            second: '2-digit'
        });
    };

    return (
        <div className="bg-[#1E2329] rounded-xl border border-[#2B3139] overflow-hidden mt-6">
            <div className="p-4 border-b border-[#2B3139] flex justify-between items-center bg-[#252B35]">
                <h3 className="text-[#EAECEF] font-bold flex items-center gap-2">
                    <Clock size={18} className="text-[#F0B90B]" />
                    策略执行历史
                </h3>
                <div className="flex items-center gap-3">
                    {/* 筛选下拉框 */}
                    <div className="flex items-center gap-2">
                        <Filter size={14} className="text-[#848E9C]" />
                        <select
                            value={actionFilter}
                            onChange={(e) => setActionFilter(e.target.value)}
                            className="bg-[#0B0E11] text-[#EAECEF] text-xs px-2 py-1 rounded border border-[#2B3139] focus:outline-none focus:border-[#F0B90B]"
                        >
                            {Object.entries(ACTION_CATEGORIES).map(([key, label]) => (
                                <option key={key} value={key}>{label}</option>
                            ))}
                        </select>
                    </div>
                    <span className="text-xs text-[#848E9C]">
                        显示 {decisions.length} / {allDecisions.length} 条
                    </span>
                </div>
            </div>

            <div className="divide-y divide-[#2B3139] max-h-[500px] overflow-y-auto">

                {decisions.map((decision) => (
                    <div key={decision.id} className="p-4 hover:bg-[#2B3139]/50 transition-colors">
                        <div
                            className="flex justify-between items-start cursor-pointer"
                            onClick={() => setExpandedId(expandedId === decision.id ? null : decision.id)}
                        >
                            <div className="flex flex-col gap-1">
                                <div className="flex items-center gap-2">
                                    <span className={`px-2 py-0.5 rounded text-xs font-bold border ${getActionColor(decision.action)}`}>
                                        {decision.action}
                                    </span>
                                    <span className="text-[#EAECEF] font-mono font-bold text-sm">
                                        {decision.symbol}
                                    </span>
                                    <span className="text-xs text-[#848E9C] font-mono">
                                        {formatTime(decision.decision_time)}
                                    </span>
                                </div>
                                <div className="text-sm text-[#EAECEF] mt-1">
                                    当前价格: <span className="font-mono text-[#F0B90B]">{decision.current_price}</span>
                                    {decision.target_price > 0 && (
                                        <span className="ml-2 text-[#848E9C]">
                                            (目标: {decision.target_price})
                                        </span>
                                    )}
                                </div>
                            </div>

                            <div className="flex items-center gap-3">
                                {decision.execution_success ? (
                                    <CheckCircle size={16} className="text-green-500" />
                                ) : decision.action !== 'WAIT' ? (
                                    <XCircle size={16} className="text-red-500" />
                                ) : null}
                                {expandedId === decision.id ? <ChevronUp size={16} className="text-[#848E9C]" /> : <ChevronDown size={16} className="text-[#848E9C]" />}
                            </div>
                        </div>

                        {/* 展开详情 */}
                        {expandedId === decision.id && (
                            <div className="mt-3 pt-3 border-t border-[#2B3139]/50 text-sm space-y-2 animate-fade-in">
                                <div className="bg-[#0B0E11] p-3 rounded border border-[#2B3139]">
                                    <div className="flex items-start gap-2">
                                        <AlertCircle size={14} className="text-[#F0B90B] mt-0.5 flex-shrink-0" />
                                        <p className="text-[#EAECEF] leading-relaxed">{decision.reason}</p>
                                    </div>
                                </div>

                                <div className="grid grid-cols-3 gap-2 text-xs text-[#848E9C]">
                                    <div className="bg-[#2B3139]/30 p-2 rounded">
                                        <span className="block mb-1">RSI (1H)</span>
                                        <span className={`font-mono font-bold ${decision.rsi_1h > 70 ? 'text-red-400' : decision.rsi_1h < 30 ? 'text-green-400' : 'text-[#EAECEF]'}`}>
                                            {decision.rsi_1h.toFixed(2)}
                                        </span>
                                    </div>
                                    <div className="bg-[#2B3139]/30 p-2 rounded">
                                        <span className="block mb-1">RSI (4H)</span>
                                        <span className="font-mono font-bold text-[#EAECEF]">{decision.rsi_4h.toFixed(2)}</span>
                                    </div>
                                    <div className="bg-[#2B3139]/30 p-2 rounded">
                                        <span className="block mb-1">MACD (4H)</span>
                                        <span className={`font-mono font-bold ${decision.macd_4h > 0 ? 'text-green-400' : 'text-red-400'}`}>
                                            {decision.macd_4h.toFixed(4)}
                                        </span>
                                    </div>
                                </div>

                                {decision.execution_error && (
                                    <div className="text-red-400 text-xs mt-2 p-2 bg-red-500/5 rounded border border-red-500/20">
                                        执行错误: {decision.execution_error}
                                    </div>
                                )}

                                <div className="text-[10px] text-[#5E6673] font-mono mt-2 text-right">
                                    Strategy ID: {decision.strategy_id.split('_').pop()}
                                </div>
                            </div>
                        )}
                    </div>
                ))}
            </div>
        </div>
    );
}

