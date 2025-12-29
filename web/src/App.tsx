import { useEffect, useState } from 'react'
import useSWR from 'swr'
import { api } from './lib/api'
import { EquityChart } from './components/EquityChart'
import { AITradersPage } from './components/AITradersPage'
import { CategoriesPage } from './components/CategoriesPage'
import { LoginPage } from './components/LoginPage'
import { RegisterPage } from './components/RegisterPage'
import { ResetPasswordPage } from './components/ResetPasswordPage'
import { CompetitionPage } from './components/CompetitionPage'
import { LandingPage } from './pages/LandingPage'
import { FAQPage } from './pages/FAQPage'
import HeaderBar from './components/landing/HeaderBar'
import AILearning from './components/AILearning'
import { TraderExecutionCard } from './components/TraderExecutionCard'
import { OrdersPanel } from './components/OrdersPanel'
import { ParsedSignalsPanel } from './components/ParsedSignalsPanel'
import { DecisionCard } from './components/DecisionCard'
import { LanguageProvider, useLanguage } from './contexts/LanguageContext'
import { AuthProvider, useAuth } from './contexts/AuthContext'
import { t, type Language } from './i18n/translations'
import { useSystemConfig } from './hooks/useSystemConfig'

import type {
  SystemStatus,
  AccountInfo,
  Position,
  Statistics,
  TraderInfo,
  StrategyDecisionHistory,
} from './types'

type Page = 'competition' | 'traders' | 'trader' | 'categories'

// 获取友好的AI模型名称
function getModelDisplayName(modelId: string): string {
  switch (modelId.toLowerCase()) {
    case 'deepseek':
      return 'DeepSeek'
    case 'qwen':
      return 'Qwen'
    case 'claude':
      return 'Claude'
    default:
      return modelId.toUpperCase()
  }
}

