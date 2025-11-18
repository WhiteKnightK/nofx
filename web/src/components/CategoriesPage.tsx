import React, { useState, useEffect } from 'react'
import { api } from '../lib/api'
import { useAuth } from '../contexts/AuthContext'
import { BookOpen, Plus, Trash2, Users, Play, Square, UserPlus, Eye, User, Copy, Check } from 'lucide-react'
import { ToastContainer, ModernModal } from './Toast'

interface Category {
  id: number
  name: string
  description: string
  owner_user_id: string
  created_at: string
  updated_at: string
}


export function CategoriesPage() {
  const { user, token } = useAuth()
  const [categories, setCategories] = useState<Category[]>([])
  const [traders, setTraders] = useState<Array<{ trader_id: string; trader_name: string; category?: string; is_running?: boolean }>>([])
  const [groupLeaders, setGroupLeaders] = useState<Array<{
    id: string
    email: string
    role: string
    categories: string[]
    trader_count: number
    created_at: string
  }>>([])
  const [loading, setLoading] = useState(true)
  const [showCreateModal, setShowCreateModal] = useState(false)
  const [showEditModal, setShowEditModal] = useState(false)
  const [editingCategory, setEditingCategory] = useState<Category | null>(null)
  const [showCreateGroupLeaderModal, setShowCreateGroupLeaderModal] = useState(false)
  const [selectedCategoryForGroupLeader, setSelectedCategoryForGroupLeader] = useState<Category | null>(null)
  const [showCreateCategoryAccountModal, setShowCreateCategoryAccountModal] = useState(false)
  const [showCategoryAccountInfoModal, setShowCategoryAccountInfoModal] = useState(false)
  const [showCategoryAccountListModal, setShowCategoryAccountListModal] = useState(false)
  const [selectedCategoryForAccount, setSelectedCategoryForAccount] = useState<Category | null>(null)
  const [categoryAccounts, setCategoryAccounts] = useState<Array<{
    id: string
    email: string
    role: string
    trader_id?: string
    category: string
    created_at: string
  }>>([])
  const [selectedAccountInfo, setSelectedAccountInfo] = useState<{
    email: string
    password?: string
    id: string
    role: string
  } | null>(null)
  const [toasts, setToasts] = useState<Array<{ id: string; message: string; type: 'success' | 'error' | 'warning' | 'info' }>>([])

  // 从localStorage加载分类账号密码
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

  // 获取用户角色
  const userRole = user?.role || 'user'
  const isUser = userRole === 'user' || userRole === 'admin'
  const isGroupLeader = userRole === 'group_leader'

  // 显示Toast提示
  const showToast = (message: string, type: 'success' | 'error' | 'warning' | 'info' = 'info') => {
    const id = Date.now().toString()
    setToasts((prev) => [...prev, { id, message, type }])
  }

  const removeToast = (id: string) => {
    setToasts((prev) => prev.filter((t) => t.id !== id))
  }

  // 加载分类列表
  const loadCategories = async () => {
    try {
      const categoriesList = await api.getCategories()
      setCategories(categoriesList)
    } catch (error: any) {
      console.error('Failed to load categories:', error)
      showToast('加载分类列表失败: ' + (error.message || '未知错误'), 'error')
    }
  }

  // 加载交易员列表（用于统计）
  const loadTraders = async () => {
    try {
      const tradersList = await api.getTraders()
      setTraders(tradersList)
    } catch (error: any) {
      console.error('Failed to load traders:', error)
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

  // 加载分类账号列表
  const loadCategoryAccounts = async () => {
    try {
      const accountsList = await api.getCategoryAccounts()
      console.log('📊 加载的分类账号列表:', accountsList)
      // 检查每个账号的详细信息
      accountsList.forEach((acc: any) => {
        console.log(`账号: ${acc.email}, role: ${acc.role}, trader_id: ${acc.trader_id}, category: ${acc.category}`)
      })
      // 检查交易员账号
      const traderAccounts = accountsList.filter((acc: any) => acc.role === 'trader_account' || acc.trader_id)
      console.log('✅ 交易员账号数量:', traderAccounts.length, traderAccounts)
      setCategoryAccounts(accountsList)
    } catch (error: any) {
      console.error('Failed to load category accounts:', error)
    }
  }

  // 创建分类账号
  const handleCreateCategoryAccount = async (options: {
    generate_random_email: boolean
    generate_random_password: boolean
    email?: string
    password?: string
    category: string
    role: string
  }) => {
    try {
      let result
      if (options.role === 'trader_account') {
        // 这里需要为交易员账号创建，需要先创建交易员然后创建账号
        // 暂时先使用模拟逻辑
        alert('交易员账号创建功能正在开发中')
        return
      } else if (options.role === 'group_leader') {
        result = await api.createGroupLeaderForCategory({
          generate_random_email: options.generate_random_email,
          generate_random_password: options.generate_random_password,
          email: options.email,
          password: options.password,
          category: options.category,
        })
      }

      if (result && typeof result === 'object' && 'email' in result) {
        // 保存密码到本地存储
        if (result.password && result.user_id) {
          const newAccounts = {
            ...categoryAccountPasswords,
            [result.user_id]: {
              email: result.email,
              password: result.password
            }
          }
          setCategoryAccountPasswords(newAccounts)
          saveCategoryAccountsToStorage(newAccounts)
        }

        showToast(`${String(options.role) === 'trader_account' ? '交易员账号' : '小组组长'}账号创建成功！账号: ${result.email}`, 'success')
      }
      setShowCreateCategoryAccountModal(false)
      setSelectedCategoryForAccount(null)
      // 刷新账号列表
      await loadCategoryAccounts()
    } catch (error: any) {
      console.error('Failed to create category account:', error)
      showToast(error.message || '创建账号失败', 'error')
    }
  }

  // 查看账号信息
  const handleViewAccountInfo = async (accountId: string) => {
    try {
      if (!accountId || accountId === 'undefined') {
        showToast('账号ID无效', 'error')
        return
      }
      const accountInfo = await api.getCategoryAccountInfo(accountId)
      // 合并本地存储的密码
      const accountWithPassword = {
        ...accountInfo,
        password: categoryAccountPasswords[accountId]?.password || accountInfo.password || ''
      }
      setSelectedAccountInfo(accountWithPassword)
      setShowCategoryAccountInfoModal(true)
    } catch (error: any) {
      console.error('Failed to load account info:', error)
      showToast('获取账号信息失败: ' + (error.message || '未知错误'), 'error')
    }
  }

  // 更新账号密码
  const handleUpdateAccountPassword = async (accountId: string, newPassword: string) => {
    try {
      await api.updateCategoryAccountPassword(accountId, newPassword)
      // 更新本地存储的密码
      const newAccounts = {
        ...categoryAccountPasswords,
        [accountId]: {
          email: selectedAccountInfo?.email || '',
          password: newPassword
        }
      }
      setCategoryAccountPasswords(newAccounts)
      saveCategoryAccountsToStorage(newAccounts)
      showToast('密码更新成功！', 'success')
      // 刷新账号信息
      if (selectedAccountInfo) {
        const updatedInfo = await api.getCategoryAccountInfo(accountId)
        // 合并本地存储的密码
        const accountWithPassword = {
          ...updatedInfo,
          password: newPassword
        }
        setSelectedAccountInfo(accountWithPassword)
      }
    } catch (error: any) {
      console.error('Failed to update password:', error)
      showToast('密码更新失败: ' + (error.message || '未知错误'), 'error')
    }
  }

  useEffect(() => {
    if (user && token) {
      setLoading(true)
      Promise.all([loadCategories(), loadTraders(), loadGroupLeaders(), loadCategoryAccounts()]).finally(() => {
        setLoading(false)
      })
    }
  }, [user, token])

  // 获取分类下的交易员
  const getCategoryTraders = (categoryName: string) => {
    return traders.filter((trader) => trader.category === categoryName)
  }

  // 获取分类统计信息
  const getCategoryStats = (categoryName: string) => {
    const categoryTraders = getCategoryTraders(categoryName)
    const total = categoryTraders.length
    const running = categoryTraders.filter((t) => t.is_running === true).length
    return { total, running }
  }

  // 获取分类下的小组组长（应该只有一个）
  const getCategoryGroupLeader = (categoryName: string) => {
    return groupLeaders.find((leader) => leader.categories.includes(categoryName))
  }

  // 创建分类
  const handleCreateCategory = async (name: string, description?: string) => {
    try {
      await api.createCategory(name, description)
      await loadCategories()
      setShowCreateModal(false)
      showToast('分类创建成功！', 'success')
    } catch (error: any) {
      console.error('Failed to create category:', error)
      showToast('创建分类失败: ' + (error.message || '未知错误'), 'error')
    }
  }

  // 更新分类
  const handleUpdateCategory = async (categoryId: number, name: string, description?: string) => {
    try {
      await api.updateCategory(categoryId, name, description)
      await loadCategories()
      setShowEditModal(false)
      setEditingCategory(null)
      showToast('分类更新成功！', 'success')
    } catch (error: any) {
      console.error('Failed to update category:', error)
      showToast('更新分类失败: ' + (error.message || '未知错误'), 'error')
    }
  }

  // 删除分类
  const handleDeleteCategory = async (categoryId: number, categoryName: string) => {
    if (!confirm(`确定要删除分类"${categoryName}"吗？\n删除后，该分类下的交易员将不再属于任何分类。`)) {
      return
    }

    try {
      await api.deleteCategory(categoryId)
      await loadCategories()
      await loadTraders()
      showToast('分类删除成功！', 'success')
    } catch (error: any) {
      console.error('Failed to delete category:', error)
      showToast('删除分类失败: ' + (error.message || '未知错误'), 'error')
    }
  }

  // 创建小组组长账号
  const handleCreateGroupLeader = async (options: {
    generate_random_email: boolean
    generate_random_password: boolean
    email?: string
    password?: string
    category: string
  }) => {
    try {
      const result = await api.createGroupLeaderForCategory(options)
      // 保存密码到本地存储
      if (result.password && result.user_id) {
        const newAccounts = {
          ...categoryAccountPasswords,
          [result.user_id]: {
            email: result.email,
            password: result.password
          }
        }
        setCategoryAccountPasswords(newAccounts)
        saveCategoryAccountsToStorage(newAccounts)
      }

      showToast(`小组组长账号创建成功！账号: ${result.email}`, 'success')
      setShowCreateGroupLeaderModal(false)
      setSelectedCategoryForGroupLeader(null)
      // 刷新小组组长列表
      await loadGroupLeaders()
    } catch (error: any) {
      console.error('Failed to create group leader:', error)
      showToast(error.message || '创建小组组长账号失败', 'error')
    }
  }

  // 小组组长：获取可以查看的分类
  const getViewableCategories = () => {
    if (isGroupLeader && user?.categories) {
      return categories.filter((cat) => user.categories?.includes(cat.name))
    }
    return categories
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center min-h-screen" style={{ color: '#EAECEF' }}>
        加载中...
      </div>
    )
  }

  const viewableCategories = getViewableCategories()

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
              background: 'linear-gradient(135deg, #10B981 0%, #34D399 100%)',
              boxShadow: '0 4px 14px rgba(16, 185, 129, 0.4)',
            }}
          >
            <BookOpen className="w-5 h-5 md:w-6 md:h-6" style={{ color: '#000' }} />
          </div>
          <div>
            <h1 className="text-xl md:text-2xl font-bold" style={{ color: '#EAECEF' }}>
              分类管理
            </h1>
            <p className="text-xs md:text-sm mt-1" style={{ color: '#848E9C' }}>
              {isGroupLeader ? '查看您管理的分类' : '管理您的交易员分类'}
            </p>
          </div>
        </div>

        {isUser && (
          <button
            onClick={() => setShowCreateModal(true)}
            className="px-3 md:px-4 py-2 rounded text-xs md:text-sm font-semibold transition-all hover:scale-105 flex items-center gap-1 md:gap-2 whitespace-nowrap"
            style={{
              background: '#10B981',
              color: '#EAECEF',
              border: '1px solid #474D57',
            }}
          >
            <Plus className="w-3 h-3 md:w-4 md:h-4" />
            创建分类
          </button>
        )}
      </div>

      {/* Categories List */}
      <div className="space-y-3 md:space-y-4">
        {viewableCategories.length > 0 ? (
          viewableCategories.map((category) => {
            const stats = getCategoryStats(category.name)
            const categoryTraders = getCategoryTraders(category.name)
            const categoryGroupLeader = getCategoryGroupLeader(category.name)

            return (
              <div
                key={category.id}
                className="p-4 md:p-6 rounded-lg transition-all hover:translate-y-[-2px]"
                style={{ background: '#0B0E11', border: '1px solid #2B3139' }}
              >
                <div className="flex flex-col gap-4">
                  {/* 分类基本信息 */}
                  <div>
                    <div className="flex items-center gap-3 mb-2">
                      <h3 className="text-lg md:text-xl font-bold" style={{ color: '#EAECEF' }}>
                        {category.name}
                      </h3>
                      {isGroupLeader && (
                        <span
                          className="px-2 py-1 rounded text-xs"
                          style={{ background: 'rgba(99, 102, 241, 0.1)', color: '#6366F1' }}
                        >
                          只读
                        </span>
                      )}
                    </div>
                    {category.description && (
                      <p className="text-sm mb-3" style={{ color: '#848E9C' }}>
                        {category.description}
                      </p>
                    )}
                    <div className="flex items-center gap-4 text-sm">
                      <div className="flex items-center gap-2" style={{ color: '#848E9C' }}>
                        <Users className="w-4 h-4" />
                        <span>交易员: {stats.total}</span>
                      </div>
                      <div className="flex items-center gap-2" style={{ color: '#0ECB81' }}>
                        <Play className="w-4 h-4" />
                        <span>运行中: {stats.running}</span>
                      </div>
                      <div className="flex items-center gap-2" style={{ color: '#848E9C' }}>
                        <Square className="w-4 h-4" />
                        <span>已停止: {stats.total - stats.running}</span>
                      </div>
                    </div>

                    {/* 交易员列表（小组组长可以查看） */}
                    {isGroupLeader && categoryTraders.length > 0 && (
                      <div className="mt-4 pt-4" style={{ borderTop: '1px solid #2B3139' }}>
                        <div className="text-sm mb-2" style={{ color: '#848E9C' }}>
                          交易员列表：
                        </div>
                        <div className="flex flex-wrap gap-2">
                          {categoryTraders.map((trader) => {
                            const isRunning = trader.is_running === true
                            return (
                              <div
                                key={trader.trader_id}
                                className="px-3 py-1.5 rounded text-xs flex items-center gap-2"
                                style={{
                                  background: isRunning
                                    ? 'rgba(14, 203, 129, 0.1)'
                                    : 'rgba(132, 142, 156, 0.1)',
                                  border: `1px solid ${isRunning ? '#0ECB81' : '#848E9C'}`,
                                  color: isRunning ? '#0ECB81' : '#848E9C',
                                }}
                              >
                                {trader.trader_name}
                                {isRunning ? (
                                  <div className="w-2 h-2 rounded-full bg-green-400" />
                                ) : (
                                  <div className="w-2 h-2 rounded-full bg-gray-500" />
                                )}
                              </div>
                            )
                          })}
                        </div>
                      </div>
                    )}
                  </div>

                  {/* 小组组长和交易员账号信息（普通用户可以查看）- 放在操作按钮上方 */}
                  {isUser && (categoryGroupLeader || categoryAccounts.filter(acc => 
                    acc.category === category.name && (acc.role === 'trader_account' || acc.trader_id)
                  ).length > 0) && (
                    <div className="pt-4" style={{ borderTop: '1px solid #2B3139' }}>
                      <div className="text-sm mb-3" style={{ color: '#848E9C' }}>
                        账号信息：
                      </div>
                      <div className="space-y-3">
                        {/* 小组组长账号 - 只显示一个 */}
                        {categoryGroupLeader && (
                          <div
                            className="flex items-center justify-between p-3 rounded-lg"
                            style={{
                              background: 'linear-gradient(135deg, rgba(16, 185, 129, 0.1), rgba(34, 197, 94, 0.05))',
                              border: '1px solid rgba(16, 185, 129, 0.3)',
                            }}
                          >
                            <div className="flex-1 min-w-0">
                              <div className="font-medium text-sm" style={{ color: '#EAECEF' }}>
                                {categoryGroupLeader.email}
                              </div>
                              <div className="text-xs" style={{ color: '#848E9C' }}>
                                管理的交易员: {categoryGroupLeader.trader_count}个
                              </div>
                            </div>
                            <div className="flex items-center gap-2">
                              <button
                                onClick={() => {
                                  // 从categoryAccounts中找到对应的小组组长账号
                                  const groupLeaderAccount = categoryAccounts.find(
                                    acc => acc.category === category.name && acc.role === 'group_leader' && acc.email === categoryGroupLeader.email
                                  )
                                  if (groupLeaderAccount) {
                                    handleViewAccountInfo(groupLeaderAccount.id)
                                  } else if (categoryGroupLeader.id) {
                                    handleViewAccountInfo(categoryGroupLeader.id)
                                  } else {
                                    showToast('无法找到小组组长账号信息', 'error')
                                  }
                                }}
                                className="px-3 py-1.5 rounded text-xs font-semibold transition-all hover:scale-105 flex items-center gap-1"
                                style={{
                                  background: 'rgba(139, 92, 246, 0.1)',
                                  color: '#8B5CF6',
                                  border: '1px solid rgba(139, 92, 246, 0.3)',
                                }}
                              >
                                <Eye className="w-3 h-3" />
                                查看
                              </button>
                              <span
                                className="px-2 py-1 rounded text-xs"
                                style={{
                                  background: 'rgba(16, 185, 129, 0.1)',
                                  color: '#10B981',
                                  border: '1px solid rgba(16, 185, 129, 0.3)',
                                }}
                              >
                                小组组长
                              </span>
                            </div>
                          </div>
                        )}

                        {/* 交易员账号列表 */}
                        {categoryAccounts.filter(acc => {
                          // 交易员账号：role === 'trader_account' 或有 trader_id
                          return acc.category === category.name && (acc.role === 'trader_account' || acc.trader_id)
                        }).map((account) => (
                          <div
                            key={account.id}
                            className="flex items-center justify-between p-3 rounded-lg"
                            style={{
                              background: 'linear-gradient(135deg, rgba(59, 130, 246, 0.1), rgba(147, 51, 234, 0.05))',
                              border: '1px solid rgba(59, 130, 246, 0.3)',
                            }}
                          >
                            <div className="flex-1 min-w-0">
                              <div className="font-medium text-sm" style={{ color: '#EAECEF' }}>
                                {account.email}
                              </div>
                              <div className="text-xs" style={{ color: '#848E9C' }}>
                                {account.trader_id ? `关联交易员ID: ${account.trader_id}` : '交易员账号'}
                              </div>
                            </div>
                            <div className="flex items-center gap-2">
                              <button
                                onClick={() => handleViewAccountInfo(account.id)}
                                className="px-3 py-1.5 rounded text-xs font-semibold transition-all hover:scale-105 flex items-center gap-1"
                                style={{
                                  background: 'rgba(139, 92, 246, 0.1)',
                                  color: '#8B5CF6',
                                  border: '1px solid rgba(139, 92, 246, 0.3)',
                                }}
                              >
                                <Eye className="w-3 h-3" />
                                查看
                              </button>
                              <span
                                className="px-2 py-1 rounded text-xs"
                                style={{
                                  background: 'rgba(59, 130, 246, 0.1)',
                                  color: '#3B82F6',
                                  border: '1px solid rgba(59, 130, 246, 0.3)',
                                }}
                              >
                                交易员账号
                              </span>
                            </div>
                          </div>
                        ))}
                      </div>
                    </div>
                  )}

                  {/* Actions - 操作按钮 - 放在最底部 */}
                  {isUser && (
                    <div className="flex flex-col gap-2 pt-4" style={{ borderTop: '1px solid #2B3139' }}>
                      {/* 只有当没有小组组长时才显示创建组长按钮 */}
                      {!categoryGroupLeader && (
                        <button
                          onClick={() => {
                            setSelectedCategoryForGroupLeader(category)
                            setShowCreateGroupLeaderModal(true)
                          }}
                          className="px-3 py-2 rounded text-sm font-semibold transition-all hover:scale-105 flex items-center justify-center gap-2 w-full"
                          style={{
                            background: 'rgba(16, 185, 129, 0.1)',
                            color: '#10B981',
                            border: '1px solid rgba(16, 185, 129, 0.3)',
                          }}
                        >
                          <UserPlus className="w-4 h-4" />
                          创建组长
                        </button>
                      )}
                      <button
                        onClick={() => handleDeleteCategory(category.id, category.name)}
                        className="px-3 py-2 rounded text-sm font-semibold transition-all hover:scale-105 flex items-center justify-center gap-2 w-full"
                        style={{
                          background: 'rgba(246, 70, 93, 0.1)',
                          color: '#F6465D',
                          border: '1px solid rgba(246, 70, 93, 0.3)',
                        }}
                      >
                        <Trash2 className="w-4 h-4" />
                        删除分类
                      </button>
                    </div>
                  )}
                </div>
              </div>
            )
          })
        ) : (
          <div
            className="text-center py-12 md:py-16 rounded-lg"
            style={{ background: '#0B0E11', border: '1px solid #2B3139' }}
          >
            <BookOpen className="w-16 h-16 md:w-24 md:h-24 mx-auto mb-3 md:mb-4 opacity-50" />
            <div className="text-base md:text-lg font-semibold mb-2" style={{ color: '#EAECEF' }}>
              {isGroupLeader ? '暂无可查看的分类' : '暂无分类'}
            </div>
            <div className="text-xs md:text-sm" style={{ color: '#848E9C' }}>
              {isGroupLeader
                ? '请联系管理员为您分配分类权限'
                : '创建第一个分类来组织您的交易员'}
            </div>
          </div>
        )}
      </div>

      {/* Create Category Modal */}
      {showCreateModal && (
        <CreateCategoryModal
          onSave={handleCreateCategory}
          onClose={() => setShowCreateModal(false)}
          onShowToast={showToast}
        />
      )}

      {/* Edit Category Modal */}
      {showEditModal && editingCategory && (
        <EditCategoryModal
          category={editingCategory}
          onSave={handleUpdateCategory}
          onClose={() => {
            setShowEditModal(false)
            setEditingCategory(null)
          }}
          onShowToast={showToast}
        />
      )}

      {/* Create Group Leader Modal */}
      {showCreateGroupLeaderModal && selectedCategoryForGroupLeader && (
        <CreateGroupLeaderForCategoryModal
          category={selectedCategoryForGroupLeader}
          onSave={handleCreateGroupLeader}
          onClose={() => {
            setShowCreateGroupLeaderModal(false)
            setSelectedCategoryForGroupLeader(null)
          }}
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
        />
      )}

      {/* Category Account List Modal */}
      {showCategoryAccountListModal && selectedCategoryForAccount && (
        <CategoryAccountListModal
          category={selectedCategoryForAccount}
          groupLeader={getCategoryGroupLeader(selectedCategoryForAccount.name)}
          categoryAccounts={categoryAccounts.filter(acc => acc.category === selectedCategoryForAccount.name)}
          onViewAccount={handleViewAccountInfo}
          onClose={() => {
            setShowCategoryAccountListModal(false)
            setSelectedCategoryForAccount(null)
          }}
        />
      )}

      {/* Category Account Info Modal */}
      {showCategoryAccountInfoModal && selectedAccountInfo && (
        <CategoryAccountInfoModal
          accountInfo={selectedAccountInfo}
          onUpdatePassword={handleUpdateAccountPassword}
          onClose={() => {
            setShowCategoryAccountInfoModal(false)
            setSelectedAccountInfo(null)
          }}
        />
      )}
    </div>
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
  onShowToast: (message: string, type?: 'success' | 'error' | 'warning' | 'info') => void
}) {
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [saving, setSaving] = useState(false)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!name.trim()) {
      onShowToast('请输入分类名称', 'warning')
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
    <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50 p-4">
      <div
        className="bg-gray-800 rounded-lg p-6 w-full max-w-md relative"
        style={{ background: '#1E2329' }}
        onClick={(e) => e.stopPropagation()}
      >
        <div className="flex items-center justify-between mb-4">
          <h3 className="text-xl font-bold" style={{ color: '#EAECEF' }}>
            创建分类
          </h3>
          <button
            onClick={onClose}
            className="text-gray-400 hover:text-gray-200"
            style={{ fontSize: '24px', lineHeight: '1' }}
          >
            ×
          </button>
        </div>

        <form onSubmit={handleSubmit}>
          <div className="space-y-4">
            <div>
              <label className="block mb-2" style={{ color: '#EAECEF' }}>
                分类名称 <span style={{ color: '#F6465D' }}>*</span>
              </label>
              <input
                type="text"
                value={name}
                onChange={(e) => setName(e.target.value)}
                className="w-full px-4 py-2 rounded"
                style={{ background: '#0B0E11', border: '1px solid #2B3139', color: '#EAECEF' }}
                placeholder="请输入分类名称"
                required
              />
            </div>

            <div>
              <label className="block mb-2" style={{ color: '#EAECEF' }}>
                分类描述（可选）
              </label>
              <textarea
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                className="w-full px-4 py-2 rounded"
                style={{ background: '#0B0E11', border: '1px solid #2B3139', color: '#EAECEF' }}
                placeholder="请输入分类描述"
                rows={3}
              />
            </div>
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
              disabled={saving || !name.trim()}
              className="flex-1 px-4 py-2 rounded text-sm font-semibold disabled:opacity-50"
              style={{ background: '#10B981', color: '#EAECEF' }}
            >
              {saving ? '创建中...' : '创建'}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}

