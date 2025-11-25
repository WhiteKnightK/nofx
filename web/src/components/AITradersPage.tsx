import React, { useState, useEffect, useMemo } from 'react'
import useSWR from 'swr'
import { api } from '../lib/api'
import type {
  TraderInfo,
  CreateTraderRequest,
  AIModel,
  Exchange,
} from '../types'

interface Category {
  id: string
  name: string
  description?: string
  owner_id: string
  created_at: string
}
import { useLanguage } from '../contexts/LanguageContext'
import { t, type Language } from '../i18n/translations'
import { useAuth } from '../contexts/AuthContext'
import { getExchangeIcon } from './ExchangeIcons'
import { getModelIcon } from './ModelIcons'
import { TraderConfigModal } from './TraderConfigModal'
import {
  TwoStageKeyModal,
  type TwoStageKeyModalResult,
} from './TwoStageKeyModal'
import {
  Bot,
  Brain,
  Landmark,
  BarChart3,
  Trash2,
  Plus,
  Users,
  AlertTriangle,
  BookOpen,
  HelpCircle,
  Radio,
  Copy,
  Check,
  ChevronDown,
  User,
  Eye,
} from 'lucide-react'
import { ToastContainer, ModernModal } from './Toast'

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

// 提取下划线后面的名称部分
function getShortName(fullName: string): string {
  const parts = fullName.split('_')
  return parts.length > 1 ? parts[parts.length - 1] : fullName
}

interface AITradersPageProps {
  onTraderSelect?: (traderId: string) => void
}

