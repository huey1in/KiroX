package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"reg_go/internal/core"
	"reg_go/internal/email"
	"reg_go/internal/proxy"
	"reg_go/internal/storage"
	"reg_go/internal/task"
)

var sensitiveLogPatterns = []struct {
	pattern     *regexp.Regexp
	replacement string
}{
	{regexp.MustCompile(`(?i)[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,}`), `<email>`},
	{regexp.MustCompile(`\b\d{6}\b`), `<code>`},
	{regexp.MustCompile(`(?i)(user_code|regCode|workflowState)=[^\s]+`), `$1=<redacted>`},
	{regexp.MustCompile(`https?://[^\s"'<>]+`), `<url>`},
}

type safeLogWriter struct{}

func (safeLogWriter) Write(data []byte) (int, error) {
	message := string(data)
	for _, entry := range sensitiveLogPatterns {
		message = entry.pattern.ReplaceAllString(message, entry.replacement)
	}
	_, err := os.Stdout.WriteString(message)
	return len(data), err
}

type mailCandidate struct {
	config email.MoeMailConfig
	domain string
}

func main() {
	solveMode := flag.Bool("solve", false, "run the captured AMS flow through 2Captcha")
	browserBootstrap := flag.Bool("browser-bootstrap", false, "create a device code and mailbox for browser-side capture")
	attemptLimit := flag.Int("attempts", 0, "maximum registration attempts (defaults to 1 for solve and 6 for capture)")
	proxyStart := flag.Int("proxy-index", 0, "start with this proxy candidate index")
	flag.Parse()
	log.SetFlags(log.Ltime)
	log.SetOutput(safeLogWriter{})

	workingDir, err := os.Getwd()
	if err != nil {
		fatal("无法确定项目目录: %v", err)
	}
	outputDir := filepath.Join(workingDir, "build", "ams-inspect")
	if err := os.Setenv("KIROX_AMS_TRACE", "1"); err != nil {
		fatal("无法启用 AMS 安全跟踪: %v", err)
	}
	if *solveMode {
		if err := os.Unsetenv("KIROX_AMS_INSPECT_DIR"); err != nil {
			fatal("无法关闭 AMS 诊断钩子: %v", err)
		}
	} else {
		if err := os.RemoveAll(outputDir); err != nil {
			fatal("无法清理旧的 AMS 诊断目录: %v", err)
		}
		if err := os.Setenv("KIROX_AMS_INSPECT_DIR", outputDir); err != nil {
			fatal("无法启用 AMS 诊断: %v", err)
		}
	}

	settings := storage.GetAppSettings()
	if !settings.WAFEnabled {
		fatal("当前设置未启用 AWS WAF 动态验证")
	}
	configs := email.GetMoeMailConfigs()
	if len(configs) == 0 {
		fatal("没有可用的 MoeMail 配置")
	}

	var mailCandidates []mailCandidate
	for _, config := range configs {
		systemConfig, configErr := email.NewMoeMailClient(config).GetSystemConfig()
		if configErr != nil {
			continue
		}
		for _, domain := range systemConfig.Domains {
			mailCandidates = append(mailCandidates, mailCandidate{config: config, domain: domain})
		}
	}
	if len(mailCandidates) == 0 {
		fatal("无法从现有 MoeMail 配置取得可用域名")
	}
	sort.SliceStable(mailCandidates, func(i, j int) bool {
		return mailCandidatePriority(mailCandidates[i]) > mailCandidatePriority(mailCandidates[j])
	})
	if *browserBootstrap {
		runBrowserBootstrap(mailCandidates[0])
		return
	}

	proxy.InitPool(storage.GetDataDir())
	proxyCandidates := collectProxyCandidates()
	if len(proxyCandidates) == 0 {
		proxyCandidates = []string{""}
	}

	maxAttempts := 6
	if *solveMode {
		maxAttempts = 1
	}
	if *attemptLimit > 0 {
		maxAttempts = *attemptLimit
	}
	for attempt := 0; attempt < maxAttempts; attempt++ {
		mailIndex := attempt % len(mailCandidates)
		if *solveMode {
			mailIndex = 0
		}
		mail := mailCandidates[mailIndex]
		proxyIndex := (*proxyStart + attempt) % len(proxyCandidates)
		if proxyIndex < 0 {
			proxyIndex += len(proxyCandidates)
		}
		taskProxy := proxyCandidates[proxyIndex]
		modeLabel := "诊断"
		if *solveMode {
			modeLabel = "验证"
		}
		fmt.Printf("AMS %s尝试 %d/%d：%s / %s / %s\n", modeLabel, attempt+1, maxAttempts, mail.config.Name, mail.domain, proxyCandidateLabel(taskProxy))
		start := task.StartTask(task.StartTaskRequest{
			Count:             1,
			Concurrency:       1,
			EmailProvider:     "moemail",
			MoeMailDomains:    []string{mail.domain},
			MoeMailConfigs:    map[string][]email.MoeMailConfig{mail.domain: {mail.config}},
			MoeMailRandomMode: false,
			Proxy:             taskProxy,
			ProxyConfigured:   true,
		})
		if message, ok := start["error"].(string); ok && message != "" {
			fmt.Printf("本次尝试无法启动: %s\n", message)
			continue
		}

		deadline := time.Now().Add(5 * time.Minute)
		for task.Manager.GetStatus()["running"].(bool) {
			if time.Now().After(deadline) {
				task.StopTask(true)
				fatal("等待真实挑战超时")
			}
			time.Sleep(time.Second)
		}

		status := task.Manager.GetStatus()
		if *solveMode {
			if status["success"].(int) > 0 {
				fmt.Println("AMS 完整注册流程验证成功。")
				return
			}
		} else {
			reportPath := filepath.Join(outputDir, "report.json")
			if _, err := os.Stat(reportPath); err == nil {
				fmt.Printf("AMS SDK 已捕获，诊断报告: %s\n", reportPath)
				return
			}
		}
		fmt.Println("本次流程在挑战出现前结束，正在更换配置或代理。")
	}
	if *solveMode {
		fatal("已完成 %d 次验证尝试，但 AMS 注册流程仍未成功", maxAttempts)
	}
	fatal("已完成 %d 次诊断尝试，但仍未捕获到 AMS SDK", maxAttempts)
}