// 编辑分类模态框
function EditCategoryModal({
  category,
  onSave,
  onClose,
  onShowToast,
}: {
  category: Category
  onSave: (categoryId: number, name: string, description?: string) => void
  onClose: () => void
  onShowToast: (message: string, type?: 'success' | 'error' | 'warning' | 'info') => void
}) {
  const [name, setName] = useState(category.name)
  const [description, setDescription] = useState(category.description || '')
  const [saving, setSaving] = useState(false)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!name.trim()) {
      onShowToast('请输入分类名称', 'warning')
      return
    }
    setSaving(true)
    try {
      await onSave(category.id, name.trim(), description.trim() || undefined)
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50 p-4">
      <div
        className="bg-gray-800 rounded-lg p-6 w-full max-w-md relative"
        style={{ background: '#1E2329' }}
        onClick={(e) => e.stopPropagation()}
      >
        <div className="flex items-center justify-between mb-4">
          <h3 className="text-xl font-bold" style={{ color: '#EAECEF' }}>
            编辑分类
          </h3>
          <button
            onClick={onClose}
            className="text-gray-400 hover:text-gray-200"
            style={{ fontSize: '24px', lineHeight: '1' }}
          >
            ×
          </button>
        </div>

        <form onSubmit={handleSubmit}>
          <div className="space-y-4">
            <div>
              <label className="block mb-2" style={{ color: '#EAECEF' }}>
                分类名称 <span style={{ color: '#F6465D' }}>*</span>
              </label>
              <input
                type="text"
                value={name}
                onChange={(e) => setName(e.target.value)}
                className="w-full px-4 py-2 rounded"
                style={{ background: '#0B0E11', border: '1px solid #2B3139', color: '#EAECEF' }}
                placeholder="请输入分类名称"
                required
              />
            </div>

            <div>
              <label className="block mb-2" style={{ color: '#EAECEF' }}>
                分类描述（可选）
              </label>
              <textarea
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                className="w-full px-4 py-2 rounded"
                style={{ background: '#0B0E11', border: '1px solid #2B3139', color: '#EAECEF' }}
                placeholder="请输入分类描述"
                rows={3}
              />
            </div>
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
              disabled={saving || !name.trim()}
              className="flex-1 px-4 py-2 rounded text-sm font-semibold disabled:opacity-50"
              style={{ background: '#10B981', color: '#EAECEF' }}
            >
              {saving ? '保存中...' : '保存'}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}