export function AITradersPage({ onTraderSelect }: AITradersPageProps) {
  const { language } = useLanguage()
  const { user, token } = useAuth()
  
  // 获取用户角色（默认为user，向后兼容）
  const userRole = user?.role || 'user'
  
  // 判断权限
  const isUser = userRole === 'user' || userRole === 'admin' // admin和user都可以配置
  const canEdit = isUser // 普通用户和管理员可以编辑自己的交易员
  const canCreate = isUser // 普通用户和管理员可以创建交易员
  const canDelete = isUser // 普通用户和管理员可以删除自己的交易员
  const canManageConfig = isUser // 配置功能（普通用户和管理员可以配置）
  const canCreateAccount = isUser // 普通用户和管理员可以创建交易员账号
  const canManageCategories = userRole === 'user' || userRole === 'admin' // 只有普通用户和管理员可以管理分类，小组组长和交易员看不到
  
  const [showCreateModal, setShowCreateModal] = useState(false)
  const [showEditModal, setShowEditModal] = useState(false)
  const [showModelModal, setShowModelModal] = useState(false)
  const [showExchangeModal, setShowExchangeModal] = useState(false)
  const [showSignalSourceModal, setShowSignalSourceModal] = useState(false)
  const [showCreateTraderAccountModal, setShowCreateTraderAccountModal] = useState(false)
  const [showCreateCategoryModal, setShowCreateCategoryModal] = useState(false)
  const [expandedCategories, setExpandedCategories] = useState<Set<string>>(new Set())
  const [showCategoryDetailModal, setShowCategoryDetailModal] = useState(false)
  const [selectedCategoryForDetail, setSelectedCategoryForDetail] = useState<any>(null)
  const [showCreateCategoryAccountModal, setShowCreateCategoryAccountModal] = useState(false)
  const [showCategoryAccountListModal, setShowCategoryAccountListModal] = useState(false)
  const [showCategoryAccountPage, setShowCategoryAccountPage] = useState(false)
  const [selectedCategoryForAccount, setSelectedCategoryForAccount] = useState<any>(null)
  const [selectedAccountInfo, setSelectedAccountInfo] = useState<any>(null)
  const [categoryAccounts, setCategoryAccounts] = useState<Array<{
    id: string
    email: string
    role: string
    trader_id?: string
    category: string
    created_at: string
  }>>([])

  // 在组件外部定义加载缓存的函数
  const loadCachedConfigs = () => {
    try {
      const cachedModels = localStorage.getItem('cached_ai_models')
      const cachedExchanges = localStorage.getItem('cached_exchanges')
      const cachedCategories = localStorage.getItem('cached_categories')
      
      return {
        models: cachedModels ? JSON.parse(cachedModels) : null,
        exchanges: cachedExchanges ? JSON.parse(cachedExchanges) : null,
        categories: cachedCategories ? JSON.parse(cachedCategories) : null,
      }
    } catch (e) {
      console.error('Failed to load cached configs:', e)
      return { models: null, exchanges: null, categories: null }
    }
  }

  // 加载初始缓存数据
  const cachedData = useMemo(() => loadCachedConfigs(), [])

  const [allModels, setAllModels] = useState<AIModel[] | undefined>(cachedData.models || undefined)
  const [allExchanges, setAllExchanges] = useState<Exchange[] | undefined>(cachedData.exchanges || undefined)
  const [categories, setCategories] = useState<Category[]>(cachedData.categories || [])
  // 从localStorage加载分类账号
  const loadCategoryAccountsFromStorage = (): Record<string, { email: string; password: string }> => {
    try {
      const stored = localStorage.getItem('category_accounts')
      return stored ? JSON.parse(stored) : {}
    } catch (error) {
      console.error('Failed to load category accounts from storage:', error)
    }
    return {}
  }

  // 保存分类账号密码到localStorage
  const saveCategoryAccountsToStorage = (accounts: Record<string, { email: string; password: string }>) => {
    try {
      localStorage.setItem('category_accounts', JSON.stringify(accounts))
    } catch (error) {
      console.error('Failed to save category accounts to storage:', error)
    }
  }

  const [categoryAccountPasswords, setCategoryAccountPasswords] = useState<Record<string, { email: string; password: string }>>(
    loadCategoryAccountsFromStorage()
  )
  const [groupLeaders, setGroupLeaders] = useState<Array<{
    id: string
    email: string
    role: string
    categories: string[]
    trader_count: number
    created_at: string
  }>>([])
  const [forceRefresh, setForceRefresh] = useState(0) // 强制刷新计数器
  const [creatingAccountForTrader, setCreatingAccountForTrader] = useState<string | null>(null)
  const [showTraderAccountInfoModal, setShowTraderAccountInfoModal] = useState(false)
  const [traderAccountInfo, setTraderAccountInfo] = useState<{
    traderId: string
    email: string
    password: string
  } | null>(null)
  // 从localStorage加载保存的账号密码信息
  const loadTraderAccountsFromStorage = (): Record<string, { email: string; password: string }> => {
    try {
      const stored = localStorage.getItem('trader_accounts')
      if (stored) {
        return JSON.parse(stored)
      }
    } catch (error) {
      console.error('Failed to load trader accounts from storage:', error)
    }
    return {}
  }

  const [traderAccounts, setTraderAccounts] = useState<Record<string, { email: string; password: string }>>(
    loadTraderAccountsFromStorage()
  )
  const [traderHasAccount, setTraderHasAccount] = useState<Record<string, boolean>>({})


  // 保存账号密码到localStorage
  const saveTraderAccountsToStorage = (accounts: Record<string, { email: string; password: string }>) => {
    try {
      localStorage.setItem('trader_accounts', JSON.stringify(accounts))
    } catch (error) {
      console.error('Failed to save trader accounts to storage:', error)
    }
  }

  const [toasts, setToasts] = useState<Array<{ id: string; message: string; type: 'success' | 'error' | 'warning' | 'info' }>>([])
  
  // 显示Toast提示
  const showToast = (message: string, type: 'success' | 'error' | 'warning' | 'info' = 'info') => {
    const id = `toast-${Date.now()}-${Math.random().toString(36).substr(2, 9)}`
    setToasts((prev) => [...prev, { id, message, type }])
  }

  const removeToast = (id: string) => {
    setToasts((prev) => prev.filter((t) => t.id !== id))
  }

  const [editingModel, setEditingModel] = useState<string | null>(null)
  const [editingExchange, setEditingExchange] = useState<string | null>(null)
  const [editingTrader, setEditingTrader] = useState<any>(null)
  const [supportedModels, setSupportedModels] = useState<AIModel[]>([])
  const [supportedExchanges, setSupportedExchanges] = useState<Exchange[]>([])
  const [userSignalSource, setUserSignalSource] = useState<{
    coinPoolUrl: string
    oiTopUrl: string
  }>({
    coinPoolUrl: '',
    oiTopUrl: '',
  })

  const { data: traders, mutate: mutateTraders } = useSWR<TraderInfo[]>(
    user && token ? 'traders' : null,
    api.getTraders,
    { refreshInterval: 5000 }
  )

  // 检查交易员是否有账号（用于显示按钮文本）
  useEffect(() => {
    const loadTraderAccountStatus = async () => {
      if (!traders || traders.length === 0) return
      
      const accountStatus: Record<string, boolean> = {}
      await Promise.all(
        traders.map(async (trader) => {
          try {
            const result = await api.getTraderAccount(trader.trader_id)
            accountStatus[trader.trader_id] = !!result.account
          } catch (error) {
            accountStatus[trader.trader_id] = false
          }
        })
      )
      setTraderHasAccount(accountStatus)
    }
    
    if (user && token && traders) {
      loadTraderAccountStatus()
    }
  }, [traders, user, token])

  // 加载AI模型和交易所配置
  useEffect(() => {
    const loadConfigs = async () => {
      if (!user || !token) {
        // 未登录时只加载公开的支持模型和交易所
        try {
          const [supportedModels, supportedExchanges] = await Promise.all([
            api.getSupportedModels(),
            api.getSupportedExchanges(),
          ])
          setSupportedModels(supportedModels)
          setSupportedExchanges(supportedExchanges)
        } catch (err) {
          console.error('Failed to load supported configs:', err)
        }
        return
      }

      try {
        const [
          modelConfigs,
          exchangeConfigs,
          supportedModels,
          supportedExchanges,
        ] = await Promise.all([
          api.getModelConfigs(),
          api.getExchangeConfigs(),
          api.getSupportedModels(),
          api.getSupportedExchanges(),
        ])
        // 🔍 调试：检查模型配置数据
        console.log('📦 加载的模型配置数据（原始）:', modelConfigs)
        console.log('📦 加载的模型配置数据（摘要）:', modelConfigs?.map(m => ({
          id: m.id,
          name: m.name,
          provider: m.provider,
          apiKey: m.apiKey ? `${m.apiKey.substring(0, 10)}...` : '(空)',
          apiKeyLength: m.apiKey?.length || 0,
          enabled: m.enabled,
          customApiUrl: m.customApiUrl || '(空)',
          customModelName: m.customModelName || '(空)',
        })))
        setAllModels(modelConfigs)
        setAllExchanges(exchangeConfigs)
        setSupportedModels(supportedModels)
        setSupportedExchanges(supportedExchanges)

        // 更新缓存
        try {
          localStorage.setItem('cached_ai_models', JSON.stringify(modelConfigs))
          localStorage.setItem('cached_exchanges', JSON.stringify(exchangeConfigs))
        } catch (e) {
          console.error('Failed to cache configs:', e)
        }

        // 加载用户信号源配置
        try {
          const signalSource = await api.getUserSignalSource()
          setUserSignalSource({
            coinPoolUrl: signalSource.coin_pool_url || '',
            oiTopUrl: signalSource.oi_top_url || '',
          })
        } catch (error) {
          console.log('📡 用户信号源配置暂未设置')
        }

        // 加载分类列表（如果用户有权限）
        if (canManageCategories) {
          try {
            const categoriesList = await api.getCategories()
            setCategories(categoriesList)
            // 更新缓存
            try {
              localStorage.setItem('cached_categories', JSON.stringify(categoriesList))
            } catch (e) {
              console.error('Failed to cache categories:', e)
            }
            // 同时加载账号和小组组长列表
            await loadCategoryAccounts()
            await loadGroupLeaders()
          } catch (error) {
            console.error('Failed to load categories:', error)
          }
        }
      } catch (error) {
        console.error('Failed to load configs:', error)
      }
    }
    loadConfigs()
  }, [user, token, canManageCategories])

  // 只显示已配置的模型和交易所
  // 🔑 注意：后端现在会返回 API Key（已解密），所以我们可以通过 enabled 或其他字段判断是否已配置
  const configuredModels =
    allModels?.filter((m) => {
      // 如果模型已启用，说明已配置
      // 或者有自定义API URL，也说明已配置
      // 或者有 API Key，也说明已配置（新增判断）
      return m.enabled || (m.customApiUrl && m.customApiUrl.trim() !== '') || (m.apiKey && m.apiKey.trim() !== '')
    }) || []
  
  // 🔍 调试：检查 configuredModels 的数据
  if (configuredModels.length > 0) {
    console.log('🔍 configuredModels 过滤后的数据:', configuredModels.map(m => ({
      id: m.id,
      name: m.name,
      apiKey: m.apiKey ? `${m.apiKey.substring(0, 20)}...` : '(空)',
      apiKeyLength: m.apiKey?.length || 0,
    })))
  }
  const configuredExchanges =
    allExchanges?.filter((e) => {
      // Aster 交易所检查特殊字段
      if (e.id === 'aster') {
        return e.asterUser && e.asterUser.trim() !== ''
      }
      // Hyperliquid 需要检查钱包地址（后端会返回这个字段）
      if (e.id === 'hyperliquid') {
        return e.hyperliquidWalletAddr && e.hyperliquidWalletAddr.trim() !== ''
      }
      // 其他交易所：如果已启用，说明已配置（后端返回的已配置交易所会有 enabled: true）
      return e.enabled
    }) || []

  // 只在创建交易员时使用已启用且配置完整的
  // 注意：后端返回的数据不包含敏感信息，所以只检查 enabled 状态和必要的非敏感字段
  // 🔧 修复：使用useMemo避免频繁重新创建数组，导致TraderConfigModal表单重置
  const enabledModels = useMemo(() => allModels?.filter((m) => m.enabled) || [], [allModels])
  const enabledExchanges = useMemo(
    () =>
    allExchanges?.filter((e) => {
      if (!e.enabled) return false

      // Aster 交易所需要特殊字段（后端会返回这些非敏感字段）
      if (e.id === 'aster') {
        return (
          e.asterUser &&
          e.asterUser.trim() !== '' &&
          e.asterSigner &&
          e.asterSigner.trim() !== ''
        )
      }

      // Hyperliquid 需要钱包地址（后端会返回这个字段）
      if (e.id === 'hyperliquid') {
        return e.hyperliquidWalletAddr && e.hyperliquidWalletAddr.trim() !== ''
      }

      // 其他交易所：如果已启用，说明已配置完整（后端只返回已配置的交易所）
      return true
      }) || [],
    [allExchanges]
  )

  // 检查模型是否正在被运行中的交易员使用（用于UI禁用）
  const isModelInUse = (modelId: string) => {
    return traders?.some((t) => t.ai_model === modelId && t.is_running)
  }

  // 检查交易所是否正在被运行中的交易员使用（用于UI禁用）
  const isExchangeInUse = (exchangeId: string) => {
    return traders?.some((t) => t.exchange_id === exchangeId && t.is_running)
  }

  // 检查模型是否被任何交易员使用（包括停止状态的）
  const isModelUsedByAnyTrader = (modelId: string) => {
    return traders?.some((t) => t.ai_model === modelId) || false
  }

  // 检查交易所是否被任何交易员使用（包括停止状态的）
  const isExchangeUsedByAnyTrader = (exchangeId: string) => {
    return traders?.some((t) => t.exchange_id === exchangeId) || false
  }

  // 获取使用特定模型的交易员列表
  const getTradersUsingModel = (modelId: string) => {
    return traders?.filter((t) => t.ai_model === modelId) || []
  }

  // 获取使用特定交易所的交易员列表
  const getTradersUsingExchange = (exchangeId: string) => {
    return traders?.filter((t) => t.exchange_id === exchangeId) || []
  }

  const handleCreateTrader = async (data: CreateTraderRequest) => {
    try {
      const model = allModels?.find((m) => m.id === data.ai_model_id)
      const exchange = allExchanges?.find((e) => e.id === data.exchange_id)

      if (!model?.enabled) {
        showToast(t('modelNotConfigured', language), 'warning')
        return
      }

      if (!exchange?.enabled) {
        showToast(t('exchangeNotConfigured', language), 'warning')
        return
      }

      await api.createTrader(data)
      setShowCreateModal(false)
      mutateTraders()
    } catch (error) {
      console.error('Failed to create trader:', error)
      showToast(t('createTraderFailed', language), 'error')
    }
  }

  const handleEditTrader = async (traderId: string) => {
    try {
      const traderConfig = await api.getTraderConfig(traderId)
      setEditingTrader(traderConfig)
      setShowEditModal(true)
    } catch (error) {
      console.error('Failed to fetch trader config:', error)
      showToast(t('getTraderConfigFailed', language), 'error')
    }
  }

  const handleSaveEditTrader = async (data: CreateTraderRequest) => {
    if (!editingTrader) return

    try {
      const model = enabledModels?.find((m) => m.id === data.ai_model_id)
      const exchange = enabledExchanges?.find((e) => e.id === data.exchange_id)

      if (!model) {
        showToast(t('modelConfigNotExist', language), 'warning')
        return
      }

      if (!exchange) {
        showToast(t('exchangeConfigNotExist', language), 'warning')
        return
      }

      const request = {
        name: data.name,
        ai_model_id: data.ai_model_id,
        exchange_id: data.exchange_id,
        initial_balance: data.initial_balance,
        system_prompt_template: data.system_prompt_template, // 回传当前提示词模板
        scan_interval_minutes: data.scan_interval_minutes,
        btc_eth_leverage: data.btc_eth_leverage,
        altcoin_leverage: data.altcoin_leverage,
        trading_symbols: data.trading_symbols,
        custom_prompt: data.custom_prompt,
        override_base_prompt: data.override_base_prompt,
        is_cross_margin: data.is_cross_margin,
        use_coin_pool: data.use_coin_pool,
        use_oi_top: data.use_oi_top,
      }

      await api.updateTrader(editingTrader.trader_id, request)
      setShowEditModal(false)
      setEditingTrader(null)
      mutateTraders()
    } catch (error) {
      console.error('Failed to update trader:', error)
      showToast(t('updateTraderFailed', language), 'error')
    }
  }

  const handleDeleteTrader = async (traderId: string) => {
    if (!confirm(t('confirmDeleteTrader', language))) return

    try {
      await api.deleteTrader(traderId)
      mutateTraders()
    } catch (error) {
      console.error('Failed to delete trader:', error)
      showToast(t('deleteTraderFailed', language), 'error')
    }
  }

  const handleToggleTrader = async (traderId: string, running: boolean) => {
    // 🚀 Optimistic UI Update (乐观更新)
    // 立即在本地更新 UI 状态，而不是等待 API 响应
    // 这样用户会感觉到操作是"即时"的
    const previousTraders = traders
    if (traders && traders.length > 0) {
      const updatedTraders = traders.map(t => 
        t.trader_id === traderId ? { ...t, is_running: !running } : t
      )
      // 更新本地缓存
      mutateTraders(updatedTraders, false)
    }

    try {
      if (running) {
        await api.stopTrader(traderId)
      } else {
        await api.startTrader(traderId)
      }
      // 成功后，重新验证数据以确保一致性
      mutateTraders()
    } catch (error) {
      console.error('Failed to toggle trader:', error)
      showToast(t('operationFailed', language), 'error')
      
      // ❌ 如果失败，回滚到之前的状态
      if (previousTraders) {
        mutateTraders(previousTraders, false)
      }
    }
  }

  const handleModelClick = (modelId: string) => {
    if (!canManageConfig) return // 没有权限，不处理
    if (!isModelInUse(modelId)) {
      setEditingModel(modelId)
      setShowModelModal(true)
    }
  }

  const handleExchangeClick = (exchangeId: string) => {
    if (!canManageConfig) return // 没有权限，不处理
    if (!isExchangeInUse(exchangeId)) {
      setEditingExchange(exchangeId)
      setShowExchangeModal(true)
    }
  }

  // 通用删除配置处理函数
  const handleDeleteConfig = async <T extends { id: string }>(config: {
    id: string
    type: 'model' | 'exchange'
    checkInUse: (id: string) => boolean
    getUsingTraders: (id: string) => any[]
    cannotDeleteKey: string
    confirmDeleteKey: string
    allItems: T[] | undefined
    clearFields: (item: T) => T
    buildRequest: (items: T[]) => any
    updateApi: (request: any) => Promise<void>
    refreshApi: () => Promise<T[]>
    setItems: (items: T[]) => void
    closeModal: () => void
    errorKey: string
  }) => {
    // 检查是否有交易员正在使用
    if (config.checkInUse(config.id)) {
      const usingTraders = config.getUsingTraders(config.id)
      const traderNames = usingTraders.map((t) => t.trader_name).join(', ')
      showToast(
        `${t(config.cannotDeleteKey, language)} - ${t('tradersUsing', language)}: ${traderNames}`,
        'warning'
      )
      return
    }

    if (!confirm(t(config.confirmDeleteKey, language))) return

    try {
      const updatedItems =
        config.allItems?.map((item) =>
          item.id === config.id ? config.clearFields(item) : item
        ) || []

      const request = config.buildRequest(updatedItems)
      await config.updateApi(request)

      // 重新获取用户配置以确保数据同步
      const refreshedItems = await config.refreshApi()
      config.setItems(refreshedItems)

      config.closeModal()
    } catch (error) {
      console.error(`Failed to delete ${config.type} config:`, error)
      showToast(t(config.errorKey, language), 'error')
    }
  }

  const handleDeleteModelConfig = async (modelId: string) => {
    await handleDeleteConfig({
      id: modelId,
      type: 'model',
      checkInUse: isModelUsedByAnyTrader,
      getUsingTraders: getTradersUsingModel,
      cannotDeleteKey: 'cannotDeleteModelInUse',
      confirmDeleteKey: 'confirmDeleteModel',
      allItems: allModels,
      clearFields: (m) => ({
        ...m,
        apiKey: '',
        customApiUrl: '',
        customModelName: '',
        enabled: false,
      }),
      buildRequest: (models) => ({
        models: Object.fromEntries(
          models.map((model) => [
            model.id, // 使用完整的 id（格式: userID_provider）
            {
              enabled: model.enabled,
              api_key: model.apiKey || '',
              custom_api_url: model.customApiUrl || '',
              custom_model_name: model.customModelName || '',
            },
          ])
        ),
      }),
      updateApi: api.updateModelConfigs,
      refreshApi: api.getModelConfigs,
      setItems: (items) => {
        // 使用函数式更新确保状态正确更新
        setAllModels([...items])
      },
      closeModal: () => {
        setShowModelModal(false)
        setEditingModel(null)
      },
      errorKey: 'deleteConfigFailed',
    })
  }

  const handleSaveModelConfig = async (
    modelId: string,
    apiKey: string,
    customApiUrl?: string,
    customModelName?: string
  ) => {
    try {
      // 创建或更新用户的模型配置
      const existingModel = allModels?.find((m) => m.id === modelId)
      let updatedModels

      // 找到要配置的模型（优先从已配置列表，其次从支持列表）
      const modelToUpdate =
        existingModel || supportedModels?.find((m) => m.id === modelId)
      if (!modelToUpdate) {
        showToast(t('modelNotExist', language), 'warning')
        return
      }

      if (existingModel) {
        // 更新现有配置
        updatedModels =
          allModels?.map((m) =>
            m.id === modelId
              ? {
                  ...m,
                  apiKey,
                  customApiUrl: customApiUrl || '',
                  customModelName: customModelName || '',
                  enabled: true,
                }
              : m
          ) || []
      } else {
        // 添加新配置
        const newModel = {
          ...modelToUpdate,
          apiKey,
          customApiUrl: customApiUrl || '',
          customModelName: customModelName || '',
          enabled: true,
        }
        updatedModels = [...(allModels || []), newModel]
      }

      const request = {
        models: Object.fromEntries(
          updatedModels.map((model) => [
            model.id, // 使用完整的 id（格式: userID_provider）
            {
              enabled: model.enabled,
              api_key: model.apiKey || '',
              custom_api_url: model.customApiUrl || '',
              custom_model_name: model.customModelName || '',
            },
          ])
        ),
      }

      await api.updateModelConfigs(request)

      // 重新获取用户配置以确保数据同步
      const refreshedModels = await api.getModelConfigs()
      setAllModels(refreshedModels)

      setShowModelModal(false)
      setEditingModel(null)
    } catch (error) {
      console.error('Failed to save model config:', error)
      showToast(t('saveConfigFailed', language), 'error')
    }
  }

  const handleDeleteExchangeConfig = async (exchangeId: string) => {
    await handleDeleteConfig({
      id: exchangeId,
      type: 'exchange',
      checkInUse: isExchangeUsedByAnyTrader,
      getUsingTraders: getTradersUsingExchange,
      cannotDeleteKey: 'cannotDeleteExchangeInUse',
      confirmDeleteKey: 'confirmDeleteExchange',
      allItems: allExchanges,
      clearFields: (e) => ({
        ...e,
        apiKey: '',
        secretKey: '',
        hyperliquidWalletAddr: '',
        asterUser: '',
        asterSigner: '',
        asterPrivateKey: '',
        enabled: false,
      }),
      buildRequest: (exchanges) => ({
        exchanges: Object.fromEntries(
          exchanges.map((exchange) => [
            exchange.id,
            {
              enabled: exchange.enabled,
              api_key: exchange.apiKey || '',
              secret_key: exchange.secretKey || '',
              testnet: exchange.testnet || false,
              hyperliquid_wallet_addr: exchange.hyperliquidWalletAddr || '',
              aster_user: exchange.asterUser || '',
              aster_signer: exchange.asterSigner || '',
              aster_private_key: exchange.asterPrivateKey || '',
            },
          ])
        ),
      }),
      updateApi: api.updateExchangeConfigsEncrypted,
      refreshApi: api.getExchangeConfigs,
      setItems: (items) => {
        // 使用函数式更新确保状态正确更新
        setAllExchanges([...items])
      },
      closeModal: () => {
        setShowExchangeModal(false)
        setEditingExchange(null)
      },
      errorKey: 'deleteExchangeConfigFailed',
    })
  }

  const handleSaveExchangeConfig = async (
    exchangeId: string,
    apiKey: string,
    secretKey?: string,
    testnet?: boolean,
    hyperliquidWalletAddr?: string,
    asterUser?: string,
    asterSigner?: string,
    asterPrivateKey?: string,
    passphrase?: string,
    userLabel?: string
  ) => {
    try {
      // 尝试解析 Provider (如果 ID 是 binance_123，Provider 是 binance)
      let provider = exchangeId
      if (exchangeId.includes('_')) {
        const parts = exchangeId.split('_')
        // 假设格式是 provider_suffix
        provider = parts[0] 
      }

      // 找到要配置的交易所（从supportedExchanges中）
      const exchangeToUpdate = supportedExchanges?.find(
        (e) => e.id === provider || e.id === exchangeId
      )
      if (!exchangeToUpdate) {
        showToast(t('exchangeNotExist', language), 'warning')
        return
      }

      const trimmedUserLabel = (userLabel || '').trim()

      // 🔑 关键修复：检查是否是编辑模式
      // 只有当 editingExchange 不为 null 时，才是真正的编辑模式
      const isEditMode = editingExchange !== null
      
      // 检查是否已存在相同ID的配置（编辑模式下，查找 editingExchange 对应的记录）
      const existingExchange = isEditMode 
        ? allExchanges?.find((e) => e.id === editingExchange)
        : allExchanges?.find((e) => e.id === exchangeId)
      
      let updatedExchanges
      // 编辑模式下使用 editingExchange 作为最终ID，添加模式下使用 exchangeId（可能会被修改为唯一ID）
      let finalExchangeId = isEditMode ? (editingExchange || exchangeId) : exchangeId
      // 默认标签：优先使用用户输入，其次是已有 label，其次是名称
      let finalLabel =
        trimmedUserLabel ||
        (existingExchange as any)?.label ||
        existingExchange?.name ||
        exchangeToUpdate.name

      if (isEditMode && existingExchange) {
        // ✅ 真正的编辑模式：更新现有配置
        updatedExchanges =
          allExchanges?.map((e) =>
            e.id === finalExchangeId
              ? {
                  ...e,
                  apiKey,
                  secretKey,
                  testnet,
                  hyperliquidWalletAddr,
                  asterUser,
                  asterSigner,
                  asterPrivateKey,
                  passphrase,
                  enabled: true,
                  provider: provider, // 确保 provider 存在
                  label: trimmedUserLabel || (e as any).label || e.name, // 优先使用用户新输入的，如果没有输入则保持原有
                }
              : e
          ) || []
      } else {
        // ✅ 添加新配置模式（即使找到了同名记录，也生成新的唯一ID）
        // 如果 exchangeId 等于 provider（基础类型，如 "binance"），生成唯一 ID
        if (exchangeId === provider) {
          finalExchangeId = `${provider}_${Date.now()}`
          // 如果用户没有输入自定义标签，则生成默认序号标签
          if (!trimmedUserLabel) {
            const index =
              (allExchanges?.filter((e) =>
                (e as any).provider === provider || e.id.startsWith(provider)
              ).length || 0) + 1
            finalLabel = `${exchangeToUpdate.name} #${index}`
          }
        }

        const newExchange = {
          ...exchangeToUpdate,
          id: finalExchangeId,
          apiKey,
          secretKey,
          testnet,
          hyperliquidWalletAddr,
          asterUser,
          asterSigner,
          asterPrivateKey,
          passphrase,
          enabled: true,
          provider: provider,
          label: finalLabel,
        }
        updatedExchanges = [...(allExchanges || []), newExchange]
      }

      const request = {
        exchanges: Object.fromEntries(
          updatedExchanges.map((exchange) => [
            exchange.id,
            {
              enabled: exchange.enabled,
              api_key: exchange.apiKey || '',
              secret_key: exchange.secretKey || '',
              passphrase: exchange.passphrase || '',
              testnet: exchange.testnet || false,
              hyperliquid_wallet_addr: exchange.hyperliquidWalletAddr || '',
              aster_user: exchange.asterUser || '',
              aster_signer: exchange.asterSigner || '',
              aster_private_key: exchange.asterPrivateKey || '',
              provider: (exchange as any).provider || (exchange.id.includes('_') ? exchange.id.split('_')[0] : exchange.id),
              label: (exchange as any).label || exchange.name
            },
          ])
        ),
      }

      await api.updateExchangeConfigsEncrypted(request)

      // 重新获取用户配置以确保数据同步
      const refreshedExchanges = await api.getExchangeConfigs()
      setAllExchanges(refreshedExchanges)
      
      // 更新缓存
      try {
        localStorage.setItem('cached_exchanges', JSON.stringify(refreshedExchanges))
      } catch (e) {
        console.error('Failed to update exchanges cache:', e)
      }

      setShowExchangeModal(false)
      setEditingExchange(null)
    } catch (error) {
      console.error('Failed to save exchange config:', error)
      showToast(t('saveConfigFailed', language), 'error')
    }
  }

  const handleAddModel = () => {
    setEditingModel(null)
    setShowModelModal(true)
  }

  const handleAddExchange = () => {
    setEditingExchange(null)
    setShowExchangeModal(true)
  }

  const handleSaveSignalSource = async (
    coinPoolUrl: string,
    oiTopUrl: string
  ) => {
    try {
      await api.saveUserSignalSource(coinPoolUrl, oiTopUrl)
      setUserSignalSource({ coinPoolUrl, oiTopUrl })
      setShowSignalSourceModal(false)
    } catch (error) {
      console.error('Failed to save signal source:', error)
      showToast(t('saveSignalSourceFailed', language), 'error')
    }
  }

  // 创建交易员账号
  const handleCreateTraderAccount = async (traderId: string, options: {
    generate_random_email: boolean
    generate_random_password: boolean
    email?: string
    password?: string
  }) => {
    try {
      const result = await api.createTraderAccount(traderId, options)
      // 保存账号信息到state和localStorage（包含密码，可以随时查看）
      const newAccounts = {
        ...traderAccounts,
        [traderId]: {
          email: result.email,
          password: result.password,
        }
      }
      setTraderAccounts(newAccounts)
      saveTraderAccountsToStorage(newAccounts)
      // 更新账号状态
      setTraderHasAccount(prev => ({
        ...prev,
        [traderId]: true,
      }))
      // 显示账号信息弹窗
      setTraderAccountInfo({
        traderId,
        email: result.email,
        password: result.password,
      })
      setShowTraderAccountInfoModal(true)
      setShowCreateTraderAccountModal(false)
      setCreatingAccountForTrader(null)
    } catch (error: any) {
      console.error('Failed to create trader account:', error)
      showToast(error.message || '创建交易员账号失败', 'error')
    }
  }


  // 创建分类
  const handleCreateCategory = async (name: string, description?: string) => {
    try {
      await api.createCategory(name, description)
      // 重新加载分类列表
      const categoriesList = await api.getCategories()
      setCategories(categoriesList)
      setShowCreateCategoryModal(false)
      showToast('分类创建成功！', 'success')
    } catch (error: any) {
      console.error('Failed to create category:', error)
      showToast('创建分类失败: ' + (error.message || '未知错误'), 'error')
    }
  }

  // 设置交易员分类（从分类详情模态框调用）
  const handleSetTraderCategory = async (traderId: string, category: string) => {
    try {
      console.log('[handleSetTraderCategory] Starting update:', { traderId, category })
      
      const response = await api.setTraderCategory(traderId, category)
      console.log('[handleSetTraderCategory] API response:', response)

      // 先本地乐观更新，立即反映到UI
      await mutateTraders((current) => {
        if (!current) return current
        return current.map(t =>
          t.trader_id === traderId ? { ...t, category } as any : t
        )
      }, { revalidate: false })

      // 再触发一次真实拉取，确保与后端一致
      console.log('[handleSetTraderCategory] Revalidating traders from server...')
      await mutateTraders()
      
      // 再等待一下确保SWR缓存已更新
      await new Promise(resolve => setTimeout(resolve, 300))
      
      const categoriesList = await api.getCategories()
      setCategories(categoriesList)

      // 强制刷新CategoryDetailModal
      setForceRefresh(prev => prev + 1)

      console.log('[handleSetTraderCategory] Update complete')

      // 不在这里显示toast，由调用者决定是否显示
      return response
    } catch (error: any) {
      console.error('[handleSetTraderCategory] Error:', error)
      const errorMessage = error.message || '未知错误'
      showToast('设置交易员分类失败: ' + errorMessage, 'error')
      throw error
    }
  }

  // 从分类中移除交易员（设置为空分类）
  const handleRemoveTraderFromCategory = async (traderId: string) => {
    try {
      await api.setTraderCategory(traderId, '')
      // 乐观更新本地缓存
      await mutateTraders((current) => {
        if (!current) return current
        return current.map(t =>
          t.trader_id === traderId ? { ...t, category: '' } as any : t
        )
      }, { revalidate: false })
      // 后台校准
      mutateTraders()
      showToast('交易员已从分类中移除！', 'success')
    } catch (error: any) {
      console.error('Failed to remove trader from category:', error)
      showToast('移除交易员失败: ' + (error.message || '未知错误'), 'error')
    }
  }

  // 加载分类账号列表
  const loadCategoryAccounts = async () => {
    try {
      const accountsList = await api.getCategoryAccounts()
      setCategoryAccounts(accountsList)
    } catch (error: any) {
      console.error('Failed to load category accounts:', error)
    }
  }

  // 加载小组组长列表
  const loadGroupLeaders = async () => {
    try {
      const groupLeadersList = await api.getGroupLeaders()
      setGroupLeaders(groupLeadersList)
    } catch (error: any) {
      console.error('Failed to load group leaders:', error)
    }
  }

  // 创建分类账号
  const handleCreateCategoryAccount = async (options: {
    generate_random_email: boolean
    generate_random_password: boolean
    email?: string
    password?: string
    category: string
    role: 'group_leader'
  }) => {
    try {
      const result = await api.createGroupLeaderForCategory({
        generate_random_email: options.generate_random_email,
        generate_random_password: options.generate_random_password,
        email: options.email,
        password: options.password,
        category: options.category,
      })

      if (result && typeof result === 'object' && 'email' in result) {
        // 保存密码到本地存储
        if (result.password && result.user_id) {
          const newAccounts = {
            ...categoryAccountPasswords,
            [result.user_id]: {
              email: result.email,
              password: result.password,
            }
          }
          setCategoryAccountPasswords(newAccounts)
          saveCategoryAccountsToStorage(newAccounts)
        }

        showToast(`小组组长账号创建成功！账号: ${result.email}`, 'success')
      }
      setShowCreateCategoryAccountModal(false)
      setSelectedCategoryForAccount(null)
      // 刷新账号列表
      await loadCategoryAccounts()
      await loadGroupLeaders()
    } catch (error: any) {
      console.error('Failed to create category account:', error)
      showToast(error.message || '创建账号失败', 'error')
    }
  }

  // 查看账号信息
  const handleViewAccountInfo = async (accountId: string) => {
    try {
      const accountInfo = await api.getCategoryAccountInfo(accountId)
      setSelectedAccountInfo(accountInfo)
      setShowCategoryAccountPage(true)
    } catch (error: any) {
      console.error('Failed to load account info:', error)
      showToast('获取账号信息失败: ' + (error.message || '未知错误'), 'error')
    }
  }

  // 按分类分组交易员
  const groupTradersByCategory = () => {
    if (!traders) return {}
    const grouped: Record<string, typeof traders> = {}
    const uncategorized: typeof traders = []

    traders.forEach((trader) => {
      const category = trader.category || ''
      if (category) {
        if (!grouped[category]) {
          grouped[category] = []
        }
        grouped[category].push(trader)
      } else {
        uncategorized.push(trader)
      }
    })

    if (uncategorized.length > 0) {
      grouped['未分类'] = uncategorized
    }

    return grouped
  }

  // 获取分类下的小组组长
  const getCategoryGroupLeaders = (categoryName: string) => {
    if (!Array.isArray(groupLeaders)) {
      return []
    }
    return groupLeaders.filter((leader) => leader.categories.includes(categoryName))
  }

  // 检查分类是否已有管理员账号
  const hasCategoryAdminAccount = (categoryName: string) => {
    if (!Array.isArray(groupLeaders)) {
      return false
    }
    return groupLeaders.some((leader) => leader.categories.includes(categoryName))
  }



  return (
    <div className="space-y-4 md:space-y-6 animate-fade-in">
      {/* Toast提示 */}
      <ToastContainer toasts={toasts} onRemove={removeToast} />

      {/* Header */}
      <div className="flex flex-col md:flex-row items-start md:items-center justify-between gap-3 md:gap-0">
        <div className="flex items-center gap-3 md:gap-4">
          <div
            className="w-10 h-10 md:w-12 md:h-12 rounded-xl flex items-center justify-center"
            style={{
              background: 'linear-gradient(135deg, #F0B90B 0%, #FCD535 100%)',
              boxShadow: '0 4px 14px rgba(240, 185, 11, 0.4)',
            }}
          >
            <Bot className="w-5 h-5 md:w-6 md:h-6" style={{ color: '#000' }} />
          </div>
          <div>
            <h1
              className="text-xl md:text-2xl font-bold flex items-center gap-2"
              style={{ color: '#EAECEF' }}
            >
              {t('aiTraders', language)}
              <span
                className="text-xs font-normal px-2 py-1 rounded"
                style={{
                  background: 'rgba(240, 185, 11, 0.15)',
                  color: '#F0B90B',
                }}
              >
                {traders?.length || 0} {t('active', language)}
              </span>
            </h1>
            <p className="text-xs" style={{ color: '#848E9C' }}>
              {t('manageAITraders', language)}
            </p>
          </div>
        </div>

        <div className="flex gap-2 md:gap-3 w-full md:w-auto overflow-hidden flex-wrap md:flex-nowrap">
          {canManageConfig && (
            <>
              <button
                onClick={handleAddModel}
                className="px-3 md:px-4 py-2 rounded text-xs md:text-sm font-semibold transition-all hover:scale-105 flex items-center gap-1 md:gap-2 whitespace-nowrap"
                style={{
                  background: '#2B3139',
                  color: '#EAECEF',
                  border: '1px solid #474D57',
                }}
              >
                <Plus className="w-3 h-3 md:w-4 md:h-4" />
                {t('aiModels', language)}
              </button>

              <button
                onClick={handleAddExchange}
                className="px-3 md:px-4 py-2 rounded text-xs md:text-sm font-semibold transition-all hover:scale-105 flex items-center gap-1 md:gap-2 whitespace-nowrap"
                style={{
                  background: '#2B3139',
                  color: '#EAECEF',
                  border: '1px solid #474D57',
                }}
              >
                <Plus className="w-3 h-3 md:w-4 md:h-4" />
                {t('exchanges', language)}
              </button>

              <button
                onClick={() => setShowSignalSourceModal(true)}
                className="px-3 md:px-4 py-2 rounded text-xs md:text-sm font-semibold transition-all hover:scale-105 flex items-center gap-1 md:gap-2 whitespace-nowrap"
                style={{
                  background: '#2B3139',
                  color: '#EAECEF',
                  border: '1px solid #474D57',
                }}
              >
                <Radio className="w-3 h-3 md:w-4 md:h-4" />
                {t('signalSource', language)}
              </button>
            </>
          )}

          {canCreate && (
            <button
              onClick={() => setShowCreateModal(true)}
              disabled={
                configuredModels.length === 0 || configuredExchanges.length === 0
              }
              className="px-3 md:px-4 py-2 rounded text-xs md:text-sm font-semibold transition-all hover:scale-105 disabled:opacity-50 disabled:cursor-not-allowed flex items-center gap-1 md:gap-2 whitespace-nowrap"
              style={{
                background:
                  configuredModels.length > 0 && configuredExchanges.length > 0
                    ? '#F0B90B'
                    : '#2B3139',
                color:
                  configuredModels.length > 0 && configuredExchanges.length > 0
                    ? '#000'
                    : '#848E9C',
              }}
            >
              <Plus className="w-4 h-4" />
              {t('createTrader', language)}
            </button>
          )}

          {canManageCategories && (
            <button
              onClick={() => setShowCreateCategoryModal(true)}
              className="px-3 md:px-4 py-2 rounded text-xs md:text-sm font-semibold transition-all hover:scale-105 flex items-center gap-1 md:gap-2 whitespace-nowrap"
              style={{
                background: '#10B981',
                color: '#EAECEF',
                border: '1px solid #474D57',
              }}
              title="创建分类"
            >
              <Plus className="w-3 h-3 md:w-4 md:h-4" />
              创建分类
            </button>
          )}

        </div>
      </div>

      {/* 信号源配置警告 */}
      {traders &&
        traders.some((t) => t.use_coin_pool || t.use_oi_top) &&
        !userSignalSource.coinPoolUrl &&
        !userSignalSource.oiTopUrl && (
          <div
            className="rounded-lg px-4 py-3 flex items-start gap-3 animate-slide-in"
            style={{
              background: 'rgba(246, 70, 93, 0.1)',
              border: '1px solid rgba(246, 70, 93, 0.3)',
            }}
          >
            <AlertTriangle
              size={20}
              className="flex-shrink-0 mt-0.5"
              style={{ color: '#F6465D' }}
            />
            <div className="flex-1">
              <div className="font-semibold mb-1" style={{ color: '#F6465D' }}>
                ⚠️ {t('signalSourceNotConfigured', language)}
              </div>
              <div className="text-sm" style={{ color: '#848E9C' }}>
                <p className="mb-2">
                  {t('signalSourceWarningMessage', language)}
                </p>
                <p>
                  <strong>{t('solutions', language)}</strong>
                </p>
                <ul className="list-disc list-inside space-y-1 ml-2 mt-1">
                  <li>点击"{t('signalSource', language)}"按钮配置API地址</li>
                  <li>或在交易员配置中禁用"使用币种池"和"使用OI Top"</li>
                  <li>或在交易员配置中设置自定义币种列表</li>
                </ul>
              </div>
              <button
                onClick={() => setShowSignalSourceModal(true)}
                className="mt-3 px-3 py-1.5 rounded text-sm font-semibold transition-all hover:scale-105"
                style={{
                  background: '#F0B90B',
                  color: '#000',
                }}
              >
                {t('configureSignalSourceNow', language)}
              </button>
            </div>
          </div>
        )}

      {/* Configuration Status - 只在有权限时显示 */}
      {canManageConfig && (
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-4 md:gap-6">
          {/* AI Models */}
          <div className="binance-card p-3 md:p-4">
          <h3
            className="text-base md:text-lg font-semibold mb-3 flex items-center gap-2"
            style={{ color: '#EAECEF' }}
          >
            <Brain
              className="w-4 h-4 md:w-5 md:h-5"
              style={{ color: '#60a5fa' }}
            />
            {t('aiModels', language)}
          </h3>
          <div className="space-y-2 md:space-y-3">
            {configuredModels.map((model) => {
              const inUse = isModelInUse(model.id)
              return (
                <div
                  key={model.id}
                  className={`flex items-center justify-between p-2 md:p-3 rounded transition-all ${
                    inUse
                      ? 'cursor-not-allowed'
                      : 'cursor-pointer hover:bg-gray-700'
                  }`}
                  style={{ background: '#0B0E11', border: '1px solid #2B3139' }}
                  onClick={() => handleModelClick(model.id)}
                >
                  <div className="flex items-center gap-2 md:gap-3">
                    <div className="w-7 h-7 md:w-8 md:h-8 flex items-center justify-center flex-shrink-0">
                      {getModelIcon(model.provider || model.id, {
                        width: 28,
                        height: 28,
                      }) || (
                        <div
                          className="w-7 h-7 md:w-8 md:h-8 rounded-full flex items-center justify-center text-xs md:text-sm font-bold"
                          style={{
                            background:
                              model.id === 'deepseek' ? '#60a5fa' : '#c084fc',
                            color: '#fff',
                          }}
                        >
                          {getShortName(model.name)[0]}
                        </div>
                      )}
                    </div>
                    <div className="min-w-0">
                      <div
                        className="font-semibold text-sm md:text-base truncate"
                        style={{ color: '#EAECEF' }}
                      >
                        {getShortName(model.name)}
                      </div>
                      <div className="text-xs" style={{ color: '#848E9C' }}>
                        {inUse
                          ? t('inUse', language)
                          : model.enabled
                            ? t('enabled', language)
                            : t('configured', language)}
                      </div>
                    </div>
                  </div>
                  <div
                    className={`w-2.5 h-2.5 md:w-3 md:h-3 rounded-full flex-shrink-0 ${model.enabled ? 'bg-green-400' : 'bg-gray-500'}`}
                  />
                </div>
              )
            })}
            {configuredModels.length === 0 && (
              <div
                className="text-center py-6 md:py-8"
                style={{ color: '#848E9C' }}
              >
                <Brain className="w-10 h-10 md:w-12 md:h-12 mx-auto mb-2 opacity-50" />
                <div className="text-xs md:text-sm">
                  {t('noModelsConfigured', language)}
                </div>
              </div>
            )}
          </div>
        </div>

        {/* Exchanges */}
        <div className="binance-card p-3 md:p-4">
          <h3
            className="text-base md:text-lg font-semibold mb-3 flex items-center gap-2"
            style={{ color: '#EAECEF' }}
          >
            <Landmark
              className="w-4 h-4 md:w-5 md:h-5"
              style={{ color: '#F0B90B' }}
            />
            {t('exchanges', language)}
          </h3>
          <div className="space-y-2 md:space-y-3">
            {configuredExchanges.map((exchange) => {
              const inUse = isExchangeInUse(exchange.id)
              return (
                <div
                  key={exchange.id}
                  className={`flex items-center justify-between p-2 md:p-3 rounded transition-all ${
                    inUse
                      ? 'cursor-not-allowed'
                      : 'cursor-pointer hover:bg-gray-700'
                  }`}
                  style={{ background: '#0B0E11', border: '1px solid #2B3139' }}
                  onClick={() => handleExchangeClick(exchange.id)}
                >
                  <div className="flex items-center gap-2 md:gap-3">
                    <div className="w-7 h-7 md:w-8 md:h-8 flex items-center justify-center flex-shrink-0">
                      {getExchangeIcon(exchange.id, { width: 28, height: 28 })}
                    </div>
                    <div className="min-w-0">
                      <div
                        className="font-semibold text-sm md:text-base truncate"
                        style={{ color: '#EAECEF' }}
                      >
                        {(exchange as any).label || getShortName(exchange.name)}
                      </div>
                      <div className="text-xs" style={{ color: '#848E9C' }}>
                        {exchange.type.toUpperCase()} •{' '}
                        {getShortName(exchange.name)} •{' '}
                        {inUse
                          ? t('inUse', language)
                          : exchange.enabled
                            ? t('enabled', language)
                            : t('configured', language)}
                      </div>
                    </div>
                  </div>
                  <div
                    className={`w-2.5 h-2.5 md:w-3 md:h-3 rounded-full flex-shrink-0 ${exchange.enabled ? 'bg-green-400' : 'bg-gray-500'}`}
                  />
                </div>
              )
            })}
            {configuredExchanges.length === 0 && (
              <div
                className="text-center py-6 md:py-8"
                style={{ color: '#848E9C' }}
              >
                <Landmark className="w-10 h-10 md:w-12 md:h-12 mx-auto mb-2 opacity-50" />
                <div className="text-xs md:text-sm">
                  {t('noExchangesConfigured', language)}
                </div>
              </div>
            )}
          </div>
        </div>
      </div>
      )}

      {/* Traders List */}
      <div className="binance-card p-4 md:p-6">
        <div className="flex items-center justify-between mb-4 md:mb-5">
          <h2
            className="text-lg md:text-xl font-bold flex items-center gap-2"
            style={{ color: '#EAECEF' }}
          >
            <Users
              className="w-5 h-5 md:w-6 md:h-6"
              style={{ color: '#F0B90B' }}
            />
            {t('currentTraders', language)}
          </h2>
        </div>

        {traders && traders.length > 0 ? (
          <div className="space-y-4 md:space-y-5">
            {(() => {
              const grouped = groupTradersByCategory()
              return Object.entries(grouped).map(([categoryName, categoryTraders]) => (
                <div key={categoryName} className="space-y-2 md:space-y-3">
                  {/* 分类标题 */}
                  <div className="flex items-center gap-2">
                    <BookOpen className="w-4 h-4 md:w-5 md:h-5" style={{ color: '#10B981' }} />
                    <h3 className="text-sm md:text-base font-semibold" style={{ color: '#10B981' }}>
                      {categoryName}
                    </h3>
                    <span className="text-xs px-2 py-0.5 rounded" style={{ background: 'rgba(16, 185, 129, 0.1)', color: '#10B981' }}>
                      {categoryTraders.length}
                    </span>
                  </div>
                  
                  {/* 该分类下的交易员 */}
                  <div className="space-y-2 md:space-y-3">
                    {categoryTraders.map((trader) => (
              <div
                key={trader.trader_id}
                className="flex flex-col md:flex-row md:items-center justify-between p-3 md:p-4 rounded transition-all hover:translate-y-[-1px] gap-3 md:gap-4"
                style={{ background: '#0B0E11', border: '1px solid #2B3139' }}
              >
                <div className="flex items-center gap-3 md:gap-4">
                  <div
                    className="w-10 h-10 md:w-12 md:h-12 rounded-full flex items-center justify-center flex-shrink-0"
                    style={{
                      background: trader.ai_model.includes('deepseek')
                        ? '#60a5fa'
                        : '#c084fc',
                      color: '#fff',
                    }}
                  >
                    <Bot className="w-5 h-5 md:w-6 md:h-6" />
                  </div>
                  <div className="min-w-0">
                    <div
                      className="font-bold text-base md:text-lg truncate"
                      style={{ color: '#EAECEF' }}
                    >
                      {trader.trader_name}
                    </div>
                    <div
                      className="text-xs md:text-sm truncate"
                      style={{
                        color: trader.ai_model.includes('deepseek')
                          ? '#60a5fa'
                          : '#c084fc',
                      }}
                    >
                      {getModelDisplayName(
                        trader.ai_model.split('_').pop() || trader.ai_model
                      )}{' '}
                      Model • {trader.exchange_id?.toUpperCase()} • {trader.scan_interval_minutes || 5}m
                    </div>
                  </div>
                </div>

                <div className="flex items-center gap-3 md:gap-4 flex-wrap md:flex-nowrap">
                  {/* Status */}
                  <div className="text-center">
                    <div className="text-xs mb-1" style={{ color: '#848E9C' }}>
                      {t('status', language)}
                    </div>
                    <div
                      className={`px-2 md:px-3 py-1 rounded text-xs font-bold ${
                        trader.is_running
                          ? 'bg-green-100 text-green-800'
                          : 'bg-red-100 text-red-800'
                      }`}
                      style={
                        trader.is_running
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
                      {trader.is_running
                        ? t('running', language)
                        : t('stopped', language)}
                    </div>
                  </div>

                  {/* Actions */}
                  <div className="flex gap-1.5 md:gap-2 flex-wrap md:flex-nowrap">
                    <button
                      onClick={() => onTraderSelect?.(trader.trader_id)}
                      className="px-2 md:px-3 py-1.5 md:py-2 rounded text-xs md:text-sm font-semibold transition-all hover:scale-105 flex items-center gap-1 whitespace-nowrap"
                      style={{
                        background: 'rgba(99, 102, 241, 0.1)',
                        color: '#6366F1',
                      }}
                    >
                      <BarChart3 className="w-3 h-3 md:w-4 md:h-4" />
                      {t('view', language)}
                    </button>

                    {canEdit && (
                      <button
                        onClick={() => handleEditTrader(trader.trader_id)}
                        disabled={trader.is_running}
                        className="px-2 md:px-3 py-1.5 md:py-2 rounded text-xs md:text-sm font-semibold transition-all hover:scale-105 disabled:opacity-50 disabled:cursor-not-allowed whitespace-nowrap"
                        style={{
                          background: trader.is_running
                            ? 'rgba(132, 142, 156, 0.1)'
                            : 'rgba(255, 193, 7, 0.1)',
                          color: trader.is_running ? '#848E9C' : '#FFC107',
                        }}
                      >
                        ✏️ {t('edit', language)}
                      </button>
                    )}

                    {canEdit && (
                      <button
                        onClick={(e) => {
                          e.preventDefault()
                          e.stopPropagation()
                          handleToggleTrader(
                            trader.trader_id,
                            trader.is_running || false
                          )
                        }}
                        className="px-2 md:px-3 py-1.5 md:py-2 rounded text-xs md:text-sm font-semibold transition-all hover:scale-105 whitespace-nowrap"
                        style={
                          trader.is_running
                            ? {
                                background: 'rgba(246, 70, 93, 0.1)',
                                color: '#F6465D',
                              }
                            : {
                                background: 'rgba(14, 203, 129, 0.1)',
                                color: '#0ECB81',
                              }
                        }
                      >
                        {trader.is_running
                          ? t('stop', language)
                          : t('start', language)}
                      </button>
                    )}

                    {canDelete && (
                      <button
                        onClick={() => handleDeleteTrader(trader.trader_id)}
                        className="px-2 md:px-3 py-1.5 md:py-2 rounded text-xs md:text-sm font-semibold transition-all hover:scale-105"
                        style={{
                          background: 'rgba(246, 70, 93, 0.1)',
                          color: '#F6465D',
                        }}
                      >
                        <Trash2 className="w-3 h-3 md:w-4 md:h-4" />
                      </button>
                    )}

                    {canCreateAccount && (
                      <button
                        onClick={async () => {
                          const traderId = trader.trader_id
                          // 先检查交易员是否有账号
                          try {
                            const accountResult = await api.getTraderAccount(traderId)
                            if (accountResult.account) {
                              // 有账号，显示账号信息（优先使用localStorage中的密码）
                              setTraderAccountInfo({
                                traderId,
                                email: traderAccounts[traderId]?.email || accountResult.account.email,
                                password: traderAccounts[traderId]?.password || '',
                              })
                              setShowTraderAccountInfoModal(true)
                            } else {
                              // 没有账号，显示创建账号弹窗
                              setCreatingAccountForTrader(traderId)
                          setShowCreateTraderAccountModal(true)
                            }
                          } catch (error) {
                            // 如果API调用失败，默认显示创建弹窗
                            setCreatingAccountForTrader(traderId)
                            setShowCreateTraderAccountModal(true)
                          }
                        }}
                        className="px-2 md:px-3 py-1.5 md:py-2 rounded text-xs md:text-sm font-semibold transition-all hover:scale-105 whitespace-nowrap"
                        style={{
                          background: 'rgba(99, 102, 241, 0.1)',
                          color: '#6366F1',
                        }}
                        title={traderHasAccount[trader.trader_id] || traderAccounts[trader.trader_id] ? "查看交易员账号" : "创建交易员账号"}
                      >
                        <Users className="w-3 h-3 md:w-4 md:h-4" />
                        {traderHasAccount[trader.trader_id] || traderAccounts[trader.trader_id] ? '查看' : '创建账号'}
                      </button>
                    )}

                  </div>
                </div>
              </div>
            ))}
                  </div>
                </div>
              ))
            })()}
          </div>
        ) : (
          <div
            className="text-center py-12 md:py-16"
            style={{ color: '#848E9C' }}
          >
            <Bot className="w-16 h-16 md:w-24 md:h-24 mx-auto mb-3 md:mb-4 opacity-50" />
            <div className="text-base md:text-lg font-semibold mb-2">
              {t('noTraders', language)}
            </div>
            <div className="text-xs md:text-sm mb-3 md:mb-4">
              {t('createFirstTrader', language)}
            </div>
            {(configuredModels.length === 0 ||
              configuredExchanges.length === 0) && (
              <div className="text-xs md:text-sm text-yellow-500">
                {configuredModels.length === 0 &&
                configuredExchanges.length === 0
                  ? t('configureModelsAndExchangesFirst', language)
                  : configuredModels.length === 0
                    ? t('configureModelsFirst', language)
                    : t('configureExchangesFirst', language)}
              </div>
            )}
          </div>
        )}
      </div>

      {/* Categories List Module */}
      {canManageCategories && (
        <div className="binance-card p-4 md:p-6">
          <div className="flex items-center justify-between mb-4 md:mb-5">
            <h2
              className="text-lg md:text-xl font-bold flex items-center gap-2"
              style={{ color: '#EAECEF' }}
            >
              <BookOpen
                className="w-5 h-5 md:w-6 md:h-6"
                style={{ color: '#10B981' }}
              />
              分类管理
            </h2>
            <button
              onClick={() => setShowCreateCategoryModal(true)}
              className="px-3 md:px-4 py-2 rounded text-xs md:text-sm font-semibold transition-all hover:scale-105 flex items-center gap-1 md:gap-2 whitespace-nowrap"
              style={{
                background: '#10B981',
                color: '#EAECEF',
              }}
            >
              <Plus className="w-3 h-3 md:w-4 md:h-4" />
              创建分类
            </button>
          </div>

          {categories.length > 0 ? (
            <div className="space-y-3 md:space-y-4">
              {categories.map((category) => {
                const categoryTraders = traders?.filter((t) => t.category && t.category === category.name) || []
                const isExpanded = expandedCategories.has(category.name)
                const stats = {
                  total: categoryTraders.length,
                  running: categoryTraders.filter((t) => t.is_running).length,
                }

                return (
                  <div
                    key={`category-${category.id}-${category.name}`}
                    className="rounded-lg transition-all"
                    style={{ background: '#0B0E11', border: '1px solid #2B3139' }}
                  >
                    {/* 分类头部 */}
                    <div
                      className="p-3 md:p-4 cursor-pointer hover:bg-gray-800 transition-colors"
                      onClick={() => {
                        const newExpanded = new Set(expandedCategories)
                        if (isExpanded) {
                          newExpanded.delete(category.name)
                        } else {
                          newExpanded.add(category.name)
                        }
                        setExpandedCategories(newExpanded)
                      }}
                    >
                      <div className="flex items-center justify-between">
                        <div className="flex items-center gap-3 flex-1">
                          <div
                            className="w-8 h-8 md:w-10 md:h-10 rounded-lg flex items-center justify-center flex-shrink-0"
                            style={{
                              background: 'linear-gradient(135deg, #10B981 0%, #34D399 100%)',
                            }}
                          >
                            <BookOpen className="w-4 h-4 md:w-5 md:h-5" style={{ color: '#000' }} />
                          </div>
                          <div className="flex-1 min-w-0">
                            <div className="flex items-center gap-2 mb-1">
                              <h3 className="text-base md:text-lg font-bold truncate" style={{ color: '#EAECEF' }}>
                                {category.name}
                              </h3>
                              <span
                                className="px-2 py-0.5 rounded text-xs font-semibold"
                                style={{ background: 'rgba(16, 185, 129, 0.1)', color: '#10B981' }}
                              >
                                {stats.total} 个交易员
                              </span>
                            </div>
                            {category.description && (
                              <p className="text-xs md:text-sm truncate" style={{ color: '#848E9C' }}>
                                {category.description}
                              </p>
                            )}
                            <div className="flex items-center gap-4 mt-2 text-xs" style={{ color: '#848E9C' }}>
                              <span>运行中: {stats.running}</span>
                              <span>已停止: {stats.total - stats.running}</span>
                            </div>
                          </div>
                        </div>
                        <div className="flex items-center gap-2">
                          <button
                            onClick={async (e) => {
                              e.stopPropagation()
                              setSelectedCategoryForAccount(category)

                              const hasAccount = hasCategoryAdminAccount(category.name)

                              if (hasAccount) {
                                // 有账号，显示账号信息
                                try {
                                  const accountResult = await api.getCategoryAccounts()
                                  const categoryAccounts = accountResult.filter(acc => acc.category === category.name)
                                  const adminAccount = categoryAccounts.find(acc => acc.role === 'group_leader')
                                  if (adminAccount) {
                                    // 合并本地存储的密码
                                    const accountWithPassword = {
                                      ...adminAccount,
                                      password: categoryAccountPasswords[adminAccount.id]?.password || ''
                                    }
                                    setSelectedAccountInfo(accountWithPassword)
                                    setShowCategoryAccountPage(true)
                                  }
                                } catch (error) {
                                  console.error('Failed to load account info:', error)
                                  showToast('获取账号信息失败', 'error')
                                }
                              } else {
                                // 没有账号，显示创建账号弹窗
                                setShowCreateCategoryAccountModal(true)
                              }
                            }}
                            className="px-3 py-1.5 rounded text-xs md:text-sm font-semibold transition-all hover:scale-105"
                            style={hasCategoryAdminAccount(category.name) ? {
                              background: 'rgba(16, 185, 129, 0.1)',
                              color: '#10B981',
                            } : {
                              background: 'rgba(99, 102, 241, 0.1)',
                              color: '#6366F1',
                            }}
                          >
                            {hasCategoryAdminAccount(category.name) ? (
                              <>
                                <Eye className="w-3 h-3 mr-1" />
                                查看账号
                              </>
                            ) : (
                              <>
                                <User className="w-3 h-3 mr-1" />
                                创建账号
                              </>
                            )}
                          </button>
                          <button
                            onClick={(e) => {
                              e.stopPropagation()
                              setSelectedCategoryForDetail(category)
                              setShowCategoryDetailModal(true)
                            }}
                            className="px-3 py-1.5 rounded text-xs md:text-sm font-semibold transition-all hover:scale-105"
                            style={{
                              background: 'rgba(99, 102, 241, 0.1)',
                              color: '#6366F1',
                            }}
                          >
                            管理
                          </button>
                          <div
                            className="w-5 h-5 flex items-center justify-center transition-transform"
                            style={{
                              transform: isExpanded ? 'rotate(180deg)' : 'rotate(0deg)',
                              color: '#848E9C',
                            }}
                          >
                            <ChevronDown className="w-4 h-4" />
                          </div>
                        </div>
                      </div>
                    </div>

                    {/* 展开的交易员列表 */}
                    {isExpanded && categoryTraders.length > 0 && (
                      <div className="px-3 md:px-4 pb-3 md:pb-4 pt-2 border-t" style={{ borderColor: '#2B3139' }}>
                        <div className="space-y-2">
                          {categoryTraders.map((trader) => (
                            <div
                              key={trader.trader_id}
                              className="flex items-center justify-between p-2 md:p-3 rounded"
                              style={{ background: '#181A20', border: '1px solid #2B3139' }}
                            >
                              <div className="flex items-center gap-2 md:gap-3 flex-1 min-w-0">
                                <div
                                  className="w-8 h-8 rounded-full flex items-center justify-center flex-shrink-0"
                                  style={{
                                    background: trader.ai_model.includes('deepseek')
                                      ? '#60a5fa'
                                      : '#c084fc',
                                    color: '#fff',
                                  }}
                                >
                                  <Bot className="w-4 h-4" />
                                </div>
                                <div className="min-w-0 flex-1">
                                  <div className="font-semibold text-sm truncate" style={{ color: '#EAECEF' }}>
                                    {trader.trader_name}
                                  </div>
                                  <div className="text-xs truncate" style={{ color: '#848E9C' }}>
                                    {getModelDisplayName(
                                      trader.ai_model.split('_').pop() || trader.ai_model
                                    )} • {trader.exchange_id?.toUpperCase()}
                                  </div>
                                </div>
                              </div>
                              <div className="flex items-center gap-2">
                                <div
                                  className="px-2 py-1 rounded text-xs font-semibold"
                                  style={{
                                    background: trader.is_running
                                      ? 'rgba(14, 203, 129, 0.1)'
                                      : 'rgba(132, 142, 156, 0.1)',
                                    color: trader.is_running ? '#0ECB81' : '#848E9C',
                                  }}
                                >
                                  {trader.is_running ? '运行中' : '已停止'}
                                </div>
                                <button
                                  onClick={() => onTraderSelect?.(trader.trader_id)}
                                  className="px-2 py-1 rounded text-xs font-semibold transition-all hover:scale-105"
                                  style={{
                                    background: 'rgba(99, 102, 241, 0.1)',
                                    color: '#6366F1',
                                  }}
                                >
                                  查看
                                </button>
                              </div>
                            </div>
                          ))}
                        </div>
                      </div>
                    )}

                    {isExpanded && categoryTraders.length === 0 && (
                      <div className="px-3 md:px-4 pb-3 md:pb-4 pt-2 border-t text-center py-4" style={{ borderColor: '#2B3139', color: '#848E9C' }}>
                        <div className="text-sm">该分类下暂无交易员</div>
                      </div>
                    )}
                  </div>
                )
              })}
            </div>
          ) : (
            <div className="text-center py-8" style={{ color: '#848E9C' }}>
              <BookOpen className="w-12 h-12 mx-auto mb-3 opacity-50" />
              <div className="text-sm">暂无分类，创建第一个分类来组织您的交易员</div>
            </div>
          )}
        </div>
      )}

      {/* Create Trader Modal */}
      {showCreateModal && (
        <TraderConfigModal
          isOpen={showCreateModal}
          isEditMode={false}
          availableModels={enabledModels}
          availableExchanges={enabledExchanges}
          onSave={handleCreateTrader}
          onClose={() => setShowCreateModal(false)}
        />
      )}

      {/* Edit Trader Modal */}
      {showEditModal && editingTrader && (
        <TraderConfigModal
          isOpen={showEditModal}
          isEditMode={true}
          traderData={editingTrader}
          availableModels={enabledModels}
          availableExchanges={enabledExchanges}
          onSave={handleSaveEditTrader}
          onClose={() => {
            setShowEditModal(false)
            setEditingTrader(null)
          }}
        />
      )}

      {/* Model Configuration Modal */}
      {showModelModal && (
        <ModelConfigModal
          allModels={supportedModels}
          configuredModels={allModels || []} // 用户已配置的模型（从后端获取，包含 API Key）
          editingModelId={editingModel}
          onSave={handleSaveModelConfig}
          onDelete={handleDeleteModelConfig}
          onClose={() => {
            setShowModelModal(false)
            setEditingModel(null)
          }}
          language={language}
        />
      )}

      {/* Exchange Configuration Modal */}
      {showExchangeModal && (
        <ExchangeConfigModal
          supportedExchanges={supportedExchanges}
          configuredExchanges={allExchanges || []}
          editingExchangeId={editingExchange}
          onSave={handleSaveExchangeConfig}
          onDelete={handleDeleteExchangeConfig}
          onClose={() => {
            setShowExchangeModal(false)
            setEditingExchange(null)
          }}
          language={language}
        />
      )}

      {/* Signal Source Configuration Modal */}
      {showSignalSourceModal && (
        <SignalSourceModal
          coinPoolUrl={userSignalSource.coinPoolUrl}
          oiTopUrl={userSignalSource.oiTopUrl}
          onSave={handleSaveSignalSource}
          onClose={() => setShowSignalSourceModal(false)}
          language={language}
        />
      )}

      {/* Create Trader Account Modal */}
      {showCreateTraderAccountModal && creatingAccountForTrader && (
        <CreateAccountModal
          traderId={creatingAccountForTrader}
          onSave={handleCreateTraderAccount}
          onClose={() => {
            setShowCreateTraderAccountModal(false)
            setCreatingAccountForTrader(null)
          }}
        />
      )}


      {/* Create Category Modal */}
      {showCreateCategoryModal && (
        <CreateCategoryModal
          onSave={handleCreateCategory}
          onClose={() => setShowCreateCategoryModal(false)}
          onShowToast={showToast}
        />
      )}

      {/* Category Detail Modal */}
      {showCategoryDetailModal && selectedCategoryForDetail && (
        <CategoryDetailModal
          key={`category-detail-${selectedCategoryForDetail.id}-${forceRefresh}-${traders?.length || 0}`} // 使用forceRefresh和traders长度确保更新
          category={selectedCategoryForDetail}
          traders={traders || []}
          onAddTrader={handleSetTraderCategory}
          onRemoveTrader={handleRemoveTraderFromCategory}
          onClose={() => {
            setShowCategoryDetailModal(false)
            setSelectedCategoryForDetail(null)
          }}
          onShowToast={showToast}
        />
      )}

      {/* Create Category Account Modal */}
      {showCreateCategoryAccountModal && selectedCategoryForAccount && (
        <CreateCategoryAccountModal
          category={selectedCategoryForAccount}
          onSave={handleCreateCategoryAccount}
          onClose={() => {
            setShowCreateCategoryAccountModal(false)
            setSelectedCategoryForAccount(null)
          }}
          onShowToast={showToast}
        />
      )}

      {/* Category Account List Modal */}
      {showCategoryAccountListModal && selectedCategoryForAccount && (
        <CategoryAccountListModal
          category={selectedCategoryForAccount}
          groupLeaders={getCategoryGroupLeaders(selectedCategoryForAccount.name)}
          categoryAccounts={categoryAccounts.filter(acc => acc.category === selectedCategoryForAccount.name)}
          onViewAccount={handleViewAccountInfo}
          onClose={() => {
            setShowCategoryAccountListModal(false)
            setSelectedCategoryForAccount(null)
          }}
        />
      )}

      {/* Category Account Info Modal */}
      {showCategoryAccountPage && selectedAccountInfo && (
        <CategoryAccountInfoModal
          accountInfo={selectedAccountInfo}
          onSave={(newPassword) => {
            // 更新账号信息中的密码
            setSelectedAccountInfo((prev: any) => prev ? {
              ...prev,
              password: newPassword,
            } : null)

            // 更新本地存储
            if (selectedAccountInfo?.id) {
              const newAccounts = {
                ...categoryAccountPasswords,
                [selectedAccountInfo.id]: {
                  email: selectedAccountInfo.email,
                  password: newPassword,
                }
              }
              setCategoryAccountPasswords(newAccounts)
              saveCategoryAccountsToStorage(newAccounts)
            }
          }}
          onClose={() => {
            setShowCategoryAccountPage(false)
            setSelectedAccountInfo(null)
          }}
          onShowToast={showToast}
        />
      )}

      {/* Trader Account Info Modal */}
      {showTraderAccountInfoModal && traderAccountInfo && (
        <TraderAccountInfoModal
          email={traderAccountInfo.email}
          password={traderAccountInfo.password}
          traderId={traderAccountInfo.traderId}
          onSave={(newPassword) => {
            // 更新state和localStorage中的密码
            const newAccounts = {
              ...traderAccounts,
              [traderAccountInfo.traderId]: {
                email: traderAccountInfo.email,
                password: newPassword,
              }
            }
            setTraderAccounts(newAccounts)
            saveTraderAccountsToStorage(newAccounts)
            // 更新弹窗中的密码
            setTraderAccountInfo(prev => prev ? {
              ...prev,
              password: newPassword,
            } : null)
          }}
          onClose={() => {
            setShowTraderAccountInfoModal(false)
            setTraderAccountInfo(null)
          }}
          language={language}
          onShowToast={showToast}
        />
      )}
    </div>
  )
}

// Tooltip Helper Component
function Tooltip({
  content,
  children,
}: {
  content: string
  children: React.ReactNode
}) {
  const [show, setShow] = useState(false)

  return (
    <div className="relative inline-block">
      <div
        onMouseEnter={() => setShow(true)}
        onMouseLeave={() => setShow(false)}
        onClick={() => setShow(!show)}
      >
        {children}
      </div>
      {show && (
        <div
          className="absolute z-10 px-3 py-2 text-sm rounded-lg shadow-lg w-64 left-1/2 transform -translate-x-1/2 bottom-full mb-2"
          style={{
            background: '#2B3139',
            color: '#EAECEF',
            border: '1px solid #474D57',
          }}
        >
          {content}
          <div
            className="absolute left-1/2 transform -translate-x-1/2 top-full"
            style={{
              width: 0,
              height: 0,
              borderLeft: '6px solid transparent',
              borderRight: '6px solid transparent',
              borderTop: '6px solid #2B3139',
            }}
          />
        </div>
      )}
    </div>
  )
}

// Signal Source Configuration Modal Component
function SignalSourceModal({
  coinPoolUrl,
  oiTopUrl,
  onSave,
  onClose,
  language,
}: {
  coinPoolUrl: string
  oiTopUrl: string
  onSave: (coinPoolUrl: string, oiTopUrl: string) => void
  onClose: () => void
  language: Language
}) {
  const [coinPool, setCoinPool] = useState(coinPoolUrl || '')
  const [oiTop, setOiTop] = useState(oiTopUrl || '')

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    onSave(coinPool.trim(), oiTop.trim())
  }

  return (
    <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50 p-4">
      <div
        className="bg-gray-800 rounded-lg p-6 w-full max-w-lg relative"
        style={{ background: '#1E2329' }}
      >
        <h3 className="text-xl font-bold mb-4" style={{ color: '#EAECEF' }}>
          {t('signalSourceConfig', language)}
        </h3>

        <form onSubmit={handleSubmit} className="space-y-4">
          <div>
            <label
              className="block text-sm font-semibold mb-2"
              style={{ color: '#EAECEF' }}
            >
              COIN POOL URL
            </label>
            <input
              type="url"
              value={coinPool}
              onChange={(e) => setCoinPool(e.target.value)}
              placeholder="https://api.example.com/coinpool"
              className="w-full px-3 py-2 rounded"
              style={{
                background: '#0B0E11',
                border: '1px solid #2B3139',
                color: '#EAECEF',
              }}
            />
            <div className="text-xs mt-1" style={{ color: '#848E9C' }}>
              {t('coinPoolDescription', language)}
            </div>
          </div>

          <div>
            <label
              className="block text-sm font-semibold mb-2"
              style={{ color: '#EAECEF' }}
            >
              OI TOP URL
            </label>
            <input
              type="url"
              value={oiTop}
              onChange={(e) => setOiTop(e.target.value)}
              placeholder="https://api.example.com/oitop"
              className="w-full px-3 py-2 rounded"
              style={{
                background: '#0B0E11',
                border: '1px solid #2B3139',
                color: '#EAECEF',
              }}
            />
            <div className="text-xs mt-1" style={{ color: '#848E9C' }}>
              {t('oiTopDescription', language)}
            </div>
          </div>

          <div
            className="p-4 rounded"
            style={{
              background: 'rgba(240, 185, 11, 0.1)',
              border: '1px solid rgba(240, 185, 11, 0.2)',
            }}
          >
            <div
              className="text-sm font-semibold mb-2"
              style={{ color: '#F0B90B' }}
            >
              ℹ️ {t('information', language)}
            </div>
            <div className="text-xs space-y-1" style={{ color: '#848E9C' }}>
              <div>{t('signalSourceInfo1', language)}</div>
              <div>{t('signalSourceInfo2', language)}</div>
              <div>{t('signalSourceInfo3', language)}</div>
            </div>
          </div>

          <div className="flex gap-3 mt-6">
            <button
              type="button"
              onClick={onClose}
              className="flex-1 px-4 py-2 rounded text-sm font-semibold"
              style={{ background: '#2B3139', color: '#848E9C' }}
            >
              {t('cancel', language)}
            </button>
            <button
              type="submit"
              className="flex-1 px-4 py-2 rounded text-sm font-semibold"
              style={{ background: '#F0B90B', color: '#000' }}
            >
              {t('save', language)}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}

// Model Configuration Modal Component
function ModelConfigModal({
  allModels, // 注意：实际传入的是 supportedModels（系统支持的模型模板）
  configuredModels, // 注意：实际传入的是 allModels（用户已配置的模型，包含 API Key）
  editingModelId,
  onSave,
  onDelete,
  onClose,
  language,
}: {
  allModels: AIModel[] // 系统支持的模型模板
  configuredModels: AIModel[] // 用户已配置的模型（包含完整数据，如 API Key）
  editingModelId: string | null
  onSave: (
    modelId: string,
    apiKey: string,
    baseUrl?: string,
    modelName?: string
  ) => void
  onDelete: (modelId: string) => void
  onClose: () => void
  language: Language
}) {
  const [selectedModelId, setSelectedModelId] = useState(editingModelId || '')
  const [apiKey, setApiKey] = useState('')
  const [baseUrl, setBaseUrl] = useState('')
  const [modelName, setModelName] = useState('')

  // 🔑 关键修复：
  // - 编辑模式：从 configuredModels（用户已配置的模型）中查找，包含完整数据（API Key）
  // - 添加模式：从 allModels（系统支持的模型模板）中查找
  const isEditMode = editingModelId !== null
  
  // 🔍 调试：打印传入的数据
  console.log('🔍 ModelConfigModal 接收的数据:', {
    editingModelId,
    isEditMode,
    configuredModelsCount: configuredModels?.length || 0,
    configuredModelIds: configuredModels?.map(m => ({ 
      id: m.id, 
      apiKey: m.apiKey ? `${m.apiKey.substring(0, 20)}...` : '(空)',
      apiKeyLength: m.apiKey?.length || 0,
      customApiUrl: m.customApiUrl || '(空)',
      customModelName: m.customModelName || '(空)',
    })) || [],
    allModelsCount: allModels?.length || 0,
  })
  // 🔍 详细打印 configuredModels 的完整数据
  if (configuredModels && configuredModels.length > 0) {
    console.log('🔍 configuredModels 完整数据:', configuredModels)
  }
  
  const selectedModel = isEditMode
    ? configuredModels?.find((m) => m.id === editingModelId) // 编辑模式：从用户已配置的模型中查找（包含 API Key）
    : allModels?.find((m) => m.id === selectedModelId) // 添加模式：从系统支持的模型模板中查找

  // 如果是编辑现有模型，初始化API Key、Base URL和Model Name
  useEffect(() => {
    console.log('🔄 useEffect 触发:', {
      editingModelId,
      configuredModelsCount: configuredModels?.length || 0,
      configuredModelIds: configuredModels?.map(m => m.id) || [],
    })
    
    if (editingModelId) {
      // 🔑 编辑模式：从 configuredModels（用户已配置的模型）中查找，包含完整数据（API Key）
      const modelToEdit = configuredModels?.find((m) => m.id === editingModelId)
      console.log('🔍 查找结果:', {
        editingModelId,
        found: !!modelToEdit,
        modelData: modelToEdit ? {
          id: modelToEdit.id,
          name: modelToEdit.name,
          provider: modelToEdit.provider,
          apiKey: modelToEdit.apiKey ? `${modelToEdit.apiKey.substring(0, 20)}...` : '(空)',
          apiKeyLength: modelToEdit.apiKey?.length || 0,
          customApiUrl: modelToEdit.customApiUrl || '(空)',
          customModelName: modelToEdit.customModelName || '(空)',
        } : null,
      })
      
      if (modelToEdit) {
        // 🔑 编辑模式：显示所有原有值（包括API Key）
        console.log('✅ 设置表单值:', {
          apiKey: modelToEdit.apiKey ? `${modelToEdit.apiKey.substring(0, 20)}...` : '(空)',
          baseUrl: modelToEdit.customApiUrl || '(空)',
          modelName: modelToEdit.customModelName || '(空)',
        })
        setApiKey(modelToEdit.apiKey || '')
        setBaseUrl(modelToEdit.customApiUrl || '')
        setModelName(modelToEdit.customModelName || '')
        // 确保 selectedModelId 也设置为 editingModelId
        if (selectedModelId !== editingModelId) {
          setSelectedModelId(editingModelId)
        }
      } else {
        console.warn('⚠️ 未找到要编辑的模型:', {
          editingModelId,
          configuredModelsCount: configuredModels?.length || 0,
          configuredModelIds: configuredModels?.map(m => m.id) || [],
          allConfiguredModels: configuredModels,
        })
      }
    } else {
      // 添加模式下，清空表单
      console.log('➕ 添加模式，清空表单')
      setApiKey('')
      setBaseUrl('')
      setModelName('')
    }
  }, [editingModelId, configuredModels, selectedModelId])

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    if (!selectedModelId || !apiKey.trim()) return

    onSave(
      selectedModelId,
      apiKey.trim(),
      baseUrl.trim() || undefined,
      modelName.trim() || undefined
    )
  }

  // 可选择的模型列表（所有支持的模型）
  const availableModels = allModels || []

  return (
    <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50 p-4">
      <div
        className="bg-gray-800 rounded-lg p-6 w-full max-w-lg relative"
        style={{ background: '#1E2329' }}
      >
        <div className="flex items-center justify-between mb-4">
          <h3 className="text-xl font-bold" style={{ color: '#EAECEF' }}>
            {editingModelId
              ? t('editAIModel', language)
              : t('addAIModel', language)}
          </h3>
          {editingModelId && (
            <button
              type="button"
              onClick={() => onDelete(editingModelId)}
              className="p-2 rounded hover:bg-red-100 transition-colors"
              style={{ background: 'rgba(246, 70, 93, 0.1)', color: '#F6465D' }}
              title={t('delete', language)}
            >
              <Trash2 className="w-4 h-4" />
            </button>
          )}
        </div>

        <form onSubmit={handleSubmit} className="space-y-4">
          {!editingModelId && (
            <div>
              <label
                className="block text-sm font-semibold mb-2"
                style={{ color: '#EAECEF' }}
              >
                {t('selectModel', language)}
              </label>
              <select
                value={selectedModelId}
                onChange={(e) => setSelectedModelId(e.target.value)}
                className="w-full px-3 py-2 rounded"
                style={{
                  background: '#0B0E11',
                  border: '1px solid #2B3139',
                  color: '#EAECEF',
                }}
                required
              >
                <option value="">{t('pleaseSelectModel', language)}</option>
                {availableModels.map((model) => (
                  <option key={model.id} value={model.id}>
                    {getShortName(model.name)} ({model.provider})
                  </option>
                ))}
              </select>
            </div>
          )}

          {selectedModel && (
            <div
              className="p-4 rounded"
              style={{ background: '#0B0E11', border: '1px solid #2B3139' }}
            >
              <div className="flex items-center gap-3 mb-3">
                <div className="w-8 h-8 flex items-center justify-center">
                  {getModelIcon(selectedModel.provider || selectedModel.id, {
                    width: 32,
                    height: 32,
                  }) || (
                    <div
                      className="w-8 h-8 rounded-full flex items-center justify-center text-sm font-bold"
                      style={{
                        background:
                          selectedModel.id === 'deepseek'
                            ? '#60a5fa'
                            : '#c084fc',
                        color: '#fff',
                      }}
                    >
                      {selectedModel.name[0]}
                    </div>
                  )}
                </div>
                <div>
                  <div className="font-semibold" style={{ color: '#EAECEF' }}>
                    {getShortName(selectedModel.name)}
                  </div>
                  <div className="text-xs" style={{ color: '#848E9C' }}>
                    {selectedModel.provider} • {selectedModel.id}
                  </div>
                </div>
              </div>
            </div>
          )}

          {selectedModel && (
            <>
              <div>
                <label
                  className="block text-sm font-semibold mb-2"
                  style={{ color: '#EAECEF' }}
                >
                  API Key
                  {editingModelId && (
                    <span className="text-xs ml-2" style={{ color: '#848E9C' }}>
                      (长度: {apiKey.length})
                    </span>
                  )}
                </label>
                <input
                  type="text"
                  value={apiKey}
                  onChange={(e) => {
                    console.log('📝 API Key 输入变化:', e.target.value.substring(0, 20) + '...')
                    setApiKey(e.target.value)
                  }}
                  placeholder={t('enterAPIKey', language)}
                  className="w-full px-3 py-2 rounded"
                  style={{
                    background: '#0B0E11',
                    border: '1px solid #2B3139',
                    color: '#EAECEF',
                  }}
                  required
                />
                {editingModelId && apiKey && (
                  <div className="text-xs mt-1" style={{ color: '#848E9C' }}>
                    已加载: {apiKey.substring(0, 30)}...
                  </div>
                )}
              </div>

              <div>
                <label
                  className="block text-sm font-semibold mb-2"
                  style={{ color: '#EAECEF' }}
                >
                  {t('customBaseURL', language)}
                </label>
                <input
                  type="url"
                  value={baseUrl}
                  onChange={(e) => setBaseUrl(e.target.value)}
                  placeholder={t('customBaseURLPlaceholder', language)}
                  className="w-full px-3 py-2 rounded"
                  style={{
                    background: '#0B0E11',
                    border: '1px solid #2B3139',
                    color: '#EAECEF',
                  }}
                />
                <div className="text-xs mt-1" style={{ color: '#848E9C' }}>
                  {t('leaveBlankForDefault', language)}
                </div>
              </div>

              <div>
                <label
                  className="block text-sm font-semibold mb-2"
                  style={{ color: '#EAECEF' }}
                >
                  Model Name (可选)
                </label>
                <input
                  type="text"
                  value={modelName}
                  onChange={(e) => setModelName(e.target.value)}
                  placeholder="例如: deepseek-chat, qwen3-max, gpt-5"
                  className="w-full px-3 py-2 rounded"
                  style={{
                    background: '#0B0E11',
                    border: '1px solid #2B3139',
                    color: '#EAECEF',
                  }}
                />
                <div className="text-xs mt-1" style={{ color: '#848E9C' }}>
                  留空使用默认模型名称
                </div>
              </div>

              <div
                className="p-4 rounded"
                style={{
                  background: 'rgba(240, 185, 11, 0.1)',
                  border: '1px solid rgba(240, 185, 11, 0.2)',
                }}
              >
                <div
                  className="text-sm font-semibold mb-2"
                  style={{ color: '#F0B90B' }}
                >
                  ℹ️ {t('information', language)}
                </div>
                <div className="text-xs space-y-1" style={{ color: '#848E9C' }}>
                  <div>{t('modelConfigInfo1', language)}</div>
                  <div>{t('modelConfigInfo2', language)}</div>
                  <div>{t('modelConfigInfo3', language)}</div>
                </div>
              </div>
            </>
          )}

          <div className="flex gap-3 mt-6">
            <button
              type="button"
              onClick={onClose}
              className="flex-1 px-4 py-2 rounded text-sm font-semibold"
              style={{ background: '#2B3139', color: '#848E9C' }}
            >
              {t('cancel', language)}
            </button>
            <button
              type="submit"
              disabled={!selectedModel || !apiKey.trim()}
              className="flex-1 px-4 py-2 rounded text-sm font-semibold disabled:opacity-50"
              style={{ background: '#F0B90B', color: '#000' }}
            >
              {t('saveConfig', language)}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}

// Exchange Configuration Modal Component
function ExchangeConfigModal({
  supportedExchanges,
  configuredExchanges,
  editingExchangeId,
  onSave,
  onDelete,
  onClose,
  language,
}: {
  supportedExchanges: Exchange[] // 系统支持的交易所列表（用于添加时选择类型）
  configuredExchanges: Exchange[] // 用户已配置的交易所列表（用于编辑时加载数据）
  editingExchangeId: string | null
  onSave: (
    exchangeId: string,
    apiKey: string,
    secretKey?: string,
    testnet?: boolean,
    hyperliquidWalletAddr?: string,
    asterUser?: string,
    asterSigner?: string,
    asterPrivateKey?: string,
    passphrase?: string,
    label?: string
  ) => Promise<void>
  onDelete: (exchangeId: string) => void
  onClose: () => void
  language: Language
}) {
  const [selectedExchangeId, setSelectedExchangeId] = useState(
    editingExchangeId || ''
  )
  const [apiKey, setApiKey] = useState('')
  const [secretKey, setSecretKey] = useState('')
  const [passphrase, setPassphrase] = useState('')
  const [testnet, setTestnet] = useState(false)
  const [showGuide, setShowGuide] = useState(false)
  const [serverIP, setServerIP] = useState<{
    public_ip: string
    message: string
  } | null>(null)
  const [loadingIP, setLoadingIP] = useState(false)
  const [copiedIP, setCopiedIP] = useState(false)

  // 币安配置指南展开状态
  const [showBinanceGuide, setShowBinanceGuide] = useState(false)

  // Aster 特定字段
  const [asterUser, setAsterUser] = useState('')
  const [asterSigner, setAsterSigner] = useState('')
  const [asterPrivateKey, setAsterPrivateKey] = useState('')

  // Hyperliquid 特定字段
  const [hyperliquidWalletAddr, setHyperliquidWalletAddr] = useState('')

  // 账号标签（仅在创建时可编辑，编辑模式保持原有标签）
  const [label, setLabel] = useState('')

  // 安全输入状态
  const [secureInputTarget, setSecureInputTarget] = useState<
    null | 'hyperliquid' | 'aster'
  >(null)

  // 🔑 关键修复：编辑模式下从 configuredExchanges 查找，添加模式下从 supportedExchanges 查找
  const isEditMode = editingExchangeId !== null
  const selectedExchange = isEditMode
    ? configuredExchanges?.find((e) => e.id === editingExchangeId) // 编辑模式：从用户配置中查找
    : supportedExchanges?.find((e) => e.id === selectedExchangeId) // 添加模式：从系统支持中查找
  
  // 获取 provider（用于判断交易所类型）
  const exchangeProvider = isEditMode && selectedExchange
    ? (selectedExchange as any).provider || selectedExchange.id.split('_')[0] // 编辑模式：从配置中获取 provider
    : selectedExchange?.id // 添加模式：使用选择的交易所ID作为provider

  // 如果是编辑现有交易所，初始化表单数据
  useEffect(() => {
    if (editingExchangeId && selectedExchange) {
      // 🔑 编辑模式：显示所有原有值（包括敏感信息）
      setApiKey(selectedExchange.apiKey || '')
      setSecretKey(selectedExchange.secretKey || '')
      setPassphrase(selectedExchange.passphrase || '') // 显示原有 passphrase
      setTestnet(selectedExchange.testnet || false)

      // Aster 字段
      setAsterUser(selectedExchange.asterUser || '')
      setAsterSigner(selectedExchange.asterSigner || '')
      setAsterPrivateKey(selectedExchange.asterPrivateKey || '') // 显示原有 private key

      // Hyperliquid 字段
      setHyperliquidWalletAddr(selectedExchange.hyperliquidWalletAddr || '')
      // 编辑模式下显示当前标签
      setLabel((selectedExchange as any).label || selectedExchange.name || '')
      
      // 编辑模式下，设置 selectedExchangeId 为 provider（用于显示交易所类型）
      const provider = (selectedExchange as any).provider || selectedExchange.id.split('_')[0]
      setSelectedExchangeId(provider)
    } else if (!editingExchangeId) {
      // 添加模式下，清空表单
      setApiKey('')
      setSecretKey('')
      setPassphrase('')
      setTestnet(false)
      setAsterUser('')
      setAsterSigner('')
      setAsterPrivateKey('')
      setHyperliquidWalletAddr('')
      setLabel('')
    }
  }, [editingExchangeId, selectedExchange, configuredExchanges])

  // 加载服务器IP（当选择binance时）
  useEffect(() => {
    if (selectedExchangeId === 'binance' && !serverIP) {
      setLoadingIP(true)
      api
        .getServerIP()
        .then((data) => {
          setServerIP(data)
        })
        .catch((err) => {
          console.error('Failed to load server IP:', err)
        })
        .finally(() => {
          setLoadingIP(false)
        })
    }
  }, [selectedExchangeId])

  const handleCopyIP = (ip: string) => {
    navigator.clipboard.writeText(ip).then(() => {
      setCopiedIP(true)
      setTimeout(() => setCopiedIP(false), 2000)
    })
  }

  // 安全输入处理函数
  const secureInputContextLabel =
    secureInputTarget === 'aster'
      ? t('asterExchangeName', language)
      : secureInputTarget === 'hyperliquid'
        ? t('hyperliquidExchangeName', language)
        : undefined

  const handleSecureInputCancel = () => {
    setSecureInputTarget(null)
  }

  const handleSecureInputComplete = ({
    value,
    obfuscationLog,
  }: TwoStageKeyModalResult) => {
    const trimmed = value.trim()
    if (secureInputTarget === 'hyperliquid') {
      setApiKey(trimmed)
    }
    if (secureInputTarget === 'aster') {
      setAsterPrivateKey(trimmed)
    }
    console.log('Secure input obfuscation log:', obfuscationLog)
    setSecureInputTarget(null)
  }

  // 掩盖敏感数据显示 (unused, kept for potential future use)
  // const maskSecret = (secret: string) => {
  //   if (!secret || secret.length === 0) return ''
  //   if (secret.length <= 8) return '*'.repeat(secret.length)
  //   return (
  //     secret.slice(0, 4) +
  //     '*'.repeat(Math.max(secret.length - 8, 4)) +
  //     secret.slice(-4)
  //   )
  // }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    
    // 🔑 关键修复：编辑模式使用 editingExchangeId，添加模式使用 selectedExchangeId
    const finalExchangeId = isEditMode ? (editingExchangeId || '') : selectedExchangeId
    if (!finalExchangeId) return

    // 根据交易所类型验证不同字段（使用 exchangeProvider 判断）
    if (exchangeProvider === 'binance') {
      if (!apiKey.trim() || !secretKey.trim()) return
      await onSave(
        finalExchangeId,
        apiKey.trim(),
        secretKey.trim(),
        testnet,
        undefined,
        undefined,
        undefined,
        undefined,
        undefined,
        label.trim() || undefined
      )
    } else if (exchangeProvider === 'hyperliquid') {
      if (!apiKey.trim() || !hyperliquidWalletAddr.trim()) return // 验证私钥和钱包地址
      await onSave(
        finalExchangeId,
        apiKey.trim(),
        '',
        testnet,
        hyperliquidWalletAddr.trim(),
        undefined,
        undefined,
        undefined,
        undefined,
        label.trim() || undefined
      )
    } else if (exchangeProvider === 'aster') {
      if (!asterUser.trim() || !asterSigner.trim() || !asterPrivateKey.trim())
        return
      await onSave(
        finalExchangeId,
        '',
        '',
        testnet,
        undefined,
        asterUser.trim(),
        asterSigner.trim(),
        asterPrivateKey.trim(),
        undefined,
        label.trim() || undefined
      )
    } else if (
      exchangeProvider === 'okx' ||
      exchangeProvider === 'bitget'
    ) {
      if (!apiKey.trim() || !secretKey.trim() || !passphrase.trim()) return
      await onSave(
        finalExchangeId,
        apiKey.trim(),
        secretKey.trim(),
        testnet,
        undefined,
        undefined,
        undefined,
        undefined,
        passphrase.trim(),
        label.trim() || undefined
      )
    } else {
      // 默认情况（其他CEX交易所）
      if (!apiKey.trim() || !secretKey.trim()) return
      await onSave(
        finalExchangeId,
        apiKey.trim(),
        secretKey.trim(),
        testnet,
        undefined,
        undefined,
        undefined,
        undefined,
        undefined,
        label.trim() || undefined
      )
    }
  }

  // 可选择的交易所列表（添加模式用 supportedExchanges，编辑模式显示当前配置）
  const availableExchanges = isEditMode ? (selectedExchange ? [selectedExchange] : []) : (supportedExchanges || [])

  return (
    <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50 p-4">
      <div
        className="bg-gray-800 rounded-lg p-6 w-full max-w-lg relative"
        style={{ background: '#1E2329' }}
      >
        <div className="flex items-center justify-between mb-4">
          <h3 className="text-xl font-bold" style={{ color: '#EAECEF' }}>
            {editingExchangeId
              ? t('editExchange', language)
              : t('addExchange', language)}
          </h3>
          <div className="flex items-center gap-2">
            {exchangeProvider === 'binance' && (
              <button
                type="button"
                onClick={() => setShowGuide(true)}
                className="px-3 py-2 rounded text-sm font-semibold transition-all hover:scale-105 flex items-center gap-2"
                style={{
                  background: 'rgba(240, 185, 11, 0.1)',
                  color: '#F0B90B',
                }}
              >
                <BookOpen className="w-4 h-4" />
                {t('viewGuide', language)}
              </button>
            )}
            {editingExchangeId && (
              <button
                type="button"
                onClick={() => onDelete(editingExchangeId)}
                className="p-2 rounded hover:bg-red-100 transition-colors"
                style={{
                  background: 'rgba(246, 70, 93, 0.1)',
                  color: '#F6465D',
                }}
                title={t('delete', language)}
              >
                <Trash2 className="w-4 h-4" />
              </button>
            )}
          </div>
        </div>

        <form onSubmit={handleSubmit} className="space-y-4">
          {/* 无论添加还是编辑模式，都允许修改标签 */}
          {selectedExchange && (
            <div>
              <label
                className="block text-sm font-semibold mb-1"
                style={{ color: '#EAECEF' }}
              >
                账号标签（可选）
              </label>
              <input
                type="text"
                value={label}
                onChange={(e) => setLabel(e.target.value)}
                placeholder={`例如：${getShortName(selectedExchange.name)} 主账号`}
                className="w-full px-3 py-2 rounded text-sm"
                style={{
                  background: '#0B0E11',
                  border: '1px solid #2B3139',
                  color: '#EAECEF',
                }}
              />
              <p className="mt-1 text-xs" style={{ color: '#848E9C' }}>
                用来区分同一交易所的多个账号，例如「Bitget 主账号」「Bitget 副账号」。
              </p>
            </div>
          )}

          {!editingExchangeId && (
            <>
              <div>
                <label
                  className="block text-sm font-semibold mb-2"
                  style={{ color: '#EAECEF' }}
                >
                  {t('selectExchange', language)}
                </label>
                <select
                  value={selectedExchangeId}
                  onChange={(e) => setSelectedExchangeId(e.target.value)}
                  className="w-full px-3 py-2 rounded"
                  style={{
                    background: '#0B0E11',
                    border: '1px solid #2B3139',
                    color: '#EAECEF',
                  }}
                  required
                >
                  <option value="">{t('pleaseSelectExchange', language)}</option>
                  {availableExchanges.map((exchange) => (
                    <option key={exchange.id} value={exchange.id}>
                      {getShortName(exchange.name)} ({exchange.type.toUpperCase()}
                      )
                    </option>
                  ))}
                </select>
              </div>
            </>
          )}

          {selectedExchange && (
            <div
              className="p-4 rounded"
              style={{ background: '#0B0E11', border: '1px solid #2B3139' }}
            >
              <div className="flex items-center gap-3 mb-3">
                <div className="w-8 h-8 flex items-center justify-center">
                  {getExchangeIcon(exchangeProvider || selectedExchange.id, {
                    width: 32,
                    height: 32,
                  })}
                </div>
                <div>
                  <div className="font-semibold" style={{ color: '#EAECEF' }}>
                    {label || (isEditMode ? ((selectedExchange as any).label || selectedExchange.name) : getShortName(selectedExchange.name))}
                  </div>
                  <div className="text-xs" style={{ color: '#848E9C' }}>
                    {selectedExchange.type.toUpperCase()} •{' '}
                    {isEditMode ? exchangeProvider : selectedExchange.id}
                  </div>
                </div>
              </div>
            </div>
          )}

          {selectedExchange && (
            <>
              {/* Binance 和其他 CEX 交易所的字段 */}
              {(exchangeProvider === 'binance' ||
                exchangeProvider === 'bitget' ||
                selectedExchange.type === 'cex') &&
                exchangeProvider !== 'hyperliquid' &&
                exchangeProvider !== 'aster' && (
                  <>
                    {/* 币安用户配置提示 (D1 方案) */}
                    {exchangeProvider === 'binance' && (
                      <div
                        className="mb-4 p-3 rounded cursor-pointer transition-colors"
                        style={{
                          background: '#1a3a52',
                          border: '1px solid #2b5278',
                        }}
                        onClick={() => setShowBinanceGuide(!showBinanceGuide)}
                      >
                        <div className="flex items-center justify-between">
                          <div className="flex items-center gap-2">
                            <span style={{ color: '#58a6ff' }}>ℹ️</span>
                            <span
                              className="text-sm font-medium"
                              style={{ color: '#EAECEF' }}
                            >
                              <strong>币安用户必读：</strong>
                              使用「现货与合约交易」API，不要用「统一账户 API」
                            </span>
                          </div>
                          <span style={{ color: '#8b949e' }}>
                            {showBinanceGuide ? '▲' : '▼'}
                          </span>
                        </div>

                        {/* 展开的详细说明 */}
                        {showBinanceGuide && (
                          <div
                            className="mt-3 pt-3"
                            style={{
                              borderTop: '1px solid #2b5278',
                              fontSize: '0.875rem',
                              color: '#c9d1d9',
                            }}
                            onClick={(e) => e.stopPropagation()}
                          >
                            <p className="mb-2" style={{ color: '#8b949e' }}>
                              <strong>原因：</strong>统一账户 API
                              权限结构不同，会导致订单提交失败
                            </p>

                            <p
                              className="font-semibold mb-1"
                              style={{ color: '#EAECEF' }}
                            >
                              正确配置步骤：
                            </p>
                            <ol
                              className="list-decimal list-inside space-y-1 mb-3"
                              style={{ paddingLeft: '0.5rem' }}
                            >
                              <li>
                                登录币安 → 个人中心 → <strong>API 管理</strong>
                              </li>
                              <li>
                                创建 API → 选择「
                                <strong>系统生成的 API 密钥</strong>」
                              </li>
                              <li>
                                勾选「<strong>现货与合约交易</strong>」（
                                <span style={{ color: '#f85149' }}>
                                  不选统一账户
                                </span>
                                ）
                              </li>
                              <li>
                                IP 限制选「<strong>无限制</strong>」或添加服务器
                                IP
                              </li>
                            </ol>

                            <p
                              className="mb-2 p-2 rounded"
                              style={{
                                background: '#3d2a00',
                                border: '1px solid #9e6a03',
                              }}
                            >
                              💡 <strong>多资产模式用户注意：</strong>
                              如果您开启了多资产模式，将强制使用全仓模式。建议关闭多资产模式以支持逐仓交易。
                            </p>

                            <a
                              href="https://www.binance.com/zh-CN/support/faq/how-to-create-api-keys-on-binance-360002502072"
                              target="_blank"
                              rel="noopener noreferrer"
                              className="inline-block text-sm hover:underline"
                              style={{ color: '#58a6ff' }}
                            >
                              📖 查看币安官方教程 ↗
                            </a>
                          </div>
                        )}
                      </div>
                    )}

                    <div>
                      <label
                        className="block text-sm font-semibold mb-2"
                        style={{ color: '#EAECEF' }}
                      >
                        {t('apiKey', language)}
                      </label>
                      <input
                        type="text"
                        value={apiKey}
                        onChange={(e) => setApiKey(e.target.value)}
                        placeholder={t('enterAPIKey', language)}
                        className="w-full px-3 py-2 rounded"
                        style={{
                          background: '#0B0E11',
                          border: '1px solid #2B3139',
                          color: '#EAECEF',
                        }}
                        required
                      />
                    </div>

                    <div>
                      <label
                        className="block text-sm font-semibold mb-2"
                        style={{ color: '#EAECEF' }}
                      >
                        {t('secretKey', language)}
                      </label>
                      <input
                        type="text"
                        value={secretKey}
                        onChange={(e) => setSecretKey(e.target.value)}
                        placeholder={t('enterSecretKey', language)}
                        className="w-full px-3 py-2 rounded"
                        style={{
                          background: '#0B0E11',
                          border: '1px solid #2B3139',
                          color: '#EAECEF',
                        }}
                        required
                      />
                    </div>

                    {(exchangeProvider === 'okx' ||
                      exchangeProvider === 'bitget') && (
                      <div>
                        <label
                          className="block text-sm font-semibold mb-2"
                          style={{ color: '#EAECEF' }}
                        >
                          {t('passphrase', language)}
                        </label>
                        <input
                          type="text"
                          value={passphrase}
                          onChange={(e) => setPassphrase(e.target.value)}
                          placeholder={t('enterPassphrase', language)}
                          className="w-full px-3 py-2 rounded"
                          style={{
                            background: '#0B0E11',
                            border: '1px solid #2B3139',
                            color: '#EAECEF',
                          }}
                          required
                        />
                      </div>
                    )}

                    {/* Binance 白名单IP提示 */}
                    {selectedExchange.id === 'binance' && (
                      <div
                        className="p-4 rounded"
                        style={{
                          background: 'rgba(240, 185, 11, 0.1)',
                          border: '1px solid rgba(240, 185, 11, 0.2)',
                        }}
                      >
                        <div
                          className="text-sm font-semibold mb-2"
                          style={{ color: '#F0B90B' }}
                        >
                          {t('whitelistIP', language)}
                        </div>
                        <div
                          className="text-xs mb-3"
                          style={{ color: '#848E9C' }}
                        >
                          {t('whitelistIPDesc', language)}
                        </div>

                        {loadingIP ? (
                          <div className="text-xs" style={{ color: '#848E9C' }}>
                            {t('loadingServerIP', language)}
                          </div>
                        ) : serverIP && serverIP.public_ip ? (
                          <div
                            className="flex items-center gap-2 p-2 rounded"
                            style={{ background: '#0B0E11' }}
                          >
                            <code
                              className="flex-1 text-sm font-mono"
                              style={{ color: '#F0B90B' }}
                            >
                              {serverIP.public_ip}
                            </code>
                            <button
                              type="button"
                              onClick={() => handleCopyIP(serverIP.public_ip)}
                              className="px-3 py-1 rounded text-xs font-semibold transition-all hover:scale-105"
                              style={{
                                background: 'rgba(240, 185, 11, 0.2)',
                                color: '#F0B90B',
                              }}
                            >
                              {copiedIP
                                ? t('ipCopied', language)
                                : t('copyIP', language)}
                            </button>
                          </div>
                        ) : null}
                      </div>
                    )}
                  </>
                )}

              {/* Hyperliquid 交易所的字段 */}
              {selectedExchange.id === 'hyperliquid' && (
                <>
                  {/* 安全提示 banner */}
                  <div
                    className="p-3 rounded mb-4"
                    style={{
                      background: 'rgba(240, 185, 11, 0.1)',
                      border: '1px solid rgba(240, 185, 11, 0.3)',
                    }}
                  >
                    <div className="flex items-start gap-2">
                      <span style={{ color: '#F0B90B', fontSize: '16px' }}>
                        🔐
                      </span>
                      <div className="flex-1">
                        <div
                          className="text-sm font-semibold mb-1"
                          style={{ color: '#F0B90B' }}
                        >
                          {t('hyperliquidAgentWalletTitle', language)}
                        </div>
                        <div
                          className="text-xs"
                          style={{ color: '#848E9C', lineHeight: '1.5' }}
                        >
                          {t('hyperliquidAgentWalletDesc', language)}
                        </div>
                      </div>
                    </div>
                  </div>

                  {/* Agent Private Key 字段 */}
                  <div>
                    <label
                      className="block text-sm font-semibold mb-2"
                      style={{ color: '#EAECEF' }}
                    >
                      {t('hyperliquidAgentPrivateKey', language)}
                    </label>
                    <input
                      type="text"
                      value={apiKey}
                      onChange={(e) => setApiKey(e.target.value)}
                      placeholder={t(
                        'enterHyperliquidAgentPrivateKey',
                        language
                      )}
                      className="w-full px-3 py-2 rounded"
                      style={{
                        background: '#0B0E11',
                        border: '1px solid #2B3139',
                        color: '#EAECEF',
                      }}
                      required
                    />
                    <div className="text-xs mt-1" style={{ color: '#848E9C' }}>
                      {t('hyperliquidAgentPrivateKeyDesc', language)}
                    </div>
                  </div>

                  {/* Main Wallet Address 字段 */}
                  <div>
                    <label
                      className="block text-sm font-semibold mb-2"
                      style={{ color: '#EAECEF' }}
                    >
                      {t('hyperliquidMainWalletAddress', language)}
                    </label>
                    <input
                      type="text"
                      value={hyperliquidWalletAddr}
                      onChange={(e) => setHyperliquidWalletAddr(e.target.value)}
                      placeholder={t(
                        'enterHyperliquidMainWalletAddress',
                        language
                      )}
                      className="w-full px-3 py-2 rounded"
                      style={{
                        background: '#0B0E11',
                        border: '1px solid #2B3139',
                        color: '#EAECEF',
                      }}
                      required
                    />
                    <div className="text-xs mt-1" style={{ color: '#848E9C' }}>
                      {t('hyperliquidMainWalletAddressDesc', language)}
                    </div>
                  </div>
                </>
              )}

              {/* Aster 交易所的字段 */}
              {selectedExchange.id === 'aster' && (
                <>
                  <div>
                    <label
                      className="block text-sm font-semibold mb-2 flex items-center gap-2"
                      style={{ color: '#EAECEF' }}
                    >
                      {t('user', language)}
                      <Tooltip content={t('asterUserDesc', language)}>
                        <HelpCircle
                          className="w-4 h-4 cursor-help"
                          style={{ color: '#F0B90B' }}
                        />
                      </Tooltip>
                    </label>
                    <input
                      type="text"
                      value={asterUser}
                      onChange={(e) => setAsterUser(e.target.value)}
                      placeholder={t('enterUser', language)}
                      className="w-full px-3 py-2 rounded"
                      style={{
                        background: '#0B0E11',
                        border: '1px solid #2B3139',
                        color: '#EAECEF',
                      }}
                      required
                    />
                  </div>

                  <div>
                    <label
                      className="block text-sm font-semibold mb-2 flex items-center gap-2"
                      style={{ color: '#EAECEF' }}
                    >
                      {t('signer', language)}
                      <Tooltip content={t('asterSignerDesc', language)}>
                        <HelpCircle
                          className="w-4 h-4 cursor-help"
                          style={{ color: '#F0B90B' }}
                        />
                      </Tooltip>
                    </label>
                    <input
                      type="text"
                      value={asterSigner}
                      onChange={(e) => setAsterSigner(e.target.value)}
                      placeholder={t('enterSigner', language)}
                      className="w-full px-3 py-2 rounded"
                      style={{
                        background: '#0B0E11',
                        border: '1px solid #2B3139',
                        color: '#EAECEF',
                      }}
                      required
                    />
                  </div>

                  <div>
                    <label
                      className="block text-sm font-semibold mb-2 flex items-center gap-2"
                      style={{ color: '#EAECEF' }}
                    >
                      {t('privateKey', language)}
                      <Tooltip content={t('asterPrivateKeyDesc', language)}>
                        <HelpCircle
                          className="w-4 h-4 cursor-help"
                          style={{ color: '#F0B90B' }}
                        />
                      </Tooltip>
                    </label>
                    <input
                      type="text"
                      value={asterPrivateKey}
                      onChange={(e) => setAsterPrivateKey(e.target.value)}
                      placeholder={t('enterPrivateKey', language)}
                      className="w-full px-3 py-2 rounded"
                      style={{
                        background: '#0B0E11',
                        border: '1px solid #2B3139',
                        color: '#EAECEF',
                      }}
                      required
                    />
                  </div>
                </>
              )}

              <div>
                <label className="flex items-center gap-2 text-sm">
                  <input
                    type="checkbox"
                    checked={testnet}
                    onChange={(e) => setTestnet(e.target.checked)}
                    className="form-checkbox rounded"
                    style={{ accentColor: '#F0B90B' }}
                  />
                  <span style={{ color: '#EAECEF' }}>
                    {t('useTestnet', language)}
                  </span>
                </label>
                <div className="text-xs mt-1" style={{ color: '#848E9C' }}>
                  {t('testnetDescription', language)}
                </div>
              </div>

              <div
                className="p-4 rounded"
                style={{
                  background: 'rgba(240, 185, 11, 0.1)',
                  border: '1px solid rgba(240, 185, 11, 0.2)',
                }}
              >
                <div
                  className="text-sm font-semibold mb-2"
                  style={{ color: '#F0B90B' }}
                >
                  <span className="inline-flex items-center gap-1">
                    <AlertTriangle className="w-4 h-4" />{' '}
                    {t('securityWarning', language)}
                  </span>
                </div>
                <div className="text-xs space-y-1" style={{ color: '#848E9C' }}>
                  {selectedExchange.id === 'aster' && (
                    <div>{t('asterUsdtWarning', language)}</div>
                  )}
                  <div>{t('exchangeConfigWarning1', language)}</div>
                  <div>{t('exchangeConfigWarning2', language)}</div>
                  <div>{t('exchangeConfigWarning3', language)}</div>
                </div>
              </div>
            </>
          )}

          <div className="flex gap-3 mt-6">
            <button
              type="button"
              onClick={onClose}
              className="flex-1 px-4 py-2 rounded text-sm font-semibold"
              style={{ background: '#2B3139', color: '#848E9C' }}
            >
              {t('cancel', language)}
            </button>
            <button
              type="submit"
              disabled={
                !selectedExchange ||
                (exchangeProvider === 'binance' &&
                  (!apiKey.trim() || !secretKey.trim())) ||
                (exchangeProvider === 'okx' &&
                  (!apiKey.trim() ||
                    !secretKey.trim() ||
                    !passphrase.trim())) ||
                (exchangeProvider === 'bitget' &&
                  (!apiKey.trim() ||
                    !secretKey.trim() ||
                    !passphrase.trim())) ||
                (exchangeProvider === 'hyperliquid' &&
                  (!apiKey.trim() || !hyperliquidWalletAddr.trim())) || // 验证私钥和钱包地址
                (exchangeProvider === 'aster' &&
                  (!asterUser.trim() ||
                    !asterSigner.trim() ||
                    !asterPrivateKey.trim())) ||
                (selectedExchange.type === 'cex' &&
                  exchangeProvider !== 'hyperliquid' &&
                  exchangeProvider !== 'aster' &&
                  exchangeProvider !== 'binance' &&
                  exchangeProvider !== 'okx' &&
                  exchangeProvider !== 'bitget' &&
                  (!apiKey.trim() || !secretKey.trim()))
              }
              className="flex-1 px-4 py-2 rounded text-sm font-semibold disabled:opacity-50"
              style={{ background: '#F0B90B', color: '#000' }}
            >
              {t('saveConfig', language)}
            </button>
          </div>
        </form>
      </div>

      {/* Binance Setup Guide Modal */}
      {showGuide && (
        <div
          className="fixed inset-0 bg-black bg-opacity-75 flex items-center justify-center z-50 p-4"
          onClick={() => setShowGuide(false)}
        >
          <div
            className="bg-gray-800 rounded-lg p-6 w-full max-w-4xl relative"
            style={{ background: '#1E2329' }}
            onClick={(e) => e.stopPropagation()}
          >
            <div className="flex items-center justify-between mb-4">
              <h3
                className="text-xl font-bold flex items-center gap-2"
                style={{ color: '#EAECEF' }}
              >
                <BookOpen className="w-6 h-6" style={{ color: '#F0B90B' }} />
                {t('binanceSetupGuide', language)}
              </h3>
              <button
                onClick={() => setShowGuide(false)}
                className="px-4 py-2 rounded text-sm font-semibold transition-all hover:scale-105"
                style={{ background: '#2B3139', color: '#848E9C' }}
              >
                {t('closeGuide', language)}
              </button>
            </div>
            <div className="overflow-y-auto max-h-[80vh]">
              <img
                src="/images/guide.png"
                alt={t('binanceSetupGuide', language)}
                className="w-full h-auto rounded"
              />
            </div>
          </div>
        </div>
      )}

      {/* Two Stage Key Modal */}
      <TwoStageKeyModal
        isOpen={secureInputTarget !== null}
        language={language}
        contextLabel={secureInputContextLabel}
        expectedLength={64}
        onCancel={handleSecureInputCancel}
        onComplete={handleSecureInputComplete}
      />
    </div>
  )
}
// Create Account Modal Component (创建交易员账号模态框)
function CreateAccountModal({
  traderId,
  onSave,
  onClose,
}: {
  traderId: string
  onSave: (traderId: string, options: {
    generate_random_email: boolean
    generate_random_password: boolean
    email?: string
    password?: string
  }) => void
  onClose: () => void
}) {
  const [generateRandomEmail, setGenerateRandomEmail] = useState(true)
  const [generateRandomPassword, setGenerateRandomPassword] = useState(true)
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [loading, setLoading] = useState(false)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    
    // 验证必填字段
    if (!generateRandomEmail && !email.trim()) {
      alert('请输入账号（邮箱）')
      return
    }
    if (!generateRandomPassword && !password.trim()) {
      alert('请输入密码')
      return
    }

    setLoading(true)
    try {
      await onSave(traderId, {
        generate_random_email: generateRandomEmail,
        generate_random_password: generateRandomPassword,
        email: generateRandomEmail ? undefined : email.trim(),
        password: generateRandomPassword ? undefined : password.trim(),
      })
    } catch (error) {
      console.error('Failed to create account:', error)
    } finally {
      setLoading(false)
    }
  }

  return (
    <div
      className="fixed inset-0 bg-black bg-opacity-75 flex items-center justify-center z-50 p-4"
      onClick={onClose}
    >
      <div
        className="bg-gray-800 rounded-lg p-6 w-full max-w-md"
        style={{ background: '#1E2329', border: '1px solid #2B3139' }}
        onClick={(e) => e.stopPropagation()}
      >
        <h3
          className="text-xl font-bold mb-4"
          style={{ color: '#EAECEF' }}
        >
          创建交易员账号
        </h3>

        <form onSubmit={handleSubmit} className="space-y-4">
          {/* 账号生成方式 */}
          <div>
            <label className="flex items-center gap-2 mb-2">
              <input
                type="checkbox"
                checked={generateRandomEmail}
                onChange={(e) => setGenerateRandomEmail(e.target.checked)}
                className="w-4 h-4"
              />
              <span style={{ color: '#EAECEF' }}>随机生成账号</span>
            </label>
            {!generateRandomEmail && (
              <input
                type="email"
                placeholder="请输入账号（邮箱）"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                className="w-full px-3 py-2 rounded"
                style={{
                  background: '#0B0E11',
                  border: '1px solid #2B3139',
                  color: '#EAECEF',
                }}
                required
              />
            )}
          </div>

          {/* 密码生成方式 */}
          <div>
            <label className="flex items-center gap-2 mb-2">
              <input
                type="checkbox"
                checked={generateRandomPassword}
                onChange={(e) => setGenerateRandomPassword(e.target.checked)}
                className="w-4 h-4"
              />
              <span style={{ color: '#EAECEF' }}>随机生成密码</span>
            </label>
            {!generateRandomPassword && (
              <input
                type="password"
                placeholder="请输入密码"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                className="w-full px-3 py-2 rounded"
                style={{
                  background: '#0B0E11',
                  border: '1px solid #2B3139',
                  color: '#EAECEF',
                }}
                required
              />
            )}
          </div>

          <div className="flex gap-3 mt-6">
            <button
              type="button"
              onClick={onClose}
              className="flex-1 px-4 py-2 rounded text-sm font-semibold"
              style={{ background: '#2B3139', color: '#848E9C' }}
            >
              取消
            </button>
            <button
              type="submit"
              disabled={loading}
              className="flex-1 px-4 py-2 rounded text-sm font-semibold disabled:opacity-50"
              style={{ background: '#F0B90B', color: '#000' }}
            >
              {loading ? '创建中...' : '创建'}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}


