package email

import (
	"os"
	"strings"
	"time"

	"reg_go/internal/storage"
)

// AddICloudAccounts 添加 iCloud 账号到持久化存储（邮箱池页粘贴 CSV）
func AddICloudAccounts(data string) map[string]interface{} {
	accounts, err := ParseICloudCSV(strings.NewReader(data))
	if err != nil {
		return map[string]interface{}{"error": err.Error()}
	}
	if len(accounts) == 0 {
		return map[string]interface{}{"error": "未解析到有效账号（格式：邮箱----取件URL，每行一个）"}
	}

	addedCount := 0
	now := time.Now().Format("2006-01-02 15:04:05")
	storage.ModifyAccountsCached(func(existing []map[string]interface{}) []map[string]interface{} {
		for _, acc := range accounts {
			exists := false
			for _, e := range existing {
				if e["provider"] == "icloud" && e["email"] == acc.Email {
					exists = true
					break
				}
			}
			if !exists {
				existing = append(existing, map[string]interface{}{
					"provider":   "icloud",
					"email":      acc.Email,
					"messagesURL": acc.MessagesURL,
					"registered": false,
					"success":    false,
					"addedAt":    now,
				})
				addedCount++
			}
		}
		return existing
	})

	return map[string]interface{}{
		"added": addedCount,
		"total": len(GetICloudAccounts()),
	}
}

// GetICloudAccounts 获取 iCloud 账号列表
func GetICloudAccounts() []map[string]interface{} {
	var out []map[string]interface{}
	for _, acc := range storage.GetAccountsCached() {
		if p, _ := acc["provider"].(string); p == "icloud" {
			out = append(out, acc)
		}
	}
	return out
}

// DeleteICloudAccount 删除单个 iCloud 账号
func DeleteICloudAccount(email string) map[string]interface{} {
	found := false
	storage.ModifyAccountsCached(func(accounts []map[string]interface{}) []map[string]interface{} {
		out := make([]map[string]interface{}, 0, len(accounts))
		for _, acc := range accounts {
			if p, _ := acc["provider"].(string); p == "icloud" {
				if e, _ := acc["email"].(string); e == email {
					found = true
					continue
				}
			}
			out = append(out, acc)
		}
		return out
	})
	if !found {
		return map[string]interface{}{"error": "账号不存在"}
	}
	return map[string]interface{}{"status": "deleted", "total": len(GetICloudAccounts())}
}

// ClearICloudAccounts 清空所有 iCloud 账号
func ClearICloudAccounts() map[string]interface{} {
	removed := 0
	storage.ModifyAccountsCached(func(accounts []map[string]interface{}) []map[string]interface{} {
		out := make([]map[string]interface{}, 0, len(accounts))
		for _, acc := range accounts {
			if p, _ := acc["provider"].(string); p == "icloud" {
				removed++
				continue
			}
			out = append(out, acc)
		}
		return out
	})
	return map[string]interface{}{"status": "ok", "removed": removed, "total": len(GetICloudAccounts())}
}

// ImportICloudFile 导入 iCloud 账号文件
func ImportICloudFile(filePath string) map[string]interface{} {
	if filePath == "" {
		return map[string]interface{}{"error": "未选择文件"}
	}
	data, err := os.ReadFile(filePath)
	if err != nil {
		return map[string]interface{}{"error": "读取文件失败: " + err.Error()}
	}
	return AddICloudAccounts(string(data))
}

// MarkICloudAccountRegistered 标记 iCloud 账号为已注册（注册成功后调用）
func MarkICloudAccountRegistered(email string) map[string]interface{} {
	found := false
	now := time.Now().Format("2006-01-02 15:04:05")
	storage.ModifyAccountsCached(func(accounts []map[string]interface{}) []map[string]interface{} {
		for i, acc := range accounts {
			if p, _ := acc["provider"].(string); p == "icloud" {
				if e, _ := acc["email"].(string); e == email {
					accounts[i]["registered"] = true
					accounts[i]["success"] = true
					accounts[i]["registeredAt"] = now
					found = true
					break
				}
			}
		}
		return accounts
	})
	if !found {
		return map[string]interface{}{"error": "账号不存在"}
	}
	return map[string]interface{}{"status": "updated"}
}
