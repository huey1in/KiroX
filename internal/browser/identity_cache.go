package browser

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math/rand"
	stdurl "net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"reg_go/internal/storage"
)

// 默认缓存有效期：6 小时
var identityCacheTTLHours atomic.Int64

func init() {
	identityCacheTTLHours.Store(6)
}

// SetIdentityCacheTTLHours changes the TTL used for subsequent cache lookups.
func SetIdentityCacheTTLHours(hours int) {
	if hours < 1 || hours > 168 {
		hours = 6
	}
	identityCacheTTLHours.Store(int64(hours))
}

type cachedIdentity struct {
	Identity  *BrowserIdentity `json:"identity"`
	CreatedAt int64            `json:"createdAt"`
}

var (
	idCacheMu sync.Mutex
	idCache   map[string]cachedIdentity
)

func identityCachePath() string {
	return filepath.Join(storage.GetCacheDir(), "identities.json")
}

// proxyKey 把代理 URL 归一化为稳定 key：仅保留 host:port，去掉用户名密码、scheme 路径。
// 空字符串（直连）也是一个合法 key，所有直连账号共享同一身份。
func proxyKey(proxyURL string) string {
	proxyURL = strings.TrimSpace(proxyURL)
	if proxyURL == "" {
		return "direct"
	}
	u, err := stdurl.Parse(proxyURL)
	if err != nil || u.Host == "" {
		// 解析失败时整串 hash，避免泄漏密码到磁盘
		sum := sha256.Sum256([]byte(proxyURL))
		return "raw:" + hex.EncodeToString(sum[:8])
	}
	return strings.ToLower(u.Host)
}

func loadIdentityCacheLocked() {
	if idCache != nil {
		return
	}
	idCache = map[string]cachedIdentity{}
	b, err := os.ReadFile(identityCachePath())
	if err != nil {
		return
	}
	var m map[string]cachedIdentity
	if json.Unmarshal(b, &m) == nil && m != nil {
		idCache = m
	}
}

func saveIdentityCacheLocked() {
	if idCache == nil {
		return
	}
	b, err := json.Marshal(idCache)
	if err != nil {
		return
	}
	os.MkdirAll(filepath.Dir(identityCachePath()), 0o755)
	tmp := identityCachePath() + ".tmp"
	if os.WriteFile(tmp, b, 0o600) == nil {
		_ = os.Rename(tmp, identityCachePath())
	}
}

// IdentityForProxy 返回与代理绑定的稳定身份；同一代理 6 小时内复用同一硬件指纹。
// 每次调用都会刷新 lsubid 前缀和 webpack hash —— 这两个在真实浏览器同一台机器上每次会话也会变。
func IdentityForProxy(proxyURL string) *BrowserIdentity {
	return cachedIdentityForProxy(proxyURL, true)
}

// StableIdentityForProxy reuses the complete cached identity, including session fields.
func StableIdentityForProxy(proxyURL string) *BrowserIdentity {
	return cachedIdentityForProxy(proxyURL, false)
}