function App() {
  const { language, setLanguage } = useLanguage()
  const { user, token, logout, isLoading } = useAuth()
  const { loading: configLoading } = useSystemConfig()
  const [route, setRoute] = useState(window.location.pathname)

  // 从URL路径读取初始页面状态（支持刷新保持页面）
  const getInitialPage = (): Page => {
    const path = window.location.pathname
    const hash = window.location.hash.slice(1) // 去掉 #

    if (path === '/traders' || hash === 'traders') return 'traders'
    if (path === '/categories' || hash === 'categories') return 'categories'
    if (path === '/dashboard' || hash === 'trader' || hash === 'details')
      return 'trader'
    return 'competition' // 默认为竞赛页面
  }

const [currentPage, setCurrentPage] = useState<Page>(getInitialPage())
const [selectedTraderId, setSelectedTraderId] = useState<string | undefined>(() => {
  // 优先从本地存储恢复上次查看的交易员
  if (typeof window !== 'undefined') {
    const lastId = localStorage.getItem('last_selected_trader_id')
    return lastId || undefined
  }
  return undefined
})
  const [lastUpdate, setLastUpdate] = useState<string>('--:--:--')

  // 监听URL变化，同步页面状态
  useEffect(() => {
    const handleRouteChange = () => {
      const path = window.location.pathname
      const hash = window.location.hash.slice(1)

      if (path === '/traders' || hash === 'traders') {
        setCurrentPage('traders')
      } else if (path === '/categories' || hash === 'categories') {
        setCurrentPage('categories')
      } else if (
        path === '/dashboard' ||
        hash === 'trader' ||
        hash === 'details'
      ) {
        setCurrentPage('trader')
      } else if (
        path === '/competition' ||
        hash === 'competition' ||
        hash === ''
      ) {
        setCurrentPage('competition')
      }
      setRoute(path)
    }

    window.addEventListener('hashchange', handleRouteChange)
    window.addEventListener('popstate', handleRouteChange)
    return () => {
      window.removeEventListener('hashchange', handleRouteChange)
      window.removeEventListener('popstate', handleRouteChange)
    }
  }, [])

  // 切换页面时更新URL hash (当前通过按钮直接调用setCurrentPage，这个函数暂时保留用于未来扩展)
  // const navigateToPage = (page: Page) => {
  //   setCurrentPage(page);
  //   window.location.hash = page === 'competition' ? '' : 'trader';
  // };

  // 获取trader列表（仅在用户登录时）
  const { data: traders, error: tradersError } = useSWR<TraderInfo[]>(
    user && token ? 'traders' : null,
    api.getTraders,
    {
      refreshInterval: 10000,
      shouldRetryOnError: false, // 避免在后端未运行时无限重试
    }
  )

// 当获取到traders后，根据本地记忆/默认规则设置选中交易员
useEffect(() => {
  if (!traders || traders.length === 0) return

  // 已经有选中的 Trader，则只需校验是否仍然存在
  if (selectedTraderId) {
    const stillExists = traders.some((t) => t.trader_id === selectedTraderId)
    if (!stillExists) {
      const fallbackId = traders[0].trader_id
      setSelectedTraderId(fallbackId)
      if (typeof window !== 'undefined') {
        localStorage.setItem('last_selected_trader_id', fallbackId)
      }
    }
    return
  }

  // 没有选中记录时，尝试从本地存储恢复
  let initialId: string | undefined
  if (typeof window !== 'undefined') {
    const lastId = localStorage.getItem('last_selected_trader_id')
    if (lastId && traders.some((t) => t.trader_id === lastId)) {
      initialId = lastId
    }
  }

  if (!initialId) {
    initialId = traders[0].trader_id
  }

  setSelectedTraderId(initialId)
  if (typeof window !== 'undefined' && initialId) {
    localStorage.setItem('last_selected_trader_id', initialId)
  }
}, [traders, selectedTraderId])

  // 如果在trader页面，获取该trader的数据
  const { data: status } = useSWR<SystemStatus>(
    currentPage === 'trader' && selectedTraderId
      ? `status-${selectedTraderId}`
      : null,
    () => api.getStatus(selectedTraderId),
    {
      refreshInterval: 15000, // 15秒刷新（配合后端15秒缓存）
      revalidateOnFocus: false, // 禁用聚焦时重新验证，减少请求
      dedupingInterval: 10000, // 10秒去重，防止短时间内重复请求
    }
  )

  const { data: account } = useSWR<AccountInfo>(
    currentPage === 'trader' && selectedTraderId
      ? `account-${selectedTraderId}`
      : null,
    () => api.getAccount(selectedTraderId),
    {
      refreshInterval: 15000, // 15秒刷新（配合后端15秒缓存）
      revalidateOnFocus: false, // 禁用聚焦时重新验证，减少请求
      dedupingInterval: 10000, // 10秒去重，防止短时间内重复请求
    }
  )

  const { data: positions } = useSWR<Position[]>(
    currentPage === 'trader' && selectedTraderId
      ? `positions-${selectedTraderId}`
      : null,
    () => api.getPositions(selectedTraderId),
    {
      refreshInterval: 15000, // 15秒刷新（配合后端15秒缓存）
      revalidateOnFocus: false, // 禁用聚焦时重新验证，减少请求
      dedupingInterval: 10000, // 10秒去重，防止短时间内重复请求
    }
  )

  const { data: stats } = useSWR<Statistics>(
    currentPage === 'trader' && selectedTraderId
      ? `statistics-${selectedTraderId}`
      : null,
    () => api.getStatistics(selectedTraderId),
    {
      refreshInterval: 30000, // 30秒刷新（统计数据更新频率较低）
      revalidateOnFocus: false,
      dedupingInterval: 20000,
    }
  )

  useEffect(() => {
    if (account) {
      const now = new Date().toLocaleTimeString()
      setLastUpdate(now)
    }
  }, [account])

  const selectedTrader = traders?.find((t) => t.trader_id === selectedTraderId)

  // Handle routing
  useEffect(() => {
    const handlePopState = () => {
      setRoute(window.location.pathname)
    }
    window.addEventListener('popstate', handlePopState)
    return () => window.removeEventListener('popstate', handlePopState)
  }, [])

  // Set current page based on route for consistent navigation state
  useEffect(() => {
    if (route === '/competition') {
      setCurrentPage('competition')
    } else if (route === '/traders') {
      setCurrentPage('traders')
    } else if (route === '/dashboard') {
      setCurrentPage('trader')
    }
  }, [route])

  // Show loading spinner while checking auth or config
  if (isLoading || configLoading) {
    return (
      <div
        className="min-h-screen flex items-center justify-center"
        style={{ background: '#0B0E11' }}
      >
        <div className="text-center">
          <img
            src="/icons/nofx.svg"
            alt="NoFx Logo"
            className="w-16 h-16 mx-auto mb-4 animate-pulse"
          />
          <p style={{ color: '#EAECEF' }}>{t('loading', language)}</p>
        </div>
      </div>
    )
  }

  // Handle specific routes regardless of authentication
  if (route === '/login') {
    return <LoginPage />
  }
  if (route === '/register') {
    return <RegisterPage />
  }
  if (route === '/faq') {
    return <FAQPage />
  }
  if (route === '/reset-password') {
    return <ResetPasswordPage />
  }
  if (route === '/competition') {
    return (
      <div
        className="min-h-screen"
        style={{ background: '#000000', color: '#EAECEF' }}
      >
        <HeaderBar
          isLoggedIn={!!user}
          currentPage="competition"
          language={language}
          onLanguageChange={setLanguage}
          user={user}
          onLogout={logout}
          onPageChange={(page) => {
            console.log('Competition page onPageChange called with:', page)
            console.log('Current route:', route, 'Current page:', currentPage)

            if (page === 'competition') {
              console.log('Navigating to competition')
              window.history.pushState({}, '', '/competition')
              setRoute('/competition')
              setCurrentPage('competition')
            } else if (page === 'traders') {
              console.log('Navigating to traders')
              window.history.pushState({}, '', '/traders')
              setRoute('/traders')
              setCurrentPage('traders')
            } else if (page === 'trader') {
              console.log('Navigating to trader/dashboard')
              window.history.pushState({}, '', '/dashboard')
              setRoute('/dashboard')
              setCurrentPage('trader')
            } else if (page === 'faq') {
              console.log('Navigating to faq')
              window.history.pushState({}, '', '/faq')
              setRoute('/faq')
            }

            console.log(
              'After navigation - route:',
              route,
              'currentPage:',
              currentPage
            )
          }}
        />
        <main className="max-w-[1920px] mx-auto px-6 py-6 pt-24">
          <CompetitionPage />
        </main>
      </div>
    )
  }

  // Show landing page for root route
  if (route === '/' || route === '') {
    return <LandingPage />
  }

  // Show main app for authenticated users on other routes
  if (!user || !token) {
    // Default to landing page when not authenticated and no specific route
    return <LandingPage />
  }

  return (
    <div
      className="min-h-screen"
      style={{ background: '#0B0E11', color: '#EAECEF' }}
    >
      <HeaderBar
        isLoggedIn={!!user}
        currentPage={currentPage}
        language={language}
        onLanguageChange={setLanguage}
        user={user}
        onLogout={logout}
        onPageChange={(page) => {
          console.log('Main app onPageChange called with:', page)

          if (page === 'competition') {
            window.history.pushState({}, '', '/competition')
            setRoute('/competition')
            setCurrentPage('competition')
          } else if (page === 'traders') {
            window.history.pushState({}, '', '/traders')
            setRoute('/traders')
            setCurrentPage('traders')
          } else if (page === 'categories') {
            window.history.pushState({}, '', '/categories')
            setRoute('/categories')
            setCurrentPage('categories')
          } else if (page === 'trader') {
            window.history.pushState({}, '', '/dashboard')
            setRoute('/dashboard')
            setCurrentPage('trader')
          } else if (page === 'faq') {
            window.history.pushState({}, '', '/faq')
            setRoute('/faq')
          }
        }}
      />

      {/* Main Content */}
      <main className="max-w-[1920px] mx-auto px-6 py-6 pt-24">
        {currentPage === 'competition' ? (
          <CompetitionPage />
        ) : currentPage === 'traders' ? (
          <AITradersPage
            onTraderSelect={(traderId) => {
              setSelectedTraderId(traderId)
              if (typeof window !== 'undefined') {
                localStorage.setItem('last_selected_trader_id', traderId)
              }
              window.history.pushState({}, '', '/dashboard')
              setRoute('/dashboard')
              setCurrentPage('trader')
            }}
          />
        ) : currentPage === 'categories' ? (
          <CategoriesPage />
        ) : (
          <TraderDetailsPage
            selectedTrader={selectedTrader}
            status={status}
            account={account}
            positions={positions}
            stats={stats}
            lastUpdate={lastUpdate}
            language={language}
            traders={traders}
            tradersError={tradersError}
            selectedTraderId={selectedTraderId}
            onTraderSelect={(traderId: string) => {
              setSelectedTraderId(traderId)
              if (typeof window !== 'undefined') {
                localStorage.setItem('last_selected_trader_id', traderId)
              }
            }}
            onNavigateToTraders={() => {
              window.history.pushState({}, '', '/traders')
              setRoute('/traders')
              setCurrentPage('traders')
            }}
          />
        )}
      </main>

      {/* Footer */}
      <footer
        className="mt-16"
        style={{ borderTop: '1px solid #2B3139', background: '#181A20' }}
      >
        <div
          className="max-w-[1920px] mx-auto px-6 py-6 text-center text-sm"
          style={{ color: '#5E6673' }}
        >
          <p>{t('footerTitle', language)}</p>
          <p className="mt-1">{t('footerWarning', language)}</p>
          <div className="mt-4 text-xs opacity-50">
            &copy; {new Date().getFullYear()} NoFX Intelligent Trading. All rights reserved.
          </div>
        </div>
      </footer>
    </div>
  )
}

