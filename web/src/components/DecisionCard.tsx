import { useState } from 'react'
import { AlertTriangle } from 'lucide-react'
import { api } from '../lib/api'
import { t, type Language } from '../i18n/translations'

// Decision Card Component with CoT Trace - Binance Style
export function DecisionCard({
  decision,
  language,
  traderId,
}: {
  decision: any
  language: Language
  traderId: string
}) {
  const [showSystemPrompt, setShowSystemPrompt] = useState(false)
  const [showInputPrompt, setShowInputPrompt] = useState(false)
  const [showCoT, setShowCoT] = useState(false)
  const [promptPreview, setPromptPreview] = useState<string | null>(null)
  const [loadingPreview, setLoadingPreview] = useState(false)

  // 字段映射适配
  const isStrategyDecision = 'decision_time' in decision
  const timestamp = isStrategyDecision ? decision.decision_time : decision.timestamp
  const cycleNumber = isStrategyDecision ? decision.id : decision.cycle_number
  const cotTrace = isStrategyDecision ? decision.raw_ai_response : decision.cot_trace

  // 对于策略执行日志，优先根据 execution_error 判断是否失败；
  // 没有错误时一律视为成功（包括 WAIT 决策），避免因为未写回 execution_success 导致全部显示为“失败”
  const success = isStrategyDecision
    ? !decision.execution_error // 有错误才算失败
    : decision.success
  const systemPrompt = decision.system_prompt
  const inputPrompt = decision.input_prompt
  const action = decision.action || ''
  const symbol = decision.symbol || ''
  const reason = decision.reason

  const handleToggleSystemPrompt = async () => {
    const next = !showSystemPrompt
    setShowSystemPrompt(next)
    if (next) {
      if (systemPrompt) {
        setPromptPreview(systemPrompt)
        return
      }
      try {
        setLoadingPreview(true)
        const data = await api.getPromptPreview(traderId)
        setPromptPreview(data.system_prompt || '')
      } catch (e) {
        // 失败时回退到记录内的system_prompt
        setPromptPreview(systemPrompt || '')
      } finally {
        setLoadingPreview(false)
      }
    }
  }

  return (
    <div
      className="rounded p-5 transition-all duration-300 hover:translate-y-[-2px]"
      style={{
        border: '1px solid #2B3139',
        background: '#1E2329',
        boxShadow: '0 2px 8px rgba(0, 0, 0, 0.3)',
      }}
    >
      {/* Header */}
      <div className="flex items-start justify-between mb-3">
        <div>
          <div className="font-semibold" style={{ color: '#EAECEF' }}>
            {t('cycle', language)} #{cycleNumber}
          </div>
          <div className="text-xs" style={{ color: '#848E9C' }}>
            {new Date(timestamp).toLocaleString()}
          </div>
        </div>
        <div
          className="px-3 py-1 rounded text-xs font-bold"
          style={
            success
              ? { background: 'rgba(14, 203, 129, 0.1)', color: '#0ECB81' }
              : { background: 'rgba(246, 70, 93, 0.1)', color: '#F6465D' }
          }
        >
          {t(success ? 'success' : 'failed', language)}
        </div>
      </div>

      {/* System Prompt - Collapsible */}
      {(systemPrompt || promptPreview !== null) && (
        <div className="mb-3">
          <button
            onClick={handleToggleSystemPrompt}
            className="flex items-center gap-2 text-sm transition-colors"
            style={{ color: '#a78bfa' }}
          >
            <span className="font-semibold">
              🎯 {t('systemPrompt', language)}
            </span>
            <span className="text-xs">
              {showSystemPrompt
                ? t('collapse', language)
                : t('expand', language)}
            </span>
          </button>
          {showSystemPrompt && (
            <div
              className="mt-2 rounded p-4 text-sm font-mono whitespace-pre-wrap max-h-96 overflow-y-auto"
              style={{
                background: '#0B0E11',
                border: '1px solid #2B3139',
                color: '#EAECEF',
              }}
            >
              {loadingPreview
                ? '加载中...'
                : (promptPreview ?? systemPrompt)}
            </div>
          )}
        </div>
      )}

      {/* Input Prompt - Collapsible */}
      {inputPrompt && (
        <div className="mb-3">
          <button
            onClick={() => setShowInputPrompt(!showInputPrompt)}
            className="flex items-center gap-2 text-sm transition-colors"
            style={{ color: '#60a5fa' }}
          >
            <span className="font-semibold">
              📥 {t('inputPrompt', language)}
            </span>
            <span className="text-xs">
              {showInputPrompt
                ? t('collapse', language)
                : t('expand', language)}
            </span>
          </button>
          {showInputPrompt && (
            <div
              className="mt-2 rounded p-4 text-sm font-mono whitespace-pre-wrap max-h-96 overflow-y-auto"
              style={{
                background: '#0B0E11',
                border: '1px solid #2B3139',
                color: '#EAECEF',
              }}
            >
              {inputPrompt}
            </div>
          )}
        </div>
      )}

      {/* AI Chain of Thought - Collapsible */}
      {cotTrace && (
        <div className="mb-3">
          <button
            onClick={() => setShowCoT(!showCoT)}
            className="flex items-center gap-2 text-sm transition-colors"
            style={{ color: '#F0B90B' }}
          >
            <span className="font-semibold">
              📤 💭 {t('aiThinking', language)}
            </span>
            <span className="text-xs">
              {showCoT ? t('collapse', language) : t('expand', language)}
            </span>
          </button>
          {showCoT && (
            <div
              className="mt-2 rounded p-4 text-sm font-mono whitespace-pre-wrap max-h-96 overflow-y-auto"
              style={{
                background: '#0B0E11',
                border: '1px solid #2B3139',
                color: '#EAECEF',
              }}
            >
              {cotTrace}
            </div>
          )}
        </div>
      )}

      {/* Decision Result */}
      {(action || symbol) && (
      <div className="mt-3 pt-3 border-t border-[#2B3139] flex items-center justify-between">
        <div className="flex items-center gap-3">
          <span className="font-bold font-mono" style={{ color: '#EAECEF' }}>
            {symbol}
          </span>
          <span
            className="font-bold text-sm"
            style={{
              color:
                action === 'wait' || action === 'WAIT'
                  ? '#F0B90B'
                  : action.toLowerCase().includes('buy') ||
                    action.toLowerCase().includes('long') || 
                    action.includes('OPEN_LONG') || 
                    action.includes('ADD_LONG')
                  ? '#0ECB81'
                  : '#F6465D',
            }}
          >
            {action}
          </span>
        </div>
      </div>
      )}

      {/* Reason (New) */}
      {reason && (
           <div className="text-xs mt-2 text-[#848E9C] italic border-t border-gray-800 pt-2">
             {reason}
           </div>
      )}

      {/* Decisions Actions (Old) */}
      {decision.decisions && decision.decisions.length > 0 && (
        <div className="space-y-2 mb-3">
          {decision.decisions.map((action: any, j: number) => (
            <div
              key={j}
              className="flex items-center gap-2 text-sm rounded px-3 py-2"
              style={{ background: '#0B0E11' }}
            >
              <span
                className="font-mono font-bold"
                style={{ color: '#EAECEF' }}
              >
                {action.symbol}
              </span>
              <span
                className="px-2 py-0.5 rounded text-xs font-bold"
                style={
                  action.action.includes('open')
                    ? {
                        background: 'rgba(96, 165, 250, 0.1)',
                        color: '#60a5fa',
                      }
                    : {
                        background: 'rgba(240, 185, 11, 0.1)',
                        color: '#F0B90B',
                      }
                }
              >
                {action.action}
              </span>
              {action.leverage > 0 && (
                <span style={{ color: '#F0B90B' }}>{action.leverage}x</span>
              )}
              {action.price > 0 && (
                <span
                  className="font-mono text-xs"
                  style={{ color: '#848E9C' }}
                >
                  @{action.price.toFixed(4)}
                </span>
              )}
              <span style={{ color: action.success ? '#0ECB81' : '#F6465D' }}>
                {action.success ? '✓' : '✗'}
              </span>
              {action.error && (
                <span className="text-xs ml-2" style={{ color: '#F6465D' }}>
                  {action.error}
                </span>
              )}
            </div>
          ))}
        </div>
      )}

      {/* Account State Summary (Old) */}
      {decision.account_state && (
        <div
          className="flex gap-4 text-xs mb-3 rounded px-3 py-2"
          style={{ background: '#0B0E11', color: '#848E9C' }}
        >
          <span>
            净值: {decision.account_state.total_balance.toFixed(2)} USDT
          </span>
          <span>
            可用: {decision.account_state.available_balance.toFixed(2)} USDT
          </span>
          <span>
            保证金率: {decision.account_state.margin_used_pct.toFixed(1)}%
          </span>
          <span>持仓: {decision.account_state.position_count}</span>
          <span
            style={{
              color:
                decision.candidate_coins &&
                decision.candidate_coins.length === 0
                  ? '#F6465D'
                  : '#848E9C',
            }}
          >
            {t('candidateCoins', language)}:{' '}
            {decision.candidate_coins?.length || 0}
          </span>
        </div>
      )}

      {/* Candidate Coins Warning (Old) */}
      {decision.candidate_coins && decision.candidate_coins.length === 0 && (
        <div
          className="text-sm rounded px-4 py-3 mb-3 flex items-start gap-3"
          style={{
            background: 'rgba(246, 70, 93, 0.1)',
            border: '1px solid rgba(246, 70, 93, 0.3)',
            color: '#F6465D',
          }}
        >
          <AlertTriangle size={16} className="flex-shrink-0 mt-0.5" />
          <div className="flex-1">
            <div className="font-semibold mb-1">
              ⚠️ {t('candidateCoinsZeroWarning', language)}
            </div>
            <div className="text-xs space-y-1" style={{ color: '#848E9C' }}>
              <div>{t('possibleReasons', language)}</div>
              <ul className="list-disc list-inside space-y-0.5 ml-2">
                <li>{t('coinPoolApiNotConfigured', language)}</li>
                <li>{t('apiConnectionTimeout', language)}</li>
                <li>{t('noCustomCoinsAndApiFailed', language)}</li>
              </ul>
              <div className="mt-2">
                <strong>{t('solutions', language)}</strong>
              </div>
              <ul className="list-disc list-inside space-y-0.5 ml-2">
                <li>{t('setCustomCoinsInConfig', language)}</li>
                <li>{t('orConfigureCorrectApiUrl', language)}</li>
                <li>{t('orDisableCoinPoolOptions', language)}</li>
              </ul>
            </div>
          </div>
        </div>
      )}

      {/* Execution Logs (Old) */}
      {decision.execution_log && Array.isArray(decision.execution_log) && decision.execution_log.length > 0 && (
        <div className="space-y-1">
          {decision.execution_log.map((log: string, k: number) => (
            <div
              key={k}
              className="text-xs font-mono"
              style={{
                color:
                  log.includes('✓') || log.includes('成功')
                    ? '#0ECB81'
                    : '#F6465D',
              }}
            >
              {log}
            </div>
          ))}
        </div>
      )}

      {/* Error Message */}
      {decision.error_message && (
        <div
          className="text-xs mt-3 p-2 rounded"
          style={{
            background: 'rgba(246, 70, 93, 0.1)',
            color: '#F6465D',
            border: '1px solid rgba(246, 70, 93, 0.2)',
          }}
        >
          {decision.error_message}
        </div>
      )}
    </div>
  )
}

