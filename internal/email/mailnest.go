package email

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

// MailNestResponse 统一响应类型
type MailNestResponse[T any] struct {
	Code string `json:"code"`
	Msg  string `json:"msg"`
	Type string `json:"type"`
	Data T      `json:"data"`
}

// MailNestClient MailNest API 客户端
type MailNestClient struct {
	ctx    context.Context
	config MailNestConfig
	client *http.Client
}

// MailNestConfig MailNest 配置
type MailNestConfig struct {
	APIKey      string `json:"apiKey"`      // API Key
	ProjectCode string `json:"projectCode"` // projectCode
}

// MailNestProvider 实现 EmailProvider 接口
type MailNestProvider struct {
	client  *MailNestClient
	address string
}

// MailNestBalanceData MailNest 余额
type MailNestBalanceData struct {
	Balance          string `json:"balance"`
	FrozenBalance    string `json:"frozen_balance"`
	AvailableBalance string `json:"available_balance"`
}

// MailNestEmailData MailNest 余额
type MailNestEmailData struct {
	Email string `json:"email"`
}

// MailNestMailData MailNest 余额
type MailNestMailData struct {
	CodeMatch string `json:"code_match"`
}

// NewMailNestClient 创建 MailNest 客户端
func NewMailNestClient(config MailNestConfig) *MailNestClient {
	return newMailNestClient(context.Background(), config)
}

func newMailNestClient(ctx context.Context, config MailNestConfig) *MailNestClient {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}
	return &MailNestClient{
		ctx:    ctx,
		config: config,
		client: &http.Client{
			Timeout:   15 * time.Second,
			Transport: tr,
		},
	}
}

// NewMailNestProvider 创建 MailNest
func NewMailNestProvider(config MailNestConfig) *MailNestProvider {
	return NewMailNestProviderContext(context.Background(), config)
}

func NewMailNestProviderContext(ctx context.Context, config MailNestConfig) *MailNestProvider {
	return NewMailNestProviderContextWithProxy(ctx, config, "")
}

func NewMailNestProviderContextWithProxy(ctx context.Context, config MailNestConfig, proxyURL string) *MailNestProvider {
	client := newMailNestClient(ctx, config)
	if proxyURL != "" {
		client.client = httpClientWithProxy(proxyURL, 15*time.Second)
	}
	return &MailNestProvider{client: client}
}

// request 发送 HTTP 请求
func (c *MailNestClient) request(method, url string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(c.ctx, method, url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.config.APIKey)
	req.Header.Set("Content-Type", "application/json")
	return c.client.Do(req)
}

// GetBalance 获取余额
func (c *MailNestClient) GetBalance() (*MailNestBalanceData, error) {
	resp, err := c.request("GET", "https://mailnest.top/api/v1/balance", nil)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("获取余额失败 %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	var balance MailNestResponse[MailNestBalanceData]
	if err := json.Unmarshal(body, &balance); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w, 响应内容: %s", err, string(body))
	}
	return &balance.Data, nil
}

func (c *MailNestClient) EmailList(address string) ([]MailNestMailData, error) {
	reqData := map[string]interface{}{
		"email": address,
	}
	reqJSON, _ := json.Marshal(reqData)
	resp, err := c.request("POST", "https://mailnest.top/api/v1/email/receive", strings.NewReader(string(reqJSON)))
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("获取邮件 %d: %s", resp.StatusCode, string(body))
	}

	var mails MailNestResponse[[]MailNestMailData]
	if err := json.NewDecoder(resp.Body).Decode(&mails); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}
	if mails.Code != "00000" {
		return nil, fmt.Errorf("没有获取到: %w", err)
	}
	return mails.Data, nil

}

// WaitForCode 轮询等待 6 位数字验证码
func (p *MailNestProvider) WaitForCode(timeout, interval int) (string, error) {
	if p == nil || p.client == nil {
		return "", fmt.Errorf("MailNestProvider 未初始化")
	}
	ctx := p.client.ctx
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if interval <= 0 {
		interval = 3
	}
	if timeout <= 0 {
		timeout = interval
	}
	maxRetries := timeout / interval
	if maxRetries < 1 {
		maxRetries = 1
	}
	log.Printf("[MailNest] 开始等待验证码 %s", p.address)
	for attempt := 1; attempt <= maxRetries; attempt++ {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		mails, err := p.client.EmailList(p.address)
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		if err != nil {
			if attempt%5 == 0 {
				log.Printf("[MailNest] 获取邮件失败: %v，重试中...", err)
			}
			if err := waitEmailPoll(ctx, time.Duration(interval)*time.Second); err != nil {
				return "", err
			}
			continue
		}
		for _, m := range mails {
			if code := m.CodeMatch; len(code) == 6 {
				log.Printf("[MailNest] 从新邮件中获取到验证码: %s", code)
				return code, nil
			}
		}
		if attempt%5 == 0 {
			log.Printf("[MailNest] [%d/%d] 暂无新邮件...", attempt, maxRetries)
		}
		if err := waitEmailPoll(ctx, time.Duration(interval)*time.Second); err != nil {
			return "", err
		}
	}
	return "", fmt.Errorf("等待验证码超时 (%ds)", timeout)
}

func (p *MailNestProvider) GetAddress() (string, error) {
	if p == nil || p.client == nil {
		return "", fmt.Errorf("MailNestProvider 未初始化")
	}
	if p.address != "" {
		return p.address, nil
	}

	reqData := map[string]interface{}{
		"count":        1,
		"project_code": p.client.config.ProjectCode,
	}
	reqJSON, _ := json.Marshal(reqData)

	resp, err := p.client.request("POST", "https://mailnest.top/api/v1/email/temporary/buy", strings.NewReader(string(reqJSON)))
	if err != nil {
		return "", fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("生成邮箱失败 %d: %s", resp.StatusCode, string(body))
	}

	var emails MailNestResponse[[]MailNestEmailData]
	if err := json.NewDecoder(resp.Body).Decode(&emails); err != nil {
		return "", fmt.Errorf("解析响应失败: %w", err)
	}
	if emails.Code != "00000" {
		return "", fmt.Errorf("没有获取到: %w", err)
	}
	if len(emails.Data) == 0 {
		return "", fmt.Errorf("MailNest 未返回邮箱地址")
	}
	p.address = emails.Data[0].Email
	return p.address, nil
}
