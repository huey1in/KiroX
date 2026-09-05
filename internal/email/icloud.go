package email

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"fmt"
	stdhtml "html"
	"io"
	"log"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	http "github.com/bogdanfinn/fhttp"
	tls_client "github.com/bogdanfinn/tls-client"
	httputil "reg_go/internal/http"
)

// ICloudAccount describes one mailbox entry from icloud.csv.
type ICloudAccount struct {
	Email       string
	MessagesURL string
}

// ParseICloudCSV parses lines in the form email----messages URL.
func ParseICloudCSV(r io.Reader) ([]ICloudAccount, error) {
	scanner := bufio.NewScanner(r)
	accounts := make([]ICloudAccount, 0)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "----", 2)
		if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
			log.Printf("跳过格式错误的 iCloud 行 %d", lineNo)
			continue
		}
		address := strings.TrimSpace(parts[0])
		messagesURL := strings.TrimSpace(parts[1])
		if parsed, err := url.Parse(messagesURL); err != nil || parsed.Scheme == "" || parsed.Host == "" {
			log.Printf("跳过无效的 iCloud 取件 URL (第 %d 行)", lineNo)
			continue
		}
		accounts = append(accounts, ICloudAccount{Email: address, MessagesURL: messagesURL})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return accounts, nil
}

// ParseICloudCSVFile parses an iCloud mailbox file from disk.
func ParseICloudCSVFile(path string) ([]ICloudAccount, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return ParseICloudCSV(f)
}

var iCloudMessageIDPattern = regexp.MustCompile(`(?i)data-id\s*=\s*["']([^"']+)["']`)
var iCloudStylePattern = regexp.MustCompile(`(?is)<style\b[^>]*>.*?</style\s*>|<script\b[^>]*>.*?</script\s*>`)
var iCloudTagPattern = regexp.MustCompile(`(?s)<[^>]*>`)

// ParseICloudMessageIDs extracts message IDs from the mailbox HTML list.
func ParseICloudMessageIDs(html string) []string {
	matches := iCloudMessageIDPattern.FindAllStringSubmatch(html, -1)
	ids := make([]string, 0, len(matches))
	seen := make(map[string]struct{}, len(matches))
	for _, match := range matches {
		if len(match) < 2 || match[1] == "" {
			continue
		}
		if _, ok := seen[match[1]]; ok {
			continue
		}
		seen[match[1]] = struct{}{}
		ids = append(ids, match[1])
	}
	return ids
}

// ICloudDetailURL builds the JSON endpoint for one message in a mailbox URL.
func ICloudDetailURL(messagesURL, messageID string) string {
	u, err := url.Parse(messagesURL)
	if err != nil {
		return ""
	}
	const marker = "/messages/"
	idx := strings.Index(u.Path, marker)
	if idx < 0 || messageID == "" {
		return ""
	}
	escapedPath := u.EscapedPath()
	idx = strings.Index(escapedPath, marker)
	if idx < 0 {
		return ""
	}
	suffix := strings.TrimPrefix(escapedPath[idx+len(marker):], "/")
	if suffix == "" {
		return ""
	}
	newEscapedPath := escapedPath[:idx] + "/message/" + url.PathEscape(messageID) + "/" + suffix
	decodedPath, err := url.PathUnescape(newEscapedPath)
	if err != nil {
		return ""
	}
	u.Path = decodedPath
	u.RawPath = newEscapedPath
	return u.String()
}

// DecodeICloudMessageBody decodes a data URL returned in the JSON message body.
func DecodeICloudMessageBody(raw string) (string, error) {
	if !strings.HasPrefix(strings.ToLower(raw), "data:") {
		return raw, nil
	}
	comma := strings.IndexByte(raw, ',')
	if comma < 0 {
		return "", fmt.Errorf("无效的 iCloud 邮件 body data URL")
	}
	meta, payload := raw[:comma], raw[comma+1:]
	if strings.Contains(strings.ToLower(meta), ";base64") {
		decoded, err := base64.StdEncoding.DecodeString(payload)
		if err != nil {
			decoded, err = base64.RawStdEncoding.DecodeString(payload)
			if err != nil {
				return "", fmt.Errorf("解码 iCloud 邮件 body 失败: %w", err)
			}
		}
		return string(decoded), nil
	}
	decoded, err := url.PathUnescape(payload)
	if err != nil {
		return "", fmt.Errorf("解码 iCloud 邮件 body 失败: %w", err)
	}
	return decoded, nil
}

// ExtractICloudCode strips HTML presentation content before extracting the OTP.
// CSS colors such as #555555 can otherwise look like six-digit codes.
func ExtractICloudCode(body string) string {
	clean := iCloudStylePattern.ReplaceAllString(body, " ")
	clean = iCloudTagPattern.ReplaceAllString(clean, " ")
	clean = stdhtml.UnescapeString(clean)
	return ExtractCode(clean)
}

type iCloudMessage struct {
	Body        string `json:"body"`
	FromAddress string `json:"fromAddress"`
	HTML        bool   `json:"html"`
	IsTrash     bool   `json:"isTrash"`
	Mailbox     string `json:"mailbox"`
	ReceivedAt  string `json:"receivedAt"`
	Subject     string `json:"subject"`
}