// Category Account Info Modal Component (分类账号信息弹窗)
function CategoryAccountInfoModal({
  accountInfo,
  onSave,
  onClose,
  onShowToast,
}: {
  accountInfo: any
  onSave: (newPassword: string) => void
  onClose: () => void
  onShowToast?: (message: string, type: 'success' | 'error' | 'warning' | 'info') => void
}) {
  const [copiedEmail, setCopiedEmail] = useState(false)
  const [copiedPassword, setCopiedPassword] = useState(false)
  const [showChangePasswordModal, setShowChangePasswordModal] = useState(false)

  const handleCopyEmail = () => {
    navigator.clipboard.writeText(accountInfo.email).then(() => {
      setCopiedEmail(true)
      setTimeout(() => setCopiedEmail(false), 2000)
    })
  }

  const handleCopyPassword = () => {
    if (accountInfo.password) {
      navigator.clipboard.writeText(accountInfo.password).then(() => {
        setCopiedPassword(true)
        setTimeout(() => setCopiedPassword(false), 2000)
      })
    }
  }

  return (
    <ModernModal
      isOpen={true}
      onClose={onClose}
      title="分类账号信息"
      size="md"
    >
      <div className="space-y-6">
          {/* 角色 - 最上面 */}
          <div>
            <label
              className="block text-sm font-medium mb-3"
              style={{ color: '#EAECEF' }}
            >
              用户类型
            </label>
            <div className="flex-1 relative">
              <input
                type="text"
                value={accountInfo.role === 'group_leader' ? '小组组长' : '交易员账号'}
                readOnly
                className="w-full px-4 py-3 rounded-xl text-sm transition-all duration-200"
                style={{
                  background: 'linear-gradient(135deg, #0B0E11 0%, #111518 100%)',
                  border: '1px solid rgba(43, 49, 57, 0.6)',
                  color: '#EAECEF',
                  boxShadow: '0 4px 12px rgba(0, 0, 0, 0.2)',
                }}
              />
              <div
                className="absolute inset-0 rounded-xl pointer-events-none"
                style={{
                  background: 'linear-gradient(135deg, rgba(59, 130, 246, 0.05), rgba(147, 51, 234, 0.05))',
                  border: '1px solid rgba(59, 130, 246, 0.1)',
                }}
              />
            </div>
          </div>

          {/* 账号（邮箱）- 中间 */}
          <div>
            <label
              className="block text-sm font-medium mb-3"
              style={{ color: '#EAECEF' }}
            >
              用户名
            </label>
            <div className="flex items-center gap-3">
              <div className="flex-1 relative">
                <input
                  type="text"
                  value={accountInfo.email}
                  readOnly
                  className="w-full px-4 py-3 rounded-xl text-sm transition-all duration-200"
                  style={{
                    background: 'linear-gradient(135deg, #0B0E11 0%, #111518 100%)',
                    border: '1px solid rgba(43, 49, 57, 0.6)',
                    color: '#EAECEF',
                    boxShadow: '0 4px 12px rgba(0, 0, 0, 0.2)',
                  }}
                />
                <div
                  className="absolute inset-0 rounded-xl pointer-events-none"
                  style={{
                    background: 'linear-gradient(135deg, rgba(59, 130, 246, 0.05), rgba(147, 51, 234, 0.05))',
                    border: '1px solid rgba(59, 130, 246, 0.1)',
                  }}
                />
      </div>
              <button
                onClick={handleCopyEmail}
                className="px-4 py-3 rounded-xl text-sm font-semibold transition-all duration-200 hover:scale-105 flex items-center gap-2 whitespace-nowrap"
                style={{
                  background: copiedEmail
                    ? 'linear-gradient(135deg, #10B981 0%, #34D399 100%)'
                    : 'linear-gradient(135deg, #2B3139 0%, #374151 100%)',
                  color: copiedEmail ? '#fff' : '#EAECEF',
                  border: '1px solid rgba(132, 142, 156, 0.2)',
                  boxShadow: '0 4px 12px rgba(0, 0, 0, 0.2)',
                }}
              >
                {copiedEmail ? (
                  <>
                    <Check className="w-4 h-4" />
                    已复制
                  </>
                ) : (
                  <>
                    <Copy className="w-4 h-4" />
                    复制
                  </>
                )}
              </button>
    </div>
          </div>

          {/* 密码 */}
          <div>
            <label
              className="block text-sm font-medium mb-3"
              style={{ color: '#EAECEF' }}
            >
              密码
            </label>
            <div className="flex items-center gap-3 mb-4">
              <div className="flex-1 relative">
                <input
                  type="text"
                  value={accountInfo.password || ''}
                  readOnly
                  className="w-full px-4 py-3 rounded-xl text-sm transition-all duration-200"
                  style={{
                    background: 'linear-gradient(135deg, #0B0E11 0%, #111518 100%)',
                    border: '1px solid rgba(43, 49, 57, 0.6)',
                    color: '#EAECEF',
                    boxShadow: '0 4px 12px rgba(0, 0, 0, 0.2)',
                  }}
                  placeholder="未设置密码"
                />
                <div
                  className="absolute inset-0 rounded-xl pointer-events-none"
                  style={{
                    background: 'linear-gradient(135deg, rgba(59, 130, 246, 0.05), rgba(147, 51, 234, 0.05))',
                    border: '1px solid rgba(59, 130, 246, 0.1)',
                  }}
                />
              </div>
              <button
                onClick={handleCopyPassword}
                className="px-4 py-3 rounded-xl text-sm font-semibold transition-all duration-200 hover:scale-105 disabled:opacity-50 disabled:cursor-not-allowed flex items-center gap-2 whitespace-nowrap"
                style={{
                  background: copiedPassword
                    ? 'linear-gradient(135deg, #10B981 0%, #34D399 100%)'
                    : !accountInfo.password
                      ? 'linear-gradient(135deg, #4B5563 0%, #6B7280 100%)'
                      : 'linear-gradient(135deg, #2B3139 0%, #374151 100%)',
                  color: copiedPassword ? '#fff' : '#EAECEF',
                  border: '1px solid rgba(132, 142, 156, 0.2)',
                  boxShadow: '0 4px 12px rgba(0, 0, 0, 0.2)',
                }}
                disabled={!accountInfo.password}
              >
                {copiedPassword ? (
                  <>
                    <Check className="w-4 h-4" />
                    已复制
                  </>
                ) : (
                  <>
                    <Copy className="w-4 h-4" />
                    复制
                  </>
                )}
              </button>
            </div>
            <button
              onClick={() => setShowChangePasswordModal(true)}
              className="w-full px-6 py-3 rounded-xl text-sm font-semibold transition-all duration-200 hover:scale-105 flex items-center justify-center gap-2"
              style={{
                background: 'linear-gradient(135deg, #6366F1 0%, #8B5CF6 100%)',
                color: '#fff',
                boxShadow: '0 4px 12px rgba(99, 102, 241, 0.3)',
              }}
            >
              <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15.232 5.232l3.536 3.536m-2.036-5.036a2.5 2.5 0 113.536 3.536L6.5 21.036H3v-3.572L16.732 3.732z" />
              </svg>
              {accountInfo.password ? '修改密码' : '设置密码'}
            </button>
          </div>

          {/* 底部操作按钮 */}
          <div className="flex gap-4 mt-8 pt-6 border-t" style={{ borderColor: 'rgba(43, 49, 57, 0.6)' }}>
            <button
              onClick={onClose}
              className="flex-1 px-6 py-3 rounded-xl text-sm font-semibold transition-all duration-200 hover:scale-105"
              style={{
                background: 'linear-gradient(135deg, #F0B90B 0%, #F59E0B 100%)',
                color: '#000',
                boxShadow: '0 4px 12px rgba(240, 185, 11, 0.3)',
              }}
            >
              关闭
            </button>
          </div>

        {/* 修改密码弹窗 */}
        {showChangePasswordModal && (
          <ChangePasswordModal
            accountId={accountInfo.id}
            onSave={(newPassword) => {
              onSave(newPassword)
              setShowChangePasswordModal(false)
            }}
            onClose={() => setShowChangePasswordModal(false)}
            onShowToast={onShowToast}
          />
        )}
      </div>
    </ModernModal>
  )
}