// 创建小组组长账号模态框（为特定分类）
function CreateGroupLeaderForCategoryModal({
  category,
  onSave,
  onClose,
}: {
  category: Category
  onSave: (options: {
    generate_random_email: boolean
    generate_random_password: boolean
    email?: string
    password?: string
    category: string
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
      await onSave({
        generate_random_email: generateRandomEmail,
        generate_random_password: generateRandomPassword,
        email: generateRandomEmail ? undefined : email.trim(),
        password: generateRandomPassword ? undefined : password.trim(),
        category: category.name,
      })
    } catch (error) {
      console.error('Failed to create group leader:', error)
    } finally {
      setLoading(false)
    }
  }

  return (
    <ModernModal
      isOpen={true}
      onClose={onClose}
      title="创建小组组长账号"
      size="md"
    >
      <div className="mb-4 p-4 rounded-xl" style={{
        background: 'linear-gradient(135deg, rgba(16, 185, 129, 0.1), rgba(34, 197, 94, 0.05))',
        border: '1px solid rgba(16, 185, 129, 0.3)'
      }}>
        <div className="text-sm font-medium mb-2" style={{ color: '#10B981' }}>
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
                : 'linear-gradient(135deg, #10B981 0%, #34D399 100%)',
              color: '#000',
              boxShadow: loading
                ? 'none'
                : '0 4px 12px rgba(16, 185, 129, 0.3)',
            }}
          >
            {loading ? (
              <>
                <div className="w-4 h-4 border-2 border-black border-t-transparent rounded-full animate-spin"></div>
                创建中...
              </>
            ) : (
              <>
                <UserPlus className="w-4 h-4" />
                创建小组组长
              </>
            )}
          </button>
        </div>
      </form>
    </ModernModal>
  )
}