// ICloudProvider reads verification messages from an apple55-style mailbox URL.
type ICloudProvider struct {
	listURL string
	address string
	client  tls_client.HttpClient
	seen    map[string]struct{}
	mu      sync.Mutex
}

// NewICloudService creates an iCloud mailbox service for one CSV account.
func NewICloudService(account ICloudAccount, proxy, chromeVer string) TempEmailService {
	return NewICloudProvider(account, proxy, chromeVer)
}

// NewICloudProvider creates an iCloud mailbox provider.
func NewICloudProvider(account ICloudAccount, proxy, chromeVer string) *ICloudProvider {
	return &ICloudProvider{
		listURL: account.MessagesURL,
		address: account.Email,
		client:  httputil.NewTLSClient(proxy, true, chromeVer),
		seen:    make(map[string]struct{}),
	}
}

// Create records existing messages and returns the configured mailbox address.
func (i *ICloudProvider) Create() string {
	ids, err := i.fetchMessageIDs()
	if err != nil {
		log.Printf("[iCloud] 初始化邮件列表失败: %v", err)
	} else {
		i.mu.Lock()
		for _, id := range ids {
			i.seen[id] = struct{}{}
		}
		i.mu.Unlock()
		log.Printf("[iCloud] 已加载 %d 封历史邮件", len(ids))
	}
	log.Printf("[iCloud] 使用邮箱: %s", i.address)
	return i.address
}

func (i *ICloudProvider) GetAddress() string { return i.address }

// WaitForCode polls the mailbox and fetches only messages not seen at Create time.
func (i *ICloudProvider) WaitForCode(timeoutSec, intervalSec int) (string, error) {
	if i.listURL == "" {
		return "", fmt.Errorf("iCloud 取件 URL 为空")
	}
	if intervalSec <= 0 {
		intervalSec = 1
	}
	if timeoutSec < intervalSec {
		timeoutSec = intervalSec
	}
	maxRetries := (timeoutSec + intervalSec - 1) / intervalSec
	for attempt := 1; attempt <= maxRetries; attempt++ {
		ids, err := i.fetchMessageIDs()
		if err == nil {
			for _, id := range ids {
				if i.hasSeen(id) {
					continue
				}
				code, fetchErr := i.fetchMessageCode(id)
				if fetchErr != nil {
					log.Printf("[iCloud] 获取邮件 %s 失败: %v", id, fetchErr)
					continue
				}
				i.markSeen(id)
				if code != "" {
					return code, nil
				}
			}
		} else if attempt%5 == 0 {
			log.Printf("[iCloud] [%d/%d] 轮询失败: %v", attempt, maxRetries, err)
		}
		if attempt < maxRetries {
			time.Sleep(time.Duration(intervalSec) * time.Second)
		}
	}
	return "", fmt.Errorf("等待 iCloud 验证码超时 (%ds)", timeoutSec)
}

func (i *ICloudProvider) hasSeen(id string) bool {
	i.mu.Lock()
	defer i.mu.Unlock()
	_, ok := i.seen[id]
	return ok
}

func (i *ICloudProvider) markSeen(id string) {
	i.mu.Lock()
	i.seen[id] = struct{}{}
	i.mu.Unlock()
}

func (i *ICloudProvider) fetchMessageIDs() ([]string, error) {
	body, err := i.get(i.listURL, "text/html,application/xhtml+xml")
	if err != nil {
		return nil, err
	}
	ids := ParseICloudMessageIDs(string(body))
	if len(ids) == 0 {
		return nil, nil
	}
	return ids, nil
}

func (i *ICloudProvider) fetchMessageCode(messageID string) (string, error) {
	detailURL := ICloudDetailURL(i.listURL, messageID)
	if detailURL == "" {
		return "", fmt.Errorf("无法构造消息详情 URL")
	}
	body, err := i.get(detailURL, "application/json")
	if err != nil {
		return "", err
	}
	var message iCloudMessage
	if err := json.Unmarshal(body, &message); err != nil {
		return "", fmt.Errorf("解析 iCloud 消息失败: %w", err)
	}
	decoded, err := DecodeICloudMessageBody(message.Body)
	if err != nil {
		return "", err
	}
	return ExtractICloudCode(message.Subject + " " + decoded), nil
}

func (i *ICloudProvider) get(rawURL, accept string) ([]byte, error) {
	req, err := http.NewRequest("GET", rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建 iCloud 请求失败: %w", err)
	}
	req.Header.Set("Accept", accept)
	req.Header.Set("User-Agent", httputil.DefaultUA)
	resp, err := i.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("iCloud 请求失败: %w", err)
	}
	defer resp.Body.Close()
	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return nil, readErr
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet := string(body)
		if len(snippet) > 200 {
			snippet = snippet[:200] + "..."
		}
		return nil, fmt.Errorf("iCloud HTTP %d: %s", resp.StatusCode, snippet)
	}
	return body, nil
}