// Trader Account Info Modal Component (交易员账号信息弹窗)
function TraderAccountInfoModal({
  email,
  password,
  traderId,
  onSave,
  onClose,
  onShowToast,
}: {
  email: string
  password: string
  traderId: string
  onSave: (newPassword: string) => void
  onClose: () => void
  language: Language
  onShowToast?: (message: string, type: 'success' | 'error' | 'warning' | 'info') => void
}) {
  const [copiedEmail, setCopiedEmail] = useState(false)
  const [copiedPassword, setCopiedPassword] = useState(false)
  const [showChangePasswordModal, setShowChangePasswordModal] = useState(false)

  const handleCopyEmail = () => {
    navigator.clipboard.writeText(email).then(() => {
      setCopiedEmail(true)
      setTimeout(() => setCopiedEmail(false), 2000)
    })
  }

  const handleCopyPassword = () => {
    if (password) {
      navigator.clipboard.writeText(password).then(() => {
        setCopiedPassword(true)
        setTimeout(() => setCopiedPassword(false), 2000)
      })
    }
  }

  return (
    <ModernModal
      isOpen={true}
      onClose={onClose}
      title="交易员账号信息"
      size="md"
    >
      <div className="space-y-6">
          {/* 账号（邮箱） */}
          <div>
            <label
              className="block text-sm font-medium mb-3"
              style={{ color: '#EAECEF' }}
            >
              账号（邮箱）
            </label>
            <div className="flex items-center gap-3">
              <div className="flex-1 relative">
                <input
                  type="text"
                  value={email}
                  readOnly
                  className="w-full px-4 py-3 rounded-xl text-sm transition-all duration-200"
                  style={{
                    background: 'linear-gradient(135deg, #0B0E11 0%, #111518 100%)',
                    border: '1px solid rgba(43, 49, 57, 0.6)',
                    color: '#EAECEF',
                    boxShadow: '0 4px 12px rgba(0, 0, 0, 0.2)',
                  }}
                />
                <div
                  className="absolute inset-0 rounded-xl pointer-events-none"
                  style={{
                    background: 'linear-gradient(135deg, rgba(59, 130, 246, 0.05), rgba(147, 51, 234, 0.05))',
                    border: '1px solid rgba(59, 130, 246, 0.1)',
                  }}
                />
      </div>
              <button
                onClick={handleCopyEmail}
                className="px-4 py-3 rounded-xl text-sm font-semibold transition-all duration-200 hover:scale-105 flex items-center gap-2 whitespace-nowrap"
                style={{
                  background: copiedEmail
                    ? 'linear-gradient(135deg, #10B981 0%, #34D399 100%)'
                    : 'linear-gradient(135deg, #2B3139 0%, #374151 100%)',
                  color: copiedEmail ? '#fff' : '#EAECEF',
                  border: '1px solid rgba(132, 142, 156, 0.2)',
                  boxShadow: '0 4px 12px rgba(0, 0, 0, 0.2)',
                }}
              >
                {copiedEmail ? (
                  <>
                    <Check className="w-4 h-4" />
                    已复制
                  </>
                ) : (
                  <>
                    <Copy className="w-4 h-4" />
                    复制
                  </>
                )}
              </button>
    </div>
          </div>

          {/* 密码 */}
          <div>
            <label
              className="block text-sm font-medium mb-3"
              style={{ color: '#EAECEF' }}
            >
              密码
            </label>
            <div className="flex items-center gap-3 mb-4">
              <div className="flex-1 relative">
                <input
                  type="text"
                  value={password || ''}
                  readOnly
                  className="w-full px-4 py-3 rounded-xl text-sm transition-all duration-200"
                  style={{
                    background: 'linear-gradient(135deg, #0B0E11 0%, #111518 100%)',
                    border: '1px solid rgba(43, 49, 57, 0.6)',
                    color: '#EAECEF',
                    boxShadow: '0 4px 12px rgba(0, 0, 0, 0.2)',
                  }}
                  placeholder="未设置密码"
                />
                <div
                  className="absolute inset-0 rounded-xl pointer-events-none"
                  style={{
                    background: 'linear-gradient(135deg, rgba(59, 130, 246, 0.05), rgba(147, 51, 234, 0.05))',
                    border: '1px solid rgba(59, 130, 246, 0.1)',
                  }}
                />
              </div>
              <button
                onClick={handleCopyPassword}
                className="px-4 py-3 rounded-xl text-sm font-semibold transition-all duration-200 hover:scale-105 disabled:opacity-50 disabled:cursor-not-allowed flex items-center gap-2 whitespace-nowrap"
                style={{
                  background: copiedPassword
                    ? 'linear-gradient(135deg, #10B981 0%, #34D399 100%)'
                    : !password
                      ? 'linear-gradient(135deg, #4B5563 0%, #6B7280 100%)'
                      : 'linear-gradient(135deg, #2B3139 0%, #374151 100%)',
                  color: copiedPassword ? '#fff' : '#EAECEF',
                  border: '1px solid rgba(132, 142, 156, 0.2)',
                  boxShadow: '0 4px 12px rgba(0, 0, 0, 0.2)',
                }}
                disabled={!password}
              >
                {copiedPassword ? (
                  <>
                    <Check className="w-4 h-4" />
                    已复制
                  </>
                ) : (
                  <>
                    <Copy className="w-4 h-4" />
                    复制
                  </>
                )}
              </button>
            </div>
            <button
              onClick={() => setShowChangePasswordModal(true)}
              className="w-full px-6 py-3 rounded-xl text-sm font-semibold transition-all duration-200 hover:scale-105 flex items-center justify-center gap-2"
              style={{
                background: 'linear-gradient(135deg, #6366F1 0%, #8B5CF6 100%)',
                color: '#fff',
                boxShadow: '0 4px 12px rgba(99, 102, 241, 0.3)',
              }}
            >
              <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15.232 5.232l3.536 3.536m-2.036-5.036a2.5 2.5 0 113.536 3.536L6.5 21.036H3v-3.572L16.732 3.732z" />
              </svg>
              {password ? '修改密码' : '设置密码'}
            </button>
          </div>

          {/* 底部操作按钮 */}
          <div className="flex gap-4 mt-8 pt-6 border-t" style={{ borderColor: 'rgba(43, 49, 57, 0.6)' }}>
            <button
              onClick={onClose}
              className="flex-1 px-6 py-3 rounded-xl text-sm font-semibold transition-all duration-200 hover:scale-105"
              style={{
                background: 'linear-gradient(135deg, #F0B90B 0%, #F59E0B 100%)',
                color: '#000',
                boxShadow: '0 4px 12px rgba(240, 185, 11, 0.3)',
              }}
            >
              关闭
            </button>
          </div>

        {/* 修改密码弹窗 */}
        {showChangePasswordModal && (
          <ChangePasswordModal
            traderId={traderId}
            onSave={(newPassword) => {
              onSave(newPassword)
              setShowChangePasswordModal(false)
            }}
            onClose={() => setShowChangePasswordModal(false)}
            onShowToast={onShowToast}
          />
        )}
      </div>
    </ModernModal>
  )
}

