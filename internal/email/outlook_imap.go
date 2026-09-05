package email

import (
	"bufio"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"reg_go/internal/storage"
)

// OutlookAccount Outlook 邮箱账号
type OutlookAccount struct {
	Email        string
	Password     string
	ClientID     string
	RefreshToken string
	Mode         string
}

// ParseOutlookLines 从文本内容直接解析 Outlook 账号 (Web UI 使用)
// 支持两种格式:
// 1. 换行分隔: 每行一个账号
// 2. 空格分隔: 账号之间用空格隔开
func ParseOutlookLines(data string) []OutlookAccount {
	var accounts []OutlookAccount
	data = strings.TrimSpace(data)
	if data == "" {
		return accounts
	}

	// 先尝试按换行分割
	lines := strings.Split(data, "\n")

	// 如果只有一行，可能是空格分隔的格式
	if len(lines) == 1 {
		// 尝试按空格分割（账号格式: email----password----clientid----token）
		// 每个账号以空格结尾，下一个账号开始
		parts := strings.Fields(data) // Fields 会按空白字符分割并去除空白
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			if acc, ok := parseOutlookAccountLine(part); ok {
				accounts = append(accounts, acc)
			}
		}
	} else {
		// 多行格式，按行解析
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			if acc, ok := parseOutlookAccountLine(line); ok {
				accounts = append(accounts, acc)
			}
		}
	}

	return accounts
}

func parseOutlookAccountLine(line string) (OutlookAccount, bool) {
	parts := strings.SplitN(strings.TrimSpace(line), "----", 5)
	if len(parts) != 4 && len(parts) != 5 {
		return OutlookAccount{}, false
	}
	mode := "imap"
	if len(parts) == 5 {
		mode = normalizeOutlookMode(parts[4])
	}
	return OutlookAccount{
		Email:        strings.TrimSpace(parts[0]),
		Password:     strings.TrimSpace(parts[1]),
		ClientID:     strings.TrimSpace(parts[2]),
		RefreshToken: strings.TrimSpace(parts[3]),
		Mode:         mode,
	}, true
}

func normalizeOutlookMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "graph":
		return "graph"
	default:
		return "imap"
	}
}

func (a OutlookAccount) mailMode() string {
	return normalizeOutlookMode(a.Mode)
}

// RefreshOutlookToken 用 refresh_token 获取 access_token（优先走全局代理，失败时降级直连）
func RefreshOutlookToken(acc OutlookAccount) (string, error) {
	return RefreshOutlookTokenWithProxy(acc, storage.GetProxy())
}

// RefreshOutlookTokenWithProxy refreshes a token using the explicit mailbox proxy policy.
func RefreshOutlookTokenWithProxy(acc OutlookAccount, proxyURL string) (string, error) {
	form := url.Values{
		"client_id":     {acc.ClientID},
		"refresh_token": {acc.RefreshToken},
		"grant_type":    {"refresh_token"},
		"scope":         {"https://outlook.office.com/IMAP.AccessAsUser.All offline_access"},
	}

	tryPost := func(p string) (resp *http.Response, err error) {
		client := httpClientWithProxy(p, 30*time.Second)
		return client.Post(
			"https://login.microsoftonline.com/consumers/oauth2/v2.0/token",
			"application/x-www-form-urlencoded",
			strings.NewReader(form.Encode()),
		)
	}
	resp, err := tryPost(proxyURL)
	if err != nil {
		return "", fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("刷新失败 %d: %s", resp.StatusCode, string(body[:min(300, len(body))]))
	}

	var result map[string]interface{}
	json.Unmarshal(body, &result)
	token, _ := result["access_token"].(string)
	if token == "" {
		return "", fmt.Errorf("响应中无 access_token")
	}
	return token, nil
}

// buildXOAuth2 构建 XOAUTH2 认证字符串
func buildXOAuth2(email, accessToken string) string {
	auth := fmt.Sprintf("user=%s\x01auth=Bearer %s\x01\x01", email, accessToken)
	return base64.StdEncoding.EncodeToString([]byte(auth))
}

// imapClient 简易 IMAP 客户端
type imapClient struct {
	conn   net.Conn
	reader *bufio.Reader
	tag    int
}

// newIMAPClient 连接 Outlook IMAP（优先走全局代理，代理被封端口时自动降级直连）
func newIMAPClient() (*imapClient, error) {
	return newIMAPClientWithProxy(storage.GetProxy())
}

func newIMAPClientWithProxy(proxyURL string) (*imapClient, error) {
	const target = "outlook.office365.com:993"
	rawConn, err := dialThroughProxy(proxyURL, "tcp", target, 15*time.Second)
	if err != nil {
		return nil, fmt.Errorf("连接失败: %v", err)
	}
	tlsConfig := &tls.Config{ServerName: "outlook.office365.com"}
	conn := tls.Client(rawConn, tlsConfig)
	if err := conn.SetDeadline(time.Now().Add(15 * time.Second)); err == nil {
		err = conn.Handshake()
		conn.SetDeadline(time.Time{})
		if err != nil {
			conn.Close()
			return nil, fmt.Errorf("TLS 握手失败: %v", err)
		}
	}

	c := &imapClient{conn: conn, reader: bufio.NewReader(conn), tag: 0}
	greeting, err := c.readLine()
	if err != nil {
		conn.Close()
		return nil, err
	}
	log.Printf("[IMAP] %s", greeting)
	return c, nil
}

