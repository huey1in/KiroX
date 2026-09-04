package proxy

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// PoolEntry 多代理池条目
type PoolEntry struct {
	ID      string `json:"id"`      // 内部 ID，UI 用
	Name    string `json:"name"`    // 用户可见名称
	URL     string `json:"url"`     // 完整代理 URL（已归一化）
	Weight  int    `json:"weight"`  // 1-100，越高被选中概率越大
	Enabled bool   `json:"enabled"` // 关闭时不参与抽签

	// 最近一次探测结果（持久化，供 IP 管理页展示）
	ProbeOK          bool   `json:"probeOk"`                    // 探测是否成功
	ProbeIP          string `json:"probeIp"`                    // 出口 IP
	ProbeCountry     string `json:"probeCountry,omitempty"`     // 国家
	ProbeCountryCode string `json:"probeCountryCode,omitempty"` // 国家代码（用于国旗）
	ProbeRegion      string `json:"probeRegion,omitempty"`      // 省份
	ProbeCity        string `json:"probeCity,omitempty"`        // 城市
	ProbeISP         string `json:"probeIsp,omitempty"`         // ISP
	ProbeType        string `json:"probeType,omitempty"`        // residential / mobile / datacenter
	ProbeMS          int    `json:"probeMs,omitempty"`          // 延迟(ms)
	ProbeError       string `json:"probeError,omitempty"`       // 失败原因
	ProbeAt          int64  `json:"probeAt,omitempty"`          // 探测时间戳(unix秒)
}

// poolFile JSON 持久化结构
type poolFile struct {
	Entries []PoolEntry `json:"entries"`
}

const (
	// Power 用于"软最大化"：>1 时拉大权重差，<1 时压平。0.6 保证哪怕权重 1 vs 100 也有 ~6% 概率被选中。
	weightPower = 0.6
)

var (
	poolMu      sync.Mutex
	poolLoaded  bool
	poolEntries []PoolEntry
	poolPath    string
)

// InitPool 在 App 启动时调用一次，传入数据目录
func InitPool(dataDir string) {
	poolMu.Lock()
	defer poolMu.Unlock()
	poolPath = filepath.Join(dataDir, "proxy_pool.json")
	poolLoaded = false
	_ = loadPoolLocked()
}

func loadPoolLocked() error {
	if poolLoaded {
		return nil
	}
	poolEntries = nil
	poolLoaded = true
	b, err := os.ReadFile(poolPath)
	if err != nil {
		return nil
	}
	var pf poolFile
	if err := json.Unmarshal(b, &pf); err != nil {
		return fmt.Errorf("解析代理池失败: %w", err)
	}
	poolEntries = pf.Entries
	return nil
}

func savePoolLocked() error {
	if poolPath == "" {
		return fmt.Errorf("代理池未初始化")
	}
	b, err := json.MarshalIndent(poolFile{Entries: poolEntries}, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(poolPath), 0o755); err != nil {
		return err
	}
	tmp := poolPath + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, poolPath)
}

// Get 按 id 返回单个代理
func Get(id string) (PoolEntry, error) {
	poolMu.Lock()
	defer poolMu.Unlock()
	loadPoolLocked()
	for _, e := range poolEntries {
		if e.ID == id {
			return e, nil
		}
	}
	return PoolEntry{}, fmt.Errorf("代理不存在")
}

// List 返回当前所有代理（含禁用项）
func List() []PoolEntry {
	poolMu.Lock()
	defer poolMu.Unlock()
	loadPoolLocked()
	out := make([]PoolEntry, len(poolEntries))
	copy(out, poolEntries)
	return out
}

func newID() string {
	return fmt.Sprintf("p_%d_%04d", time.Now().UnixNano(), rand.Intn(10000))
}

