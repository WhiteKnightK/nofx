package gmail

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"nofx/config"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	"github.com/emersion/go-message/charset"
	"github.com/emersion/go-message/mail"
	"golang.org/x/text/encoding/simplifiedchinese"
)

func init() {
	// 【新增】注册中文编码处理器，解决 "unhandled charset gb2312" 问题
	// gb2312, gbk 统一使用 gb18030 编码器处理
	charset.RegisterEncoding("gb2312", simplifiedchinese.GB18030)
	charset.RegisterEncoding("gbk", simplifiedchinese.GB18030)
	charset.RegisterEncoding("gb18030", simplifiedchinese.GB18030)
}

// Monitor Gmail监听器
type Monitor struct {
	config     *config.GmailConfig
	stopChan   chan struct{}
	SignalChan chan *Email // 用于发送邮件内容的通道（包含正文和时间等元信息）
	lastCheck  time.Time

	// 【新增】已处理指纹缓存 (Subject + Date)
	processedCache map[string]bool
	mu             sync.Mutex
}

// Email 封装策略邮件的关键信息（正文 + 元数据）
type Email struct {
	Body      string
	Subject   string
	From      string
	Date      time.Time
	MessageID string
}

// NewMonitor 创建新的监听器
func NewMonitor(cfg *config.GmailConfig) *Monitor {
	return &Monitor{
		config:         cfg,
		stopChan:       make(chan struct{}),
		SignalChan:     make(chan *Email, 10), // 缓冲10条
		processedCache: make(map[string]bool),
	}
}

// Start 启动监听
func (m *Monitor) Start() {
	if !m.config.Enabled {
		log.Println("📭 Gmail监听未启用")
		return
	}

	log.Printf("📧 启动Gmail监听: %s", m.config.User)
	go m.loop()
}

// Stop 停止监听
func (m *Monitor) Stop() {
	close(m.stopChan)
}

func (m *Monitor) loop() {
	// 【说明】将轮询间隔从 1 分钟缩短到 20 秒，加快策略检测速度
	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()

	// 首次立即检查
	if err := m.CheckEmails(); err != nil {
		log.Printf("❌ Gmail检查失败: %v", err)
	}

	for {
		select {
		case <-ticker.C:
			if err := m.CheckEmails(); err != nil {
				log.Printf("❌ Gmail检查失败: %v", err)
			}
		case <-m.stopChan:
			return
		}
	}
}