// 创建分类账号模态框
function CreateCategoryAccountModal({
  category,
  onSave,
  onClose,
}: {
  category: Category
  onSave: (options: {
    generate_random_email: boolean
    generate_random_password: boolean
    email?: string
    password?: string
    category: string
    role: string
  }) => void
  onClose: () => void
}) {
  const [generateRandomEmail, setGenerateRandomEmail] = useState(true)
  const [generateRandomPassword, setGenerateRandomPassword] = useState(true)
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [role, setRole] = useState<'trader_account' | 'group_leader'>('group_leader')
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
      await onSave({
        generate_random_email: generateRandomEmail,
        generate_random_password: generateRandomPassword,
        email: generateRandomEmail ? undefined : email.trim(),
        password: generateRandomPassword ? undefined : password.trim(),
        category: category.name,
        role: role,
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
        {/* 角色选择 */}
        <div>
          <label className="block text-sm font-medium mb-4" style={{ color: '#EAECEF' }}>
            账号角色
          </label>
          <div className="space-y-3">
            <label className="flex items-center gap-3 p-3 rounded-xl cursor-pointer transition-all duration-200 hover:scale-[1.01]"
                   style={{
                     background: role === 'group_leader'
                       ? 'linear-gradient(135deg, rgba(16, 185, 129, 0.2), rgba(34, 197, 94, 0.1))'
                       : 'linear-gradient(135deg, #0B0E11 0%, #111518 100%)',
                     border: role === 'group_leader'
                       ? '1px solid rgba(16, 185, 129, 0.4)'
                       : '1px solid rgba(43, 49, 57, 0.6)',
                     boxShadow: role === 'group_leader'
                       ? '0 4px 12px rgba(16, 185, 129, 0.2)'
                       : '0 2px 8px rgba(0, 0, 0, 0.15)',
                   }}>
              <input
                type="radio"
                value="group_leader"
                checked={role === 'group_leader'}
                onChange={(e) => setRole(e.target.value as 'group_leader')}
                className="w-4 h-4"
                style={{
                  accentColor: '#10B981',
                }}
              />
              <div className="flex-1">
                <div className="font-medium text-sm" style={{ color: '#EAECEF' }}>
                  小组组长
                </div>
                <div className="text-xs mt-1" style={{ color: '#848E9C' }}>
                  可以查看和管理该分类下的所有交易员
                </div>
              </div>
            </label>
            <label className="flex items-center gap-3 p-3 rounded-xl cursor-pointer transition-all duration-200 hover:scale-[1.01]"
                   style={{
                     background: role === 'trader_account'
                       ? 'linear-gradient(135deg, rgba(59, 130, 246, 0.2), rgba(147, 51, 234, 0.1))'
                       : 'linear-gradient(135deg, #0B0E11 0%, #111518 100%)',
                     border: role === 'trader_account'
                       ? '1px solid rgba(59, 130, 246, 0.4)'
                       : '1px solid rgba(43, 49, 57, 0.6)',
                     boxShadow: role === 'trader_account'
                       ? '0 4px 12px rgba(59, 130, 246, 0.2)'
                       : '0 2px 8px rgba(0, 0, 0, 0.15)',
                   }}>
              <input
                type="radio"
                value="trader_account"
                checked={role === 'trader_account'}
                onChange={(e) => setRole(e.target.value as 'trader_account')}
                className="w-4 h-4"
                style={{
                  accentColor: '#3B82F6',
                }}
              />
              <div className="flex-1">
                <div className="font-medium text-sm" style={{ color: '#EAECEF' }}>
                  交易员账号
                </div>
                <div className="text-xs mt-1" style={{ color: '#848E9C' }}>
                  专门用于运行交易策略的独立账号
                </div>
              </div>
            </label>
          </div>
        </div>

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

// 分类账号信息模态框
function CategoryAccountInfoModal({
  accountInfo,
  onUpdatePassword,
  onClose,
}: {
  accountInfo: {
    email: string
    password?: string
    id: string
    role: string
  }
  onUpdatePassword: (accountId: string, newPassword: string) => void
  onClose: () => void
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
      title="账号信息"
      size="md"
    >
      <div className="space-y-6">
        {/* 用户类型 - 最上面 */}
        <div>
          <label
            className="block text-sm font-medium mb-3"
            style={{ color: '#EAECEF' }}
          >
            用户类型
          </label>
          <div className="px-4 py-3 rounded-xl" style={{
            background: 'linear-gradient(135deg, #0B0E11 0%, #111518 100%)',
            border: '1px solid rgba(43, 49, 57, 0.6)',
          }}>
            <span className="text-sm font-medium" style={{
              color: accountInfo.role === 'group_leader' ? '#10B981' :
                     accountInfo.role === 'trader_account' ? '#3B82F6' : '#EAECEF'
            }}>
              {accountInfo.role === 'group_leader' ? '小组组长' :
               accountInfo.role === 'trader_account' ? '交易员账号' :
               accountInfo.role}
            </span>
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
              disabled={!accountInfo.password}
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
              onUpdatePassword(accountInfo.id, newPassword)
              setShowChangePasswordModal(false)
            }}
            onClose={() => setShowChangePasswordModal(false)}
          />
        )}
      </div>
    </ModernModal>
  )
}

