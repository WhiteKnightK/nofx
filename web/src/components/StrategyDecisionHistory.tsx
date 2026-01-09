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
    wait: '观望',
    open: '开仓类',
    close: '平仓类',
    limit: '限价委托',
    cancel: '撤单',
    sltp: '止盈止损'
};

// 判断动作属于哪个分类
const getActionCategory = (action: string): string => {
    const lowerAction = action.toLowerCase();

    // 观望类
    if (lowerAction === 'wait' || lowerAction === 'hold') return 'wait';

    // 限价委托类 (place_long_order, place_short_order)
    if (lowerAction.includes('place_') && lowerAction.includes('_order')) return 'limit';

    // 撤单类
    if (lowerAction === 'cancel_order' || lowerAction.includes('cancel')) return 'cancel';

    // 开仓类 (open_long, open_short, add_long, add_short)
    if (lowerAction.includes('open_') || lowerAction.includes('add_')) return 'open';

    // 平仓类 (close_long, close_short, partial_close, emergency_close)
    if (lowerAction.includes('close') || lowerAction.includes('partial')) return 'close';

    // 止盈止损类 (set_tp_order, set_sl_order, update_stop_loss, update_take_profit)
    if (lowerAction.includes('tp') || lowerAction.includes('sl') ||
        lowerAction.includes('stop') || lowerAction.includes('profit') ||
        lowerAction.includes('loss')) return 'sltp';

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
        const upperAction = action.toUpperCase();
        // 开多仓类 (green)
        if (upperAction.includes('LONG') && !upperAction.includes('CLOSE')) return 'text-green-500 bg-green-500/10 border-green-500/20';
        // 开空仓类 (red)
        if (upperAction.includes('SHORT') && !upperAction.includes('CLOSE')) return 'text-red-500 bg-red-500/10 border-red-500/20';
        // 平仓类 (pink/rose)
        if (upperAction.includes('CLOSE')) return 'text-rose-400 bg-rose-400/10 border-rose-400/20';
        // 观望 (yellow)
        if (upperAction === 'WAIT' || upperAction === 'HOLD') return 'text-yellow-500 bg-yellow-500/10 border-yellow-500/20';
        // 限价委托 (orange)
        if (upperAction.includes('PLACE_') && upperAction.includes('_ORDER')) return 'text-orange-400 bg-orange-400/10 border-orange-400/20';
        // 撤单 (purple)
        if (upperAction.includes('CANCEL')) return 'text-purple-400 bg-purple-400/10 border-purple-400/20';
        // 止盈止损 (blue)
        if (upperAction.includes('STOP') || upperAction.includes('TP') || upperAction.includes('SL')) return 'text-blue-500 bg-blue-500/10 border-blue-500/20';
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

