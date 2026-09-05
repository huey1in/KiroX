package email

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"reg_go/internal/storage"
)

// getMoeMailConfigPath 获取 MoeMail 配置文件路径
func getMoeMailConfigPath() string {
	return filepath.Join(storage.GetDataDir(), "moemail.json")
}

// GetMoeMailConfigs 获取 MoeMail 配置列表
func GetMoeMailConfigs() []MoeMailConfig {
	data, err := os.ReadFile(getMoeMailConfigPath())
	if err != nil {
		return []MoeMailConfig{}
	}

	var configs []MoeMailConfig
	if err := json.Unmarshal(data, &configs); err != nil {
		log.Printf("[MoeMail] 配置文件格式无效，已重置: %v", err)
		os.Remove(getMoeMailConfigPath())
		return []MoeMailConfig{}
	}

	return configs
}

// SaveMoeMailConfigs 保存 MoeMail 配置列表
func SaveMoeMailConfigs(configsJSON string) map[string]interface{} {
	var configs []MoeMailConfig
	if err := json.Unmarshal([]byte(configsJSON), &configs); err != nil {
		return map[string]interface{}{"error": "配置格式错误: " + err.Error()}
	}

	// 验证配置
	for i, cfg := range configs {
		if cfg.Name == "" {
			return map[string]interface{}{"error": fmt.Sprintf("第 %d 个配置缺少名称", i+1)}
		}
		if cfg.URL == "" {
			return map[string]interface{}{"error": fmt.Sprintf("配置 %s 缺少 URL", cfg.Name)}
		}
		if cfg.APIKey == "" {
			return map[string]interface{}{"error": fmt.Sprintf("配置 %s 缺少 API Key", cfg.Name)}
		}
	}

	jsonData, _ := json.Marshal(configs)
	os.MkdirAll(filepath.Dir(getMoeMailConfigPath()), 0755)
	if err := os.WriteFile(getMoeMailConfigPath(), jsonData, 0600); err != nil {
		return map[string]interface{}{"error": "保存失败: " + err.Error()}
	}

	log.Printf("[MoeMail] 已保存 %d 个配置", len(configs))
	return map[string]interface{}{"success": true}
}

// TestMoeMailConnection 测试 MoeMail 连接
func TestMoeMailConnection(configJSON string) map[string]interface{} {
	var config MoeMailConfig
	if err := json.Unmarshal([]byte(configJSON), &config); err != nil {
		return map[string]interface{}{"error": "配置格式错误: " + err.Error()}
	}

	client := NewMoeMailClient(config)
	sysConfig, err := client.GetSystemConfig()
	if err != nil {
		return map[string]interface{}{"error": "连接失败: " + err.Error()}
	}

	return map[string]interface{}{
		"success":     true,
		"domains":     sysConfig.Domains,
		"domainCount": len(sysConfig.Domains),
	}
}
