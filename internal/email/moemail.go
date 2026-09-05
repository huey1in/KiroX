package email

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// MoeMailConfig MoeMail 配置
type MoeMailConfig struct {
	Name   string `json:"name"`   // 配置名称（用户自定义）
	URL    string `json:"url"`    // API 基础 URL
	APIKey string `json:"apiKey"` // API Key
}

// MoeMailClient MoeMail API 客户端
type MoeMailClient struct {
	ctx    context.Context
	config MoeMailConfig
	client *http.Client
}

// MoeMailSystemConfig 系统配置响应
type MoeMailSystemConfig struct {
	EmailDomains string   `json:"emailDomains"` // 可用域名（逗号分隔字符串）
	Domains      []string // 解析后的域名列表（不参与 JSON）
}

// MoeMailEmail 邮箱信息
type MoeMailEmail struct {
	ID         string `json:"id"`
	Email      string `json:"email"`      // 完整邮箱地址
	Address    string `json:"address"`    // 兼容旧格式
	Name       string `json:"name"`       // 邮箱名称（@前面部分）
	Domain     string `json:"domain"`     // 域名
	ExpiryTime int64  `json:"expiryTime"` // 毫秒
	CreatedAt  int64  `json:"createdAt"`
}

// MoeMailMessage 邮件信息
type MoeMailMessage struct {
	ID          string `json:"id"`
	FromAddress string `json:"from_address"`
	ToAddress   string `json:"to_address"`
	Subject     string `json:"subject"`
	Content     string `json:"content"` // 纯文本内容
	HTML        string `json:"html"`
	CreatedAt   string `json:"createdAt"`
}

// MoeMailMessagesResponse 邮件列表响应
type MoeMailMessagesResponse struct {
	Messages   []MoeMailMessage `json:"messages"`
	NextCursor string           `json:"nextCursor"`
}

// NewMoeMailClient 创建 MoeMail 客户端
func NewMoeMailClient(config MoeMailConfig) *MoeMailClient {
	return newMoeMailClient(context.Background(), config)
}

func newMoeMailClient(ctx context.Context, config MoeMailConfig) *MoeMailClient {
	return &MoeMailClient{
		ctx:    ctx,
		config: config,
		client: &http.Client{Timeout: 15 * time.Second},
	}
}

