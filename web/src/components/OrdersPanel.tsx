import useSWR from 'swr'
import { api } from '../lib/api'

interface OrdersPanelProps {
  traderId: string
  symbol?: string
}

/**
 * 委托单展示面板（含止盈止损计划委托）
 */
// 小工具：格式化方向/动作/时间，尽量贴近交易所展示
const formatDirection = (order: any) => {
  const side = (order.side || order.holdSide || '').toString().toLowerCase()
  const tradeSide = (order.trade_side || order.tradeSide || '').toString().toLowerCase()
  const posSide = (order.pos_side || order.posSide || '').toString().toLowerCase()
  const reduceOnly = !!(order.reduce_only ?? order.reduceOnly)

  if (!side && !tradeSide && !posSide) return '—'

  // Bitget 常见组合：tradeSide=open/close + side=buy/sell
  if ((tradeSide === 'open' || tradeSide === 'close') && (side === 'buy' || side === 'sell')) {
    if (tradeSide === 'open' && side === 'buy') return '开多'
    if (tradeSide === 'open' && side === 'sell') return '开空'
    if (tradeSide === 'close' && side === 'buy') return '平空'
    if (tradeSide === 'close' && side === 'sell') return '平多'
  }

  // reduceOnly 兜底：只减仓时 buy/sell 更像平仓方向
  if (reduceOnly && (side === 'buy' || side === 'sell')) {
    if (side === 'buy') return '平空'
    if (side === 'sell') return '平多'
  }

  // 已有标准枚举
  if (side === 'open_long') return '开多'
  if (side === 'open_short') return '开空'
  if (side === 'close_long') return '平多'
  if (side === 'close_short') return '平空'
  if (side === 'long') return '多'
  if (side === 'short') return '空'

  // posSide + buy/sell 兜底（不同交易所/版本字段差异）
  if ((posSide === 'long' || posSide === 'short') && (side === 'buy' || side === 'sell')) {
    if (posSide === 'long' && side === 'buy') return '开多'
    if (posSide === 'short' && side === 'sell') return '开空'
    if (posSide === 'long' && side === 'sell') return '平多'
    if (posSide === 'short' && side === 'buy') return '平空'
  }

  // 兜底处理包含这些关键字的情况
  if (side.includes('open') && side.includes('long')) return '开多'
  if (side.includes('open') && side.includes('short')) return '开空'
  if (side.includes('close') && side.includes('long')) return '平多'
  if (side.includes('close') && side.includes('short')) return '平空'

  // buy/sell 最后兜底展示
  if (side === 'buy') return '买入'
  if (side === 'sell') return '卖出'
  return side || tradeSide || posSide
}

const formatStatus = (status: string) => {
  const s = (status || '').toLowerCase()
  if (s === 'live') return '进行中'
  if (s === 'partially_filled') return '部分成交'
  if (s === 'filled') return '已成交'
  if (s === 'canceled') return '已取消'
  return status || '—'
}

const formatCreatedAt = (value: any) => {
  if (!value) return ''
  let ts = value
  if (typeof value === 'string') {
    const parsed = parseInt(value, 10)
    if (!isNaN(parsed)) ts = parsed
  }
  if (typeof ts === 'number') {
    // 兼容秒和毫秒
    if (ts < 10000000000) ts *= 1000
    const date = new Date(ts)
    return `${date.getMonth() + 1}-${date.getDate()} ${date.getHours().toString().padStart(2, '0')}:${date.getMinutes().toString().padStart(2, '0')}:${date.getSeconds().toString().padStart(2, '0')}`
  }
  return ''
}

