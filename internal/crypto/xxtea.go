package crypto

import (
	"context"
	"encoding/base64"
	"fmt"
	"hash/crc32"
	"io"
	"log"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	fhttp "github.com/bogdanfinn/fhttp"
	httputil "reg_go/internal/http"
)

const (
	delta       uint32 = 0x9E3779B9
	fallbackVer        = "4.0.0"
	identifier         = "ECdITeCs"
)

var (
	fallbackKey = [4]uint32{1888420705, 2576816180, 2347232058, 874813317}

	appJSConfig = newAppJSConfigLoader()
)

// xxteaConfig 是密钥/TES 版本/identifier 的不可变快照。
// 热路径 (每次指纹加密) 通过 atomic.Pointer 无锁读取, 不再与其他并发任务争抢互斥锁。
type xxteaConfig struct {
	Key        [4]uint32
	Version    string
	Identifier string
}

type appJSConfigLoader struct {
	once  sync.Once
	ready chan struct{}
	cfg   atomic.Pointer[xxteaConfig]
}

func newAppJSConfigLoader() *appJSConfigLoader {
	return &appJSConfigLoader{ready: make(chan struct{})}
}

func (l *appJSConfigLoader) start(load func() xxteaConfig) {
	l.once.Do(func() {
		go func() {
			cfg := load()
			l.cfg.Store(&cfg)
			close(l.ready)
		}()
	})
}

func (l *appJSConfigLoader) wait(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-l.ready:
		return ctx.Err()
	}
}

// WarmAppJSConfig starts the shared download without blocking registration setup.
func WarmAppJSConfig(proxy, chromeVer, userAgent, secUA string) {
	appJSConfig.start(func() xxteaConfig {
		cfg := xxteaConfig{
			Version:    fallbackVer,
			Identifier: identifier,
		}

		js := fetchAppJS(proxy, chromeVer, userAgent, secUA)
		if js != "" {
			key, ident, ver := extractFromAppJS(js)
			if key != nil {
				cfg.Key = *key
			}
			if ident != "" {
				cfg.Identifier = ident
			}
			if ver != "" {
				cfg.Version = ver
			}
		}
		if cfg.Key == [4]uint32{} {
			log.Println("[xxtea] 使用 fallback 密钥")
			cfg.Key = fallbackKey
		}
		return cfg
	})
}

// RefreshAppJSConfigContext waits for a complete snapshot before it is used.
// One cancelled caller must not cancel initialization for other registrations.
func RefreshAppJSConfigContext(ctx context.Context, proxy, chromeVer, userAgent, secUA string) error {
	if ctx != nil && ctx.Err() != nil {
		return ctx.Err()
	}
	WarmAppJSConfig(proxy, chromeVer, userAgent, secUA)
	return appJSConfig.wait(ctx)
}

func RefreshAppJSConfig(proxy, chromeVer, userAgent, secUA string) {
	_ = RefreshAppJSConfigContext(context.Background(), proxy, chromeVer, userAgent, secUA)
}

func GetTESVersion() string {
	if cfg := appJSConfig.cfg.Load(); cfg != nil {
		return cfg.Version
	}
	return fallbackVer
}

func GetIdentifier() string {
	if cfg := appJSConfig.cfg.Load(); cfg != nil {
		return cfg.Identifier
	}
	return identifier
}

func GetActiveKey() [4]uint32 {
	if cfg := appJSConfig.cfg.Load(); cfg != nil {
		return cfg.Key
	}
	return fallbackKey
}

// EncryptFingerprint 加密指纹 JSON 字符串
func EncryptFingerprint(jsonStr string) string {
	crc := crc32.ChecksumIEEE([]byte(jsonStr))
	plaintext := fmt.Sprintf("%08X#%s", crc, jsonStr)
	key := GetActiveKey()
	encrypted := xxteaEncrypt(plaintext, key)
	encoded := base64.StdEncoding.EncodeToString(encrypted)
	return GetIdentifier() + ":" + encoded
}

func xxteaEncrypt(plaintext string, key [4]uint32) []byte {
	if len(plaintext) == 0 {
		return nil
	}
	n := (len(plaintext) + 3) / 4
	v := make([]uint32, n)
	for i := 0; i < n; i++ {
		var b0, b1, b2, b3 byte
		if 4*i < len(plaintext) {
			b0 = plaintext[4*i]
		}
		if 4*i+1 < len(plaintext) {
			b1 = plaintext[4*i+1]
		}
		if 4*i+2 < len(plaintext) {
			b2 = plaintext[4*i+2]
		}
		if 4*i+3 < len(plaintext) {
			b3 = plaintext[4*i+3]
		}
		v[i] = uint32(b0) | uint32(b1)<<8 | uint32(b2)<<16 | uint32(b3)<<24
	}
	rounds := 6 + 52/n
	z := v[n-1]
	var total uint32
	for r := 0; r < rounds; r++ {
		total += delta
		e := (total >> 2) & 3
		for p := 0; p < n; p++ {
			y := v[(p+1)%n]
			mx := ((z>>5 ^ y<<2) + (y>>3 ^ z<<4)) ^ ((total ^ y) + (key[(uint32(p)&3)^e] ^ z))
			v[p] += mx
			z = v[p]
		}
	}
	result := make([]byte, n*4)
	for i, val := range v {
		result[4*i] = byte(val)
		result[4*i+1] = byte(val >> 8)
		result[4*i+2] = byte(val >> 16)
		result[4*i+3] = byte(val >> 24)
	}
	return result
}

