package email

import (
	"os"
	"time"

	"reg_go/internal/storage"
)

func isOutlookAccount(account map[string]interface{}) bool {
	provider, _ := account["provider"].(string)
	return provider == "" || provider == "outlook"
}

// AddOutlookAccounts 添加 Outlook 账号到持久化存储
func AddOutlookAccounts(data string) map[string]interface{} {
	accounts := ParseOutlookLines(data)
	if len(accounts) == 0 {
		return map[string]interface{}{"error": "未解析到有效账号"}
	}

	addedCount := 0
	now := time.Now().Format("2006-01-02 15:04:05")
	storage.ModifyAccountsCached(func(existing []map[string]interface{}) []map[string]interface{} {
		for _, acc := range accounts {
			exists := false
			for _, e := range existing {
				if isOutlookAccount(e) && e["email"] == acc.Email {
					exists = true
					break
				}
			}
			if !exists {
				existing = append(existing, map[string]interface{}{
					"email":        acc.Email,
					"password":     acc.Password,
					"clientId":     acc.ClientID,
					"refreshToken": acc.RefreshToken,
					"mode":         acc.mailMode(),
					"registered":   false,
					"success":      false,
					"addedAt":      now,
				})
				addedCount++
			}
		}
		return existing
	})

	return map[string]interface{}{
		"added": addedCount,
		"total": len(GetOutlookAccounts()),
	}
}

// GetOutlookAccounts 获取 Outlook 账号列表
func GetOutlookAccounts() []map[string]interface{} {
	out := make([]map[string]interface{}, 0)
	for _, account := range storage.GetAccountsCached() {
		if isOutlookAccount(account) {
			out = append(out, account)
		}
	}
	return out
}

// UpdateAccountStatus 更新账号注册状态（纯内存操作，异步刷盘）
func UpdateAccountStatus(email string, registered bool, success bool) map[string]interface{} {
	found := false
	now := time.Now().Format("2006-01-02 15:04:05")
	storage.ModifyAccountsCached(func(accounts []map[string]interface{}) []map[string]interface{} {
		for i, acc := range accounts {
			if isOutlookAccount(acc) && acc["email"] == email {
				accounts[i]["registered"] = registered
				accounts[i]["success"] = success
				accounts[i]["registeredAt"] = now
				found = true
				break
			}
		}
		return accounts
	})
	if !found {
		return map[string]interface{}{"error": "账号不存在"}
	}
	return map[string]interface{}{"status": "updated"}
}

// DeleteOutlookAccount 删除单个 Outlook 账号（纯内存操作，异步刷盘）
func DeleteOutlookAccount(email string) map[string]interface{} {
	found := false
	newLen := 0
	storage.ModifyAccountsCached(func(accounts []map[string]interface{}) []map[string]interface{} {
		newAccounts := make([]map[string]interface{}, 0, len(accounts))
		for _, acc := range accounts {
			if isOutlookAccount(acc) && acc["email"] == email {
				found = true
				continue
			}
			newAccounts = append(newAccounts, acc)
		}
		for _, account := range newAccounts {
			if isOutlookAccount(account) {
				newLen++
			}
		}
		return newAccounts
	})
	if !found {
		return map[string]interface{}{"error": "账号不存在"}
	}
	return map[string]interface{}{
		"status": "deleted",
		"total":  newLen,
	}
}

// ClearOutlookAccounts 清空所有 Outlook 账号
func ClearOutlookAccounts() map[string]interface{} {
	storage.ModifyAccountsCached(func(accounts []map[string]interface{}) []map[string]interface{} {
		out := make([]map[string]interface{}, 0, len(accounts))
		for _, account := range accounts {
			if !isOutlookAccount(account) {
				out = append(out, account)
			}
		}
		return out
	})
	return map[string]interface{}{"status": "cleared"}
}

// ClearRegisteredOutlookAccounts 仅清除已标记为已注册的账号（成功/失败均算）
func ClearRegisteredOutlookAccounts() map[string]interface{} {
	removed := 0
	newLen := 0
	storage.ModifyAccountsCached(func(accounts []map[string]interface{}) []map[string]interface{} {
		out := make([]map[string]interface{}, 0, len(accounts))
		for _, acc := range accounts {
			if reg, _ := acc["registered"].(bool); isOutlookAccount(acc) && reg {
				removed++
				continue
			}
			out = append(out, acc)
		}
		for _, account := range out {
			if isOutlookAccount(account) {
				newLen++
			}
		}
		return out
	})
	return map[string]interface{}{"status": "ok", "removed": removed, "total": newLen}
}

// ImportOutlookFile 导入 Outlook 账号文件
func ImportOutlookFile(filePath string) map[string]interface{} {
	if filePath == "" {
		return map[string]interface{}{"error": "未选择文件"}
	}

	// 读取文件内容
	data, err := os.ReadFile(filePath)
	if err != nil {
		return map[string]interface{}{"error": "读取文件失败: " + err.Error()}
	}

	// 使用现有的解析和添加逻辑
	return AddOutlookAccounts(string(data))
}
