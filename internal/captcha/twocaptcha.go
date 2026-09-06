package captcha

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const defaultAPIBase = "https://api.2captcha.com"

type AWSWAFOptions struct {
	WebsiteURL      string
	WebsiteKey      string
	IV              string
	Context         string
	JSAPIScript     string
	ChallengeScript string
	CaptchaScript   string
	Proxy           string
}

type Client struct {
	APIKey       string
	APIBase      string
	HTTPClient   *http.Client
	PollInterval time.Duration
}

type AWSWAFSolution struct {
	ExistingToken  string
	CaptchaVoucher string
}

// APIError identifies a failed solver request without retaining its payload.
type APIError struct {
	ErrorID     int
	Code        string
	Description string
	Operation   string
	TaskType    string
	TaskID      int64
}

func (e *APIError) Error() string {
	metadata := []string{fmt.Sprintf("errorId=%d", e.ErrorID)}
	if e.Code != "" {
		metadata = append(metadata, e.Code)
	}
	metadata = append(metadata, "operation="+e.Operation, "type="+e.TaskType)
	if e.TaskID > 0 {
		metadata = append(metadata, fmt.Sprintf("taskId=%d", e.TaskID))
	}
	detail := e.Description
	if detail == "" {
		detail = "打码服务返回错误"
	}
	return fmt.Sprintf("2Captcha: %s (%s)", detail, strings.Join(metadata, ", "))
}

func NewClient(apiKey string) *Client {
	return &Client{
		APIKey:       strings.TrimSpace(apiKey),
		APIBase:      defaultAPIBase,
		HTTPClient:   &http.Client{Timeout: 30 * time.Second},
		PollInterval: 5 * time.Second,
	}
}

type apiResponse struct {
	ErrorID          int    `json:"errorId"`
	ErrorCode        string `json:"errorCode"`
	ErrorDescription string `json:"errorDescription"`
	TaskID           int64  `json:"taskId"`
	Status           string `json:"status"`
	Solution         struct {
		ExistingToken  string `json:"existing_token"`
		CaptchaVoucher string `json:"captcha_voucher"`
		Text           string `json:"text"`
	} `json:"solution"`
}

func (c *Client) SolveImageToText(ctx context.Context, image []byte) (string, error) {
	if c == nil || strings.TrimSpace(c.APIKey) == "" {
		return "", fmt.Errorf("2Captcha API Key 未配置")
	}
	if len(image) == 0 {
		return "", fmt.Errorf("AMCS 验证图片为空")
	}
	const taskType = "ImageToTextTask"
	created, err := c.post(ctx, "/createTask", map[string]interface{}{
		"clientKey": c.APIKey,
		"task": map[string]interface{}{
			"type":      taskType,
			"body":      base64.StdEncoding.EncodeToString(image),
			"phrase":    false,
			"case":      true,
			"numeric":   0,
			"math":      false,
			"minLength": 1,
			"maxLength": 127,
		},
	}, taskType, 0)
	if err != nil {
		return "", err
	}
	if created.TaskID == 0 {
		return "", fmt.Errorf("2Captcha 未返回 taskId")
	}
	log.Printf("[WAF] 2Captcha AMCS 图片任务已创建: taskId=%d", created.TaskID)

	interval := c.PollInterval
	if interval <= 0 {
		interval = 5 * time.Second
	}
	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(interval):
		}
		result, err := c.post(ctx, "/getTaskResult", map[string]interface{}{
			"clientKey": c.APIKey,
			"taskId":    created.TaskID,
		}, taskType, created.TaskID)
		if err != nil {
			return "", err
		}
		switch result.Status {
		case "processing":
			continue
		case "ready":
			answer := strings.TrimSpace(result.Solution.Text)
			if answer == "" {
				return "", fmt.Errorf("2Captcha 图片识别结果为空")
			}
			return answer, nil
		default:
			return "", fmt.Errorf("2Captcha 返回未知任务状态: %s", result.Status)
		}
	}
}

func (c *Client) SolveAWSWAF(ctx context.Context, options AWSWAFOptions) (string, error) {
	solution, err := c.SolveAWSWAFResult(ctx, options)
	if err != nil {
		return "", err
	}
	if solution.ExistingToken == "" {
		return "", fmt.Errorf("2Captcha 结果未返回 existing_token")
	}
	return solution.ExistingToken, nil
}