// Add 新增一条代理。url 会被外部归一化后传入。
func Add(entry PoolEntry) (PoolEntry, error) {
	entry.URL = strings.TrimSpace(entry.URL)
	if entry.URL == "" {
		return entry, fmt.Errorf("代理地址不能为空")
	}
	if entry.Weight <= 0 {
		entry.Weight = 50
	}
	if entry.Weight > 100 {
		entry.Weight = 100
	}
	entry.Name = strings.TrimSpace(entry.Name)
	if entry.Name == "" {
		entry.Name = entry.URL
	}
	poolMu.Lock()
	defer poolMu.Unlock()
	loadPoolLocked()
	for _, e := range poolEntries {
		if e.URL == entry.URL {
			return entry, fmt.Errorf("该代理已存在")
		}
	}
	if entry.ID == "" {
		entry.ID = newID()
	}
	entry.Enabled = true
	poolEntries = append(poolEntries, entry)
	if err := savePoolLocked(); err != nil {
		// 回滚
		poolEntries = poolEntries[:len(poolEntries)-1]
		return entry, err
	}
	return entry, nil
}

// Update 修改一条（按 id 匹配）。url 不允许改成已存在的另一条。
func Update(id string, patch PoolEntry) (PoolEntry, error) {
	poolMu.Lock()
	defer poolMu.Unlock()
	loadPoolLocked()
	idx := -1
	for i, e := range poolEntries {
		if e.ID == id {
			idx = i
			break
		}
	}
	if idx < 0 {
		return PoolEntry{}, fmt.Errorf("代理不存在")
	}
	cur := poolEntries[idx]
	if name := strings.TrimSpace(patch.Name); name != "" {
		cur.Name = name
	}
	if u := strings.TrimSpace(patch.URL); u != "" && u != cur.URL {
		for j, e := range poolEntries {
			if j != idx && e.URL == u {
				return PoolEntry{}, fmt.Errorf("该代理 URL 已存在")
			}
		}
		cur.URL = u
	}
	if patch.Weight > 0 {
		w := patch.Weight
		if w > 100 {
			w = 100
		}
		cur.Weight = w
	}
	cur.Enabled = patch.Enabled
	poolEntries[idx] = cur
	if err := savePoolLocked(); err != nil {
		return PoolEntry{}, err
	}
	return cur, nil
}

// Delete 按 id 删除
func Delete(id string) error {
	poolMu.Lock()
	defer poolMu.Unlock()
	loadPoolLocked()
	for i, e := range poolEntries {
		if e.ID == id {
			poolEntries = append(poolEntries[:i], poolEntries[i+1:]...)
			return savePoolLocked()
		}
	}
	return fmt.Errorf("代理不存在")
}

// SetProbe 保存某条代理的探测结果（按 id），并持久化到 JSON
func SetProbe(id string, info Info, ms int) error {
	poolMu.Lock()
	defer poolMu.Unlock()
	loadPoolLocked()
	for i := range poolEntries {
		if poolEntries[i].ID == id {
			poolEntries[i].ProbeOK = info.OK
			poolEntries[i].ProbeIP = info.IP
			poolEntries[i].ProbeCountry = info.Country
			poolEntries[i].ProbeCountryCode = info.CountryCode
			poolEntries[i].ProbeRegion = info.Region
			poolEntries[i].ProbeCity = info.City
			poolEntries[i].ProbeISP = info.ISP
			poolEntries[i].ProbeType = info.ProxyType
			poolEntries[i].ProbeMS = ms
			poolEntries[i].ProbeError = info.Error
			poolEntries[i].ProbeAt = time.Now().Unix()
			return savePoolLocked()
		}
	}
	return fmt.Errorf("代理不存在")
}

// PickRandom 按权重抽签返回一个启用的代理 URL；池为空或全部禁用返回空串。
// 使用 weightPower 软化：让低权重也有非零概率被命中，避免全部任务落到单一代理。
func PickRandom() string {
	poolMu.Lock()
	defer poolMu.Unlock()
	loadPoolLocked()

	type cand struct {
		url  string
		soft float64
	}
	candidates := make([]cand, 0, len(poolEntries))
	var total float64
	for _, e := range poolEntries {
		if e.URL == "" {
			continue
		}
		w := e.Weight
		if w <= 0 {
			w = 1
		}
		soft := math.Pow(float64(w), weightPower)
		candidates = append(candidates, cand{e.URL, soft})
		total += soft
	}
	if total <= 0 || len(candidates) == 0 {
		return ""
	}
	r := rand.Float64() * total
	for _, c := range candidates {
		r -= c.soft
		if r <= 0 {
			return c.url
		}
	}
	return candidates[len(candidates)-1].url
}