// Change Password Modal Component (修改密码弹窗)
function ChangePasswordModal({
  traderId,
  accountId,
  onSave,
  onClose,
  onShowToast,
}: {
  traderId?: string
  accountId?: string
  onSave: (newPassword: string) => void
  onClose: () => void
  onShowToast?: (message: string, type: 'success' | 'error' | 'warning' | 'info') => void
}) {
  const [newPassword, setNewPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [saving, setSaving] = useState(false)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()

    if (!newPassword.trim()) {
      if (onShowToast) {
        onShowToast('密码不能为空', 'warning')
      } else {
        alert('密码不能为空')
      }
      return
    }

    if (newPassword !== confirmPassword) {
      if (onShowToast) {
        onShowToast('两次输入的密码不一致', 'warning')
      } else {
        alert('两次输入的密码不一致')
      }
      return
    }

    setSaving(true)
    try {
      if (accountId) {
        // 分类账号密码更新
        await api.updateCategoryAccountPassword(accountId, newPassword.trim())
        onSave(newPassword.trim())
      } else if (traderId) {
        // 交易员账号密码更新
        const result = await api.updateTraderAccountPassword(traderId, newPassword.trim())
        onSave(result.password)
      }
      if (onShowToast) {
        onShowToast('密码修改成功！', 'success')
      }
    } catch (error: any) {
      console.error('Failed to update password:', error)
      if (onShowToast) {
        onShowToast('密码修改失败: ' + (error.message || '未知错误'), 'error')
      }
    } finally {
      setSaving(false)
    }
  }

  return (
    <ModernModal
      isOpen={true}
      onClose={onClose}
      title="修改密码"
      size="sm"
    >
      <form onSubmit={handleSubmit} className="space-y-6">
          {/* 新密码 */}
          <div>
            <label
              className="block text-sm font-medium mb-3"
              style={{ color: '#EAECEF' }}
            >
              新密码
            </label>
            <div className="relative">
              <input
                type="password"
                value={newPassword}
                onChange={(e) => setNewPassword(e.target.value)}
                className="w-full px-4 py-3 rounded-xl text-sm transition-all duration-200 focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                style={{
                  background: 'linear-gradient(135deg, #0B0E11 0%, #111518 100%)',
                  border: '1px solid rgba(43, 49, 57, 0.6)',
                  color: '#EAECEF',
                  boxShadow: '0 4px 12px rgba(0, 0, 0, 0.2)',
                }}
                placeholder="请输入新密码"
                required
              />
              <div
                className="absolute inset-0 rounded-xl pointer-events-none opacity-0 transition-opacity duration-200"
                style={{
                  background: 'linear-gradient(135deg, rgba(59, 130, 246, 0.1), rgba(147, 51, 234, 0.1))',
                  border: '1px solid rgba(59, 130, 246, 0.3)',
                }}
              />
            </div>
          </div>

          {/* 确认密码 */}
          <div>
            <label
              className="block text-sm font-medium mb-3"
              style={{ color: '#EAECEF' }}
            >
              确认密码
            </label>
            <div className="relative">
              <input
                type="password"
                value={confirmPassword}
                onChange={(e) => setConfirmPassword(e.target.value)}
                className="w-full px-4 py-3 rounded-xl text-sm transition-all duration-200 focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                style={{
                  background: 'linear-gradient(135deg, #0B0E11 0%, #111518 100%)',
                  border: '1px solid rgba(43, 49, 57, 0.6)',
                  color: '#EAECEF',
                  boxShadow: '0 4px 12px rgba(0, 0, 0, 0.2)',
                  borderColor: newPassword && confirmPassword && newPassword !== confirmPassword
                    ? 'rgba(246, 70, 93, 0.6)'
                    : 'rgba(43, 49, 57, 0.6)',
                }}
                placeholder="请再次输入新密码"
                required
              />
              <div
                className="absolute inset-0 rounded-xl pointer-events-none opacity-0 transition-opacity duration-200"
                style={{
                  background: newPassword && confirmPassword && newPassword !== confirmPassword
                    ? 'linear-gradient(135deg, rgba(246, 70, 93, 0.1), rgba(246, 70, 93, 0.05))'
                    : 'linear-gradient(135deg, rgba(59, 130, 246, 0.1), rgba(147, 51, 234, 0.1))',
                  border: newPassword && confirmPassword && newPassword !== confirmPassword
                    ? '1px solid rgba(246, 70, 93, 0.3)'
                    : '1px solid rgba(59, 130, 246, 0.3)',
                }}
              />
            </div>
            {newPassword && confirmPassword && newPassword !== confirmPassword && (
              <p className="text-xs mt-2" style={{ color: '#F6465D' }}>
                两次输入的密码不一致
              </p>
            )}
          </div>

          {/* 底部操作按钮 */}
          <div className="flex gap-4 mt-8 pt-6 border-t" style={{ borderColor: 'rgba(43, 49, 57, 0.6)' }}>
            <button
              type="button"
              onClick={onClose}
              className="flex-1 px-6 py-3 rounded-xl text-sm font-semibold transition-all duration-200 hover:scale-105"
              style={{
                background: 'linear-gradient(135deg, #2B3139 0%, #374151 100%)',
                color: '#848E9C',
                border: '1px solid rgba(132, 142, 156, 0.2)',
              }}
            >
              取消
            </button>
            <button
              type="submit"
              disabled={saving || !newPassword.trim() || newPassword !== confirmPassword}
              className="flex-1 px-6 py-3 rounded-xl text-sm font-semibold transition-all duration-200 hover:scale-105 disabled:opacity-50 disabled:cursor-not-allowed flex items-center justify-center gap-2"
              style={{
                background: saving || !newPassword.trim() || newPassword !== confirmPassword
                  ? 'linear-gradient(135deg, #4B5563 0%, #6B7280 100%)'
                  : 'linear-gradient(135deg, #F0B90B 0%, #F59E0B 100%)',
                color: '#000',
                boxShadow: saving || !newPassword.trim() || newPassword !== confirmPassword
                  ? 'none'
                  : '0 4px 12px rgba(240, 185, 11, 0.3)',
              }}
            >
              {saving ? (
                <>
                  <div className="w-4 h-4 border-2 border-black border-t-transparent rounded-full animate-spin"></div>
                  保存中...
                </>
              ) : (
                <>
                  <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
                  </svg>
                  确认修改
                </>
              )}
            </button>
          </div>
        </form>
    </ModernModal>
  )
}