func (c *Client) SolveAWSWAFResult(ctx context.Context, options AWSWAFOptions) (AWSWAFSolution, error) {
	if c == nil || strings.TrimSpace(c.APIKey) == "" {
		return AWSWAFSolution{}, fmt.Errorf("2Captcha API Key 未配置")
	}
	if strings.TrimSpace(options.WebsiteURL) == "" {
		return AWSWAFSolution{}, fmt.Errorf("AWS WAF Website URL 未配置")
	}
	websiteKey := strings.TrimSpace(options.WebsiteKey)
	jsapiScript := strings.TrimSpace(options.JSAPIScript)
	proxy := strings.TrimSpace(options.Proxy)
	hasChallengeParams := websiteKey != "" && strings.TrimSpace(options.IV) != "" && strings.TrimSpace(options.Context) != ""
	hasJSAPIParams := jsapiScript != "" && (websiteKey != "" || proxy != "")
	if !hasChallengeParams && !hasJSAPIParams {
		return AWSWAFSolution{}, fmt.Errorf("AWS WAF 需要 websiteKey + iv + context；jsapiScript 在无代理模式下还需要 websiteKey")
	}

	task := map[string]interface{}{
		"type":       "AmazonTaskProxyless",
		"websiteURL": strings.TrimSpace(options.WebsiteURL),
	}
	if hasChallengeParams {
		putIfNotEmpty(task, "websiteKey", websiteKey)
		putIfNotEmpty(task, "iv", options.IV)
		putIfNotEmpty(task, "context", options.Context)
		putIfNotEmpty(task, "challengeScript", options.ChallengeScript)
		putIfNotEmpty(task, "captchaScript", options.CaptchaScript)
	} else {
		putIfNotEmpty(task, "jsapiScript", options.JSAPIScript)
		putIfNotEmpty(task, "websiteKey", websiteKey)
	}
	if proxy != "" {
		proxyFields, err := parseProxy(options.Proxy)
		if err != nil {
			return AWSWAFSolution{}, err
		}
		task["type"] = "AmazonTask"
		for key, value := range proxyFields {
			task[key] = value
		}
	}

	created, err := c.post(ctx, "/createTask", map[string]interface{}{
		"clientKey": c.APIKey,
		"task":      task,
	}, task["type"].(string), 0)
	if err != nil {
		return AWSWAFSolution{}, err
	}
	if created.TaskID == 0 {
		return AWSWAFSolution{}, fmt.Errorf("2Captcha 未返回 taskId")
	}
	log.Printf("[WAF] 2Captcha 任务已创建: taskId=%d, type=%s", created.TaskID, task["type"])

	interval := c.PollInterval
	if interval <= 0 {
		interval = 5 * time.Second
	}
	for {
		select {
		case <-ctx.Done():
			return AWSWAFSolution{}, ctx.Err()
		case <-time.After(interval):
		}
		result, err := c.post(ctx, "/getTaskResult", map[string]interface{}{
			"clientKey": c.APIKey,
			"taskId":    created.TaskID,
		}, task["type"].(string), created.TaskID)
		if err != nil {
			return AWSWAFSolution{}, err
		}
		switch result.Status {
		case "processing":
			continue
		case "ready":
			solution := AWSWAFSolution{
				ExistingToken:  strings.TrimSpace(result.Solution.ExistingToken),
				CaptchaVoucher: strings.TrimSpace(result.Solution.CaptchaVoucher),
			}
			if solution.ExistingToken == "" && solution.CaptchaVoucher == "" {
				return AWSWAFSolution{}, fmt.Errorf("2Captcha 结果未返回验证数据")
			}
			return solution, nil
		default:
			return AWSWAFSolution{}, fmt.Errorf("2Captcha 返回未知任务状态: %s", result.Status)
		}
	}
}

func (c *Client) post(ctx context.Context, path string, payload interface{}, taskType string, taskID int64) (apiResponse, error) {
	var result apiResponse
	body, err := json.Marshal(payload)
	if err != nil {
		return result, err
	}
	base := strings.TrimRight(c.APIBase, "/")
	if base == "" {
		base = defaultAPIBase
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+path, bytes.NewReader(body))
	if err != nil {
		return result, err
	}
	req.Header.Set("Content-Type", "application/json")
	client := c.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return result, fmt.Errorf("2Captcha 请求失败: %w", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return result, fmt.Errorf("读取 2Captcha 响应失败: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return result, fmt.Errorf("2Captcha HTTP %d", resp.StatusCode)
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return result, fmt.Errorf("2Captcha 响应格式无效")
	}
	if result.ErrorID != 0 {
		return result, &APIError{
			ErrorID:     result.ErrorID,
			Code:        strings.TrimSpace(result.ErrorCode),
			Description: strings.TrimSpace(result.ErrorDescription),
			Operation:   strings.TrimPrefix(path, "/"),
			TaskType:    taskType,
			TaskID:      taskID,
		}
	}
	return result, nil
}

func putIfNotEmpty(target map[string]interface{}, key, value string) {
	if value = strings.TrimSpace(value); value != "" {
		target[key] = value
	}
}

// RemoteWorkerProxy returns an empty value when a proxy can only be reached
// from the local machine and therefore cannot be used by a 2Captcha worker.
func RemoteWorkerProxy(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	host := strings.TrimSpace(parsed.Hostname())
	if strings.EqualFold(host, "localhost") {
		return ""
	}
	if ip := net.ParseIP(host); ip != nil && (ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() || ip.IsLinkLocalUnicast()) {
		return ""
	}
	return raw
}

func parseProxy(raw string) (map[string]interface{}, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Hostname() == "" || parsed.Port() == "" {
		return nil, fmt.Errorf("2Captcha 代理地址无效")
	}
	proxyType := strings.ToLower(parsed.Scheme)
	switch proxyType {
	case "http", "https":
		proxyType = "http"
	case "socks4", "socks5", "socks5h":
		if proxyType == "socks5h" {
			proxyType = "socks5"
		}
	default:
		return nil, fmt.Errorf("2Captcha 不支持代理协议: %s", parsed.Scheme)
	}
	port, err := strconv.Atoi(parsed.Port())
	if err != nil || port < 1 || port > 65535 {
		return nil, fmt.Errorf("2Captcha 代理端口无效")
	}
	fields := map[string]interface{}{
		"proxyType":    proxyType,
		"proxyAddress": parsed.Hostname(),
		"proxyPort":    port,
	}
	if parsed.User != nil {
		fields["proxyLogin"] = parsed.User.Username()
		if password, ok := parsed.User.Password(); ok {
			fields["proxyPassword"] = password
		}
	}
	return fields, nil
}
