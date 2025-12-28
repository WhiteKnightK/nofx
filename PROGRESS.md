# 项目改造进度：信号跟随与 AI 执行系统

## 1. 核心目标
将原有的“AI 自主决策交易系统”改造成“**策略信号跟随 + AI 辅助执行系统**”。
- **信号源**：Web3 团队邮件（通过 IMAP 读取）。
- **决策大脑**：AI 解析邮件生成结构化策略（JSON）和原始分析报告。
- **执行手脚**：每个 Trader 独立运行，定期调用 AI（Strategy Executor）结合当前技术指标（RSI/MACD）判断是否满足策略条件（如补仓、止盈）。

## 2. 已完成工作 (Completed)

### 后端 (Backend)
- **Gmail 监听模块**：实现了 IMAP 协议监听，自动读取最近 24 小时的策略邮件。
- **信号解析器 (Signal Parser)**：
    - 集成 LLM (DeepSeek/Qwen) 解析邮件全文。
    - 生成标准 JSON 策略结构 (`SignalDecision`)。
    - **新增**：保存邮件原始文本 (`RawContent`) 用于前端展示。
- **全局策略管理 (Global Manager)**：
    - 单例模式，维护当前唯一的活跃策略。
    - 暴露 API (`GET /strategy/active`) 供前端获取。
- **数据库改造**：
    - 新增表 `trader_strategy_status`，记录每个 Trader 的独立执行状态（Entry Price, Status, PnL）。
- **执行逻辑改造**：
    - 实现了 `market.CalculateRSI` 和 `CalculateMACD` (纯 Go 实现，无 CGO)。
    - 实现了 `AutoTrader.CheckAndExecuteStrategyWithAI`：
        - 拉取 1h/4h K 线。
        - 计算指标。
        - 调用 AI (Prompt: `strategy_executor.txt`) 进行二次确认。
        - 执行交易并更新数据库状态。
    - 替换了原有的 `RunSignalMode` 循环，频率调整为 **1分钟**。

### 前端 (Frontend)
- **全局策略看板 (`StrategyStatusCard`)**：
    - 位于 Dashboard 顶部。
    - 可视化展示当前策略（方向、杠杆）。
    - 进度条展示（SL --- Entry --- TP）。
    - **新增**：支持点击查看完整策略分析报告（Markdown 渲染）。
- **配置页清理**：
    - 移除了“系统提示词模板”选择（不再需要）。
    - 保留并优化了“附加提示词”说明（用于特定账户的微调）。
- **列表页优化**：
    - Trader 状态显示改为 `📡 Following Global Strategy`。

## 3. 待办事项 (Pending / Next Steps)

### 前端 (Frontend)
- **Trader 独立状态卡片 (`TraderExecutionCard`)**：
    - 在 Trader 详情页（点击列表进入后的页面）添加一个新卡片。
    - 调用 `GET /traders/:id/strategy-status`。
    - 展示该账户的：实际入场价、当前阶段（等待补仓/已补仓）、实际盈亏。
    - **目的**：区分“全局策略理论值”和“个人账户实际值”。

### 后端 (Backend)
- **集成测试**：
    - 需要模拟一封邮件发送到邮箱，验证从“解析 -> 存储 -> 前端显示 -> 执行”的全链路。
- **AI Prompt 调优**：
    - `strategy_executor.txt` 目前比较基础，可能需要根据实际运行情况调整 AI 对指标的判断敏感度。

### 系统 (System)
- **多账户测试**：创建多个 Trader，验证它们是否能同时独立执行策略，且状态互不干扰。

## 4. 如何继续
1. **编译运行**：`go build -o nofx_server main.go` 或 `./quick_build_push.sh`。
2. **前端开发**：着手开发 `TraderExecutionCard`。
3. **观察日志**：关注 `🤖 [AI执行]` 开头的日志，查看 AI 是否在按预期判断行情。