// 创建分类模态框
function CreateCategoryModal({
  onSave,
  onClose,
  onShowToast,
}: {
  onSave: (name: string, description?: string) => void
  onClose: () => void
  onShowToast?: (message: string, type?: 'success' | 'error' | 'warning' | 'info') => void
}) {
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [saving, setSaving] = useState(false)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!name.trim()) {
      onShowToast?.('请输入分类名称', 'warning')
      return
    }
    setSaving(true)
    try {
      await onSave(name.trim(), description.trim() || undefined)
    } finally {
      setSaving(false)
    }
  }

  return (
    <ModernModal
      isOpen={true}
      onClose={onClose}
      title="创建分类"
      size="md"
    >
      <form onSubmit={handleSubmit}>
        <div className="space-y-6">
          <div>
            <label className="block text-sm font-medium mb-3" style={{ color: '#EAECEF' }}>
              分类名称 <span style={{ color: '#F6465D' }}>*</span>
            </label>
            <div className="relative">
              <input
                type="text"
                value={name}
                onChange={(e) => setName(e.target.value)}
                className="w-full px-4 py-3 rounded-xl text-sm transition-all duration-200 focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                style={{
                  background: 'linear-gradient(135deg, #0B0E11 0%, #111518 100%)',
                  border: '1px solid rgba(43, 49, 57, 0.6)',
                  color: '#EAECEF',
                  boxShadow: '0 4px 12px rgba(0, 0, 0, 0.2)',
                }}
                placeholder="请输入分类名称"
                required
              />
              <div
                className="absolute inset-0 rounded-xl pointer-events-none opacity-0 transition-opacity duration-200 peer-focus:opacity-100"
                style={{
                  background: 'linear-gradient(135deg, rgba(59, 130, 246, 0.1), rgba(147, 51, 234, 0.1))',
                  border: '1px solid rgba(59, 130, 246, 0.3)',
                }}
              />
            </div>
          </div>

          <div>
            <label className="block text-sm font-medium mb-3" style={{ color: '#EAECEF' }}>
              分类描述（可选）
            </label>
            <div className="relative">
              <textarea
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                className="w-full px-4 py-3 rounded-xl text-sm transition-all duration-200 focus:ring-2 focus:ring-blue-500 focus:border-transparent resize-none"
                style={{
                  background: 'linear-gradient(135deg, #0B0E11 0%, #111518 100%)',
                  border: '1px solid rgba(43, 49, 57, 0.6)',
                  color: '#EAECEF',
                  boxShadow: '0 4px 12px rgba(0, 0, 0, 0.2)',
                }}
                placeholder="请输入分类描述"
                rows={4}
              />
              <div
                className="absolute inset-0 rounded-xl pointer-events-none opacity-0 transition-opacity duration-200"
                style={{
                  background: 'linear-gradient(135deg, rgba(59, 130, 246, 0.1), rgba(147, 51, 234, 0.1))',
                  border: '1px solid rgba(59, 130, 246, 0.3)',
                }}
              />
            </div>
          </div>
        </div>

        <div className="flex gap-4 mt-8 pt-6 border-t" style={{ borderColor: 'rgba(43, 49, 57, 0.6)' }}>
          <button
            type="button"
            onClick={onClose}
            className="flex-1 px-6 py-3 rounded-xl text-sm font-semibold transition-all duration-200 hover:scale-105"
            style={{
              background: 'linear-gradient(135deg, #2B3139 0%, #374151 100%)',
              color: '#848E9C',
              border: '1px solid rgba(132, 142, 156, 0.2)',
            }}
          >
            取消
          </button>
          <button
            type="submit"
            disabled={saving || !name.trim()}
            className="flex-1 px-6 py-3 rounded-xl text-sm font-semibold transition-all duration-200 hover:scale-105 disabled:opacity-50 disabled:cursor-not-allowed flex items-center justify-center gap-2"
            style={{
              background: saving || !name.trim()
                ? 'linear-gradient(135deg, #4B5563 0%, #6B7280 100%)'
                : 'linear-gradient(135deg, #10B981 0%, #34D399 100%)',
              color: '#EAECEF',
              boxShadow: saving || !name.trim()
                ? 'none'
                : '0 4px 12px rgba(16, 185, 129, 0.3)',
            }}
          >
            {saving ? (
              <>
                <div className="w-4 h-4 border-2 border-white border-t-transparent rounded-full animate-spin"></div>
                创建中...
              </>
            ) : (
              <>
                <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 6v6m0 0v6m0-6h6m-6 0H6" />
                </svg>
                创建分类
              </>
            )}
          </button>
        </div>
      </form>
    </ModernModal>
  )
}