func runBrowserBootstrap(mail mailCandidate) {
	provider, err := email.NewMoeMailProvider(mail.config, email.GenerateEmailName(1), time.Hour.Milliseconds(), mail.domain)
	if err != nil {
		fatal("无法创建浏览器诊断邮箱: %v", err)
	}
	cfg := core.NewConfig()
	r := core.NewRegistrar(cfg)
	defer r.Client.CloseIdleConnections()
	if err := r.Step1OIDC(); err != nil {
		fatal("无法创建 OIDC 客户端: %v", err)
	}
	if err := r.Step2Device(); err != nil {
		fatal("无法创建设备授权: %v", err)
	}
	fmt.Printf("BROWSER_URL=%s/start/#/device?user_code=%s\n", cfg.ViewBase, r.UserCode)
	fmt.Printf("BROWSER_EMAIL=%s\n", provider.GetAddress())
	code, err := provider.WaitForCode(300, 1)
	if err != nil {
		fatal("等待浏览器流程验证码失败: %v", err)
	}
	fmt.Printf("BROWSER_OTP=%s\n", code)
}

func mailCandidatePriority(candidate mailCandidate) int {
	priority := 0
	if strings.EqualFold(candidate.domain, "huey1in.com") {
		priority += 10
	}
	if strings.Contains(candidate.config.Name, "2") {
		priority += 5
	}
	return priority
}

func collectProxyCandidates() []string {
	entries := proxy.List()
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].ProbeOK != entries[j].ProbeOK {
			return entries[i].ProbeOK
		}
		if entries[i].ProbeType != entries[j].ProbeType {
			return entries[i].ProbeType == "residential"
		}
		return entries[i].ProbeMS < entries[j].ProbeMS
	})
	values := make([]string, 0, len(entries)+2)
	for _, entry := range entries {
		if entry.Enabled && entry.ProbeOK && strings.EqualFold(entry.ProbeType, "residential") {
			values = append(values, strings.TrimSpace(entry.URL))
		}
	}
	values = append(values, strings.TrimSpace(storage.GetProxy()))
	for _, entry := range entries {
		if entry.Enabled {
			values = append(values, strings.TrimSpace(entry.URL))
		}
	}
	values = append(values, "")
	seen := make(map[string]bool)
	result := make([]string, 0, len(values))
	for _, value := range values {
		if seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	return result
}

func proxyCandidateLabel(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "直连"
	}
	for _, entry := range proxy.List() {
		if strings.TrimSpace(entry.URL) != value {
			continue
		}
		kind := strings.TrimSpace(entry.ProbeType)
		if kind == "" {
			kind = "类型未知"
		}
		if entry.ProbeOK {
			return kind + "（探测正常）"
		}
		return kind + "（未通过探测）"
	}
	return "全局代理"
}

func fatal(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
