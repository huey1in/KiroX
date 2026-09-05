package http

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/url"
	"strings"
	"sync/atomic"

	fhttp "github.com/bogdanfinn/fhttp"
	tls_client "github.com/bogdanfinn/tls-client"
	"github.com/bogdanfinn/tls-client/profiles"
)

var requestTimeoutSeconds atomic.Int64

func init() {
	requestTimeoutSeconds.Store(60)
}

// SetRequestTimeoutSeconds changes the timeout used by newly created TLS clients.
func SetRequestTimeoutSeconds(seconds int) {
	if seconds < 10 || seconds > 180 {
		seconds = 60
	}
	requestTimeoutSeconds.Store(int64(seconds))
}

const (
	DefaultUA    = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/137.0.0.0 Safari/537.36"
	DefaultSecUA = `"Chromium";v="137", "Not/A)Brand";v="24", "Google Chrome";v="137"`
)

// Hex4 生成 4 位随机十六进制
func Hex4() string {
	const chars = "0123456789abcdef"
	b := make([]byte, 4)
	for i := range b {
		b[i] = chars[rand.Intn(16)]
	}
	return string(b)
}

// Awsccc 生成 awsccc cookie 值
func Awsccc() string {
	d := map[string]interface{}{
		"e": 1, "p": 1, "f": 1, "a": 1,
		"i": fmt.Sprintf("%s-%s-4%s-%s-%s%s%s",
			Hex4()+Hex4(), Hex4(), Hex4()[1:], Hex4(), Hex4(), Hex4(), Hex4()),
		"v": "1",
	}
	b, _ := json.Marshal(d)
	return base64.StdEncoding.EncodeToString(b)
}

// UbidGen 生成 ubid cookie 值
func UbidGen() string {
	d7 := make([]byte, 7)
	d6 := make([]byte, 6)
	for i := range d7 {
		d7[i] = byte('0' + rand.Intn(10))
	}
	for i := range d6 {
		d6[i] = byte('0' + rand.Intn(10))
	}
	return fmt.Sprintf("186-%s-%s", string(d7), string(d6))
}

// VisitorID 生成随机 visitor ID
func VisitorID() string {
	return fmt.Sprintf("%s%s-%s-7%s-%s-%s%s%s",
		Hex4(), Hex4(), Hex4(), Hex4()[1:], Hex4(), Hex4(), Hex4(), Hex4())
}

// PKCE 生成 PKCE code_verifier 和 code_challenge
func PKCE() (verifier, challenge string) {
	raw := make([]byte, 32)
	for i := range raw {
		raw[i] = byte(rand.Intn(256))
	}
	verifier = base64.RawURLEncoding.EncodeToString(raw)
	h := sha256.Sum256([]byte(verifier))
	challenge = base64.RawURLEncoding.EncodeToString(h[:])
	return
}

// NewTLSClient 创建带 TLS 指纹伪装的 HTTP 客户端
// chromeVer 可选，忽略（始终使用 Chrome_144 profile）
func NewTLSClient(proxy string, followRedirect bool, chromeVer ...string) tls_client.HttpClient {
	opts := []tls_client.HttpClientOption{
		tls_client.WithTimeoutSeconds(int(requestTimeoutSeconds.Load())),
		tls_client.WithClientProfile(profiles.Chrome_144),
		tls_client.WithInsecureSkipVerify(),
	}
	if !followRedirect {
		opts = append(opts, tls_client.WithNotFollowRedirects())
	}
	client, err := tls_client.NewHttpClient(tls_client.NewNoopLogger(), opts...)
	if err != nil {
		panic(fmt.Sprintf("创建 TLS 客户端失败: %v", err))
	}
	if proxy != "" {
		client.SetProxy(proxy)
	}
	return client
}

// NewNoRedirectTLSClient 创建不跟随重定向的 TLS 客户端
func NewNoRedirectTLSClient(proxy string, chromeVer ...string) tls_client.HttpClient {
	return NewTLSClient(proxy, false)
}

// ExtractParam 从 URL 中提取查询参数
func ExtractParam(rawURL, key string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return u.Query().Get(key)
}

// SplitAfter 从字符串中提取分隔符后的内容
func SplitAfter(s, sep string) string {
	idx := strings.Index(s, sep)
	if idx < 0 {
		return ""
	}
	rest := s[idx+len(sep):]
	if i := strings.IndexByte(rest, '&'); i >= 0 {
		return rest[:i]
	}
	return rest
}

// GetNestedMap 获取嵌套 map
func GetNestedMap(data map[string]interface{}, keys ...string) map[string]interface{} {
	current := data
	for _, k := range keys {
		next, ok := current[k].(map[string]interface{})
		if !ok {
			return nil
		}
		current = next
	}
	return current
}

// GetNestedStringMap 获取嵌套的 string map
func GetNestedStringMap(data map[string]interface{}, key string) map[string]string {
	if data == nil {
		return nil
	}
	nested, ok := data[key].(map[string]interface{})
	if !ok {
		return nil
	}
	result := make(map[string]string)
	for k, v := range nested {
		if s, ok := v.(string); ok {
			result[k] = s
		}
	}
	return result
}

// SetHeaders 设置请求头 (保持顺序)
func SetHeaders(req *fhttp.Request, headers map[string]string) {
	var order []string
	for k, v := range headers {
		req.Header.Set(k, v)
		order = append(order, strings.ToLower(k))
	}
	req.Header[fhttp.HeaderOrderKey] = order
}

// SaveCookies 从 Set-Cookie 头中提取并保存 cookies
func SaveCookies(cookies map[string]string, headers map[string][]string) {
	skip := map[string]bool{
		"path": true, "domain": true, "expires": true,
		"max-age": true, "secure": true, "httponly": true, "samesite": true,
	}
	for _, vals := range headers {
		for _, raw := range vals {
			if !strings.Contains(raw, "=") {
				continue
			}
			kv := strings.SplitN(strings.Split(raw, ";")[0], "=", 2)
			if len(kv) == 2 {
				k := strings.TrimSpace(kv[0])
				v := strings.TrimSpace(kv[1])
				if !skip[strings.ToLower(k)] && k != "" {
					cookies[k] = v
				}
			}
		}
	}
}
