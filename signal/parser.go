package signal

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"

	"nofx/mcp"
)

type Parser struct {
	mcpClient *mcp.Client
	prompt    string
}

func NewParser(client *mcp.Client) (*Parser, error) {
	// 读取 Prompt 模板
	content, err := ioutil.ReadFile("prompts/signal_parser.txt")
	if err != nil {
		return nil, fmt.Errorf("读取Prompt模板失败: %w", err)
	}

	return &Parser{
		mcpClient: client,
		prompt:    string(content),
	}, nil
}

func (p *Parser) Parse(emailContent string) (*SignalDecision, error) {
	// 🛑 关键修复：检查并补救 AI Key 丢失问题
	// 有时候全局初始化可能因为某些原因未能正确设置 Key，这里做最后一道防线
	if p.mcpClient.APIKey == "" {
		log.Println("⚠️ [Parser] 检测到 AI Key 为空，尝试从环境变量重新加载...")
		
		deepSeekKey := os.Getenv("DEEPSEEK_API_KEY")
		if deepSeekKey != "" {
			p.mcpClient.SetDeepSeekAPIKey(deepSeekKey, "", "")
			log.Printf("🔧 [Parser] 已恢复 DeepSeek Key (长度: %d)", len(deepSeekKey))
		} else {
			qwenKey := os.Getenv("QWEN_API_KEY")
			if qwenKey != "" {
				p.mcpClient.SetQwenAPIKey(qwenKey, "", "")
				log.Printf("🔧 [Parser] 已恢复 Qwen Key (长度: %d)", len(qwenKey))
			} else {
				log.Println("❌ [Parser] 环境变量中也未找到 AI Key (DEEPSEEK_API_KEY 或 QWEN_API_KEY)")
			}
		}
	}

	// 替换内容
	prompt := strings.Replace(p.prompt, "{{EMAIL_CONTENT}}", emailContent, 1)

	// 调用 AI
	// 注意：这里我们将整个 Prompt 作为 System Prompt 发送，或者 User Prompt
	// 既然是任务型指令，作为 System Prompt 更合适，或者直接作为单次对话
	
	systemPrompt := "你是一个严格的JSON解析助手。"
	userPrompt := prompt

	resp, err := p.mcpClient.CallWithMessages(systemPrompt, userPrompt)
	if err != nil {
		return nil, fmt.Errorf("AI调用失败: %w", err)
	}

	// 清洗 Markdown 代码块标记
	cleanJSON := strings.TrimSpace(resp)
	cleanJSON = strings.TrimPrefix(cleanJSON, "```json")
	cleanJSON = strings.TrimPrefix(cleanJSON, "```")
	cleanJSON = strings.TrimSuffix(cleanJSON, "```")
	cleanJSON = strings.TrimSpace(cleanJSON)

	// 反序列化
	var decision SignalDecision
	if err := json.Unmarshal([]byte(cleanJSON), &decision); err != nil {
		log.Printf("解析失败的JSON: %s", cleanJSON)
		return nil, fmt.Errorf("JSON解析失败: %w", err)
	}

	// 简单的验证
	if decision.Symbol == "" || decision.Direction == "" {
		return nil, fmt.Errorf("解析结果缺失关键信息(Symbol/Direction)")
	}

	// 保存原始邮件内容用于前端展示
	decision.RawContent = emailContent

	return &decision, nil
}