export function OrdersPanel({ traderId, symbol }: OrdersPanelProps) {
  const { data, error, mutate } = useSWR(
    traderId ? `/api/orders?trader_id=${traderId}${symbol ? `&symbol=${symbol}` : ''}` : null,
    () => api.getOrders(traderId, symbol),
    {
      refreshInterval: 5000,
      keepPreviousData: true
    }
  )

  if (error) {
    return (
      <div className="rounded-lg p-4" style={{ backgroundColor: '#1E2329' }}>
        <div className="flex items-center gap-2 mb-3">
          <span className="text-lg">📋</span>
          <span className="font-semibold" style={{ color: '#EAECEF' }}>
            当前委托
          </span>
        </div>
        <div className="text-center py-4" style={{ color: '#848E9C' }}>
          加载委托失败: {error.message}
        </div>
      </div>
    )
  }

  const orders = data?.orders || []

  // 按类型分组
  const planOrders = orders.filter((o: any) => o.order_category === 'plan')
  const normalOrders = orders.filter((o: any) => o.order_category === 'normal')

  return (
    <div className="rounded-lg p-4" style={{ backgroundColor: '#1E2329' }}>
      <div className="flex items-center justify-between mb-4">
        <div className="flex items-center gap-2">
          <span className="text-lg">📋</span>
          <span className="font-semibold text-lg" style={{ color: '#EAECEF' }}>
            当前委托
          </span>
          <span
            className="text-xs px-2 py-0.5 rounded"
            style={{ backgroundColor: '#2B3139', color: '#848E9C' }}
          >
            共 {orders.length} 个
          </span>
        </div>
        <button
          onClick={() => mutate()}
          className="text-xs px-2 py-1 rounded hover:bg-[#2B3139]"
          style={{ color: '#F0B90B' }}
        >
          刷新
        </button>
      </div>

      {orders.length === 0 ? (
        <div className="text-center py-10 bg-[#191D23] rounded-lg border border-dashed border-[#2B3139]">
          <div className="text-4xl mb-2 opacity-20">📋</div>
          <div className="text-sm" style={{ color: '#848E9C' }}>
            暂无活跃委托单
          </div>
        </div>
      ) : (
        <div className="space-y-6">
          {/* 计划委托 (止盈止损) */}
          {planOrders.length > 0 && (
            <div>
              <div className="flex items-center gap-2 mb-2 px-1">
                <div className="w-1 h-3 bg-[#F0B90B] rounded-full"></div>
                <div className="text-xs font-bold uppercase tracking-wider" style={{ color: '#848E9C' }}>
                  计划委托 ({planOrders.length})
                </div>
              </div>
              <div className="grid grid-cols-1 gap-2">
                {planOrders.map((order: any) => {
                  const side = (order.side || '').toLowerCase()
                  const isLong = side.includes('long') || side === 'long'
                  const isTP = order.type === 'take_profit'
                  const color = isTP ? '#0ECB81' : '#F6465D'

                  return (
                    <div
                      key={order.order_id}
                      className="p-3 rounded-md border border-[#2B3139] hover:border-[#474D57] transition-colors"
                      style={{ backgroundColor: '#2B3139' }}
                    >
                      <div className="flex justify-between items-start mb-2">
                        <div className="flex items-center gap-2">
                          <span className="font-bold text-[#EAECEF] font-mono">
                            {order.symbol}
                          </span>
                          <span className="text-[10px] px-1.5 py-0.5 rounded bg-[#474D57] text-[#EAECEF]">
                            {isTP ? '止盈' : '止损'}
                          </span>
                          <span
                            className="text-[10px] px-1.5 py-0.5 rounded font-bold"
                            style={{
                              backgroundColor: isLong ? 'rgba(14, 203, 129, 0.15)' : 'rgba(246, 70, 93, 0.15)',
                              color: isLong ? '#0ECB81' : '#F6465D'
                            }}
                          >
                            {formatDirection(order)}
                          </span>
                        </div>
                        <div className="text-xs font-mono" style={{ color: '#848E9C' }}>
                          {formatCreatedAt(order.created_at)}
                        </div>
                      </div>

                      <div className="grid grid-cols-2 gap-4">
                        <div>
                          <div className="text-[10px]" style={{ color: '#848E9C' }}>触发价</div>
                          <div className="text-sm font-bold font-mono" style={{ color }}>
                            ${order.price?.toFixed(2) || '—'}
                          </div>
                        </div>
                        <div>
                          <div className="text-[10px]" style={{ color: '#848E9C' }}>数量</div>
                          <div className="text-sm font-mono text-[#EAECEF]">
                            {order.quantity?.toFixed(4) || '—'}
                          </div>
                        </div>
                      </div>

                      <div className="mt-2 pt-2 border-t border-[#363C44] flex justify-between items-center text-[10px]">
                        <div style={{ color: '#5E6673' }}>
                          ID: {order.order_id}
                        </div>
                        <div className="font-bold" style={{ color: '#F0B90B' }}>
                          {formatStatus(order.status)}
                        </div>
                      </div>
                    </div>
                  )
                })}
              </div>
            </div>
          )}

          {/* 普通委托 (限价/市价) */}
          {normalOrders.length > 0 && (
            <div>
              <div className="flex items-center gap-2 mb-2 px-1">
                <div className="w-1 h-3 bg-[#60a5fa] rounded-full"></div>
                <div className="text-xs font-bold uppercase tracking-wider" style={{ color: '#848E9C' }}>
                  限价/普通委托 ({normalOrders.length})
                </div>
              </div>
              <div className="grid grid-cols-1 gap-2">
                {normalOrders.map((order: any) => {
                  const side = (order.side || '').toLowerCase()
                  const tradeSide = (order.trade_side || '').toLowerCase()
                  const reduceOnly = !!order.reduce_only
                  const isLong =
                    side.includes('long') ||
                    (tradeSide === 'open' && side === 'buy') ||
                    (tradeSide === 'close' && side === 'sell')

                  return (
                    <div
                      key={order.order_id}
                      className="p-3 rounded-md border border-[#2B3139] hover:border-[#474D57] transition-colors"
                      style={{ backgroundColor: '#2B3139' }}
                    >
                      <div className="flex justify-between items-start mb-2">
                        <div className="flex items-center gap-2">
                          <span className="font-bold text-[#EAECEF] font-mono">
                            {order.symbol}
                          </span>
                          <span className="text-[10px] px-1.5 py-0.5 rounded bg-[#474D57] text-[#EAECEF]">
                            {order.type === 'limit' ? '限价' : '市价'}
                          </span>
                          {reduceOnly && (
                            <span className="text-[10px] px-1.5 py-0.5 rounded bg-[#191D23] text-[#F0B90B] border border-[#2B3139]">
                              只减仓
                            </span>
                          )}
                          {(order.margin_mode || order.margin_coin) && (
                            <span className="text-[10px] px-1.5 py-0.5 rounded bg-[#191D23] text-[#848E9C] border border-[#2B3139]">
                              {order.margin_mode ? order.margin_mode : '—'} {order.margin_coin ? order.margin_coin : ''}
                            </span>
                          )}
                          <span
                            className="text-[10px] px-1.5 py-0.5 rounded font-bold"
                            style={{
                              backgroundColor: isLong ? 'rgba(14, 203, 129, 0.15)' : 'rgba(246, 70, 93, 0.15)',
                              color: isLong ? '#0ECB81' : '#F6465D'
                            }}
                          >
                            {formatDirection(order)}
                          </span>
                        </div>
                        <div className="text-xs font-mono" style={{ color: '#848E9C' }}>
                          {formatCreatedAt(order.created_at)}
                        </div>
                      </div>

                      <div className="grid grid-cols-5 gap-2">
                        <div>
                          <div className="text-[10px]" style={{ color: '#848E9C' }}>价格</div>
                          <div className="text-sm font-bold font-mono text-[#EAECEF]">
                            {order.type === 'limit' ? `$${order.price?.toFixed(2) || '—'}` : '市价'}
                          </div>
                        </div>
                        <div>
                          <div className="text-[10px]" style={{ color: '#848E9C' }}>数量</div>
                          <div className="text-sm font-mono text-[#EAECEF]">
                            {order.quantity?.toFixed(4) || '—'}
                          </div>
                        </div>
                        <div>
                          <div className="text-[10px]" style={{ color: '#848E9C' }}>价值</div>
                          <div className="text-sm font-mono text-[#EAECEF]">
                            {order.position_value ? `$${order.position_value.toFixed(2)}` : '—'}
                          </div>
                        </div>
                        <div>
                          <div className="text-[10px]" style={{ color: '#848E9C' }}>杠杆</div>
                          <div className="text-sm font-mono text-[#EAECEF]">
                            {order.leverage ? `${order.leverage}x` : '—'}
                          </div>
                        </div>
                        <div>
                          <div className="text-[10px]" style={{ color: '#848E9C' }}>已成交</div>
                          <div className="text-sm font-mono" style={{ color: order.filled_size > 0 ? '#0ECB81' : '#848E9C' }}>
                            {order.filled_size?.toFixed(4) || '0.0000'}
                          </div>
                        </div>
                      </div>

                      <div className="mt-2 pt-2 border-t border-[#363C44] flex justify-between items-center text-[10px]">
                        <div style={{ color: '#5E6673' }} className="flex gap-2">
                          <span>ID: {order.order_id}</span>
                          {order.avg_price > 0 && <span>均价: ${order.avg_price.toFixed(2)}</span>}
                        </div>
                        <div className="font-bold" style={{ color: '#F0B90B' }}>
                          {formatStatus(order.status)}
                        </div>
                      </div>
                    </div>
                  )
                })}
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  )
}