// 分类详情模态框
function CategoryDetailModal({
  category,
  traders,
  onAddTrader,
  onRemoveTrader,
  onClose,
  onShowToast,
}: {
  category: any
  traders: Array<{ trader_id: string; trader_name: string; category?: string; owner_user_id?: string; ai_model?: string; exchange_id?: string }>
  onAddTrader: (traderId: string, category: string) => Promise<any>
  onRemoveTrader: (traderId: string) => void
  onClose: () => void
  onShowToast?: (message: string, type?: 'success' | 'error' | 'warning' | 'info') => void
}) {
  const { user } = useAuth()
  const [showAddModal, setShowAddModal] = useState(false)
  const [selectedTraderToAdd, setSelectedTraderToAdd] = useState<string>('')
  const [saving, setSaving] = useState(false)
  
  // 当traders prop更新时，强制重新渲染
  useEffect(() => {
    console.log('[CategoryDetailModal] Traders prop updated:', {
      total: traders.length,
      category: category.name,
      categoryTraders: traders.filter(t => t.category === category.name).length
    })
    // 强制更新组件状态
    setSelectedTraderToAdd('')
  }, [traders, category.name])

  // 获取该分类下的交易员
  const categoryTraders = useMemo(() => {
    return traders.filter((t) => t.category === category.name)
  }, [traders, category.name])

  // 获取可以添加的交易员（不属于任何分类的，且属于当前用户的）
  const availableTraders = useMemo(() => {
    const filtered = traders.filter((t) => {
      // 不属于任何分类（traderCategory为空字符串、null或undefined）
      const traderCategory = t.category
      const hasNoCategory = !traderCategory || traderCategory === '' || traderCategory === null || traderCategory === undefined
      // 属于当前用户（如果后端返回了owner_user_id，则检查；如果没有返回，则允许，因为可能是旧数据）
      const belongsToUser = t.owner_user_id === undefined || (user && t.owner_user_id === user.id)
      // 不能是当前分类下的交易员
      const notInCurrentCategory = traderCategory !== category.name
      return hasNoCategory && belongsToUser && notInCurrentCategory
    })

    console.log('[CategoryDetailModal] Available traders updated:', {
      total: traders.length,
      available: filtered.length,
      category: category.name,
      traders: filtered.map(t => ({ id: t.trader_id, name: t.trader_name, category: t.category }))
    })

    return filtered
  }, [traders, user?.id, category.name]) // 添加user?.id作为依赖

  const handleAddTrader = async (traderId?: string) => {
    const traderIdToAdd = traderId || selectedTraderToAdd
    if (!traderIdToAdd) {
      onShowToast?.('请选择要添加的交易员', 'warning')
      return
    }

    // 检查交易员是否已经属于其他分类
    const trader = traders.find((t) => t.trader_id === traderIdToAdd)
    if (trader?.category && trader.category !== '' && trader.category !== category.name) {
      onShowToast?.('该交易员已属于其他分类，无法添加', 'error')
      return
    }

    setSaving(true)
    setSelectedTraderToAdd(traderIdToAdd)
    try {
      console.log('[CategoryDetailModal] Adding trader:', traderIdToAdd, 'to category:', category.name)
      await onAddTrader(traderIdToAdd, category.name)

      // 等待数据刷新完成
      console.log('[CategoryDetailModal] Waiting for data refresh...')
      await new Promise(resolve => setTimeout(resolve, 1000))

      // 关闭添加模态框
      setShowAddModal(false)
      setSelectedTraderToAdd('')
      
      // 通知父组件关闭并重新打开详情弹窗以刷新数据
      onClose()
      
      onShowToast?.('交易员添加成功！请重新打开分类查看', 'success')
      console.log('[CategoryDetailModal] Trader added successfully')
    } catch (error: any) {
      console.error('[CategoryDetailModal] Failed to add trader:', error)
      onShowToast?.('添加交易员失败: ' + (error.message || '未知错误'), 'error')
    } finally {
      setSaving(false)
    }
  }

  const handleRemoveTrader = async (traderId: string) => {
    if (!confirm('确定要从该分类中移除此交易员吗？')) {
      return
    }
    setSaving(true)
    try {
      await onRemoveTrader(traderId)
    } finally {
      setSaving(false)
    }
  }

  return (
    <>
      <ModernModal
        isOpen={true}
        onClose={onClose}
        title={`分类详情：${category.name}`}
        size="lg"
      >
        {category.description && (
          <div className="mb-6 p-4 rounded-xl" style={{ background: 'rgba(16, 185, 129, 0.05)', border: '1px solid rgba(16, 185, 129, 0.2)' }}>
            <p className="text-sm" style={{ color: '#848E9C' }}>
              {category.description}
            </p>
          </div>
        )}

        {/* 添加交易员按钮 */}
        <div className="mb-6">
          <button
            onClick={() => setShowAddModal(true)}
            className="px-6 py-3 rounded-xl text-sm font-semibold transition-all duration-200 hover:scale-105 hover:shadow-lg flex items-center gap-3"
            style={{
              background: 'linear-gradient(135deg, #10B981 0%, #34D399 100%)',
              color: '#EAECEF',
              boxShadow: '0 4px 12px rgba(16, 185, 129, 0.3)',
            }}
          >
            <Plus className="w-5 h-5" />
            添加交易员
          </button>
        </div>

        {/* 交易员列表 */}
        <div className="space-y-3">
          {categoryTraders.length > 0 ? (
            categoryTraders.map((trader) => (
              <div
                key={trader.trader_id}
                className="p-4 rounded-xl transition-all duration-200 hover:scale-[1.02]"
                style={{
                  background: 'linear-gradient(135deg, #0B0E11 0%, #111518 100%)',
                  border: '1px solid rgba(43, 49, 57, 0.6)',
                  boxShadow: '0 4px 12px rgba(0, 0, 0, 0.2)',
                }}
              >
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-4 flex-1 min-w-0">
                    <div
                      className="w-12 h-12 rounded-xl flex items-center justify-center flex-shrink-0"
                      style={{
                        background: 'linear-gradient(135deg, #6366F1 0%, #8B5CF6 100%)',
                        color: '#fff',
                        boxShadow: '0 4px 12px rgba(99, 102, 241, 0.3)',
                      }}
                    >
                      <Bot className="w-6 h-6" />
                    </div>
                    <div className="min-w-0 flex-1">
                      <div className="font-bold text-base truncate" style={{ color: '#EAECEF' }}>
                        {trader.trader_name}
                      </div>
                      <div className="text-sm truncate mt-1" style={{ color: '#848E9C' }}>
                        ID: {trader.trader_id}
                      </div>
                    </div>
                  </div>
                  <button
                    onClick={() => handleRemoveTrader(trader.trader_id)}
                    disabled={saving}
                    className="px-4 py-2 rounded-lg text-sm font-semibold transition-all duration-200 hover:scale-105 disabled:opacity-50 flex items-center gap-2"
                    style={{
                      background: 'linear-gradient(135deg, rgba(246, 70, 93, 0.2), rgba(246, 70, 93, 0.1))',
                      color: '#F6465D',
                      border: '1px solid rgba(246, 70, 93, 0.3)',
                    }}
                  >
                    <Trash2 className="w-4 h-4" />
                    移除
                  </button>
                </div>
              </div>
            ))
          ) : (
            <div className="text-center py-12">
              <div
                className="w-16 h-16 rounded-full mx-auto mb-4 flex items-center justify-center"
                style={{ background: 'rgba(132, 142, 156, 0.1)' }}
              >
                <Bot className="w-8 h-8" style={{ color: '#848E9C' }} />
              </div>
              <div className="text-base font-medium" style={{ color: '#848E9C' }}>
                该分类下暂无交易员
              </div>
              <div className="text-sm mt-2" style={{ color: '#5A5F65' }}>
                点击上方按钮添加交易员到此分类
              </div>
            </div>
          )}
        </div>

        {/* 底部操作按钮 */}
        <div className="flex gap-3 mt-8 pt-6 border-t" style={{ borderColor: 'rgba(43, 49, 57, 0.6)' }}>
          <button
            onClick={onClose}
            className="flex-1 px-6 py-3 rounded-xl text-sm font-semibold transition-all duration-200 hover:scale-105"
            style={{
              background: 'linear-gradient(135deg, #2B3139 0%, #374151 100%)',
              color: '#848E9C',
              border: '1px solid rgba(132, 142, 156, 0.2)',
            }}
          >
            关闭
          </button>
        </div>
      </ModernModal>

      {/* 添加交易员模态框 */}
      <ModernModal
        isOpen={showAddModal}
        onClose={() => {
          setShowAddModal(false)
          setSelectedTraderToAdd('')
        }}
        title="添加交易员到分类"
        size="xl"
      >
        {availableTraders.length > 0 ? (
          <div className="space-y-3 max-h-96 overflow-y-auto">
            {availableTraders.map((trader) => (
              <div
                key={trader.trader_id}
                className="p-4 rounded-xl transition-all duration-200 hover:scale-[1.005] hover:bg-gray-800/30 cursor-pointer group"
                style={{
                  background: 'linear-gradient(135deg, #0B0E11 0%, #111518 100%)',
                  border: '1px solid rgba(43, 49, 57, 0.6)',
                  boxShadow: '0 2px 8px rgba(0, 0, 0, 0.15)',
                }}
                onClick={() => handleAddTrader(trader.trader_id)}
              >
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-4 flex-1 min-w-0">
                    <div
                      className="w-12 h-12 rounded-xl flex items-center justify-center flex-shrink-0"
                      style={{
                        background: trader.ai_model?.includes('deepseek')
                          ? 'linear-gradient(135deg, #60a5fa 0%, #3b82f6 100%)'
                          : 'linear-gradient(135deg, #c084fc 0%, #a855f7 100%)',
                        color: '#fff',
                        boxShadow: '0 4px 12px rgba(96, 165, 250, 0.3)',
                      }}
                    >
                      <Bot className="w-6 h-6" />
                    </div>
                    <div className="min-w-0 flex-1">
                      <div className="font-bold text-lg truncate" style={{ color: '#EAECEF' }}>
                        {trader.trader_name}
                      </div>
                      <div
                        className="text-sm truncate mt-1 flex items-center gap-2"
                        style={{ color: '#848E9C' }}
                      >
                        <span
                          className="px-2 py-1 rounded-md text-xs font-medium"
                          style={{
                            background: trader.ai_model?.includes('deepseek')
                              ? 'rgba(96, 165, 250, 0.2)'
                              : 'rgba(192, 132, 252, 0.2)',
                            color: trader.ai_model?.includes('deepseek')
                              ? '#60a5fa'
                              : '#c084fc',
                          }}
                        >
                          {trader.ai_model
                            ? trader.ai_model.split('_').pop()?.toUpperCase() || trader.ai_model
                            : 'Unknown'} Model
                        </span>
                        <span>•</span>
                        <span>{trader.exchange_id?.toUpperCase() || 'N/A'}</span>
                      </div>
                    </div>
                  </div>
                  <div className="flex items-center gap-3">
                    <div
                      className={`px-3 py-2 rounded-lg text-sm font-semibold transition-all duration-200 ${
                        saving && selectedTraderToAdd === trader.trader_id ? 'animate-pulse' : ''
                      }`}
                      style={{
                        background: saving && selectedTraderToAdd === trader.trader_id
                          ? 'linear-gradient(135deg, #F59E0B 0%, #D97706 100%)'
                          : 'linear-gradient(135deg, #10B981 0%, #059669 100%)',
                        color: '#EAECEF',
                        boxShadow: '0 4px 12px rgba(16, 185, 129, 0.3)',
                      }}
                    >
                      {saving && selectedTraderToAdd === trader.trader_id ? (
                        <div className="flex items-center gap-2">
                          <div className="w-4 h-4 border-2 border-white border-t-transparent rounded-full animate-spin"></div>
                          添加中...
                        </div>
                      ) : (
                        <div className="flex items-center gap-2">
                          <Plus className="w-4 h-4" />
                          添加到分类
                        </div>
                      )}
                    </div>
                  </div>
                </div>
              </div>
            ))}
          </div>
        ) : (
          <div className="text-center py-12">
            <div
              className="w-20 h-20 rounded-full mx-auto mb-6 flex items-center justify-center"
              style={{ background: 'rgba(132, 142, 156, 0.1)' }}
            >
              <Users className="w-10 h-10" style={{ color: '#848E9C' }} />
            </div>
            <div className="text-lg font-medium mb-2" style={{ color: '#EAECEF' }}>
              没有可添加的交易员
            </div>
            <div className="text-sm" style={{ color: '#848E9C' }}>
              所有交易员都已属于其他分类，或您没有权限添加交易员
            </div>
          </div>
        )}
      </ModernModal>
    </>
  )
}

