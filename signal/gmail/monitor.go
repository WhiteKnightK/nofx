package gmail

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"strings"
	"sync"
	"time"

	"nofx/config"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	"github.com/emersion/go-message/mail"
)

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
	Body    string
	Subject string
	From    string
	Date    time.Time
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
		subject := msg.Envelope.Subject
		from := ""
		if len(msg.Envelope.From) > 0 {
			from = msg.Envelope.From[0].PersonalName
		}

		// 🔍 核心快速过滤逻辑：标题包含 "Web3团队发布"
		// 只要标题包含关键词，我们就认为它是目标邮件，准备下载正文
		if strings.Contains(subject, "Web3团队发布") || strings.Contains(from, "Web3团队") {
			// 【去重】检查是否已经处理过此标题和日期的邮件
			fingerprint := fmt.Sprintf("%s|%s", subject, msg.Envelope.Date.Format(time.RFC3339))
			m.mu.Lock()
			if m.processedCache[fingerprint] {
				m.mu.Unlock()
				log.Printf("⏭ 跳过重复邮件: %s", subject)
				continue
			}
			m.processedCache[fingerprint] = true
			m.mu.Unlock()

			targetUids.AddNum(msg.Uid)
			uidToEnvelope[msg.Uid] = msg.Envelope
			log.Printf("🎯 发现目标邮件(待下载): [%s] %s", from, subject)
		}
	}

	if err := <-done; err != nil {
		return fmt.Errorf("获取信封失败: %w", err)
	}

	if targetUids.Empty() {
		return nil
	}

	// 2. 第二步：只针对目标邮件，下载正文（这里使用 UIDFetch，避免 UID/Seq 混用导致漏邮件）
	section := &imap.BodySectionName{}
	bodyItems := []imap.FetchItem{section.FetchItem(), imap.FetchUid}
	bodyMessages := make(chan *imap.Message, 10)
	bodyDone := make(chan error, 1)
	go func() {
		bodyDone <- c.UidFetch(targetUids, bodyItems, bodyMessages)
	}()

	for msg := range bodyMessages {
		envelope := uidToEnvelope[msg.Uid]
		if envelope == nil {
			continue
		}

		// 解析正文
		r := msg.GetBody(section)
		if r == nil {
			continue
		}

		mr, err := mail.CreateReader(r)
		if err != nil {
			log.Printf("解析邮件结构失败: %v", err)
			continue
		}

		body := ""
		for {
			p, err := mr.NextPart()
			if err == io.EOF {
				break
			} else if err != nil {
				break
			}

			switch p.Header.(type) {
			case *mail.InlineHeader:
				b, _ := ioutil.ReadAll(p.Body)
				contentType := p.Header.Get("Content-Type")
				if strings.Contains(contentType, "text/plain") {
					body = string(b)
				} else if strings.Contains(contentType, "text/html") && body == "" {
					body = string(b)
				}
			}
		}

		if body != "" {
			// 发送到通道
			email := &Email{
				Body:    body,
				Subject: envelope.Subject,
				From:    envelope.From[0].PersonalName,
				Date:    envelope.Date,
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
		}
	}

	if err := <-bodyDone; err != nil {
		return err
	}

	// 本轮扫描完成，记录“最后检查时间”，下一轮只处理之后的新邮件
	m.lastCheck = time.Now()
	return nil
}
