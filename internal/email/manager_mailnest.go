package email

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"

	"reg_go/internal/storage"
)

func getMailNestConfigPath() string {
	return filepath.Join(storage.GetDataDir(), "mailnest.json")
}

func TestMailNestConnection(configJSON string) map[string]interface{} {
	var config MailNestConfig
	if err := json.Unmarshal([]byte(configJSON), &config); err != nil {
		return map[string]interface{}{"error": "配置格式错误: " + err.Error()}
	}

	client := NewMailNestClient(config)
	balance, err := client.GetBalance()
	if err != nil {
		return map[string]interface{}{"error": "连接失败: " + err.Error()}
	}

	return map[string]interface{}{
		"success": true,
		"balance": balance.Balance,
	}
}

func SaveMailNestConfig(jsonData string) map[string]interface{} {
	path := getMailNestConfigPath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return map[string]interface{}{"error": "保存失败: " + err.Error()}
	}
	if err := os.WriteFile(path, []byte(jsonData), 0600); err != nil {
		return map[string]interface{}{"error": "保存失败: " + err.Error()}
	}
	log.Printf("[MailNest] config saved")
	return map[string]interface{}{"success": true}
}

func GetMailNestConfig() MailNestConfig {
	data, err := os.ReadFile(getMailNestConfigPath())
	if err != nil {
		return MailNestConfig{}
	}
	var config MailNestConfig
	if err := json.Unmarshal(data, &config); err != nil {
		log.Printf("[MailNest] invalid config file, resetting: %v", err)
		_ = os.Remove(getMailNestConfigPath())
		return MailNestConfig{}
	}
	return config
}