// 创建分类账号模态框
function CreateCategoryAccountModal({
  category,
  onSave,
  onClose,
  onShowToast,
}: {
  category: any
  onSave: (options: {
    generate_random_email: boolean
    generate_random_password: boolean
    email?: string
    password?: string
    category: string
    role: 'group_leader'
  }) => void
  onClose: () => void
  onShowToast?: (message: string, type: 'success' | 'error' | 'warning' | 'info') => void
}) {
  const [generateRandomEmail, setGenerateRandomEmail] = useState(true)
  const [generateRandomPassword, setGenerateRandomPassword] = useState(true)
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [loading, setLoading] = useState(false)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()

    // 验证必填字段
    if (!generateRandomEmail && !email.trim()) {
      if (onShowToast) {
        onShowToast('请输入账号（邮箱）', 'warning')
      } else {
        alert('请输入账号（邮箱）')
      }
      return
    }
    if (!generateRandomPassword && !password.trim()) {
      if (onShowToast) {
        onShowToast('请输入密码', 'warning')
      } else {
        alert('请输入密码')
      }
      return
    }

    setLoading(true)
    try {
      await onSave({
        generate_random_email: generateRandomEmail,
        generate_random_password: generateRandomPassword,
        email: generateRandomEmail ? undefined : email.trim(),
        password: generateRandomPassword ? undefined : password.trim(),
        category: category.name,
        role: 'group_leader',
      })
    } catch (error) {
      console.error('Failed to create category account:', error)
    } finally {
      setLoading(false)
    }
  }

  return (
    <ModernModal
      isOpen={true}
      onClose={onClose}
      title="创建分类账号"
      size="md"
    >
      <div className="mb-4 p-4 rounded-xl" style={{
        background: 'linear-gradient(135deg, rgba(59, 130, 246, 0.1), rgba(139, 92, 246, 0.05))',
        border: '1px solid rgba(59, 130, 246, 0.3)'
      }}>
        <div className="text-sm font-medium mb-2" style={{ color: '#3B82F6' }}>
          目标分类
        </div>
        <div className="flex items-center gap-3">
          <div className="font-semibold" style={{ color: '#EAECEF' }}>
            {category.name}
          </div>
          {category.description && (
            <div className="text-sm" style={{ color: '#848E9C' }}>
              {category.description}
            </div>
          )}
        </div>
      </div>

      <form onSubmit={handleSubmit} className="space-y-6">

        {/* 账号生成方式 */}
        <div>
          <div className="flex items-center gap-3 mb-4">
            <input
              id="generateEmail"
              type="checkbox"
              checked={generateRandomEmail}
              onChange={(e) => setGenerateRandomEmail(e.target.checked)}
              className="w-4 h-4 rounded border-2 border-gray-600 focus:ring-2 focus:ring-blue-500 focus:border-transparent"
              style={{
                accentColor: '#10B981',
              }}
            />
            <label htmlFor="generateEmail" className="text-sm font-medium" style={{ color: '#EAECEF' }}>
              随机生成账号
            </label>
          </div>
          {!generateRandomEmail && (
            <div className="relative">
              <input
                type="email"
                placeholder="请输入账号（邮箱）"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                className="w-full px-4 py-3 rounded-xl text-sm transition-all duration-200 focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                style={{
                  background: 'linear-gradient(135deg, #0B0E11 0%, #111518 100%)',
                  border: '1px solid rgba(43, 49, 57, 0.6)',
                  color: '#EAECEF',
                  boxShadow: '0 4px 12px rgba(0, 0, 0, 0.2)',
                }}
                required
              />
              <div
                className="absolute inset-0 rounded-xl pointer-events-none opacity-0 transition-opacity duration-200"
                style={{
                  background: 'linear-gradient(135deg, rgba(59, 130, 246, 0.1), rgba(147, 51, 234, 0.1))',
                  border: '1px solid rgba(59, 130, 246, 0.3)',
                }}
              />
            </div>
          )}
        </div>

        {/* 密码生成方式 */}
        <div>
          <div className="flex items-center gap-3 mb-4">
            <input
              id="generatePassword"
              type="checkbox"
              checked={generateRandomPassword}
              onChange={(e) => setGenerateRandomPassword(e.target.checked)}
              className="w-4 h-4 rounded border-2 border-gray-600 focus:ring-2 focus:ring-blue-500 focus:border-transparent"
              style={{
                accentColor: '#10B981',
              }}
            />
            <label htmlFor="generatePassword" className="text-sm font-medium" style={{ color: '#EAECEF' }}>
              随机生成密码
            </label>
          </div>
          {!generateRandomPassword && (
            <div className="relative">
              <input
                type="password"
                placeholder="请输入密码"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                className="w-full px-4 py-3 rounded-xl text-sm transition-all duration-200 focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                style={{
                  background: 'linear-gradient(135deg, #0B0E11 0%, #111518 100%)',
                  border: '1px solid rgba(43, 49, 57, 0.6)',
                  color: '#EAECEF',
                  boxShadow: '0 4px 12px rgba(0, 0, 0, 0.2)',
                }}
                required
              />
              <div
                className="absolute inset-0 rounded-xl pointer-events-none opacity-0 transition-opacity duration-200"
                style={{
                  background: 'linear-gradient(135deg, rgba(59, 130, 246, 0.1), rgba(147, 51, 234, 0.1))',
                  border: '1px solid rgba(59, 130, 246, 0.3)',
                }}
              />
            </div>
          )}
        </div>

        <div className="flex gap-4 mt-8 pt-6 border-t" style={{ borderColor: 'rgba(43, 49, 57, 0.6)' }}>
          <button
            type="button"
            onClick={onClose}
            className="flex-1 px-6 py-3 rounded-xl text-sm font-semibold transition-all duration-200 hover:scale-105"
            style={{
              background: 'linear-gradient(135deg, #2B3139 0%, #374151 100%)',
              color: '#848E9C',
              border: '1px solid rgba(132, 142, 156, 0.2)',
            }}
          >
            取消
          </button>
          <button
            type="submit"
            disabled={loading}
            className="flex-1 px-6 py-3 rounded-xl text-sm font-semibold transition-all duration-200 hover:scale-105 disabled:opacity-50 disabled:cursor-not-allowed flex items-center justify-center gap-2"
            style={{
              background: loading
                ? 'linear-gradient(135deg, #4B5563 0%, #6B7280 100%)'
                : 'linear-gradient(135deg, #3B82F6 0%, #6366F1 100%)',
              color: '#000',
              boxShadow: loading
                ? 'none'
                : '0 4px 12px rgba(59, 130, 246, 0.3)',
            }}
          >
            {loading ? (
              <>
                <div className="w-4 h-4 border-2 border-black border-t-transparent rounded-full animate-spin"></div>
                创建中...
              </>
            ) : (
              <>
                <User className="w-4 h-4" />
                创建账号
              </>
            )}
          </button>
        </div>
      </form>
    </ModernModal>
  )
}

// 分类账号列表模态框
function CategoryAccountListModal({
  category,
  groupLeaders,
  categoryAccounts,
  onViewAccount,
  onClose,
}: {
  category: any
  groupLeaders: Array<{
    id: string
    email: string
    role: string
    categories: string[]
    trader_count: number
    created_at: string
  }>
  categoryAccounts: Array<{
    id: string
    email: string
    role: string
    trader_id?: string
    category: string
    created_at: string
  }>
  onViewAccount: (accountId: string) => void
  onClose: () => void
}) {
  const allAccounts = [
    ...groupLeaders.map(gl => ({ ...gl, type: 'group_leader' as const })),
    ...categoryAccounts.map(ca => ({ ...ca, type: ca.role as 'trader_account' | 'group_leader' }))
  ]

  return (
    <ModernModal
      isOpen={true}
      onClose={onClose}
      title={`${category.name} - 账号列表`}
      size="lg"
    >
      <div className="mb-4 p-4 rounded-xl" style={{
        background: 'linear-gradient(135deg, rgba(139, 92, 246, 0.1), rgba(168, 85, 247, 0.05))',
        border: '1px solid rgba(139, 92, 246, 0.3)'
      }}>
        <div className="text-sm font-medium mb-2" style={{ color: '#8B5CF6' }}>
          分类信息
        </div>
        <div className="flex items-center justify-between">
          <div>
            <div className="font-semibold" style={{ color: '#EAECEF' }}>
              {category.name}
            </div>
            {category.description && (
              <div className="text-sm mt-1" style={{ color: '#848E9C' }}>
                {category.description}
              </div>
            )}
          </div>
          <div className="text-sm" style={{ color: '#8B5CF6' }}>
            共 {allAccounts.length} 个账号
          </div>
        </div>
      </div>

      <div className="space-y-4 max-h-96 overflow-y-auto">
        {allAccounts.length > 0 ? (
          allAccounts.map((account) => (
            <div
              key={account.id}
              className="flex items-center justify-between p-4 rounded-xl transition-all duration-200 hover:scale-[1.01]"
              style={{
                background: 'linear-gradient(135deg, #0B0E11 0%, #111518 100%)',
                border: '1px solid rgba(43, 49, 57, 0.6)',
                boxShadow: '0 4px 12px rgba(0, 0, 0, 0.2)',
              }}
            >
              <div className="flex-1 min-w-0">
                <div className="flex items-center gap-3 mb-2">
                  <div className="font-medium text-sm" style={{ color: '#EAECEF' }}>
                    {account.email}
                  </div>
                  <span
                    className="px-2 py-1 rounded text-xs"
                    style={{
                      background: account.type === 'group_leader'
                        ? 'rgba(16, 185, 129, 0.1)'
                        : 'rgba(59, 130, 246, 0.1)',
                      color: account.type === 'group_leader' ? '#10B981' : '#3B82F6',
                      border: `1px solid ${account.type === 'group_leader' ? 'rgba(16, 185, 129, 0.3)' : 'rgba(59, 130, 246, 0.3)'}`,
                    }}
                  >
                    {account.type === 'group_leader' ? '小组组长' : '交易员账号'}
                  </span>
                </div>
                <div className="text-xs space-y-1" style={{ color: '#848E9C' }}>
                  <div>创建时间: {new Date(account.created_at).toLocaleString()}</div>
                  {account.type === 'group_leader' && 'trader_count' in account && (
                    <div>管理的交易员: {account.trader_count}个</div>
                  )}
                  {account.type === 'trader_account' && account.trader_id && (
                    <div>关联交易员ID: {account.trader_id}</div>
                  )}
                </div>
              </div>

              <button
                onClick={() => onViewAccount(account.id)}
                className="px-4 py-2 rounded-lg text-sm font-semibold transition-all duration-200 hover:scale-105 flex items-center gap-2 whitespace-nowrap"
                style={{
                  background: 'linear-gradient(135deg, #8B5CF6 0%, #A855F7 100%)',
                  color: '#fff',
                  boxShadow: '0 4px 12px rgba(139, 92, 246, 0.3)',
                }}
              >
                <Eye className="w-4 h-4" />
                查看详情
              </button>
            </div>
          ))
        ) : (
          <div className="text-center py-12">
            <div
              className="w-16 h-16 rounded-full mx-auto mb-4 flex items-center justify-center"
              style={{ background: 'rgba(139, 92, 246, 0.1)' }}
            >
              <User className="w-8 h-8" style={{ color: '#8B5CF6' }} />
            </div>
            <div className="text-lg font-semibold mb-2" style={{ color: '#EAECEF' }}>
              暂无账号
            </div>
            <div className="text-sm" style={{ color: '#848E9C' }}>
              该分类下还没有创建任何账号
            </div>
          </div>
        )}
      </div>

      <div className="flex gap-4 mt-8 pt-6 border-t" style={{ borderColor: 'rgba(43, 49, 57, 0.6)' }}>
        <button
          onClick={onClose}
          className="flex-1 px-6 py-3 rounded-xl text-sm font-semibold transition-all duration-200 hover:scale-105"
          style={{
            background: 'linear-gradient(135deg, #F0B90B 0%, #F59E0B 100%)',
            color: '#000',
            boxShadow: '0 4px 12px rgba(240, 185, 11, 0.3)',
          }}
        >
          关闭
        </button>
      </div>
    </ModernModal>
  )
}
