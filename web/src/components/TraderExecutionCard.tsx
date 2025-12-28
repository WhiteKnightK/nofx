import { useState } from 'react';
import { Activity, ArrowDown, ArrowUp, Layers, AlertTriangle, FileText } from 'lucide-react';
import {
    ResponsiveContainer,
    ComposedChart,
    XAxis,
    YAxis,
    Tooltip,
    Scatter,
    Line,
    ReferenceLine,
} from 'recharts';
import { ModernModal } from './Toast';
import type { Position } from '../types';

// 智能内容渲染组件 (适配 Markdown, HTML 和 纯文本邮件)
export function SmartContentRenderer({ content }: { content: string }) {
    if (!content) return <div className="text-gray-500 italic text-sm">暂无详细内容</div>;

    // 预处理：将 &nbsp; 等空格实体转换为普通空格，避免直接显示实体
    const normalized = content
        .replace(/&nbsp;/gi, ' ')
        .replace(/\u00A0/g, ' ');

    // 检测是否为 HTML 内容 (包含常见 HTML 标签)
    const isHtml = /<[a-z][\s\S]*>/i.test(normalized) || /&nbsp;|&lt;|&gt;/i.test(normalized);

    if (isHtml) {
        return (
            <div 
                className="prose prose-invert prose-sm max-w-none text-[#EAECEF] bg-[#0B0E11] p-4 rounded-lg border border-[#2B3139]"
                style={{ fontFamily: 'sans-serif' }}
            >
                <div dangerouslySetInnerHTML={{ __html: normalized }} />
            </div>
        );
    }

    // 如果不是 HTML，使用增强版 Markdown/Text 渲染器
    return (
        <div className="text-[#EAECEF] space-y-2 font-sans text-sm md:text-base leading-relaxed bg-[#0B0E11] p-4 rounded-lg border border-[#2B3139]">
            {normalized.split('\n').map((line, i) => {
                const trimmed = line.trim();
                
                // 处理空行
                if (trimmed === '') return <br key={i} className="mb-2" />;

                // 1. 处理标题 (### )
                if (trimmed.startsWith('### ')) {
                    return <h3 key={i} className="text-lg font-bold text-[#F0B90B] mt-4 mb-2 pb-1 border-b border-gray-700">{trimmed.replace(/^#+\s+/, '')}</h3>
                }
                if (trimmed.startsWith('## ')) {
                    return <h2 key={i} className="text-xl font-bold text-[#F0B90B] mt-5 mb-3">{trimmed.replace(/^#+\s+/, '')}</h2>
                }
                if (trimmed.startsWith('# ')) {
                    return <h1 key={i} className="text-2xl font-bold text-white mt-6 mb-4">{trimmed.replace(/^#+\s+/, '')}</h1>
                }

                // 2. 处理邮件头
                const isHeader = /^(发件人|收件人|主题|时间|From|To|Subject|Date)[:：]/i.test(trimmed);
                if (isHeader) {
                    const [label, ...valueParts] = trimmed.split(/[:：]/);
                    const value = valueParts.join('：').trim();
                    return (
                        <div key={i} className="text-sm border-l-2 border-[#F0B90B] pl-2 py-0.5 my-1 bg-[#2B3139]/30">
                            <span className="text-[#848E9C] font-semibold">{label}: </span>
                            <span className="text-[#EAECEF] font-mono">{value}</span>
                        </div>
                    );
                }

                // 3. 处理列表
                if (trimmed.match(/^(\*|-)\s/)) {
                    const content = trimmed.replace(/^(\*|-)\s/, '');
                    const parts = content.split(/(\*\*.*?\*\*)/g);
                    
                    return (
                        <div key={i} className="flex gap-2 ml-2 my-1">
                            <span className="text-[#F0B90B]">•</span>
                            <span>
                                {parts.map((part, idx) => {
                                    if (part.startsWith('**') && part.endsWith('**')) {
                                        return <strong key={idx} className="text-white font-semibold">{part.slice(2, -2)}</strong>
                                    }
                                    return part;
                                })}
                            </span>
                        </div>
                    )
                }

                // 4. 处理加粗
                if (trimmed.includes('**')) {
                    const parts = trimmed.split(/(\*\*.*?\*\*)/g);
                    return (
                        <p key={i} className="text-gray-300">
                            {parts.map((part, idx) => {
                                if (part.startsWith('**') && part.endsWith('**')) {
                                    return <strong key={idx} className="text-white font-semibold">{part.slice(2, -2)}</strong>
                                }
                                return part;
                            })}
                        </p>
                    )
                }
                
                return <p key={i} className="text-gray-300">{line}</p>
            })}
        </div>
    )
}

interface TraderExecutionCardProps {
  traderId: string
  strategy: any
  currentPrice: number
  updatedAt: string
  status?: any
  position?: Position
}

// 【功能】策略关键价位可视化图表
const StrategyLevelsChart = ({
    slPrice,
    entryPrice,
    tpPrices,
    addPrices,
    currentPrice,
}: {
    slPrice: number;
    entryPrice: number;
    tpPrices: number[];
    addPrices: number[];
    currentPrice: number;
}) => {
    const levels = [
        { label: 'SL', price: slPrice, type: 'sl' },
        { label: 'Entry', price: entryPrice, type: 'entry' },
        ...addPrices.map((p, idx) => ({ label: `Add${idx + 1}`, price: p, type: 'add' })),
        ...tpPrices.map((p, idx) => ({ label: `TP${idx + 1}`, price: p, type: 'tp' })),
        { label: 'Now', price: currentPrice, type: 'now' },
    ].filter((x) => Number.isFinite(x.price));

    const minPrice = Math.min(...levels.map((l) => l.price));
    const maxPrice = Math.max(...levels.map((l) => l.price));
    const padding = Math.max(1, (maxPrice - minPrice) * 0.05);

    const colorMap: Record<string, string> = {
        sl: '#F6465D',
        entry: '#60A5FA',
        add: '#A78BFA',
        tp: '#0ECB81',
        now: '#F0B90B',
    };

    return (
        <div className="bg-[#0B0E11] rounded-lg border border-[#2B3139] p-4">
            <div className="flex items-center justify-between mb-3">
                <div className="text-sm font-semibold text-[#EAECEF]">Strategy Map</div>
                <div className="text-xs text-[#848E9C]">关键价位快速预览</div>
            </div>
            <div className="h-56">
                <ResponsiveContainer width="100%" height="100%">
                    <ComposedChart data={levels} margin={{ left: 8, right: 8, top: 10, bottom: 10 }}>
                        <XAxis
                            dataKey="price"
                            type="number"
                            domain={[minPrice - padding, maxPrice + padding]}
                            tickFormatter={(v) => v.toFixed(0)}
                            stroke="#5E6673"
                            tick={{ fill: '#A0AEC0', fontSize: 12 }}
                        />
                        <YAxis hide type="category" dataKey="label" />
                        <Tooltip
                            contentStyle={{
                                background: '#11151A',
                                border: '1px solid #2B3139',
                                borderRadius: 8,
                                color: '#EAECEF',
                            }}
                            formatter={(value: number, _name, item) => [`${value}`, item.payload.label]}
                        />
                        <ReferenceLine
                            x={currentPrice}
                            stroke="#F0B90B"
                            strokeDasharray="4 4"
                            strokeWidth={2}
                            label={{ position: 'top', value: 'Now', fill: '#F0B90B', fontSize: 12 }}
                        />
                        <Line
                            type="monotone"
                            dataKey="price"
                            stroke="#2B3139"
                            strokeWidth={2}
                            dot={false}
                            isAnimationActive={false}
                        />
                        <Scatter
                            dataKey="price"
                            fill="#EAECEF"
                            shape={(props: any) => {
                                const color = colorMap[props.payload.type] || '#EAECEF';
                                return (
                                    <circle
                                        cx={props.cx}
                                        cy={props.cy}
                                        r={6}
                                        fill={color}
                                        stroke="#0B0E11"
                                        strokeWidth={2}
                                    />
                                );
                            }}
                            isAnimationActive={false}
                        />
                    </ComposedChart>
                </ResponsiveContainer>
            </div>
        </div>
    );
};

export function TraderExecutionCard({ traderId, strategy, status: traderStatus, currentPrice, updatedAt, position }: TraderExecutionCardProps) {
  const [showDetails, setShowDetails] = useState(false);
  
  if (!strategy) return null;

  const current_price = currentPrice;
  const globalUpdatedAt = updatedAt;
  const isLong = strategy.direction.toUpperCase() === 'LONG';
  
  // 交易员实际持仓（如果存在）
  const positionEntryPrice = position?.entry_price ?? 0
  const positionUnrealizedPnlPct = position?.unrealized_pnl_pct

  // 入场价展示优先使用真实持仓入场价，其次使用策略计划价
  const displayEntryPrice = positionEntryPrice || strategy.entry.price_target

  // 交易员实际状态记录（用于生命周期/已实现盈亏）
  const executionStatus = traderStatus?.status || 'WAITING';
  const realizedPnL = traderStatus?.realized_pnl || 0;

  // 计算理论浮动盈亏% (基于全局策略入场价)
  let theoreticalPnlPercent = 0;
  if (strategy.entry.price_target > 0 && current_price > 0) {
      if (isLong) {
          theoreticalPnlPercent = ((current_price - strategy.entry.price_target) / strategy.entry.price_target) * 100 * strategy.leverage_recommend;
      } else {
          theoreticalPnlPercent = ((strategy.entry.price_target - current_price) / strategy.entry.price_target) * 100 * strategy.leverage_recommend;
      }
  }

  // 计算实际浮动盈亏%：
  // 优先使用交易所返回的未实现收益率（更精确），无持仓时显示 0
  let actualPnlPercent = 0
  if (position && typeof positionUnrealizedPnlPct === 'number') {
    actualPnlPercent = positionUnrealizedPnlPct
  }

  // 进度条计算逻辑
  const entryPrice = strategy.entry.price_target;
  const tp1Price = strategy.take_profits?.[0]?.price || (isLong ? entryPrice * 1.05 : entryPrice * 0.95);
  const slPrice = strategy.stop_loss?.price || (isLong ? entryPrice * 0.95 : entryPrice * 1.05);

  const totalRange = Math.abs(tp1Price - slPrice);
  
  // 计算当前价格进度
  let progress = 0;
  if (totalRange > 0) {
      if (isLong) {
          progress = ((current_price - slPrice) / totalRange) * 100;
      } else {
          progress = ((slPrice - current_price) / totalRange) * 100;
      }
  }
  const cursorPosition = Math.min(Math.max(progress, 0), 100);

  // 计算实际入场点在进度条上的位置
  const getPosition = (price: number) => {
      if (totalRange <= 0) return 0;
      if (isLong) {
          return Math.min(Math.max(((price - slPrice) / totalRange) * 100, 0), 100);
      } else {
          return Math.min(Math.max(((slPrice - price) / totalRange) * 100, 0), 100);
      }
  }

  const theoreticalEntryPos = getPosition(entryPrice);
  const actualEntryPos = positionEntryPrice > 0 ? getPosition(positionEntryPrice) : -1;

  // 状态颜色映射
  const getStatusColor = (status: string) => {
      switch (status) {
          case 'WAITING': return 'text-yellow-500 bg-yellow-500/10 border-yellow-500/30';
          case 'ENTRY': return 'text-blue-500 bg-blue-500/10 border-blue-500/30';
          case 'ADD_1': 
          case 'ADD_2': return 'text-purple-500 bg-purple-500/10 border-purple-500/30';
          case 'CLOSED': return 'text-gray-400 bg-gray-500/10 border-gray-500/30';
          default: return 'text-gray-400';
      }
  };

  const directionLabel = isLong ? '做多' : '做空'

  return (
  <>
    <div className="bg-gradient-to-br from-[#11151A] via-[#1E2329] to-[#141A1F] rounded-2xl border border-[#2B3139] shadow-[0_18px_45px_rgba(0,0,0,0.65)] relative overflow-hidden group hover:border-[#F0B90B]/60 hover:shadow-[0_22px_60px_rgba(240,185,11,0.25)] transition-all duration-300 mb-8">
       {/* 顶部指示条 */}
       <div className={`absolute top-0 left-0 right-0 h-1 ${isLong ? 'bg-green-500' : 'bg-red-500'}`} />

       <div className="p-5 flex flex-col md:flex-row gap-6">
          
          {/* 左侧：策略概览 */}
          <div className="flex-shrink-0 md:w-64 flex flex-col justify-between">
              <div>
                  <div className="flex items-center gap-2 mb-2">
                      <span className="px-3 py-1 bg-[#2B3139] text-[#C4CCD6] text-xs rounded border border-[#474D57] font-mono">
                          {strategy.signal_id.split('_').pop() || 'SIGNAL'}
                      </span>
                      <span className="text-sm text-[#F0B90B] flex items-center gap-1">
                          <Activity size={10} /> 
                          📡 跟随全局策略
                      </span>
                  </div>
                  <div className="flex items-center gap-3 mb-2">
                      <h2 className="text-4xl font-bold text-[#EAECEF] tracking-tight">{strategy.symbol}</h2>
                  </div>
                  <div className="flex items-center gap-2 mb-4">
                      <div className={`flex items-center gap-1 px-3 py-1 rounded text-base font-bold ${
                          isLong ? 'bg-green-500/10 text-green-500' : 'bg-red-500/10 text-red-500'
                      }`}>
                          {isLong ? <ArrowUp size={16} /> : <ArrowDown size={16} />}
                          <span>{directionLabel}</span>
                          <span className="text-xs opacity-70 ml-1">({strategy.direction})</span>
                      </div>
                      <div className={`px-3 py-1 rounded text-sm font-bold border ${getStatusColor(executionStatus)}`}>
                          {executionStatus}
                      </div>
                  </div>
              </div>

              <div>
                  <div className="grid grid-cols-2 gap-4 mt-2">
                      <div>
                          <div className="text-sm text-[#848E9C] mb-1">执行浮盈/亏 (Actual)</div>
                          <div className={`text-2xl font-mono font-bold ${actualPnlPercent >= 0 ? 'text-green-500' : 'text-red-500'}`}>
                              {actualPnlPercent > 0 ? '+' : ''}{actualPnlPercent.toFixed(2)}%
                          </div>
                          <div className="text-xs text-[#5E6673] mt-0.5">
                              Target: {theoreticalPnlPercent > 0 ? '+' : ''}{theoreticalPnlPercent.toFixed(2)}%
                          </div>
                      </div>
                       <div>
                           <div className="text-sm text-[#848E9C] mb-1">已实现盈亏</div>
                           <div className={`text-2xl font-mono font-bold ${realizedPnL >= 0 ? 'text-green-500' : 'text-red-500'}`}>
                              {realizedPnL > 0 ? '+' : ''}{realizedPnL.toFixed(2)}
                           </div>
                      </div>
                  </div>
              </div>
          </div>

          {/* 中间：可视化进度 */}
          <div className="flex-1 flex flex-col justify-center py-3">
              {/* 关键价格概览 */}
              <div className="grid grid-cols-3 gap-4 text-sm text-[#A0AEC0] mb-3">
                  <div className="flex flex-col">
                      <span className="tracking-wider text-[#5E6673]">当前价格</span>
                      <span className="font-mono text-base text-[#F0B90B]">{current_price.toFixed(2)}</span>
                  </div>
                  <div className="flex flex-col">
                      <span className="tracking-wider text-[#5E6673]">入场价</span>
                      <span className="font-mono text-base text-[#60A5FA]">{displayEntryPrice.toFixed(2)}</span>
                  </div>
                  <div className="flex flex-col">
                      <span className="tracking-wider text-[#5E6673]">止损 / 第一止盈</span>
                      <span className="font-mono text-base">
                          <span className="text-[#F6465D] mr-2">{slPrice.toFixed(2)}</span>
                          <span className="text-[#0ECB81]">{tp1Price.toFixed(2)}</span>
                      </span>
                  </div>
              </div>

              {/* 进度条轨道 */}
              <div className="relative h-2 bg-[#2B3139] rounded-full w-full my-8">
                  
                  {/* SL 标记 */}
                  <div className="absolute top-1/2 -translate-y-1/2 w-3 h-3 bg-red-500 rounded-full border-2 border-[#1E2329] z-10" style={{ left: '0%' }}></div>
                  <div className="absolute -bottom-9 left-0 -translate-x-1/2 flex flex-col items-center">
                      <span className="text-xs text-red-500 font-bold">止损 SL</span>
                      <span className="text-xs text-[#848E9C] font-mono">{slPrice}</span>
                  </div>

                  {/* 理论 Entry 标记 (虚线/半透明) */}
                  <div className="absolute top-1/2 -translate-y-1/2 w-2 h-2 bg-blue-500/50 rounded-full z-10" style={{ left: `${theoreticalEntryPos}%` }}></div>
                  <div className="absolute -top-9 -translate-x-1/2 flex flex-col items-center" style={{ left: `${theoreticalEntryPos}%` }}>
                      <span className="text-xs text-[#848E9C] font-mono">{entryPrice}</span>
                      <span className="text-xs text-blue-500/50 font-bold">计划入场</span>
                  </div>

                  {/* 实际 Entry 标记 (实心) */}
                  {actualEntryPos >= 0 && (
                      <>
                      <div className="absolute top-1/2 -translate-y-1/2 w-3 h-3 bg-blue-500 rounded-full border-2 border-[#1E2329] z-20" style={{ left: `${actualEntryPos}%` }}></div>
                      <div className="absolute top-4 -translate-x-1/2 flex flex-col items-center" style={{ left: `${actualEntryPos}%` }}>
                          <span className="text-[10px] text-blue-500 font-bold">Actual</span>
                          <span className="text-[10px] text-[#EAECEF] font-mono">{positionEntryPrice.toFixed(2)}</span>
                      </div>
                      </>
                  )}

                  {/* TP1 标记 */}
                  <div className="absolute top-1/2 -translate-y-1/2 w-3 h-3 bg-green-500 rounded-full border-2 border-[#1E2329] z-10" style={{ left: '100%' }}></div>
                  <div className="absolute -bottom-9 right-0 translate-x-1/2 flex flex-col items-center">
                      <span className="text-xs text-green-500 font-bold">TP1 止盈</span>
                      <span className="text-xs text-[#848E9C] font-mono">{tp1Price}</span>
                  </div>

                  {/* 当前价格游标 */}
                  <div 
                      className="absolute top-1/2 -translate-y-1/2 z-30 transition-all duration-1000 ease-out"
                      style={{ left: `${cursorPosition}%` }}
                  >
                      <div className="-translate-x-1/2 flex flex-col items-center">
                          <div className="w-4 h-4 bg-[#EAECEF] rounded-full border-4 border-[#F0B90B] shadow-[0_0_10px_rgba(240,185,11,0.5)]" />
                          <div className="mt-2 text-xs text-[#F0B90B] font-mono">
                              现价 {current_price.toFixed(2)}
                          </div>
                      </div>
                  </div>
              </div>

              {/* 进度文字说明 */}
                  <div className="flex justify-between text-sm text-[#A0AEC0] mt-1">
                  <span>
                      当前价格朝止盈方向前进：
                      <span className="font-mono text-[#F0B90B] ml-1">
                          {cursorPosition.toFixed(0)}%
                      </span>
                  </span>
                  <span>
                      模式：{isLong ? '做多，价格越高越接近 TP1' : '做空，价格越低越接近 TP1'}
                  </span>
              </div>

              {/* 补仓状态 */}
              <div className="mt-6 bg-[#0B0E11] rounded-lg border border-[#2B3139] p-3 flex justify-between items-center">
                  <div className="flex items-center gap-4">
                      <div className="text-sm text-[#848E9C] flex items-center gap-1">
                          <Layers size={12} /> 执行步骤 (Execution Steps)
                      </div>
                      {strategy.adds && strategy.adds.length > 0 ? (
                          <div className="flex gap-2">
                              {strategy.adds.map((add: any, idx: number) => {
                                  const stepName = `ADD_${idx + 1}`;
                                  const isCompleted = executionStatus === stepName || executionStatus === `ADD_${idx + 2}` || (executionStatus === 'CLOSED' && realizedPnL !== 0);
                                  
                                  return (
                                      <div key={idx} className={`text-xs px-2 py-0.5 rounded border ${
                                          isCompleted
                                          ? 'bg-green-500/10 border-green-500/30 text-green-400'
                                          : 'bg-[#2B3139] border-[#474D57] text-[#848E9C]'
                                      }`}>
                                          ADD #{idx + 1} @ {add.price}
                                      </div>
                                  )
                              })}
                          </div>
                      ) : (
                          <span className="text-sm text-[#5E6673] italic">No adds planned</span>
                      )}
                  </div>
                  
                  <div className="text-sm text-[#5E6673] font-mono">
                      Last Update: {new Date(traderStatus?.updated_at || globalUpdatedAt).toLocaleTimeString()}
                  </div>
              </div>
          </div>

          {/* 右侧：操作区 */}
          <div className="flex-shrink-0 md:w-32 flex flex-col justify-end border-l border-[#2B3139] pl-6 ml-2">
              <button 
                  onClick={() => setShowDetails(true)}
                  className="w-full py-2 bg-[#2B3139] hover:bg-[#363C45] text-[#EAECEF] text-xs font-medium rounded transition-colors flex items-center justify-center gap-1 group"
              >
                  <FileText size={12} />
                  策略详情
              </button>
          </div>
       </div>
    </div>

    {/* 详情弹窗 */}
    <ModernModal
        isOpen={showDetails}
        onClose={() => setShowDetails(false)}
        title="📝 完整策略分析报告"
        size="lg"
    >
        <div className="space-y-6">
            <div className="bg-[#2B3139]/50 p-4 rounded-lg border border-[#474D57]/50">
                <div className="text-xs text-[#848E9C] uppercase tracking-wider mb-2 font-bold">Strategy Summary</div>
                <p className="text-[#EAECEF] text-sm leading-relaxed">{strategy.raw_text_summary}</p>
            </div>
            <div>
                <div className="text-xs text-[#848E9C] uppercase tracking-wider mb-4 font-bold border-b border-[#2B3139] pb-2">Full Analysis</div>
                {strategy.raw_content ? (
                    <div className="space-y-4">
                        <StrategyLevelsChart
                            slPrice={strategy.stop_loss?.price || 0}
                            entryPrice={strategy.entry?.price_target || 0}
                            tpPrices={(strategy.take_profits || []).map((tp: any) => tp.price)}
                            addPrices={(strategy.adds || []).map((a: any) => a.price)}
                            currentPrice={current_price || 0}
                        />
                        <div className="max-h-[60vh] overflow-y-auto">
                            <SmartContentRenderer content={strategy.raw_content} />
                        </div>
                    </div>
                ) : (
                    <div className="text-center py-10 text-gray-500">
                        <AlertTriangle className="w-8 h-8 mx-auto mb-2 opacity-50" />
                        <p>暂无完整报告内容</p>
                    </div>
                )}
            </div>
        </div>
    </ModernModal>
    </>
  );
}
