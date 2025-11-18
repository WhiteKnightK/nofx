import type {
  SystemStatus,
  AccountInfo,
  Position,
  DecisionRecord,
  Statistics,
  TraderInfo,
  AIModel,
  Exchange,
  CreateTraderRequest,
  UpdateModelConfigRequest,
  UpdateExchangeConfigRequest,
  CompetitionData,
} from '../types'
import { CryptoService } from './crypto'
import { httpClient } from './httpClient'

const API_BASE = '/api'

// Helper function to get auth headers
function getAuthHeaders(): Record<string, string> {
  const token = localStorage.getItem('auth_token')
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
  }

  if (token) {
    headers['Authorization'] = `Bearer ${token}`
  }

  return headers
}

export const api = {
  // AI交易员管理接口
  async getTraders(): Promise<TraderInfo[]> {
    const res = await httpClient.get(`${API_BASE}/my-traders`, getAuthHeaders())
    if (!res.ok) throw new Error('获取trader列表失败')
    return res.json()
  },

  // 获取公开的交易员列表（无需认证）
  async getPublicTraders(): Promise<any[]> {
    const res = await httpClient.get(`${API_BASE}/traders`)
    if (!res.ok) throw new Error('获取公开trader列表失败')
    return res.json()
  },

  async createTrader(request: CreateTraderRequest): Promise<TraderInfo> {
    // 创建交易员可能需要较长时间（查询余额），设置3分钟超时
    const controller = new AbortController()
    const timeoutId = setTimeout(() => controller.abort(), 180000) // 3分钟
    
    try {
      const res = await fetch(`${API_BASE}/traders`, {
        method: 'POST',
        headers: getAuthHeaders(),
        body: JSON.stringify(request),
        signal: controller.signal,
      })
      clearTimeout(timeoutId)
      
      if (!res.ok) {
        const errorData = await res.json().catch(() => ({}))
        throw new Error(errorData.error || '创建交易员失败')
      }
      return res.json()
    } catch (error: any) {
      clearTimeout(timeoutId)
      if (error.name === 'AbortError') {
        throw new Error('创建交易员超时，请稍后重试')
      }
      throw error
    }
  },

  async deleteTrader(traderId: string): Promise<void> {
    const res = await httpClient.delete(
      `${API_BASE}/traders/${traderId}`,
      getAuthHeaders()
    )
    if (!res.ok) throw new Error('删除交易员失败')
  },

  async startTrader(traderId: string): Promise<void> {
    const res = await httpClient.post(
      `${API_BASE}/traders/${traderId}/start`,
      undefined,
      getAuthHeaders()
    )
    if (!res.ok) throw new Error('启动交易员失败')
  },

  async stopTrader(traderId: string): Promise<void> {
    const res = await httpClient.post(
      `${API_BASE}/traders/${traderId}/stop`,
      undefined,
      getAuthHeaders()
    )
    if (!res.ok) throw new Error('停止交易员失败')
  },

  async updateTraderPrompt(
    traderId: string,
    customPrompt: string
  ): Promise<void> {
    const res = await httpClient.put(
      `${API_BASE}/traders/${traderId}/prompt`,
      { custom_prompt: customPrompt },
      getAuthHeaders()
    )
    if (!res.ok) throw new Error('更新自定义策略失败')
  },

  async getTraderConfig(traderId: string): Promise<any> {
    const res = await httpClient.get(
      `${API_BASE}/traders/${traderId}/config`,
      getAuthHeaders()
    )
    if (!res.ok) throw new Error('获取交易员配置失败')
    return res.json()
  },

  async updateTrader(
    traderId: string,
    request: CreateTraderRequest
  ): Promise<TraderInfo> {
    const res = await httpClient.put(
      `${API_BASE}/traders/${traderId}`,
      request,
      getAuthHeaders()
    )
    if (!res.ok) throw new Error('更新交易员失败')
    return res.json()
  },

  // AI模型配置接口
  async getModelConfigs(): Promise<AIModel[]> {
    const res = await httpClient.get(`${API_BASE}/models`, getAuthHeaders())
    if (!res.ok) throw new Error('获取模型配置失败')
    return res.json()
  },

  // 获取系统支持的AI模型列表（无需认证）
  async getSupportedModels(): Promise<AIModel[]> {
    const res = await httpClient.get(`${API_BASE}/supported-models`)
    if (!res.ok) throw new Error('获取支持的模型失败')
    return res.json()
  },

  async updateModelConfigs(request: UpdateModelConfigRequest): Promise<void> {
    // 获取RSA公钥
    const publicKey = await CryptoService.fetchPublicKey()

    // 初始化加密服务
    await CryptoService.initialize(publicKey)

    // 获取用户信息（从localStorage或其他地方）
    const userId = localStorage.getItem('user_id') || ''
    const sessionId = sessionStorage.getItem('session_id') || ''

    // 加密敏感数据
    const encryptedPayload = await CryptoService.encryptSensitiveData(
      JSON.stringify(request),
      userId,
      sessionId
    )

    // 发送加密数据
    const res = await httpClient.put(
      `${API_BASE}/models`,
      encryptedPayload,
      getAuthHeaders()
    )
    if (!res.ok) throw new Error('更新模型配置失败')
  },


  // 交易所配置接口
  async getExchangeConfigs(): Promise<Exchange[]> {
    const res = await httpClient.get(`${API_BASE}/exchanges`, getAuthHeaders())
    if (!res.ok) throw new Error('获取交易所配置失败')
    return res.json()
  },

  // 获取系统支持的交易所列表（无需认证）
  async getSupportedExchanges(): Promise<Exchange[]> {
    const res = await httpClient.get(`${API_BASE}/supported-exchanges`)
    if (!res.ok) throw new Error('获取支持的交易所失败')
    return res.json()
  },

  async updateExchangeConfigs(
    request: UpdateExchangeConfigRequest
  ): Promise<void> {
    const res = await httpClient.put(
      `${API_BASE}/exchanges`,
      request,
      getAuthHeaders()
    )
    if (!res.ok) throw new Error('更新交易所配置失败')
  },

  // 使用加密传输更新交易所配置
  async updateExchangeConfigsEncrypted(
    request: UpdateExchangeConfigRequest
  ): Promise<void> {
    // 获取RSA公钥
    const publicKey = await CryptoService.fetchPublicKey()

    // 初始化加密服务
    await CryptoService.initialize(publicKey)

    // 获取用户信息（从localStorage或其他地方）
    const userId = localStorage.getItem('user_id') || ''
    const sessionId = sessionStorage.getItem('session_id') || ''

    // 加密敏感数据
    const encryptedPayload = await CryptoService.encryptSensitiveData(
      JSON.stringify(request),
      userId,
      sessionId
    )

    // 发送加密数据
    const res = await httpClient.put(
      `${API_BASE}/exchanges`,
      encryptedPayload,
      getAuthHeaders()
    )
    if (!res.ok) throw new Error('更新交易所配置失败')
  },

  // 获取系统状态（支持trader_id）
  async getStatus(traderId?: string): Promise<SystemStatus> {
    const url = traderId
      ? `${API_BASE}/status?trader_id=${traderId}`
      : `${API_BASE}/status`
    const res = await httpClient.get(url, getAuthHeaders())
    if (!res.ok) throw new Error('获取系统状态失败')
    return res.json()
  },

  // 获取账户信息（支持trader_id）
  async getAccount(traderId?: string): Promise<AccountInfo> {
    const url = traderId
      ? `${API_BASE}/account?trader_id=${traderId}`
      : `${API_BASE}/account`
    const res = await httpClient.request(url, {
      cache: 'no-store',
      headers: {
        ...getAuthHeaders(),
        'Cache-Control': 'no-cache',
      },
    })
    if (!res.ok) throw new Error('获取账户信息失败')
    const data = await res.json()
    console.log('Account data fetched:', data)
    return data
  },

  // 获取持仓列表（支持trader_id）
  async getPositions(traderId?: string): Promise<Position[]> {
    const url = traderId
      ? `${API_BASE}/positions?trader_id=${traderId}`
      : `${API_BASE}/positions`
    const res = await httpClient.get(url, getAuthHeaders())
    if (!res.ok) throw new Error('获取持仓列表失败')
    return res.json()
  },

  // 获取决策日志（支持trader_id）
  async getDecisions(traderId?: string): Promise<DecisionRecord[]> {
    const url = traderId
      ? `${API_BASE}/decisions?trader_id=${traderId}`
      : `${API_BASE}/decisions`
    const res = await httpClient.get(url, getAuthHeaders())
    if (!res.ok) throw new Error('获取决策日志失败')
    return res.json()
  },

  // 获取最新决策（支持trader_id）
  async getLatestDecisions(traderId?: string): Promise<DecisionRecord[]> {
    const url = traderId
      ? `${API_BASE}/decisions/latest?trader_id=${traderId}`
      : `${API_BASE}/decisions/latest`
    const res = await httpClient.get(url, getAuthHeaders())
    if (!res.ok) throw new Error('获取最新决策失败')
    return res.json()
  },

  // 获取统计信息（支持trader_id）
  async getStatistics(traderId?: string): Promise<Statistics> {
    const url = traderId
      ? `${API_BASE}/statistics?trader_id=${traderId}`
      : `${API_BASE}/statistics`
    const res = await httpClient.get(url, getAuthHeaders())
    if (!res.ok) throw new Error('获取统计信息失败')
    return res.json()
  },

  // 获取收益率历史数据（支持trader_id）
  async getEquityHistory(traderId?: string): Promise<any[]> {
    const url = traderId
      ? `${API_BASE}/equity-history?trader_id=${traderId}`
      : `${API_BASE}/equity-history`
    const res = await httpClient.get(url, getAuthHeaders())
    if (!res.ok) throw new Error('获取历史数据失败')
    return res.json()
  },

  // 批量获取多个交易员的历史数据（无需认证）
  async getEquityHistoryBatch(traderIds: string[]): Promise<any> {
    const res = await httpClient.post(`${API_BASE}/equity-history-batch`, {
      trader_ids: traderIds,
    })
    if (!res.ok) throw new Error('获取批量历史数据失败')
    return res.json()
  },

  // 获取前5名交易员数据（无需认证）
  async getTopTraders(): Promise<any[]> {
    const res = await httpClient.get(`${API_BASE}/top-traders`)
    if (!res.ok) throw new Error('获取前5名交易员失败')
    return res.json()
  },

  // 获取公开交易员配置（无需认证）
  async getPublicTraderConfig(traderId: string): Promise<any> {
    const res = await httpClient.get(`${API_BASE}/trader/${traderId}/config`)
    if (!res.ok) throw new Error('获取公开交易员配置失败')
    return res.json()
  },

  // 获取AI学习表现分析（支持trader_id）
  async getPerformance(traderId?: string): Promise<any> {
    const url = traderId
      ? `${API_BASE}/performance?trader_id=${traderId}`
      : `${API_BASE}/performance`
    const res = await httpClient.get(url, getAuthHeaders())
    if (!res.ok) throw new Error('获取AI学习数据失败')
    return res.json()
  },

  // 获取竞赛数据（无需认证）
  async getCompetition(): Promise<CompetitionData> {
    const res = await httpClient.get(`${API_BASE}/competition`)
    if (!res.ok) throw new Error('获取竞赛数据失败')
    return res.json()
  },

  // 用户信号源配置接口
  async getUserSignalSource(): Promise<{
    coin_pool_url: string
    oi_top_url: string
  }> {
    const res = await httpClient.get(
      `${API_BASE}/user/signal-sources`,
      getAuthHeaders()
    )
    if (!res.ok) throw new Error('获取用户信号源配置失败')
    return res.json()
  },

  async saveUserSignalSource(
    coinPoolUrl: string,
    oiTopUrl: string
  ): Promise<void> {
    const res = await httpClient.post(
      `${API_BASE}/user/signal-sources`,
      {
        coin_pool_url: coinPoolUrl,
        oi_top_url: oiTopUrl,
      },
      getAuthHeaders()
    )
    if (!res.ok) throw new Error('保存用户信号源配置失败')
  },

  // 获取服务器IP（需要认证，用于白名单配置）
  async getServerIP(): Promise<{
    public_ip: string
    message: string
  }> {
    const res = await httpClient.get(`${API_BASE}/server-ip`, getAuthHeaders())
    if (!res.ok) throw new Error('获取服务器IP失败')
    return res.json()
  },

  // 用户账户信息接口
  async getUserAccount(): Promise<{
    id: string
    email: string
    role: string
    trader_id?: string
    categories?: string[]
  }> {
    const res = await httpClient.get(`${API_BASE}/user/account`, getAuthHeaders())
    if (!res.ok) throw new Error('获取用户账户信息失败')
    return res.json()
  },

  // 分类管理接口
  async getCategories(): Promise<any[]> {
    const res = await httpClient.get(`${API_BASE}/categories`, getAuthHeaders())
    if (!res.ok) throw new Error('获取分类列表失败')
    const data = await res.json()
    return data.categories || []
  },

  async createCategory(name: string, description?: string): Promise<any> {
    const res = await httpClient.post(
      `${API_BASE}/categories`,
      { name, description: description || '' },
      getAuthHeaders()
    )
    if (!res.ok) throw new Error('创建分类失败')
    return res.json()
  },

  async updateCategory(
    categoryId: number,
    name: string,
    description?: string
  ): Promise<any> {
    const res = await httpClient.put(
      `${API_BASE}/categories/${categoryId}`,
      { name, description: description || '' },
      getAuthHeaders()
    )
    if (!res.ok) throw new Error('更新分类失败')
    return res.json()
  },

  async deleteCategory(categoryId: number): Promise<void> {
    const res = await httpClient.delete(
      `${API_BASE}/categories/${categoryId}`,
      getAuthHeaders()
    )
    if (!res.ok) throw new Error('删除分类失败')
  },

  // 创建交易员账号
  async createTraderAccount(
    traderId: string,
    options: {
      generate_random_email?: boolean
      generate_random_password?: boolean
      email?: string
      password?: string
    }
  ): Promise<{
    user_id: string
    email: string
    password: string
    role: string
    trader_id: string
  }> {
    const res = await httpClient.post(
      `${API_BASE}/traders/${traderId}/create-account`,
      options,
      getAuthHeaders()
    )
    if (!res.ok) throw new Error('创建交易员账号失败')
    return res.json()
  },

  // 获取交易员账号信息
  async getTraderAccount(traderId: string): Promise<{
    account: {
      user_id: string
      email: string
      created_at: string
    } | null
  }> {
    const res = await httpClient.get(
      `${API_BASE}/traders/${traderId}/account`,
      getAuthHeaders()
    )
    if (!res.ok) throw new Error('获取交易员账号信息失败')
    return res.json()
  },

  // 更新交易员账号密码
  async updateTraderAccountPassword(
    traderId: string,
    password: string
  ): Promise<{
    message: string
    password: string
  }> {
    const res = await httpClient.put(
      `${API_BASE}/traders/${traderId}/account/password`,
      { password },
      getAuthHeaders()
    )
    if (!res.ok) {
      const errorData = await res.json().catch(() => ({}))
      throw new Error(errorData.error || '更新密码失败')
    }
    return res.json()
  },

  // 创建小组组长账号
  async createGroupLeader(options: {
    generate_random_email?: boolean
    generate_random_password?: boolean
    email?: string
    password?: string
    categories: string[]
  }): Promise<{
    user_id: string
    email: string
    password: string
    role: string
    categories: string[]
  }> {
    const res = await httpClient.post(
      `${API_BASE}/group-leaders/create`,
      options,
      getAuthHeaders()
    )
    if (!res.ok) throw new Error('创建小组组长账号失败')
    return res.json()
  },

  // 为特定分类创建小组组长账号
  async createGroupLeaderForCategory(options: {
    generate_random_email?: boolean
    generate_random_password?: boolean
    email?: string
    password?: string
    category: string
  }): Promise<{
    user_id: string
    email: string
    password: string
    role: string
    categories: string[]
  }> {
    const res = await httpClient.post(
      `${API_BASE}/group-leaders/create-for-category`,
      options,
      getAuthHeaders()
    )
    if (!res.ok) throw new Error('创建小组组长账号失败')
    return res.json()
  },

  // 获取小组组长列表
  async getGroupLeaders(): Promise<Array<{
    id: string
    email: string
    role: string
    categories: string[]
    trader_count: number
    created_at: string
  }>> {
    const res = await httpClient.get(
      `${API_BASE}/group-leaders`,
      getAuthHeaders()
    )
    if (!res.ok) throw new Error('获取小组组长列表失败')
    const data = await res.json()
    return data.leaders || []
  },

  // 获取分类账号列表
  async getCategoryAccounts(): Promise<Array<{
    id: string
    email: string
    role: string
    trader_id?: string
    category: string
    created_at: string
  }>> {
    const res = await httpClient.get(
      `${API_BASE}/category-accounts`,
      getAuthHeaders()
    )
    if (!res.ok) throw new Error('获取分类账号列表失败')
    return res.json()
  },

  // 获取分类账号信息
  async getCategoryAccountInfo(accountId: string): Promise<{
    id: string
    email: string
    role: string
    password?: string
  }> {
    const res = await httpClient.get(
      `${API_BASE}/category-accounts/${accountId}`,
      getAuthHeaders()
    )
    if (!res.ok) throw new Error('获取账号信息失败')
    return res.json()
  },

  // 更新分类账号密码
  async updateCategoryAccountPassword(accountId: string, newPassword: string): Promise<void> {
    const res = await httpClient.put(
      `${API_BASE}/category-accounts/${accountId}/password`,
      { password: newPassword },
      getAuthHeaders()
    )
    if (!res.ok) throw new Error('更新密码失败')
  },

  // 设置交易员分类
  async setTraderCategory(traderId: string, category: string): Promise<any> {
    const res = await httpClient.post(
      `${API_BASE}/traders/${traderId}/category`,
      { category },
      getAuthHeaders()
    )
    if (!res.ok) {
      const errorData = await res.json().catch(() => ({}))
      throw new Error(errorData.error || '设置交易员分类失败')
    }
    return await res.json().catch(() => ({}))
  },
}
