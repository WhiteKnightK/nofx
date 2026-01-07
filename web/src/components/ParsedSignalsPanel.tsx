import { useState, useMemo } from 'react'
import useSWR from 'swr'
import { api } from '../lib/api'

interface ParsedSignalsPanelProps {
  strategyStatuses?: any[]
}

/**
 * 全量解析信号库面板
 */
export function ParsedSignalsPanel({ strategyStatuses }: ParsedSignalsPanelProps) {
  const { data: signals, error, mutate } = useSWR(
    'parsed-signals',
    api.getParsedSignals,
    { refreshInterval: 10000 }
  )

  // 筛选和排序状态
  const [filterSymbol, setFilterSymbol] = useState('')
  const [sortBy, setSortBy] = useState<'alternate' | 'time' | 'symbol' | 'progress' | 'price'>('alternate')

  // 分页状态
  const [currentPage, setCurrentPage] = useState(1)
  const pageSize = 10

  // 处理数据过滤和排序
  const processedSignals = useMemo(() => {
    if (!signals || !Array.isArray(signals)) return []

    // 1. 基础过滤
    let list = [...signals]
    if (filterSymbol) {
      list = list.filter(s => s.symbol.toLowerCase().includes(filterSymbol.toLowerCase()))
    }

    // 2. 根据不同规则排序
    if (sortBy === 'time') {
      return list.sort((a, b) => new Date(b.received_at).getTime() - new Date(a.received_at).getTime())
    }

    if (sortBy === 'symbol') {
      return list.sort((a, b) => a.symbol.localeCompare(b.symbol))
    }

    if (sortBy === 'price') {
      return list.sort((a, b) => {
        const priceA = JSON.parse(a.content_json || '{}').entry?.price_target || 0
        const priceB = JSON.parse(b.content_json || '{}').entry?.price_target || 0
        return priceB - priceA
      })
    }

    // 默认：交替排序逻辑 (Alternate)
    // 逻辑：按交易对分组，组内按时间倒序，然后交叉取值
    const groups: Record<string, any[]> = {}
    list.forEach(s => {
      if (!groups[s.symbol]) groups[s.symbol] = []
      groups[s.symbol].push(s)
    })

    // 各组内部按时间倒序
    Object.keys(groups).forEach(sym => {
      groups[sym].sort((a, b) => new Date(b.received_at).getTime() - new Date(a.received_at).getTime())
    })

    // 获取所有交易对名称并按字母序排列 (BTC, ETH...)
    const sortedSymbols = Object.keys(groups).sort()

    const interleaved: any[] = []
    let hasMore = true
    let round = 0

    while (hasMore) {
      hasMore = false
      for (const sym of sortedSymbols) {
        if (groups[sym][round]) {
          interleaved.push(groups[sym][round])
          hasMore = true
        }
      }
      round++
    }

    return interleaved
  }, [signals, filterSymbol, sortBy])

  // 分页后的数据
  const totalPages = Math.ceil(processedSignals.length / pageSize)
  const paginatedSignals = useMemo(() => {
    const start = (currentPage - 1) * pageSize
    return processedSignals.slice(start, start + pageSize)
  }, [processedSignals, currentPage, pageSize])

  // 当筛选条件变化时重置到第一页
  useMemo(() => {
    setCurrentPage(1)
  }, [filterSymbol, sortBy])

  if (error) {
    return (
      <div className="rounded-2xl p-8 border border-rose-500/20 bg-rose-500/5 text-center">
        <div className="text-rose-400 font-bold mb-2 flex items-center justify-center gap-2">
          <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
          加载信号库失败
        </div>
        <button onClick={() => mutate()} className="text-xs text-rose-400/60 hover:text-rose-400 underline">
          重试
        </button>
      </div>
    )
  }

  return (
    <div className="rounded-2xl border border-[#2B3139] overflow-hidden shadow-2xl" style={{ backgroundColor: '#1E2329' }}>
      <div className="flex flex-col md:flex-row md:items-center justify-between p-5 border-b border-[#2B3139] bg-white/5 gap-4">
        <div className="flex items-center gap-3">
          <div className="w-10 h-10 rounded-xl bg-blue-500/10 flex items-center justify-center text-blue-400">
            <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10" />
            </svg>
          </div>
          <div>
            <h3 className="text-lg font-bold text-[#EAECEF]">全量策略信号库</h3>
            <p className="text-[10px] text-[#848E9C] uppercase tracking-wider font-semibold">SIGNAL REPOSITORY (LATEST 100)</p>
          </div>
        </div>

        {/* 筛选和工具栏 */}
        <div className="flex flex-wrap items-center gap-3">
          <div className="relative">
            <input
              type="text"
              placeholder="搜索交易对..."
              value={filterSymbol}
              onChange={(e) => setFilterSymbol(e.target.value)}
              className="bg-[#0B0E11] border border-[#2B3139] rounded-xl px-4 py-2 text-xs text-[#EAECEF] w-32 focus:border-blue-500 focus:outline-none transition-all"
            />
          </div>

          <select
            value={sortBy}
            onChange={(e: any) => setSortBy(e.target.value)}
            className="bg-[#0B0E11] border border-[#2B3139] rounded-xl px-3 py-2 text-xs text-[#EAECEF] focus:outline-none focus:border-blue-500 transition-all cursor-pointer"
          >
            <option value="alternate">交替排序 (默认)</option>
            <option value="time">按收到时间</option>
            <option value="symbol">按资产名称</option>
            <option value="price">按目标价格</option>
          </select>

          <button
            onClick={() => mutate()}
            className="px-4 py-2 rounded-xl text-xs font-bold bg-[#2B3139] text-[#EAECEF] hover:bg-white/10 active:scale-95 transition-all flex items-center gap-2 border border-white/5"
          >
            <svg className="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
            </svg>
            刷新
          </button>
        </div>
      </div>

      <div className="overflow-x-auto">
        <table className="w-full text-left text-sm border-collapse">
          <thead className="bg-black/20 text-[#848E9C] border-b border-[#2B3139]">
            <tr>
              <th className="px-6 py-4 text-[10px] font-black uppercase tracking-widest">周期 / ID</th>
              <th className="px-6 py-4 text-[10px] font-black uppercase tracking-widest">资产 / 方向</th>
              <th className="px-6 py-4 text-[10px] font-black uppercase tracking-widest">收到时间</th>
              <th className="px-6 py-4 text-[10px] font-black uppercase tracking-widest">执行进度</th>
              <th className="px-6 py-4 text-[10px] font-black uppercase tracking-widest text-right">目标价位</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-[#2B3139]">
            {(!paginatedSignals || paginatedSignals.length === 0) ? (
              <tr>
                <td colSpan={5} className="px-6 py-20 text-center text-[#5E6673]">
                  <div className="w-16 h-16 rounded-full bg-white/5 flex items-center justify-center mx-auto mb-4 opacity-20">
                    <svg className="w-8 h-8" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M20 13V6a2 2 0 00-2-2H6a2 2 0 00-2 2v7m16 0v5a2 2 0 01-2 2H6a2 2 0 01-2-2v-5m16 0h-2.586a1 1 0 00-.707.293l-2.414 2.414a1 1 0 01-.707.293h-3.172a1 1 0 01-.707-.293l-2.414-2.414A1 1 0 006.586 13H4" />
                    </svg>
                  </div>
                  <p className="text-sm font-medium">暂无匹配的信号记录</p>
                </td>
              </tr>
            ) : (
              (() => {
                // 先为每个交易对找出「最新」的一条信号（基于全部信号中的时间顺序）
                const latestBySymbol: Record<string, string> = {}
                if (signals && Array.isArray(signals)) {
                  [...signals].sort((a, b) => new Date(b.received_at).getTime() - new Date(a.received_at).getTime()).forEach((sig: any) => {
                    if (!latestBySymbol[sig.symbol]) {
                      latestBySymbol[sig.symbol] = sig.signal_id
                    }
                  })
                }

                return (paginatedSignals as any[]).map((sig: any) => {
                  const isLong = sig.direction === 'LONG'
                  const content = JSON.parse(sig.content_json || '{}')
                  const rawStatus = strategyStatuses?.find(
                    (s: any) => s.strategy_id === sig.signal_id
                  )
                  const isLatestForSymbol =
                    latestBySymbol[sig.symbol] === sig.signal_id

                  // 根据「是否为该交易对最新信号」+ 数据库状态，推导展示用状态
                  type DisplayKind = 'NONE' | 'WAITING' | 'RUNNING' | 'CLOSED' | 'EXPIRED'
                  let displayKind: DisplayKind = 'NONE'

                  if (rawStatus) {
                    if (!isLatestForSymbol && rawStatus.status !== 'CLOSED') {
                      // 同一币种的旧策略，且未真正 CLOSED，一律视为已过期
                      displayKind = 'EXPIRED'
                    } else if (rawStatus.status === 'CLOSED') {
                      displayKind = 'CLOSED'
                    } else if (rawStatus.status === 'WAITING') {
                      displayKind = 'WAITING'
                    } else {
                      // ENTRY / ADD_1 / ADD_2 等都归类为 RUNNING
                      displayKind = 'RUNNING'
                    }
                  } else {
                    // 没有任何执行记录：如果不是最新策略，也说明它已经被更新替代
                    displayKind = isLatestForSymbol ? 'NONE' : 'EXPIRED'
                  }

                  const realizedPnL =
                    rawStatus && typeof rawStatus.realized_pnl === 'number'
                      ? rawStatus.realized_pnl
                      : 0

                  // 徽章样式 & 文案
                  let badgeClass = ''
                  let dotClass = ''
                  let badgeText = ''
                  switch (displayKind) {
                    case 'WAITING':
                      badgeClass =
                        'text-yellow-500 bg-yellow-500/5 border-yellow-500/20'
                      dotClass = 'bg-yellow-500'
                      badgeText = '等待中'
                      break
                    case 'RUNNING':
                      badgeClass =
                        'text-emerald-500 bg-emerald-500/5 border-emerald-500/20 shadow-lg shadow-emerald-500/5'
                      dotClass = 'bg-emerald-500 animate-pulse'
                      // 对运行中的状态进行中文映射
                      const statusMap: Record<string, string> = {
                        'ENTRY': '已入场',
                        'ADD_1': '一次补仓',
                        'ADD_2': '二次补仓',
                        'RUNNING': '运行中'
                      }
                      badgeText = statusMap[rawStatus?.status || ''] || rawStatus?.status || '运行中'
                      break
                    case 'CLOSED':
                      badgeClass =
                        'text-gray-500 bg-gray-500/5 border-gray-500/20'
                      dotClass = 'bg-gray-500'
                      badgeText = '已关闭'
                      break
                    case 'EXPIRED':
                      badgeClass =
                        'text-orange-400 bg-orange-500/5 border-orange-500/30'
                      dotClass = 'bg-orange-400'
                      badgeText = '已过期'
                      break
                    case 'NONE':
                    default:
                      badgeClass = 'text-blue-400 bg-blue-500/5 border-blue-500/20'
                      dotClass = 'bg-blue-400'
                      badgeText = '等待入场'
                      break
                  }

                  // 处理 ID 显示：如果是超长哈希，只显示前 8 位
                  const displayId = sig.signal_id.length > 16
                    ? sig.signal_id.substring(0, 8).toUpperCase()
                    : (sig.signal_id.split('_').pop()?.toUpperCase() || 'SIGNAL');

                  return (
                    <tr key={sig.signal_id} className="group hover:bg-white/[0.02] transition-colors">
                      <td className="px-6 py-5">
                        <div className="font-black text-[#EAECEF] text-xs tracking-tighter">
                          {displayId}
                        </div>
                        <div className="text-[9px] text-[#5E6673] font-mono mt-1 opacity-40 group-hover:opacity-100 transition-opacity">
                          {sig.signal_id.substring(0, 16)}...
                        </div>
                      </td>
                      <td className="px-6 py-5">
                        <div className="flex items-center gap-2">
                          <span className="font-black text-[#EAECEF] tracking-tight">{sig.symbol}</span>
                          <span className={`text-[9px] px-1.5 py-0.5 rounded font-black border uppercase ${isLong
                              ? 'bg-emerald-500/10 text-emerald-500 border-emerald-500/20'
                              : 'bg-rose-500/10 text-rose-500 border-rose-500/20'
                            }`}>
                            {isLong ? 'Buy Long' : 'Sell Short'}
                          </span>
                        </div>
                      </td>
                      <td className="px-6 py-5">
                        <div className="text-[#848E9C] text-xs font-medium">
                          {new Date(sig.received_at).toLocaleDateString()}
                        </div>
                        <div className="text-[10px] text-[#5E6673] mt-0.5 font-mono">
                          {new Date(sig.received_at).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })}
                        </div>
                      </td>
                      <td className="px-6 py-5">
                        <div className="flex flex-col gap-1.5">
                          <div
                            className={`text-[10px] px-2 py-0.5 rounded-lg inline-flex items-center gap-1.5 font-black border uppercase ${badgeClass}`}
                          >
                            <span className={`w-1 h-1 rounded-full ${dotClass}`}></span>
                            {badgeText}
                          </div>
                          {/* 只有真正 CLOSED 的策略才展示已实现盈亏 */}
                          {displayKind === 'CLOSED' && realizedPnL !== 0 && (
                            <div
                              className={`text-[10px] font-black tracking-tighter ${realizedPnL > 0 ? 'text-emerald-500' : 'text-rose-500'
                                }`}
                            >
                              {realizedPnL > 0 ? '▲' : '▼'}{' '}
                              {Math.abs(realizedPnL).toFixed(2)} USDT
                            </div>
                          )}
                        </div>
                      </td>
                      <td className="px-6 py-5 text-right">
                        <div className="text-xs text-[#EAECEF] font-black tracking-tight">
                          {content.entry?.price_target || '—'}
                        </div>
                        <div className="flex items-center justify-end gap-2 mt-1.5 text-[10px] font-bold">
                          <span className="text-rose-500/80">SL: {content.stop_loss?.price || '—'}</span>
                          <div className="w-px h-2 bg-[#2B3139]"></div>
                          <span className="text-emerald-500/80">TP: {(content.take_profits?.[0]?.price) || '—'}</span>
                        </div>
                      </td>
                    </tr>
                  )
                })
              })()
            )}
          </tbody>
        </table>
      </div>

      {/* 分页控制 */}
      {totalPages > 1 && (
        <div className="flex items-center justify-between p-4 border-t border-[#2B3139] bg-black/20">
          <div className="text-xs text-[#848E9C]">
            共 {processedSignals.length} 条记录，第 {currentPage} / {totalPages} 页
          </div>
          <div className="flex items-center gap-2">
            <button
              onClick={() => setCurrentPage(p => Math.max(1, p - 1))}
              disabled={currentPage === 1}
              className="px-3 py-1.5 rounded-lg text-xs font-bold border transition-all disabled:opacity-30 disabled:cursor-not-allowed"
              style={{
                background: currentPage === 1 ? '#1E2329' : '#2B3139',
                borderColor: '#3B4149',
                color: '#EAECEF'
              }}
            >
              ← 上一页
            </button>

            {/* 页码按钮 */}
            <div className="flex items-center gap-1">
              {Array.from({ length: Math.min(5, totalPages) }, (_, i) => {
                let pageNum: number
                if (totalPages <= 5) {
                  pageNum = i + 1
                } else if (currentPage <= 3) {
                  pageNum = i + 1
                } else if (currentPage >= totalPages - 2) {
                  pageNum = totalPages - 4 + i
                } else {
                  pageNum = currentPage - 2 + i
                }
                return (
                  <button
                    key={pageNum}
                    onClick={() => setCurrentPage(pageNum)}
                    className={`w-8 h-8 rounded-lg text-xs font-bold transition-all ${currentPage === pageNum
                        ? 'bg-blue-500 text-white'
                        : 'bg-[#2B3139] text-[#848E9C] hover:bg-[#3B4149]'
                      }`}
                  >
                    {pageNum}
                  </button>
                )
              })}
            </div>

            <button
              onClick={() => setCurrentPage(p => Math.min(totalPages, p + 1))}
              disabled={currentPage === totalPages}
              className="px-3 py-1.5 rounded-lg text-xs font-bold border transition-all disabled:opacity-30 disabled:cursor-not-allowed"
              style={{
                background: currentPage === totalPages ? '#1E2329' : '#2B3139',
                borderColor: '#3B4149',
                color: '#EAECEF'
              }}
            >
              下一页 →
            </button>
          </div>
        </div>
      )}
    </div>
  )
}

