package crypto

import (
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
	delta      uint32 = 0x9E3779B9
	fallbackVer       = "4.0.0"
	identifier        = "ECdITeCs"
)

var (
	fallbackKey = [4]uint32{1888420705, 2576816180, 2347232058, 874813317}

	configOnce sync.Once
	configPtr  atomic.Pointer[xxteaConfig]
)

// xxteaConfig 是密钥/TES 版本/identifier 的不可变快照。
// 热路径 (每次指纹加密) 通过 atomic.Pointer 无锁读取, 不再与其他并发任务争抢互斥锁。
type xxteaConfig struct {
	Key        [4]uint32
	Version    string
	Identifier string
}

// RefreshAppJSConfig 从 app.js 刷新 XXTEA 密钥和 TES 版本。
// 首次调用由 sync.Once 保证只有一个 goroutine 拉取 app.js,
// 网络请求期间不再持有任何锁, 后续调用零开销直接返回。
// (旧实现把整个下载过程放在全局互斥锁内, 导致所有并发注册流程在启动时串行排队)
func RefreshAppJSConfig(proxy, chromeVer, userAgent, secUA string) {
	configOnce.Do(func() {
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
		configPtr.Store(&cfg)
	})
}

func GetTESVersion() string {
	if cfg := configPtr.Load(); cfg != nil {
		return cfg.Version
	}
	return fallbackVer
}

func GetIdentifier() string {
	if cfg := configPtr.Load(); cfg != nil {
		return cfg.Identifier
	}
	return identifier
}

func GetActiveKey() [4]uint32 {
	if cfg := configPtr.Load(); cfg != nil {
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

// appJSFetchTimeout 首次下载 app.js 的最长等待时间。
// 并发批次启动前由 main 预热, 这里兜底防止慢代理把整批任务卡在启动阶段。
const appJSFetchTimeout = 15 * time.Second

func fetchAppJS(proxy, chromeVer, userAgent, secUA string) string {
	ch := make(chan string, 1)
	go func() {
		client := httputil.NewTLSClient(proxy, true, chromeVer)
		req, _ := fhttp.NewRequest("GET", "https://us-east-1.signin.aws/assets/js/app.js", nil)
		httputil.SetHeaders(req, appJSRequestHeaders(chromeVer, userAgent, secUA))
		resp, err := client.Do(req)
		if err != nil {
			log.Printf("[xxtea] 下载 app.js 失败: %v", err)
			return
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		ch <- string(b)
	}()
	select {
	case js := <-ch:
		return js
	case <-time.After(appJSFetchTimeout):
		log.Printf("[xxtea] 下载 app.js 超时 (%s), 使用 fallback 密钥", appJSFetchTimeout)
		return ""
	}
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
	re := regexp.MustCompile(`var\s+\w+\s*=\s*\[(\d+),\s*"([A-Za-z0-9]+)",\s*(\d+),\s*(\d+),\s*(\d+)\]`)
	m := re.FindStringSubmatch(js)
	if len(m) == 6 {
		nums := make([]uint32, 4)
		for i, idx := range []int{1, 3, 4, 5} {
			v, _ := strconv.ParseUint(m[idx], 10, 32)
			nums[i] = uint32(v)
		}
		k := [4]uint32{nums[2], nums[0], nums[3], nums[1]}
		key = &k
		ident = m[2]
	}
	reVer := regexp.MustCompile(`FWCIM_VERSION\s*=\s*"(\d+\.\d+\.\d+)"`)
	vm := reVer.FindStringSubmatch(js)
	if len(vm) == 2 {
		ver = vm[1]
	}
	return key, ident, ver
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
