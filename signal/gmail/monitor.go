package gmail

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"strings"
	"time"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	"github.com/emersion/go-message/mail"
	"nofx/config"
)

// Monitor Gmail监听器
type Monitor struct {
	config      *config.GmailConfig
	stopChan    chan struct{}
	SignalChan  chan string // 用于发送邮件内容的通道
	lastCheck   time.Time
}

// NewMonitor 创建新的监听器
func NewMonitor(cfg *config.GmailConfig) *Monitor {
	return &Monitor{
		config:     cfg,
		stopChan:   make(chan struct{}),
		SignalChan: make(chan string, 10), // 缓冲10条
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
	ticker := time.NewTicker(1 * time.Minute) // 每分钟检查一次
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

	// 搜索邮件：最近24小时内的所有邮件（不管是否已读）
	criteria := imap.NewSearchCriteria()
	since := time.Now().Add(-24 * time.Hour)
	criteria.Since = since
	// 注意：不设置 WithoutFlags，这样已读邮件也能被搜索到
	
	uids, err := c.Search(criteria)
	if err != nil {
		return fmt.Errorf("搜索邮件失败: %w", err)
	}

	if len(uids) == 0 {
		return nil
	}

	seqset := new(imap.SeqSet)
	seqset.AddNum(uids...)

	// 获取邮件内容
	section := &imap.BodySectionName{}
	items := []imap.FetchItem{section.FetchItem(), imap.FetchEnvelope}

	messages := make(chan *imap.Message, 10)
	done := make(chan error, 1)
	go func() {
		done <- c.Fetch(seqset, items, messages)
	}()

	for msg := range messages {
		// 简单的过滤器：检查标题或发件人
		subject := msg.Envelope.Subject
		from := ""
		if len(msg.Envelope.From) > 0 {
			from = msg.Envelope.From[0].PersonalName
		}
		
		// 🔍 核心过滤逻辑：匹配 "Web3团队"
		isTarget := false
		if strings.Contains(subject, "Web3团队") || strings.Contains(from, "Web3团队") {
			isTarget = true
		}

		// 调试日志：显示所有收到的未读邮件信息，方便排查
		log.Printf("📩 检查邮件: From=[%s], Subject=[%s], IsTarget=%v", from, subject, isTarget)

		if !isTarget {
			continue
		}

		log.Printf("📨 收到目标邮件: [%s] %s", from, subject)

		// 解析正文
		r := msg.GetBody(section)
		if r == nil {
			continue
		}

		// 使用 go-message 解析邮件体
		mr, err := mail.CreateReader(r)
		if err != nil {
			log.Printf("解析邮件结构失败: %v", err)
			continue
		}

		// 提取正文文本
		body := ""
		for {
			p, err := mr.NextPart()
			if err == io.EOF {
				break
			} else if err != nil {
				log.Printf("读取邮件部分失败: %v", err)
				break
			}

			switch p.Header.(type) {
			case *mail.InlineHeader:
				// 这是正文部分
				b, _ := ioutil.ReadAll(p.Body)
				contentType := p.Header.Get("Content-Type")
				if strings.Contains(contentType, "text/plain") {
					body = string(b)
				} else if strings.Contains(contentType, "text/html") && body == "" {
					// 如果还没有纯文本，先存HTML（实际最好是HTML转Text，这里简化）
					body = string(b) 
				}
			}
		}

		if body != "" {
			// 🔒 二次安检：防止误判 (生活/工作邮件混用时的关键保护)
			// 检查内容指纹：必须包含 "策略名称：Web3团队" 或 "策略分析报告" 这种强特征
			// 只有真正包含策略内容的邮件才会被放行
			hasFingerprint := false
			if strings.Contains(body, "策略名称：Web3团队") || 
			   strings.Contains(body, "策略名称: Web3团队") || // 兼容半角
			   strings.Contains(body, "策略分析报告") {
				hasFingerprint = true
			}

			if !hasFingerprint {
				log.Printf("⚠️ 邮件 [%s] 正文不包含关键策略特征(如'策略分析报告')，跳过解析", subject)
				continue
			}

			// 发送到通道
			m.SignalChan <- body
			
			// 标记为已读
			item := imap.FormatFlagsOp(imap.AddFlags, true)
			flags := []interface{}{imap.SeenFlag}
			if err := c.Store(seqset, item, flags, nil); err != nil {
				log.Printf("标记已读失败: %v", err)
			}
		}
	}

	return <-done
}