// CheckEmails 连接IMAP并检查邮件
func (m *Monitor) CheckEmails() error {
	// 连接到服务器
	c, err := client.DialTLS(fmt.Sprintf("%s:%d", m.config.Host, m.config.Port), nil)
	if err != nil {
		return fmt.Errorf("连接IMAP失败: %w", err)
	}
	defer c.Logout()

	// 登录
	if err := c.Login(m.config.User, m.config.Password); err != nil {
		return fmt.Errorf("登录失败: %w", err)
	}

	// 选择收件箱
	_, err = c.Select("INBOX", false)
	if err != nil {
		return fmt.Errorf("选择收件箱失败: %w", err)
	}

	// 搜索邮件：最近一段时间内的邮件
	criteria := imap.NewSearchCriteria()

	// 首次启动：回溯 48 小时，保证历史有效策略能被一次性扫进来
	// 后续轮询：只从上次检查时间往后扫，避免对旧邮件反复调用 AI，导致“每轮都要几分钟”
	var since time.Time
	if m.lastCheck.IsZero() {
		since = time.Now().Add(-72 * time.Hour)
	} else {
		since = m.lastCheck
	}
	criteria.Since = since
	// 增加搜索条件：标题包含 "Web3团队"
	// 注意：有些 IMAP 服务器对中文搜索支持不一，如果失效可以去掉
	// criteria.Header.Add("Subject", "Web3团队")

	uids, err := c.Search(criteria)
	if err != nil {
		return fmt.Errorf("搜索邮件失败: %w", err)
	}

	if len(uids) == 0 {
		return nil
	}

	// 1. 第一步：只获取信封（标题、发件人、日期），不下载正文
	seqset := new(imap.SeqSet)
	seqset.AddNum(uids...)
	items := []imap.FetchItem{imap.FetchEnvelope, imap.FetchUid}

	messages := make(chan *imap.Message, 50)
	done := make(chan error, 1)
	go func() {
		done <- c.Fetch(seqset, items, messages)
	}()

	targetUids := new(imap.SeqSet)
	uidToEnvelope := make(map[uint32]*imap.Envelope)

	for msg := range messages {
		// 防御性检查：避免底层返回 nil 消息导致后续解引用 panic
		if msg == nil || msg.Envelope == nil {
			log.Printf("⚠️ Received nil message or envelope, skip this record")
			continue
		}

		subject := msg.Envelope.Subject
		fromName := ""
		fromEmail := ""
		if len(msg.Envelope.From) > 0 {
			fromName = msg.Envelope.From[0].PersonalName
			fromEmail = fmt.Sprintf("%s@%s", msg.Envelope.From[0].MailboxName, msg.Envelope.From[0].HostName)
		}

		// 🔍 核心安全过滤逻辑：
		// 1. 检查是否为白名单发送者
		isWhitelisted := false
		if config.GlobalDB != nil {
			whitelisted, err := config.GlobalDB.IsEmailWhitelisted(fromEmail)
			if err == nil && whitelisted {
				isWhitelisted = true
			}
		}

		// 2. 预判是否可能是策略邮件（不论是否白名单，都先初筛标题或发件人名，减少正文下载压力）
		// 注意：正文下载后的“关键词检查”才是最终防线
		isPotentialStrategy := strings.Contains(subject, "Web3团队发布") ||
			strings.Contains(fromName, "Web3团队") ||
			isWhitelisted

		if isPotentialStrategy {
			targetUids.AddNum(msg.Uid)
			log.Printf("targetUids: %v", targetUids)
			uidToEnvelope[msg.Uid] = msg.Envelope
			log.Printf("🎯 发现目标邮件(待下载): [%s] <%s> %s (白名单: %v)", fromName, fromEmail, subject, isWhitelisted)
		}
	}

	if err := <-done; err != nil {
		return fmt.Errorf("获取信封失败: %w", err)
	}

	if targetUids.Empty() {
		return nil
	}

	// 2. 第二步：只针对目标邮件，下载正文（这里使用 UIDFetch，避免 UID/Seq 混用导致漏邮件）
	log.Printf("📥 开始批量下载邮件正文，共 %d 封目标邮件...", len(targetUids.Set))
	section := &imap.BodySectionName{}
	bodyItems := []imap.FetchItem{section.FetchItem(), imap.FetchUid}
	// 增加缓冲大小，防止 fetch 阻塞
	bodyMessages := make(chan *imap.Message, len(targetUids.Set)+10)
	bodyDone := make(chan error, 1)
	go func() {
		bodyDone <- c.UidFetch(targetUids, bodyItems, bodyMessages)
	}()

	processedCount := 0
	for msg := range bodyMessages {
		processedCount++
		envelope := uidToEnvelope[msg.Uid]
		if envelope == nil {
			continue
		}
		if msg == nil {
			log.Printf("⚠️ Received nil body message, skip [index=%d]", processedCount)
			continue
		}

		// 【去重】这里才真正标记“已处理”，确保只有在成功解析正文并投递到通道后，
		// 才会被视为已消费。否则如果在下载/解析阶段出错，就会导致邮件永久丢失。
		// 这里才检查是否已处理过
		fingerprint := fmt.Sprintf("%s|%s", envelope.Subject, envelope.Date.Format(time.RFC3339))
		m.mu.Lock()
		if m.processedCache[fingerprint] {
			m.mu.Unlock()
			fromName := ""
			if len(envelope.From) > 0 {
				fromName = envelope.From[0].PersonalName
			}
			log.Printf("⏭ 跳过重复邮件 [%d/%d]: %s (接收时间: %s, 发布者: %s)",
				processedCount, len(targetUids.Set), envelope.Subject, envelope.Date.Format(time.RFC3339), fromName)
			continue
		}
		m.mu.Unlock()

		// 解析正文
		log.Printf("📥 [%d/%d] 正在下载/解析邮件正文... [UID: %d] %s", processedCount, len(targetUids.Set), msg.Uid, envelope.Subject)
		r := msg.GetBody(section)
		if r == nil {
			log.Printf("⚠️ 获取邮件 Body 失败: [UID: %d] %s", msg.Uid, envelope.Subject)
			continue
		}

		mr, err := mail.CreateReader(r)
		if err != nil {
			log.Printf("❌ 解析邮件结构失败: [UID: %d] %s, err: %v", msg.Uid, envelope.Subject, err)
			continue
		}

		body := ""
		partCount := 0
		for {
			p, err := mr.NextPart()
			if err == io.EOF {
				log.Printf("🏁 邮件 Part 读取完毕 [UID: %d] %s, 共 %d 个 Part", msg.Uid, envelope.Subject, partCount)
				break
			} else if err != nil {
				log.Printf("⚠️ 读取邮件 Part 失败: [UID: %d] %s, err: %v", msg.Uid, envelope.Subject, err)
				break
			}
			partCount++

			switch h := p.Header.(type) {
			case *mail.InlineHeader:
				b, _ := ioutil.ReadAll(p.Body)
				contentType := h.Get("Content-Type")
				log.Printf("🔍 发现邮件 Part: %s (长度: %d)", contentType, len(b))
				if strings.Contains(contentType, "text/plain") {
					body = string(b)
				} else if strings.Contains(contentType, "text/html") && body == "" {
					body = string(b)
				}
			}
		}

		if body != "" {
			// 🔒 最终安全校验：检查关键字或密钥
			fromEmail := ""
			if len(envelope.From) > 0 {
				fromEmail = fmt.Sprintf("%s@%s", envelope.From[0].MailboxName, envelope.From[0].HostName)
			}

			isWhitelisted := false
			if config.GlobalDB != nil {
				whitelisted, _ := config.GlobalDB.IsEmailWhitelisted(fromEmail)
				isWhitelisted = whitelisted
			}

			hasKeywords := strings.Contains(body, "入场价格") ||
				strings.Contains(body, "补仓价格") ||
				strings.Contains(body, "做空") ||
				strings.Contains(body, "做多")

			secretHash := os.Getenv("STRATEGY_SECRET_HASH")
			hasSecret := secretHash != "" && strings.Contains(body, secretHash)

			isValid := false
			if isWhitelisted && hasKeywords {
				isValid = true
			} else if hasSecret && hasKeywords {
				isValid = true
				log.Printf("🔑 发现有效密钥策略邮件: %s", envelope.Subject)
			}

			if !isValid {
				log.Printf("🛡️ 拦截无效或未经授权的策略邮件: %s (白名单: %v, 关键字: %v, 密钥: %v)",
					envelope.Subject, isWhitelisted, hasKeywords, hasSecret)
				// 即使无效也标记为已处理，防止下一轮反复扫描无效邮件
				m.mu.Lock()
				m.processedCache[fingerprint] = true
				m.mu.Unlock()
				continue
			}

			log.Printf("✅ 成功提取正文 (长度: %d) -> 推送到解析队列: %s", len(body), envelope.Subject)
			// 只有在真正拿到正文并成功构造 Email 之后，才标记为已处理
			m.mu.Lock()
			m.processedCache[fingerprint] = true
			m.mu.Unlock()

			// 发送到通道
			fromName := ""
			if len(envelope.From) > 0 {
				fromName = envelope.From[0].PersonalName
			}
			email := &Email{
				Body:      body,
				Subject:   envelope.Subject,
				From:      fromName,
				Date:      envelope.Date, // 使用邮件原始接收时间
				MessageID: envelope.MessageId,
			}
			if email.MessageID == "" {
				email.MessageID = fmt.Sprintf("uid_%d", msg.Uid)
			}
			m.SignalChan <- email

			// 标记为已读
			uSet := new(imap.SeqSet)
			uSet.AddNum(msg.Uid)
			item := imap.FormatFlagsOp(imap.AddFlags, true)
			flags := []interface{}{imap.SeenFlag}
			if err := c.UidStore(uSet, item, flags, nil); err != nil {
				log.Printf("标记已读失败: %v", err)
			}
		} else {
			log.Printf("❌ 邮件正文提取为空 [UID: %d] %s", msg.Uid, envelope.Subject)
		}
	}

	if err := <-bodyDone; err != nil {
		log.Printf("❌ UidFetch 最终返回错误: %v", err)
		return err
	}

	log.Printf("✨ 本轮 Gmail 扫描完成")
	// 本轮扫描完成，记录“最后检查时间”，下一轮只处理之后的新邮件
	m.lastCheck = time.Now()
	return nil
}