// The deadline covers both receiving headers and reading the response body.
const appJSFetchTimeout = 15 * time.Second

func fetchAppJS(proxy, chromeVer, userAgent, secUA string) string {
	ctx, cancel := context.WithTimeout(context.Background(), appJSFetchTimeout)
	defer cancel()
	client := httputil.NewTLSClient(proxy, true, chromeVer)
	defer client.CloseIdleConnections()
	js, err := downloadAppJS(ctx, client, appJSRequestHeaders(chromeVer, userAgent, secUA))
	if err != nil {
		log.Printf("[xxtea] 下载 app.js 失败: %v", err)
		return ""
	}
	return js
}

type appJSClient interface {
	Do(*fhttp.Request) (*fhttp.Response, error)
}

func downloadAppJS(ctx context.Context, client appJSClient, headers map[string]string) (string, error) {
	req, err := fhttp.NewRequestWithContext(ctx, "GET", "https://us-east-1.signin.aws/assets/js/app.js", nil)
	if err != nil {
		return "", err
	}
	httputil.SetHeaders(req, headers)
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != fhttp.StatusOK {
		return "", fmt.Errorf("app.js HTTP %d", resp.StatusCode)
	}
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func appJSRequestHeaders(chromeVer, userAgent, secUA string) map[string]string {
	if chromeVer == "" {
		chromeVer = "144.0.0.0"
	}
	if userAgent == "" {
		userAgent = httputil.DefaultUA
	}
	if secUA == "" {
		secUA = httputil.DefaultSecUA
	}
	return map[string]string{
		"User-Agent":      userAgent,
		"Accept":          "*/*",
		"Accept-Language": "en-US,en;q=0.9",
		"Referer":         "https://us-east-1.signin.aws/",
		"sec-ch-ua":       secUA,
		"sec-fetch-dest":  "script",
		"sec-fetch-mode":  "no-cors",
		"sec-fetch-site":  "same-origin",
	}
}

func extractFromAppJS(js string) (*[4]uint32, string, string) {
	var key *[4]uint32
	var ident, ver string
	// The provider obfuscates both the array contents and the material order.
	// Match the provide() return expression as well as the array so unrelated
	// numeric arrays in the minified bundle cannot be selected accidentally.
	re := regexp.MustCompile(`var\s+([A-Za-z_$][\w$]*)\s*=\s*\[\s*(\d+)\s*,\s*(\d+)\s*,\s*(\d+)\s*,\s*"([A-Za-z0-9]+)"\s*,\s*(\d+)[^\]]*\]`)
	for _, m := range re.FindAllStringSubmatch(js, -1) {
		if len(m) != 7 {
			continue
		}
		name := regexp.QuoteMeta(m[1])
		provider := regexp.MustCompile(`return\s*\{\s*identifier\s*:\s*` + name + `\[3\]\s*,\s*material\s*:\s*\[\s*` + name + `\[1\]\s*,\s*` + name + `\[0\]\s*,\s*` + name + `\[2\]\s*,\s*` + name + `\[4\]\s*\]`)
		if !provider.MatchString(js) {
			continue
		}
		nums := make([]uint32, 4)
		for i, idx := range []int{2, 3, 4, 6} {
			v, err := strconv.ParseUint(m[idx], 10, 32)
			if err != nil {
				nums = nil
				break
			}
			nums[i] = uint32(v)
		}
		if nums != nil {
			k := [4]uint32{nums[1], nums[0], nums[2], nums[3]}
			key = &k
			ident = m[5]
			break
		}
	}
	ver = extractFWCIMVersion(js)
	return key, ident, ver
}

func extractFWCIMVersion(js string) string {
	re := regexp.MustCompile(`FWCIM_VERSION(?:"\s*\]|)\s*=\s*"(\d+\.\d+\.\d+)"`)
	match := re.FindStringSubmatch(js)
	if len(match) == 2 {
		return match[1]
	}
	return ""
}

func DecryptFingerprint(encrypted string) (string, error) {
	parts := strings.SplitN(encrypted, ":", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("格式错误")
	}
	data, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return "", err
	}
	raw := xxteaDecrypt(data, GetActiveKey())
	if idx := strings.Index(raw[:min(16, len(raw))], "#"); idx >= 0 {
		return raw[idx+1:], nil
	}
	return raw, nil
}

func xxteaDecrypt(data []byte, key [4]uint32) string {
	n := len(data) / 4
	if n < 2 {
		return ""
	}
	v := make([]uint32, n)
	for i := 0; i < n; i++ {
		v[i] = uint32(data[4*i]) | uint32(data[4*i+1])<<8 |
			uint32(data[4*i+2])<<16 | uint32(data[4*i+3])<<24
	}
	rounds := 6 + 52/n
	total := uint32(rounds) * delta
	y := v[0]
	for r := 0; r < rounds; r++ {
		e := (total >> 2) & 3
		for p := n - 1; p >= 0; p-- {
			z := v[(p-1+n)%n]
			mx := ((z>>5 ^ y<<2) + (y>>3 ^ z<<4)) ^ ((total ^ y) + (key[(uint32(p)&3)^e] ^ z))
			v[p] -= mx
			y = v[p]
		}
		total -= delta
	}
	var sb strings.Builder
	for _, val := range v {
		sb.WriteByte(byte(val))
		sb.WriteByte(byte(val >> 8))
		sb.WriteByte(byte(val >> 16))
		sb.WriteByte(byte(val >> 24))
	}
	return strings.TrimRight(sb.String(), "\x00")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