// 修改密码模态框
function ChangePasswordModal({
  accountId: _accountId,
  onSave,
  onClose,
}: {
  accountId: string
  onSave: (newPassword: string) => void
  onClose: () => void
}) {
  const [newPassword, setNewPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [loading, setLoading] = useState(false)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()

    if (!newPassword.trim()) {
      alert('请输入新密码')
      return
    }

    if (newPassword !== confirmPassword) {
      alert('两次输入的密码不一致')
      return
    }

    setLoading(true)
    try {
      await onSave(newPassword)
    } catch (error) {
      console.error('Failed to update password:', error)
    } finally {
      setLoading(false)
    }
  }

  return (
    <ModernModal
      isOpen={true}
      onClose={onClose}
      title="修改密码"
      size="sm"
    >
      <form onSubmit={handleSubmit} className="space-y-4">
        <div>
          <label className="block text-sm font-medium mb-2" style={{ color: '#EAECEF' }}>
            新密码
          </label>
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
        </div>

        <div>
          <label className="block text-sm font-medium mb-2" style={{ color: '#EAECEF' }}>
            确认新密码
          </label>
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
            }}
            placeholder="请再次输入新密码"
            required
          />
        </div>

        <div className="flex gap-3 mt-6">
          <button
            type="button"
            onClick={onClose}
            className="flex-1 px-4 py-3 rounded-xl text-sm font-semibold transition-all duration-200 hover:scale-105"
            style={{
              background: 'linear-gradient(135deg, #2B3139 0%, #374151 100%)',
              color: '#848E9C',
            }}
          >
            取消
          </button>
          <button
            type="submit"
            disabled={loading || !newPassword.trim() || newPassword !== confirmPassword}
            className="flex-1 px-4 py-3 rounded-xl text-sm font-semibold transition-all duration-200 hover:scale-105 disabled:opacity-50 flex items-center justify-center gap-2"
            style={{
              background: loading || !newPassword.trim() || newPassword !== confirmPassword
                ? 'linear-gradient(135deg, #4B5563 0%, #6B7280 100%)'
                : 'linear-gradient(135deg, #10B981 0%, #34D399 100%)',
              color: '#000',
            }}
          >
            {loading ? (
              <>
                <div className="w-4 h-4 border-2 border-black border-t-transparent rounded-full animate-spin"></div>
                保存中...
              </>
            ) : (
              '保存'
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
  groupLeader,
  categoryAccounts,
  onViewAccount,
  onClose,
}: {
  category: Category
  groupLeader?: {
    id: string
    email: string
    role: string
    categories: string[]
    trader_count: number
    created_at: string
  }
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
    ...(groupLeader ? [{ ...groupLeader, type: 'group_leader' as const }] : []),
    ...categoryAccounts.map(ca => ({ ...ca, type: ca.role as 'trader_account' | 'group_leader' }))
  ]

  return (
    <ModernModal
      isOpen={true}
      onClose={onClose}
      title={`${category.name} - 账号信息`}
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
            小组组长: {groupLeader ? 1 : 0}个 | 交易员账号: {categoryAccounts.length}个
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
              暂无账号信息
            </div>
            <div className="text-sm" style={{ color: '#848E9C' }}>
              该分类下还没有创建小组组长或交易员账号
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

