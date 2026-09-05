package data

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

var accountsFileMutexes sync.Map

func accountsFileMutex(path string) *sync.RWMutex {
	if absolute, err := filepath.Abs(path); err == nil {
		path = absolute
	}
	path = filepath.Clean(path)
	if runtime.GOOS == "windows" {
		path = strings.ToLower(path)
	}
	mu, _ := accountsFileMutexes.LoadOrStore(path, &sync.RWMutex{})
	return mu.(*sync.RWMutex)
}

// SaveKiroSuccess 以明文 JSON 数组形式把成功注册的账号写入 outDir/accounts.json。
// 同邮箱以最新一条覆盖；仅处理成功记录（失败/封号不落盘，只留在运行日志）。
func SaveKiroSuccess(result map[string]interface{}, outDir string) error {
	if result == nil || result["status"] != "success" {
		return nil
	}
	emailAddr, _ := result["email"].(string)
	if emailAddr == "" {
		return fmt.Errorf("缺少 email 字段")
	}

	at, _ := result["aws_token"].(map[string]interface{})
	if at == nil {
		at = map[string]interface{}{}
	}
	verify, _ := result["verify"].(map[string]interface{})
	item := map[string]interface{}{
		"refreshToken": at["refreshToken"],
		"provider":     "BuilderId",
		"clientId":     result["client_id"],
		"clientSecret": result["client_secret"],
		"region":       "us-east-1",
		"email":        emailAddr,
		"time":         time.Now().Format("2006-01-02 15:04:05"),
	}
	if verify != nil {
		item["creditUsed"] = verify["credit_used"]
		item["creditLimit"] = verify["credit_limit"]
	}

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("创建输出目录失败: %w", err)
	}
	path := filepath.Join(outDir, "accounts.json")
	// Keep the full read-merge-write operation exclusive for this output file.
	mu := accountsFileMutex(path)
	mu.Lock()
	defer mu.Unlock()

	existing, err := loadJSONArray(path)
	if err != nil {
		return fmt.Errorf("读取 accounts.json 失败: %w", err)
	}

	merged := make([]map[string]interface{}, 0, len(existing)+1)
	for _, e := range existing {
		if em, _ := e["email"].(string); em == emailAddr {
			continue
		}
		merged = append(merged, e)
	}
	merged = append(merged, item)

	if err := writeJSONArrayAtomic(path, merged); err != nil {
		return fmt.Errorf("写入 accounts.json 失败: %w", err)
	}
	log.Printf("[Kiro] 结果已保存: %s", path)
	return nil
}

// LoadAccounts 读取 outDir/accounts.json 中保存的账号列表（按写入顺序返回）。
func LoadAccounts(outDir string) ([]map[string]interface{}, error) {
	path := filepath.Join(outDir, "accounts.json")
	mu := accountsFileMutex(path)
	mu.RLock()
	defer mu.RUnlock()
	return loadJSONArray(path)
}

// DeleteAccount 从 outDir/accounts.json 中移除指定邮箱的账号；返回是否实际删除。
func DeleteAccount(outDir, email string) (bool, error) {
	path := filepath.Join(outDir, "accounts.json")
	mu := accountsFileMutex(path)
	mu.Lock()
	defer mu.Unlock()
	existing, err := loadJSONArray(path)
	if err != nil || len(existing) == 0 {
		return false, err
	}
	out := make([]map[string]interface{}, 0, len(existing))
	removed := false
	for _, e := range existing {
		if em, _ := e["email"].(string); em == email {
			removed = true
			continue
		}
		out = append(out, e)
	}
	if !removed {
		return false, nil
	}
	if err := writeJSONArrayAtomic(path, out); err != nil {
		return false, err
	}
	return true, nil
}

func loadJSONArray(path string) ([]map[string]interface{}, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	if len(b) == 0 {
		return nil, nil
	}
	var arr []map[string]interface{}
	if err := json.Unmarshal(b, &arr); err != nil {
		return nil, err
	}
	return arr, nil
}

func writeJSONArrayAtomic(path string, arr []map[string]interface{}) error {
	b, err := json.MarshalIndent(arr, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