func (c *imapClient) sendCommand(cmd string) (string, error) {
	c.tag++
	tagStr := fmt.Sprintf("A%03d", c.tag)
	line := fmt.Sprintf("%s %s\r\n", tagStr, cmd)
	_, err := c.conn.Write([]byte(line))
	if err != nil {
		return "", err
	}
	return tagStr, nil
}

func (c *imapClient) readLine() (string, error) {
	line, err := c.reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}

func (c *imapClient) readUntilTag(tag string) ([]string, string, error) {
	var lines []string
	for {
		line, err := c.readLine()
		if err != nil {
			return lines, "", err
		}
		if strings.HasPrefix(line, tag+" ") {
			return lines, line, nil
		}
		lines = append(lines, line)
	}
}

func (c *imapClient) authenticate(email, accessToken string) error {
	xoauth2 := buildXOAuth2(email, accessToken)
	tag, err := c.sendCommand("AUTHENTICATE XOAUTH2 " + xoauth2)
	if err != nil {
		return err
	}
	_, result, err := c.readUntilTag(tag)
	if err != nil {
		return err
	}
	if !strings.Contains(result, "OK") {
		return fmt.Errorf("认证失败: %s", result)
	}
	log.Println("[IMAP] 认证成功")

	// 发送 NOOP 确保会话完全就绪（Outlook 有时认证后需要额外握手）
	for i := 0; i < 3; i++ {
		noopTag, err := c.sendCommand("NOOP")
		if err != nil {
			return err
		}
		_, noopResult, err := c.readUntilTag(noopTag)
		if err != nil {
			return err
		}
		if strings.Contains(noopResult, "OK") {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}

	// Outlook Exchange 后端认证后需要额外时间建立 mailbox 连接，否则 SELECT 会返回 "not connected"
	time.Sleep(2 * time.Second)
	return nil
}

func (c *imapClient) selectInbox() (int, error) {
	tag, err := c.sendCommand("SELECT INBOX")
	if err != nil {
		return 0, err
	}
	lines, result, err := c.readUntilTag(tag)
	if err != nil {
		return 0, err
	}
	if strings.Contains(result, "OK") {
		total := 0
		for _, line := range lines {
			if strings.Contains(line, "EXISTS") {
				fmt.Sscanf(line, "* %d EXISTS", &total)
			}
		}
		return total, nil
	}
	// "not connected" 表示 Outlook 后端尚未就绪，同连接重试无效，由调用方重连后重试
	errMsg := strings.TrimSpace(result)
	if len(errMsg) > 80 {
		errMsg = errMsg[:80] + "..."
	}
	return 0, fmt.Errorf("SELECT 失败: %s", errMsg)
}

func (c *imapClient) close() {
	c.sendCommand("LOGOUT")
	c.conn.Close()
}

// fetchLatestBody 获取指定邮件的正文并解码
func (c *imapClient) fetchLatestBody(seq int) (string, error) {
	if seq <= 0 {
		return "", fmt.Errorf("无效的邮件序号")
	}
	tag, err := c.sendCommand(fmt.Sprintf("FETCH %d (BODY.PEEK[TEXT])", seq))
	if err != nil {
		return "", err
	}
	lines, result, err := c.readUntilTag(tag)
	if err != nil {
		return "", err
	}
	if !strings.Contains(result, "OK") {
		return "", fmt.Errorf("FETCH TEXT 失败: %s", result)
	}

	var rawLines []string
	inBody := false
	for _, line := range lines {
		if strings.Contains(line, "FETCH") {
			inBody = true
			continue
		}
		if line == ")" {
			continue
		}
		if inBody {
			rawLines = append(rawLines, line)
		}
	}

	raw := strings.Join(rawLines, "\n")

	// 尝试解码 MIME base64 内容
	parts := strings.Split(raw, "------=_Part_")
	var decoded string
	for _, part := range parts {
		if strings.Contains(part, "base64") {
			idx := strings.Index(part, "base64")
			content := part[idx+6:]
			b64 := strings.Map(func(r rune) rune {
				if r == ' ' || r == '\n' || r == '\r' || r == '\t' {
					return -1
				}
				return r
			}, content)
			if data, err := base64.StdEncoding.DecodeString(b64); err == nil {
				decoded += string(data) + " "
			}
		}
	}
	if decoded != "" {
		return decoded, nil
	}

	// 整体 base64 解码
	cleaned := strings.Map(func(r rune) rune {
		if r == ' ' || r == '\n' || r == '\r' || r == '\t' {
			return -1
		}
		return r
	}, raw)
	if data, err := base64.StdEncoding.DecodeString(cleaned); err == nil {
		return string(data), nil
	}

	return raw, nil
}

// WaitForOTP 通过 IMAP 轮询等待 AWS 验证码
func WaitForOTP(acc OutlookAccount, beforeCount, timeout, interval int) (string, error) {
	return WaitForOTPWithProxy(acc, beforeCount, timeout, interval, storage.GetProxy())
}

// WaitForOTPWithProxy polls Outlook through the selected mailbox network policy.
func WaitForOTPWithProxy(acc OutlookAccount, beforeCount, timeout, interval int, proxyURL string) (string, error) {
	codeRegex := regexp.MustCompile(`\b(\d{6})\b`)
	if acc.mailMode() == "graph" {
		log.Printf("[Outlook Graph] 等待验证码, 邮箱=%s, 发送前邮件数=%d", acc.Email, beforeCount)
		return waitForOTPGraph(acc, beforeCount, timeout, interval, codeRegex, proxyURL)
	}

	log.Printf("[Outlook IMAP] 等待验证码, 邮箱=%s, 发送前邮件数=%d", acc.Email, beforeCount)
	accessToken, err := RefreshOutlookTokenWithProxy(acc, proxyURL)
	if err != nil {
		return "", fmt.Errorf("刷新 Outlook Token 失败: %v", err)
	}

	maxRetries := timeout / interval
	consecutiveSelectFail := 0
	maxConsecutiveSelectFail := 3 // 连续 3 次 SELECT 失败则提前放弃，避免单账号卡住整批
	for attempt := 1; attempt <= maxRetries; attempt++ {
		client, err := newIMAPClientWithProxy(proxyURL)
		if err != nil {
			if attempt%5 == 0 {
				log.Printf("[Outlook IMAP] 连接失败: %v, 重试中...", err)
			}
			time.Sleep(time.Duration(interval) * time.Second)
			continue
		}

		if err := client.authenticate(acc.Email, accessToken); err != nil {
			client.close()
			accessToken, _ = RefreshOutlookTokenWithProxy(acc, proxyURL)
			time.Sleep(time.Duration(interval) * time.Second)
			continue
		}

		total, err := client.selectInbox()
		if err != nil {
			client.close()
			consecutiveSelectFail++
			if consecutiveSelectFail >= maxConsecutiveSelectFail {
				log.Printf("[Outlook IMAP] 邮箱 %s 连续 %d 次 SELECT 失败，放弃等待", acc.Email, consecutiveSelectFail)
				return "", fmt.Errorf("IMAP SELECT 连续失败 %d 次: %v", consecutiveSelectFail, err)
			}
			log.Printf("[Outlook IMAP] SELECT 失败 (%d/%d): %v", consecutiveSelectFail, maxConsecutiveSelectFail, err)
			time.Sleep(time.Duration(interval) * time.Second)
			continue
		}
		consecutiveSelectFail = 0 // 成功则重置

		if total <= beforeCount {
			client.close()
			if attempt%5 == 0 {
				log.Printf("[Outlook IMAP] [%d/%d] 暂无新邮件 (当前%d封)...", attempt, maxRetries, total)
			}
			time.Sleep(time.Duration(interval) * time.Second)
			continue
		}

		for i := total; i > beforeCount; i-- {
			body, err := client.fetchLatestBody(i)
			if err != nil {
				continue
			}
			code := extractCodeFromText(body, codeRegex)
			if code != "" {
				log.Printf("[Outlook IMAP] 获取到验证码: %s", code)
				client.close()
				return code, nil
			}
		}

		client.close()
		if attempt%5 == 0 {
			log.Printf("[Outlook IMAP] [%d/%d] 新邮件中未找到验证码...", attempt, maxRetries)
		}
		time.Sleep(time.Duration(interval) * time.Second)
	}
	return "", fmt.Errorf("等待验证码超时 (%ds)", timeout)
}

// GetInboxCount 获取收件箱当前邮件数量（带完整重连重试）
func GetInboxCount(acc OutlookAccount) (int, error) {
	return GetInboxCountWithProxy(acc, storage.GetProxy())
}

// GetInboxCountWithProxy reads the baseline count through the explicit mailbox proxy.
func GetInboxCountWithProxy(acc OutlookAccount, proxyURL string) (int, error) {
	if acc.mailMode() == "graph" {
		return getInboxCountGraph(acc, proxyURL)
	}
	accessToken, err := RefreshOutlookTokenWithProxy(acc, proxyURL)
	if err != nil {
		return 0, fmt.Errorf("刷新 Outlook Token 失败: %v", err)
	}

	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(1+attempt) * time.Second)
		}
		client, err := newIMAPClientWithProxy(proxyURL)
		if err != nil {
			lastErr = fmt.Errorf("连接 IMAP 失败: %v", err)
			continue
		}
		if err := client.authenticate(acc.Email, accessToken); err != nil {
			client.close()
			lastErr = fmt.Errorf("IMAP 认证失败: %v", err)
			continue
		}
		total, err := client.selectInbox()
		if err != nil {
			client.close()
			lastErr = fmt.Errorf("选择收件箱失败: %v", err)
			log.Printf("[IMAP] GetInboxCount 失败，重连重试 %d/3...", attempt+1)
			continue
		}
		client.close()
		return total, nil
	}
	return 0, lastErr
}