// IdentityForOffsets resamples each fingerprint domain independently according
// to the curve values: browser, hardware, display, rendering, and session.
func IdentityForOffsets(proxyURL string, offsets []int) *BrowserIdentity {
	values := normalizeIdentityOffsets(offsets)
	allFresh := true
	selected := [5]bool{}
	selectedCount := 0
	for _, value := range values {
		if value != 100 {
			allFresh = false
		}
	}
	if allFresh {
		return RandomIdentity()
	}
	for i, value := range values {
		selected[i] = resampleIdentityDomain(value)
		if selected[i] {
			selectedCount++
		}
	}

	identity := StableIdentityForProxy(proxyURL)
	if selectedCount == 0 {
		return identity
	}
	if selectedCount == 1 && selected[4] {
		return refreshVolatile(identity)
	}
	fresh := RandomIdentity()
	if selected[0] {
		identity.ChromeVer = fresh.ChromeVer
		identity.UA = fresh.UA
		identity.SecUA = fresh.SecUA
		identity.Plugins = fresh.Plugins
	}
	if selected[1] {
		identity.GPUVendor = fresh.GPUVendor
		identity.GPUModel = fresh.GPUModel
		identity.WebGLExts = fresh.WebGLExts
		identity.DeviceMemory = fresh.DeviceMemory
		identity.HardwareConcurrency = fresh.HardwareConcurrency
		identity.Platform = fresh.Platform
	}
	if selected[2] {
		identity.Screen = fresh.Screen
		identity.TimezoneHours = fresh.TimezoneHours
	}
	if selected[3] {
		identity.CanvasHash = fresh.CanvasHash
		identity.HistogramBase = fresh.HistogramBase
		identity.MathTan = fresh.MathTan
		identity.MathSin = fresh.MathSin
		identity.MathCos = fresh.MathCos
	}
	if selected[4] {
		identity.LsubidPrefixSignin = fresh.LsubidPrefixSignin
		identity.LsubidPrefixProfile = fresh.LsubidPrefixProfile
		identity.WebpackHash = fresh.WebpackHash
	}
	return identity
}

func normalizeIdentityOffsets(offsets []int) [5]int {
	values := [5]int{0, 0, 0, 15, 100}
	if len(offsets) != len(values) {
		return values
	}
	for i, value := range offsets {
		if value < 0 {
			value = 0
		} else if value > 100 {
			value = 100
		}
		values[i] = value
	}
	return values
}

func resampleIdentityDomain(intensity int) bool {
	return intensity >= 100 || (intensity > 0 && rand.Intn(100) < intensity)
}

func cachedIdentityForProxy(proxyURL string, refreshSession bool) *BrowserIdentity {
	key := proxyKey(proxyURL)

	idCacheMu.Lock()
	defer idCacheMu.Unlock()
	loadIdentityCacheLocked()

	now := time.Now().Unix()
	if entry, ok := idCache[key]; ok && entry.Identity != nil {
		if now-entry.CreatedAt < int64((time.Duration(identityCacheTTLHours.Load()) * time.Hour).Seconds()) {
			if refreshSession {
				return refreshVolatile(entry.Identity)
			}
			return cloneIdentity(entry.Identity)
		}
	}

	id := RandomIdentity()
	idCache[key] = cachedIdentity{Identity: id, CreatedAt: now}
	saveIdentityCacheLocked()
	if refreshSession {
		return refreshVolatile(id)
	}
	return cloneIdentity(id)
}

func cloneIdentity(base *BrowserIdentity) *BrowserIdentity {
	clone := *base
	return &clone
}

// refreshVolatile 复制身份并刷新少数每次会话都会变的字段。
// 这样硬件指纹（UA / Chrome 版本 / GPU / 屏幕 / Math / Canvas / 内存 / 核数）保持稳定，
// 只有真实浏览器每次会话也变的 lsubid / webpackHash 重新随机。
func refreshVolatile(base *BrowserIdentity) *BrowserIdentity {
	clone := *base
	clone.LsubidPrefixSignin = lsubidPrefixes[rand.Intn(len(lsubidPrefixes))]
	clone.LsubidPrefixProfile = lsubidPrefixes[rand.Intn(len(lsubidPrefixes))]
	// webpack hash 同样每次刷新
	hashRaw := sha256.Sum256([]byte(clone.UA))
	hexed := hex.EncodeToString(hashRaw[:])
	// 截取首 10 位，并掺入随机偏移以模拟版本间小幅滚动
	clone.WebpackHash = hexed[rand.Intn(20):][:10]
	return &clone
}

// ResetIdentityCache 清空缓存（用于「强制刷新指纹」按钮，未来可选）
func ResetIdentityCache() {
	idCacheMu.Lock()
	defer idCacheMu.Unlock()
	idCache = map[string]cachedIdentity{}
	_ = os.Remove(identityCachePath())
}