// request 发送 HTTP 请求
func (c *MoeMailClient) request(method, path string, body io.Reader) (*http.Response, error) {
	url := strings.TrimRight(c.config.URL, "/") + path
	req, err := http.NewRequestWithContext(c.ctx, method, url, body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("X-API-Key", c.config.APIKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	return c.client.Do(req)
}

// GetSystemConfig 获取系统配置
func (c *MoeMailClient) GetSystemConfig() (*MoeMailSystemConfig, error) {
	resp, err := c.request("GET", "/api/config", nil)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("获取配置失败 %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	var config MoeMailSystemConfig
	if err := json.Unmarshal(body, &config); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w, 响应内容: %s", err, string(body))
	}

	// 从 emailDomains 字符串解析域名列表
	if config.EmailDomains != "" {
		for _, domain := range strings.Split(config.EmailDomains, ",") {
			domain = strings.TrimSpace(domain)
			if domain != "" {
				config.Domains = append(config.Domains, domain)
			}
		}
	}

	if len(config.Domains) == 0 {
		return nil, fmt.Errorf("API 未返回可用域名")
	}

	return &config, nil
}

// GenerateEmail 生成临时邮箱
func (c *MoeMailClient) GenerateEmail(name string, expiryTime int64, domain string) (*MoeMailEmail, error) {
	reqData := map[string]interface{}{
		"name":       name,
		"expiryTime": expiryTime,
		"domain":     domain,
	}
	reqJSON, _ := json.Marshal(reqData)

	resp, err := c.request("POST", "/api/emails/generate", strings.NewReader(string(reqJSON)))
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("生成邮箱失败 %d: %s", resp.StatusCode, string(body))
	}

	var email MoeMailEmail
	if err := json.NewDecoder(resp.Body).Decode(&email); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	// 兼容两种响应格式
	if email.Email != "" && email.Address == "" {
		email.Address = email.Email
	}

	return &email, nil
}

// GetMessages 获取邮件列表
func (c *MoeMailClient) GetMessages(emailID, cursor string) (*MoeMailMessagesResponse, error) {
	path := fmt.Sprintf("/api/emails/%s", emailID)
	if cursor != "" {
		path += "?cursor=" + cursor
	}

	resp, err := c.request("GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("获取邮件列表失败 %d: %s", resp.StatusCode, string(body))
	}

	var result MoeMailMessagesResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	return &result, nil
}

// MoeMailProvider 实现 EmailProvider 接口
type MoeMailProvider struct {
	client              *MoeMailClient
	emailID             string
	address             string
	initialMessageCount int // 创建时的邮件数量
}

// GenerateEmailName 生成随机邮箱名
func GenerateEmailName(taskIndex int) string {
	// 使用纳秒时间戳 + 任务序号作为随机种子
	seed := time.Now().UnixNano() + int64(taskIndex)*1000000
	if seed < 0 {
		seed = -seed
	}

	// 生成 10-15 位随机字母数字组合
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	length := 10 + int(seed%6) // 10-15 位

	name := make([]byte, length)
	for i := range name {
		seed = seed*1103515245 + 12345 // 线性同余生成器
		if seed < 0 {
			seed = -seed
		}
		name[i] = charset[seed%int64(len(charset))]
	}

	return string(name)
}

// NewMoeMailProvider 创建 MoeMail 提供商
func NewMoeMailProvider(config MoeMailConfig, name string, expiryTime int64, domain string) (*MoeMailProvider, error) {
	return NewMoeMailProviderContext(context.Background(), config, name, expiryTime, domain)
}

func NewMoeMailProviderContext(ctx context.Context, config MoeMailConfig, name string, expiryTime int64, domain string) (*MoeMailProvider, error) {
	client := newMoeMailClient(ctx, config)

	// 实时获取可用域名列表，验证域名是否可用
	sysConfig, err := client.GetSystemConfig()
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if err != nil {
		log.Printf("[MoeMail] 警告：无法获取系统配置: %v，尝试直接使用域名", err)
	} else {
		// 验证域名是否在可用列表中
		domainValid := false
		for _, availableDomain := range sysConfig.Domains {
			if availableDomain == domain {
				domainValid = true
				break
			}
		}

		if !domainValid {
			// 域名不可用，尝试使用第一个可用域名
			if len(sysConfig.Domains) > 0 {
				oldDomain := domain
				domain = sysConfig.Domains[0]
				log.Printf("[MoeMail] 域名 %s 不可用，自动切换到 %s", oldDomain, domain)
			} else {
				return nil, fmt.Errorf("域名 %s 不在可用列表中，且没有其他可用域名", domain)
			}
		}
	}

	// 生成临时邮箱
	email, err := client.GenerateEmail(name, expiryTime, domain)
	if err != nil {
		return nil, fmt.Errorf("生成邮箱失败: %w", err)
	}

	// 立即记录初始邮件数量
	initialCount := 0
	initialMessages, err := client.GetMessages(email.ID, "")
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if err != nil {
		log.Printf("[MoeMail] 获取初始邮件列表失败: %v，假设为0", err)
	} else if initialMessages != nil {
		initialCount = len(initialMessages.Messages)
	}
	log.Printf("[MoeMail] 邮箱创建完成: %s，初始邮件数: %d", email.Address, initialCount)

	return &MoeMailProvider{
		client:              client,
		emailID:             email.ID,
		address:             email.Address,
		initialMessageCount: initialCount,
	}, nil
}

// GetAddress 返回邮箱地址
func (p *MoeMailProvider) GetAddress() string {
	return p.address
}

// WaitForCode 轮询等待验证码
func (p *MoeMailProvider) WaitForCode(timeout, interval int) (string, error) {
	ctx := p.client.ctx
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if interval <= 0 {
		interval = 3
	}
	maxRetries := timeout / interval
	codeRegex := regexp.MustCompile(`\b(\d{6})\b`)

	// 使用创建时记录的初始邮件数量
	beforeCount := p.initialMessageCount
	log.Printf("[MoeMail] 开始等待验证码，初始邮件数: %d", beforeCount)

	for attempt := 1; attempt <= maxRetries; attempt++ {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		// 获取邮件列表
		messages, err := p.client.GetMessages(p.emailID, "")
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		if err != nil {
			if attempt%5 == 0 {
				log.Printf("[MoeMail] 获取邮件失败: %v, 重试中...", err)
			}
			if err := waitEmailPoll(ctx, time.Duration(interval)*time.Second); err != nil {
				return "", err
			}
			continue
		}

		currentCount := len(messages.Messages)
		if currentCount <= beforeCount {
			if attempt%5 == 0 {
				log.Printf("[MoeMail] [%d/%d] 暂无新邮件 (当前%d封)...", attempt, maxRetries, currentCount)
			}
			if err := waitEmailPoll(ctx, time.Duration(interval)*time.Second); err != nil {
				return "", err
			}
			continue
		}

		// 只检查新邮件（按时间倒序，最新的在前面）
		newMessageCount := currentCount - beforeCount
		log.Printf("[MoeMail] 检测到 %d 封新邮件", newMessageCount)

		for i := 0; i < newMessageCount && i < len(messages.Messages); i++ {
			msg := messages.Messages[i]

			// 优先从文本内容提取
			if code := extractCodeFromText(msg.Content, codeRegex); code != "" {
				log.Printf("[MoeMail] 从新邮件中获取到验证码: %s", code)
				return code, nil
			}

			// 从 HTML 内容提取
			if code := extractCodeFromText(msg.HTML, codeRegex); code != "" {
				log.Printf("[MoeMail] 从新邮件中获取到验证码: %s", code)
				return code, nil
			}

			// 从主题提取
			if code := extractCodeFromText(msg.Subject, codeRegex); code != "" {
				log.Printf("[MoeMail] 从新邮件中获取到验证码: %s", code)
				return code, nil
			}
		}

		if attempt%5 == 0 {
			log.Printf("[MoeMail] [%d/%d] 新邮件中未找到验证码...", attempt, maxRetries)
		}
		if err := waitEmailPoll(ctx, time.Duration(interval)*time.Second); err != nil {
			return "", err
		}
	}

	return "", fmt.Errorf("等待验证码超时 (%ds)", timeout)
}

// extractCodeFromText 从文本中提取验证码
func extractCodeFromText(text string, regex *regexp.Regexp) string {
	matches := regex.FindStringSubmatch(text)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// ExtractCode 从邮件文本中提取 AWS 验证码 (含黑名单过滤)。
func ExtractCode(text string) string {
	patterns := []string{
		`(?i)(?:verification code|code|验证码)[：:\s]+(\d{6})`,
		`(\d{6})\s*(?:is your|为您的)`,
	}
	for _, p := range patterns {
		re := regexp.MustCompile(p)
		m := re.FindStringSubmatch(text)
		if len(m) == 2 && !isBlacklisted(m[1]) {
			return m[1]
		}
	}
	re := regexp.MustCompile(`\b(\d{6})\b`)
	m := re.FindStringSubmatch(text)
	if len(m) == 2 && !isBlacklisted(m[1]) {
		return m[1]
	}
	return ""
}

func isBlacklisted(code string) bool {
	return code == "000000" || code == "111111" || code == "123456"
}
