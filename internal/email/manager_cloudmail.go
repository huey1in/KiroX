package email

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"reg_go/internal/storage"
)

// getCloudMailConfigPath 配置文件路径
func getCloudMailConfigPath() string {
	return filepath.Join(storage.GetDataDir(), "cloudmail.json")
}

// GetCloudMailConfigs 读取配置列表
func GetCloudMailConfigs() []CloudMailConfig {
	data, err := os.ReadFile(getCloudMailConfigPath())
	if err != nil {
		return []CloudMailConfig{}
	}

	var configs []CloudMailConfig
	if err := json.Unmarshal(data, &configs); err != nil {
		log.Printf("[CloudMail] 配置文件格式无效，已重置: %v", err)
		os.Remove(getCloudMailConfigPath())
		return []CloudMailConfig{}
	}
	return configs
}

// SaveCloudMailConfigs 保存配置列表
func SaveCloudMailConfigs(configsJSON string) map[string]interface{} {
	var configs []CloudMailConfig
	if err := json.Unmarshal([]byte(configsJSON), &configs); err != nil {
		return map[string]interface{}{"error": "配置格式错误: " + err.Error()}
	}

	for i, cfg := range configs {
		if cfg.Name == "" {
			return map[string]interface{}{"error": fmt.Sprintf("第 %d 个配置缺少名称", i+1)}
		}
		if cfg.URL == "" {
			return map[string]interface{}{"error": fmt.Sprintf("配置 %s 缺少 URL", cfg.Name)}
		}
		if cfg.Email == "" {
			return map[string]interface{}{"error": fmt.Sprintf("配置 %s 缺少管理员邮箱", cfg.Name)}
		}
		if cfg.Password == "" {
			return map[string]interface{}{"error": fmt.Sprintf("配置 %s 缺少密码", cfg.Name)}
		}
		// 域名字段可选：未填则注册时自动从 /api/setting/websiteConfig 拉取
	}

	jsonData, _ := json.Marshal(configs)
	os.MkdirAll(filepath.Dir(getCloudMailConfigPath()), 0755)
	if err := os.WriteFile(getCloudMailConfigPath(), jsonData, 0600); err != nil {
		return map[string]interface{}{"error": "保存失败: " + err.Error()}
	}

	log.Printf("[CloudMail] 已保存 %d 个配置", len(configs))
	return map[string]interface{}{"success": true}
}

// TestCloudMailConnection 测试配置：genToken + websiteConfig + emailList 探活，并回显服务器域名
func TestCloudMailConnection(configJSON string) map[string]interface{} {
	var config CloudMailConfig
	if err := json.Unmarshal([]byte(configJSON), &config); err != nil {
		return map[string]interface{}{"error": "配置格式错误: " + err.Error()}
	}

	client := NewCloudMailClient(config)
	domains, err := client.TestConnection()
	if err != nil {
		return map[string]interface{}{"error": "连接失败: " + err.Error()}
	}

	// 优先使用服务器返回的域名；为空则回退到用户填写的
	if len(domains) == 0 {
		domains = config.Domains
	}

	return map[string]interface{}{
		"success":     true,
		"domains":     domains,
		"domainCount": len(domains),
	}
}