// Trader Details Page Component
function TraderDetailsPage({
  selectedTrader,
  status,
  account,
  positions,
  lastUpdate,
  language,
  traders,
  tradersError,
  selectedTraderId,
  onTraderSelect,
  onNavigateToTraders,
}: {
  selectedTrader?: TraderInfo
  traders?: TraderInfo[]
  tradersError?: Error
  selectedTraderId?: string
  onTraderSelect: (traderId: string) => void
  onNavigateToTraders: () => void
  status?: SystemStatus
  account?: AccountInfo
  positions?: Position[]
  stats?: Statistics
  lastUpdate: string
  language: Language
}) {
  // AI 决策筛选状态：latest（最新50条）| open（所有开仓）| close（所有平仓）
  const [decisionFilter, setDecisionFilter] = useState<'latest' | 'open' | 'close'>('latest')
  
  // 获取决策历史（根据筛选模式动态获取）
  const { data: decisionsData, mutate: mutateDecisions } = useSWR<{
    decisions: StrategyDecisionHistory[]
    total: number
    mode: string
  }>(
    selectedTraderId ? `strategy-decisions-${selectedTraderId}-${decisionFilter}` : null,
    () => api.getStrategyDecisions(selectedTraderId!, decisionFilter, 50),
    {
      refreshInterval: 30000,
      revalidateOnFocus: false,
      dedupingInterval: 20000,
    }
  )
  
  const decisions = decisionsData?.decisions || []
  
  // Fetch active strategies and statuses for the selected trader
  const { data: strategiesData, mutate: mutateStrategies } = useSWR(
    selectedTraderId ? `activeStrategies` : null,
    api.getActiveStrategies,
    { 
      refreshInterval: 5000,
      keepPreviousData: true, // 保持旧数据，防止闪烁
      fallbackData: []        // 默认空数组
    }
  )

  const { data: strategyStatuses, mutate: mutateStatuses } = useSWR(
    selectedTraderId ? `strategyStatuses-${selectedTraderId}` : null,
    () => api.getTraderStrategyStatuses(selectedTraderId!),
    { 
      refreshInterval: 5000,
      keepPreviousData: true, // 保持旧数据，防止闪烁
      fallbackData: []        // 默认空数组
    }
  )

  // 平仓状态管理
  const [closingPosition, setClosingPosition] = useState<string | null>(null)

  // 平仓操作
  const handleClosePosition = async (pos: Position) => {
    const confirmMsg = `确认平仓？\n\n交易对: ${pos.symbol}\n方向: ${pos.side === 'long' ? '多' : '空'}仓\n数量: ${pos.quantity}\n未实现盈亏: ${pos.unrealized_pnl.toFixed(2)} USDT (${pos.unrealized_pnl_pct.toFixed(2)}%)`
    
    if (!confirm(confirmMsg)) {
      return
    }

    const posKey = `${pos.symbol}-${pos.side}`
    setClosingPosition(posKey)

    try {
      const response = await api.post(`/api/positions/close?trader_id=${selectedTraderId}`, {
        symbol: pos.symbol,
        side: pos.side,
        quantity: pos.quantity,
      })

      console.log('平仓成功:', response)
      
      // 显示成功消息并提示用户刷新
      alert(`✅ 平仓成功！\n\n交易对: ${pos.symbol}\n方向: ${pos.side === 'long' ? '多' : '空'}仓\n\n页面将自动刷新以更新持仓列表`)
      
      // 刷新页面以更新所有数据
      window.location.reload()
    } catch (err: any) {
      console.error('平仓失败:', err)
      const errorMsg = err.response?.data?.error || err.message || '平仓失败'
      alert(`❌ 平仓失败: ${errorMsg}`)
    } finally {
      setClosingPosition(null)
    }
  }

  // If API failed with error, show empty state (likely backend not running)
  if (tradersError) {
    return (
      <div className="flex items-center justify-center min-h-[60vh]">
        <div className="text-center max-w-md mx-auto px-6">
          {/* Icon */}
          <div
            className="w-24 h-24 mx-auto mb-6 rounded-full flex items-center justify-center"
            style={{
              background: 'rgba(240, 185, 11, 0.1)',
              border: '2px solid rgba(240, 185, 11, 0.3)',
            }}
          >
            <svg
              className="w-12 h-12"
              style={{ color: '#F0B90B' }}
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z"
              />
            </svg>
          </div>

          {/* Title */}
          <h2 className="text-2xl font-bold mb-3" style={{ color: '#EAECEF' }}>
            {t('dashboardEmptyTitle', language)}
          </h2>

          {/* Description */}
          <p className="text-base mb-6" style={{ color: '#848E9C' }}>
            {t('dashboardEmptyDescription', language)}
          </p>

          {/* CTA Button */}
          <button
            onClick={onNavigateToTraders}
            className="px-6 py-3 rounded-lg font-semibold transition-all hover:scale-105 active:scale-95"
            style={{
              background: 'linear-gradient(135deg, #F0B90B 0%, #FCD535 100%)',
              color: '#0B0E11',
              boxShadow: '0 4px 12px rgba(240, 185, 11, 0.3)',
            }}
          >
            {t('goToTradersPage', language)}
          </button>
        </div>
      </div>
    )
  }

  // If traders is loaded and empty, show empty state
  if (traders && traders.length === 0) {
    return (
      <div className="flex items-center justify-center min-h-[60vh]">
        <div className="text-center max-w-md mx-auto px-6">
          {/* Icon */}
          <div
            className="w-24 h-24 mx-auto mb-6 rounded-full flex items-center justify-center"
            style={{
              background: 'rgba(240, 185, 11, 0.1)',
              border: '2px solid rgba(240, 185, 11, 0.3)',
            }}
          >
            <svg
              className="w-12 h-12"
              style={{ color: '#F0B90B' }}
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z"
              />
            </svg>
          </div>

          {/* Title */}
          <h2 className="text-2xl font-bold mb-3" style={{ color: '#EAECEF' }}>
            {t('dashboardEmptyTitle', language)}
          </h2>

          {/* Description */}
          <p className="text-base mb-6" style={{ color: '#848E9C' }}>
            {t('dashboardEmptyDescription', language)}
          </p>

          {/* CTA Button */}
          <button
            onClick={onNavigateToTraders}
            className="px-6 py-3 rounded-lg font-semibold transition-all hover:scale-105 active:scale-95"
            style={{
              background: 'linear-gradient(135deg, #F0B90B 0%, #FCD535 100%)',
              color: '#0B0E11',
              boxShadow: '0 4px 12px rgba(240, 185, 11, 0.3)',
            }}
          >
            {t('goToTradersPage', language)}
          </button>
        </div>
      </div>
    )
  }

  // If traders is still loading or selectedTrader is not ready, show skeleton
  if (!selectedTrader) {
    return (
      <div className="space-y-6">
        {/* Loading Skeleton - Binance Style */}
        <div className="binance-card p-6 animate-pulse">
          <div className="skeleton h-8 w-48 mb-3"></div>
          <div className="flex gap-4">
            <div className="skeleton h-4 w-32"></div>
            <div className="skeleton h-4 w-24"></div>
            <div className="skeleton h-4 w-28"></div>
          </div>
        </div>
        <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
          {[1, 2, 3, 4].map((i) => (
            <div key={i} className="binance-card p-5 animate-pulse">
              <div className="skeleton h-4 w-24 mb-3"></div>
              <div className="skeleton h-8 w-32"></div>
            </div>
          ))}
        </div>
        <div className="binance-card p-6 animate-pulse">
          <div className="skeleton h-6 w-40 mb-4"></div>
          <div className="skeleton h-64 w-full"></div>
        </div>
      </div>
    )
  }

  // 后端已按 mode 过滤（latest/open/close），前端直接使用
  const filteredDecisions = decisions || []

  return (
    <div>
      {/* Trader Header */}
      <div
        className="mb-6 rounded p-6 animate-scale-in"
        style={{
          background:
            'linear-gradient(135deg, rgba(240, 185, 11, 0.15) 0%, rgba(252, 213, 53, 0.05) 100%)',
          border: '1px solid rgba(240, 185, 11, 0.2)',
          boxShadow: '0 0 30px rgba(240, 185, 11, 0.15)',
        }}
      >
        <div className="flex items-start justify-between mb-3">
          <h2
            className="text-2xl font-bold flex items-center gap-2"
            style={{ color: '#EAECEF' }}
          >
            <span
              className="w-10 h-10 rounded-full flex items-center justify-center text-xl"
              style={{
                background: 'linear-gradient(135deg, #F0B90B 0%, #FCD535 100%)',
              }}
            >
              🤖
            </span>
            {selectedTrader.trader_name}
          </h2>

          {/* Trader Selector */}
          {traders && traders.length > 0 && (
            <div className="flex items-center gap-2">
              <span className="text-sm" style={{ color: '#848E9C' }}>
                {t('switchTrader', language)}:
              </span>
              <select
                value={selectedTraderId}
                onChange={(e) => onTraderSelect(e.target.value)}
                className="rounded px-3 py-2 text-sm font-medium cursor-pointer transition-colors"
                style={{
                  background: '#1E2329',
                  border: '1px solid #2B3139',
                  color: '#EAECEF',
                }}
              >
                {traders.map((trader) => (
                  <option key={trader.trader_id} value={trader.trader_id}>
                    {trader.trader_name}
                  </option>
                ))}
              </select>
            </div>
          )}
        </div>
        <div
          className="flex items-center gap-4 text-sm"
          style={{ color: '#848E9C' }}
        >
          <span>
            AI Model:{' '}
            <span
              className="font-semibold"
              style={{
                color: selectedTrader.ai_model.includes('qwen')
                  ? '#c084fc'
                  : '#60a5fa',
              }}
            >
              {getModelDisplayName(
                selectedTrader.ai_model.split('_').pop() ||
                  selectedTrader.ai_model
              )}
            </span>
          </span>
          {status && (
            <>
              <span>•</span>
              <span>Cycles: {status.call_count}</span>
              <span>•</span>
              <span>Runtime: {status.runtime_minutes} min</span>
            </>
          )}
        </div>
      </div>

      {/* Debug Info */}
      {account && (
        <div
          className="mb-4 p-3 rounded text-xs font-mono"
          style={{ background: '#1E2329', border: '1px solid #2B3139' }}
        >
          <div style={{ color: '#848E9C' }}>
            🔄 Last Update: {lastUpdate} | Total Equity:{' '}
            {account?.total_equity?.toFixed(2) || '0.00'} | Available:{' '}
            {account?.available_balance?.toFixed(2) || '0.00'} | P&L:{' '}
            {account?.total_pnl?.toFixed(2) || '0.00'} (
            {account?.total_pnl_pct?.toFixed(2) || '0.00'}%)
          </div>
        </div>
      )}

      {/* Account Overview */}
      <div className="grid grid-cols-1 md:grid-cols-4 gap-4 mb-8">
        <StatCard
          title={t('totalEquity', language)}
          value={`${account?.total_equity?.toFixed(2) || '0.00'} USDT`}
          change={account?.total_pnl_pct || 0}
          positive={(account?.total_pnl ?? 0) > 0}
        />
        <StatCard
          title={t('availableBalance', language)}
          value={`${account?.available_balance?.toFixed(2) || '0.00'} USDT`}
          subtitle={`${account?.available_balance && account?.total_equity ? ((account.available_balance / account.total_equity) * 100).toFixed(1) : '0.0'}% ${t('free', language)}`}
        />
        <StatCard
          title={t('totalPnL', language)}
          value={`${account?.total_pnl !== undefined && account.total_pnl >= 0 ? '+' : ''}${account?.total_pnl?.toFixed(2) || '0.00'} USDT`}
          change={account?.total_pnl_pct || 0}
          positive={(account?.total_pnl ?? 0) >= 0}
        />
        <StatCard
          title={t('positions', language)}
          value={`${account?.position_count || 0}`}
          subtitle={`${t('margin', language)}: ${account?.margin_used_pct?.toFixed(1) || '0.0'}%`}
        />
      </div>

      {/* 主要内容区：左右分屏 */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6 mb-6">
        {/* 左侧：图表 + 持仓 */}
        <div className="space-y-6">
          {/* Equity Chart */}
          <div className="animate-slide-in" style={{ animationDelay: '0.1s' }}>
            <EquityChart traderId={selectedTrader.trader_id} />
          </div>

          {/* Current Positions */}
          <div
            className="binance-card p-6 animate-slide-in hover:shadow-[0_0_20px_rgba(240,185,11,0.05)] transition-all duration-300"
            style={{ animationDelay: '0.15s' }}
          >
            <div className="flex items-center justify-between mb-5">
              <h2
                className="text-xl font-bold flex items-center gap-2"
                style={{ color: '#EAECEF' }}
              >
                <span className="w-1 h-6 bg-[#F0B90B] rounded-full mr-1"></span>
                📈 {t('currentPositions', language)}
              </h2>
              <div className="flex items-center gap-3">
                <button 
                  onClick={() => window.location.reload()}
                  className="p-1.5 rounded hover:bg-[#2B3139] transition-colors"
                  title="刷新"
                >
                  <svg className="w-4 h-4 text-[#848E9C] hover:text-[#F0B90B]" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
                  </svg>
                </button>
                {positions && positions.length > 0 && (
                  <div
                    className="text-xs px-3 py-1 rounded font-bold"
                    style={{
                      background: 'rgba(240, 185, 11, 0.1)',
                      color: '#F0B90B',
                      border: '1px solid rgba(240, 185, 11, 0.2)',
                    }}
                  >
                    {positions.length} {t('active', language)}
                  </div>
                )}
              </div>
            </div>
            {positions && positions.length > 0 ? (
              <div className="overflow-x-auto">
                <table className="w-full text-sm">
                  <thead className="text-left border-b border-gray-800">
                    <tr>
                      <th className="pb-3 font-semibold text-gray-400">
                        {t('symbol', language)}
                      </th>
                      <th className="pb-3 font-semibold text-gray-400">
                        {t('side', language)}
                      </th>
                      <th className="pb-3 font-semibold text-gray-400">
                        {t('entryPrice', language)}
                      </th>
                      <th className="pb-3 font-semibold text-gray-400">
                        {t('markPrice', language)}
                      </th>
                      <th className="pb-3 font-semibold text-gray-400">
                        {t('quantity', language)}
                      </th>
                      <th className="pb-3 font-semibold text-gray-400">
                        {t('positionValue', language)}
                      </th>
                      <th className="pb-3 font-semibold text-gray-400">
                        {t('leverage', language)}
                      </th>
                      <th className="pb-3 font-semibold text-gray-400">
                        {t('unrealizedPnL', language)}
                      </th>
                      <th className="pb-3 font-semibold text-gray-400">
                        {t('liqPrice', language)}
                      </th>
                      <th className="pb-3 font-semibold text-gray-400">
                        操作
                      </th>
                    </tr>
                  </thead>
                  <tbody>
                    {positions.map((pos, i) => (
                      <tr
                        key={i}
                        className="border-b border-gray-800 last:border-0"
                      >
                        <td className="py-3 font-mono font-semibold">
                          {pos.symbol}
                        </td>
                        <td className="py-3">
                          <span
                            className="px-2 py-1 rounded text-xs font-bold"
                            style={
                              pos.side === 'long'
                                ? {
                                    background: 'rgba(14, 203, 129, 0.1)',
                                    color: '#0ECB81',
                                  }
                                : {
                                    background: 'rgba(246, 70, 93, 0.1)',
                                    color: '#F6465D',
                                  }
                            }
                          >
                            {t(
                              pos.side === 'long' ? 'long' : 'short',
                              language
                            )}
                          </span>
                        </td>
                        <td
                          className="py-3 font-mono"
                          style={{ color: '#EAECEF' }}
                        >
                          {pos.entry_price.toFixed(4)}
                        </td>
                        <td
                          className="py-3 font-mono"
                          style={{ color: '#EAECEF' }}
                        >
                          {pos.mark_price.toFixed(4)}
                        </td>
                        <td
                          className="py-3 font-mono"
                          style={{ color: '#EAECEF' }}
                        >
                          {pos.quantity.toFixed(4)}
                        </td>
                        <td
                          className="py-3 font-mono font-bold"
                          style={{ color: '#EAECEF' }}
                        >
                          {(pos.quantity * pos.mark_price).toFixed(2)} USDT
                        </td>
                        <td
                          className="py-3 font-mono"
                          style={{ color: '#F0B90B' }}
                        >
                          {pos.leverage}x
                        </td>
                        <td className="py-3 font-mono">
                          <span
                            style={{
                              color:
                                pos.unrealized_pnl >= 0 ? '#0ECB81' : '#F6465D',
                              fontWeight: 'bold',
                            }}
                          >
                            {pos.unrealized_pnl >= 0 ? '+' : ''}
                            {pos.unrealized_pnl.toFixed(2)} (
                            {pos.unrealized_pnl_pct.toFixed(2)}%)
                          </span>
                        </td>
                        <td
                          className="py-3 font-mono"
                          style={{ color: '#848E9C' }}
                        >
                          {pos.liquidation_price.toFixed(4)}
                        </td>
                        <td className="py-3">
                          <button
                            onClick={() => handleClosePosition(pos)}
                            disabled={closingPosition === `${pos.symbol}-${pos.side}`}
                            className="px-3 py-1 rounded text-xs font-bold transition-all"
                            style={{
                              background: closingPosition === `${pos.symbol}-${pos.side}` 
                                ? 'rgba(132, 142, 156, 0.1)' 
                                : 'rgba(246, 70, 93, 0.1)',
                              color: closingPosition === `${pos.symbol}-${pos.side}` 
                                ? '#848E9C' 
                                : '#F6465D',
                              border: `1px solid ${closingPosition === `${pos.symbol}-${pos.side}` ? '#848E9C' : '#F6465D'}`,
                              cursor: closingPosition === `${pos.symbol}-${pos.side}` ? 'not-allowed' : 'pointer',
                            }}
                          >
                            {closingPosition === `${pos.symbol}-${pos.side}` ? '平仓中...' : '平仓'}
                          </button>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            ) : (
              <div className="text-center py-16" style={{ color: '#848E9C' }}>
                <div className="text-6xl mb-4 opacity-50">📊</div>
                <div className="text-lg font-semibold mb-2">
                  {t('noPositions', language)}
                </div>
                <div className="text-sm">
                  {t('noActivePositions', language)}
                </div>
              </div>
            )}
          </div>
        </div>
        {/* 左侧结束 */}

      {/* 右侧：Recent Decisions - 卡片容器 */}
        <div
          className="binance-card p-6 animate-slide-in h-fit lg:sticky lg:top-24 lg:max-h-[calc(100vh-120px)] hover:shadow-[0_0_20px_rgba(99,102,241,0.05)] transition-all duration-300"
          style={{ animationDelay: '0.2s' }}
        >
          {/* 标题 */}
          <div
            className="flex items-center gap-3 mb-5 pb-4 border-b"
            style={{ borderColor: '#2B3139' }}
          >
            <div
              className="w-10 h-10 rounded-xl flex items-center justify-center text-xl"
              style={{
                background: 'linear-gradient(135deg, #6366F1 0%, #8B5CF6 100%)',
                boxShadow: '0 4px 14px rgba(99, 102, 241, 0.4)',
              }}
            >
              🧠
            </div>
            <div className="flex-1">
              <div className="flex items-center justify-between">
                <h2 className="text-xl font-bold" style={{ color: '#EAECEF' }}>
                  {t('recentDecisions', language)}
                </h2>
                <button 
                  onClick={() => mutateDecisions()}
                  className="p-1 rounded hover:bg-[#2B3139] transition-colors"
                >
                  <svg className="w-3.5 h-3.5 text-[#848E9C] hover:text-[#6366F1]" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
                  </svg>
                </button>
              </div>
              {decisions && decisions.length > 0 && (
                <div className="text-xs" style={{ color: '#848E9C' }}>
                  {t('lastCycles', language, { count: decisions.length })}
                </div>
              )}
            </div>
          </div>

          {/* 决策筛选器 */}
          <div
            className="flex items-center gap-2 mb-3 text-xs"
            style={{ color: '#848E9C' }}
          >
            <span>筛选决策:</span>
            <button
              className={`px-2 py-0.5 rounded border text-xs ${
                decisionFilter === 'latest'
                  ? 'border-[#F0B90B] text-[#F0B90B] bg-[#F0B90B]/10'
                  : 'border-transparent hover:border-[#2B3139]'
              }`}
              onClick={() => setDecisionFilter('latest')}
            >
              最新50条
            </button>
            <button
              className={`px-2 py-0.5 rounded border text-xs ${
                decisionFilter === 'open'
                  ? 'border-green-500 text-green-400 bg-green-500/10'
                  : 'border-transparent hover:border-[#2B3139]'
              }`}
              onClick={() => setDecisionFilter('open')}
            >
              所有开仓
            </button>
            <button
              className={`px-2 py-0.5 rounded border text-xs ${
                decisionFilter === 'close'
                  ? 'border-red-500 text-red-400 bg-red-500/10'
                  : 'border-transparent hover:border-[#2B3139]'
              }`}
              onClick={() => setDecisionFilter('close')}
            >
              所有平仓
            </button>
          </div>

          {/* 决策列表 - 可滚动 */}
          <div
            className="space-y-4 overflow-y-auto pr-2"
            style={{ maxHeight: 'calc(100vh - 280px)' }}
          >
            {filteredDecisions && filteredDecisions.length > 0 ? (
              filteredDecisions.map((decision: any, i: number) => (
                <DecisionCard
                  key={i}
                  decision={decision}
                  language={language}
                  traderId={selectedTrader.trader_id}
                />
              ))
            ) : (
              <div className="py-16 text-center">
                <div className="text-6xl mb-4 opacity-30">🧠</div>
                <div
                  className="text-lg font-semibold mb-2"
                  style={{ color: '#EAECEF' }}
                >
                  {t('noDecisionsYet', language)}
                </div>
                <div className="text-sm" style={{ color: '#848E9C' }}>
                  {t('aiDecisionsWillAppear', language)}
                </div>
              </div>
            )}
          </div>
        </div>
        {/* 右侧结束 */}
      </div>

      {/* Trader Execution Status - Multi-Strategy List */}
      <div className="mb-6 animate-slide-in" style={{ animationDelay: '0.25s' }}>
        {(() => {
          const activeRenderList = (strategiesData || []).filter((item: any) => {
            const status = strategyStatuses?.find((s: any) => s.strategy_id === item.strategy.signal_id);
            const fallbackStatus = !status
              ? strategyStatuses?.find((s: any) => s.strategy_id === '' && !s.strategy_id)
              : null;
            const finalStatus = status || fallbackStatus;
            return !(finalStatus && finalStatus.status === 'CLOSED');
          });

          return (
            <>
              <div className="flex items-center justify-between mb-4">
                <h3 className="text-xl font-bold text-[#EAECEF] flex items-center gap-2">
                    <span className="w-1 h-6 bg-[#6366F1] rounded-full mr-1"></span>
                    🚀 活跃策略池 ({activeRenderList.length})
                </h3>
                <button 
                  onClick={() => {
                    mutateStrategies();
                    mutateStatuses();
                  }}
                  className="p-1.5 rounded hover:bg-[#2B3139] transition-colors"
                  title="强制刷新"
                >
                  <svg className="w-4 h-4 text-[#848E9C] hover:text-[#6366F1]" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
                  </svg>
                </button>
              </div>
              {activeRenderList.length > 0 ? (
                activeRenderList.map((item: any, idx: number) => {
                  const status = strategyStatuses?.find((s: any) => s.strategy_id === item.strategy.signal_id);
                  const fallbackStatus = !status
                    ? strategyStatuses?.find((s: any) => s.strategy_id === '' && !s.strategy_id)
                    : null;
                  const finalStatus = status || fallbackStatus;
                  const symbolPosition = positions?.find((p) => p.symbol === item.strategy.symbol);

                  return (
                    <TraderExecutionCard
                      key={item.strategy.signal_id || idx}
                      strategy={item.strategy}
                      currentPrice={item.current_price}
                      updatedAt={item.updated_at}
                      status={finalStatus}
                      position={symbolPosition}
                    />
                  );
                })
              ) : (
                <div className="bg-[#1E2329] rounded-lg border border-[#2B3139] p-8 text-center text-[#848E9C]">
                    暂无活跃策略
                </div>
              )}
            </>
          );
        })()}

        {/* 📊 全量策略信号库 */}
        <div className="mt-8">
            <ParsedSignalsPanel 
                strategyStatuses={strategyStatuses}
            />
        </div>
      </div>

      {/* 当前委托展示（止盈止损等） */}
      <div className="mb-6 animate-slide-in" style={{ animationDelay: '0.27s' }}>
        <OrdersPanel traderId={selectedTrader.trader_id} />
      </div>

      {/* 历史策略列表：根据 CLOSED 状态 + 决策历史汇总 */}
      {(() => {
        const closedStatuses = (strategyStatuses || []).filter((s: any) => s.status === 'CLOSED')
        if (!closedStatuses.length) return null

        const closedIdSet = new Set<string>(closedStatuses.map((s: any) => s.strategy_id))
        const used = new Set<string>()
        const historyItems: Array<{ decision: StrategyDecisionHistory; status: any }> = []

        for (const d of decisions as any[]) {
          if (!d || !closedIdSet.has(d.strategy_id) || used.has(d.strategy_id)) continue
          const st = closedStatuses.find((s: any) => s.strategy_id === d.strategy_id)
          historyItems.push({ decision: d, status: st })
          used.add(d.strategy_id)
        }

        if (!historyItems.length) return null

        return (
          <div className="mb-6 animate-slide-in" style={{ animationDelay: '0.28s' }}>
            <h3 className="text-xl font-bold text-[#EAECEF] mb-4 flex items-center gap-2">
              📚 历史策略 ({historyItems.length})
            </h3>
            <div className="space-y-3">
              {historyItems.map(({ decision, status }) => (
                <div
                  key={decision.strategy_id}
                  className="binance-card p-4 flex flex-col md:flex-row md:items-center justify-between gap-3 border border-[#2B3139]"
                >
                  <div className="flex items-center gap-3">
                    <div className="text-lg font-bold text-[#EAECEF] font-mono">
                      {decision.symbol}
                    </div>
                    <div className="text-xs px-2 py-0.5 rounded-full font-mono" style={{ background: '#2B3139', color: '#C4CCD6' }}>
                      {decision.action}
                    </div>
                    {status && (
                      <div className="text-xs px-2 py-0.5 rounded-full" style={{ background: 'rgba(132,142,156,0.15)', color: '#A0AEC0' }}>
                        {status.status}
                      </div>
                    )}
                  </div>
                  <div className="flex items-center gap-6 text-xs md:text-sm text-[#A0AEC0]">
                    <div>
                      <div>最后决策时间</div>
                      <div className="font-mono">
                        {new Date(decision.decision_time as any).toLocaleString()}
                      </div>
                    </div>
                    <div>
                      <div>最后价格</div>
                      <div className="font-mono">{decision.current_price?.toFixed?.(2) ?? decision.current_price}</div>
                    </div>
                    {status && (
                      <div>
                        <div>已实现盈亏</div>
                        <div className="font-mono">
                          {(status.realized_pnl ?? 0) >= 0 ? '+' : ''}
                          {(status.realized_pnl ?? 0).toFixed(2)}
                        </div>
                      </div>
                    )}
                  </div>
                </div>
              ))}
            </div>
          </div>
        )
      })()}

      {/* AI Learning & Performance Analysis */}
      <div className="mb-6 animate-slide-in" style={{ animationDelay: '0.3s' }}>
        <AILearning traderId={selectedTrader.trader_id} />
      </div>
    </div>
  )
}

// Stat Card Component - Binance Style Enhanced
function StatCard({
  title,
  value,
  change,
  positive,
  subtitle,
}: {
  title: string
  value: string
  change?: number
  positive?: boolean
  subtitle?: string
}) {
  return (
    <div className="binance-card p-5 group hover:border-[#F0B90B]/40 transition-all duration-300">
      <div
        className="text-[11px] mb-2 font-bold uppercase tracking-[0.1em] flex items-center justify-between"
        style={{ color: '#848E9C' }}
      >
        <span>{title}</span>
        <div className="w-1 h-1 rounded-full bg-[#2B3139] group-hover:bg-[#F0B90B] transition-colors" />
      </div>
      <div
        className="text-2xl font-bold mb-1 font-mono tracking-tight"
        style={{ color: '#EAECEF' }}
      >
        {value}
      </div>
      <div className="flex items-center justify-between">
        {change !== undefined ? (
          <div
            className="text-sm font-mono font-bold flex items-center gap-1"
            style={{ color: positive ? '#0ECB81' : '#F6465D' }}
          >
            <span className="text-[10px]">{positive ? '▲' : '▼'}</span>
            <span>{positive ? '+' : ''}{change.toFixed(2)}%</span>
          </div>
        ) : <div />}
        {subtitle && (
          <div className="text-[11px] font-medium" style={{ color: '#5E6673' }}>
            {subtitle}
          </div>
        )}
      </div>
    </div>
  )
}

// Wrap App with providers
export default function AppWithProviders() {
  return (
    <LanguageProvider>
      <AuthProvider>
        <App />
      </AuthProvider>
    </LanguageProvider>
  )
}
