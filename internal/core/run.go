package core

import (
	"errors"
	"log"
	"regexp"
	"strings"
	"time"

	"reg_go/internal/crypto"
)

// urlRegex 用于把日志/错误里的完整 URL 脱敏为 <endpoint>，避免暴露后端地址。
var urlRegex = regexp.MustCompile(`https?://[^\s"'<>]+`)

// scrubURLs 把字符串里所有 http(s):// URL 替换成 <endpoint>。
// 用于向 UI 日志暴露的错误信息,避免泄漏 aws/amazonaws/kiro 等后端域名。
func scrubURLs(s string) string {
	return urlRegex.ReplaceAllString(s, "<endpoint>")
}

// formatError 将技术错误转换为用户友好的错误信息
func (r *Registrar) formatError(step string, err error) string {
	if r.ctxCancelled() {
		return "任务已取消"
	}
	var serviceErr *ServiceError
	if errors.As(err, &serviceErr) {
		if serviceErr.Code == "AUTHENTICATION_FAILED" {
			message := "设置密码失败: AWS 身份验证失败，验证令牌可能无效或已过期"
			if serviceErr.Captcha {
				message += "，响应包含 CAPTCHA 验证信息"
			}
			return message + serviceErrorMetadata(serviceErr)
		}
		message := friendlyStepName(step) + "失败"
		if serviceErr.Message != "" {
			message += ": " + serviceErr.Message
		}
		return message + serviceErrorMetadata(serviceErr)
	}

	errMsg := scrubURLs(err.Error())

	// 网络连接错误
	if strings.Contains(errMsg, "dial tcp") || strings.Contains(errMsg, "connectex") {
		if strings.Contains(errMsg, "refused") {
			return "网络连接失败: 代理服务器拒绝连接，请检查代理设置"
		}
		if strings.Contains(errMsg, "timeout") {
			return "网络连接超时，请检查网络或代理设置"
		}
		return "网络连接失败，请检查网络或代理设置"
	}

	// HTTP 状态码错误
	if strings.Contains(errMsg, "IP或浏览器指纹被检测") {
		return "注册失败: IP或浏览器指纹被检测，请更换代理或重新生成指纹"
	}
	if strings.Contains(errMsg, "请求过于频繁") {
		return "注册失败: 请求过于频繁，请稍后重试"
	}
	if strings.Contains(errMsg, "服务暂时不可用") {
		return "注册失败: AWS 服务暂时不可用，请稍后重试"
	}

	// OTP 相关错误
	if strings.Contains(errMsg, "INVALID_OTP") {
		return "验证码错误: 验证码无效或已过期，请重试"
	}
	if strings.Contains(errMsg, "等待验证码超时") {
		return "验证码接收超时，请检查邮箱服务或稍后重试"
	}

	// 邮箱相关错误
	if strings.Contains(errMsg, "邮箱已注册") {
		return "邮箱已被注册，跳过"
	}
	if strings.Contains(errMsg, "获取邮件失败") {
		return "邮箱服务异常，无法接收验证码"
	}

	// 注册被拦截
	if strings.Contains(errMsg, "BLOCKED") || strings.Contains(errMsg, "注册请求被拦截") {
		return "注册被拦截: 请更换IP或稍后重试"
	}

	// 账号封禁
	if strings.Contains(errMsg, "suspended") {
		return "账号已被封禁"
	}

	// 加密相关错误
	if strings.Contains(errMsg, "JWE 加密失败") || strings.Contains(errMsg, "未配置远程 JWE 加密") {
		return "密码加密失败，请检查加密服务配置"
	}

	// 其他已知错误直接返回
	if strings.Contains(errMsg, "注册失败:") || strings.Contains(errMsg, "失败:") {
		return errMsg
	}

	// 未知错误，添加步骤信息
	return friendlyStepName(step) + "失败: " + errMsg
}

func serviceErrorMetadata(err *ServiceError) string {
	metadata := err.Code
	if err.RequestID != "" {
		if metadata != "" {
			metadata += ", "
		}
		metadata += "requestId=" + err.RequestID
	}
	if metadata == "" {
		return ""
	}
	return " (" + metadata + ")"
}

func friendlyStepName(step string) string {
	stepNames := map[string]string{
		"OIDC":           "初始化注册",
		"Device":         "设备注册",
		"Email":          "邮箱验证",
		"Portal":         "门户访问",
		"WorkflowInit":   "工作流初始化",
		"SubmitEmail":    "提交邮箱",
		"Signup":         "注册流程",
		"SignupInit":     "注册初始化",
		"ProfileInit":    "配置初始化",
		"ProfileStart":   "配置启动",
		"SendOTP":        "发送验证码",
		"GetOTP":         "接收验证码",
		"CreateIdentity": "创建身份",
		"SetPassword":    "设置密码",
		"SSOWorkflow":    "SSO 工作流",
		"SSOToken":       "获取登录凭证",
		"KiroAuthorize":  "授权 Kiro 访问",
		"KiroExchange":   "获取访问令牌",
	}

	if friendlyStep := stepNames[step]; friendlyStep != "" {
		return friendlyStep
	}
	return step
}

// ctxCancelled 检查 context 是否已取消
func (r *Registrar) ctxCancelled() bool {
	return r.Ctx != nil && r.Ctx.Err() != nil
}

// Run 执行完整注册流程
func (r *Registrar) Run() map[string]interface{} {
	if r.Client != nil {
		defer r.Client.CloseIdleConnections()
	}
	// 入口处立即检查 context
	if r.ctxCancelled() {
		return map[string]interface{}{"status": "failed", "error": "任务已取消", "email": r.Email}
	}

	// Overlap shared config loading with initialization; wait before fingerprint work.
	crypto.WarmAppJSConfig(r.Cfg.Proxy, r.Identity.ChromeVer, r.Identity.UA, r.Identity.SecUA)

	steps := []struct {
		name string
		fn   func() error
	}{
		{"OIDC", r.Step1OIDC},
		{"Device", r.Step2Device},
		{"Email", r.Step3Email},
		{"Portal", r.Step4Portal},
		{"WorkflowInit", r.Step5WorkflowInit},
	}

	prefix := "[Kiro]"
	if r.TaskLabel != "" {
		prefix = "[Kiro][" + r.TaskLabel + "]"
	}

	for _, s := range steps {
		if r.ctxCancelled() {
			return map[string]interface{}{"status": "failed", "error": "任务已取消", "email": r.Email}
		}
		if s.name == "Portal" {
			if err := crypto.RefreshAppJSConfigContext(r.context(), r.Cfg.Proxy, r.Identity.ChromeVer, r.Identity.UA, r.Identity.SecUA); err != nil {
				return map[string]interface{}{"status": "failed", "error": r.formatError(s.name, err), "email": r.Email}
			}
		}
		if err := s.fn(); err != nil {
			friendlyErr := r.formatError(s.name, err)
			log.Printf("%s %s", prefix, friendlyErr)
			return map[string]interface{}{"status": "failed", "error": friendlyErr, "email": r.Email}
		}
	}

	// 步骤 6: 提交邮箱
	status, err := r.Step6SubmitEmail()
	if err != nil {
		friendlyErr := r.formatError("SubmitEmail", err)
		log.Printf("%s %s", prefix, friendlyErr)
		return map[string]interface{}{"status": "failed", "error": friendlyErr, "email": r.Email}
	}

	if status == "signup" {
		signupSteps := []struct {
			name string
			fn   func() error
		}{
			{"Signup", r.Step7Signup},
			{"SignupInit", r.Step7_5SignupInit},
			{"ProfileInit", r.Step7_8ProfileInit},
			{"ProfileStart", r.Step8ProfileStart},
			{"SendOTP", r.Step9SendOTP},
		}
		for _, s := range signupSteps {
			if r.ctxCancelled() {
				return map[string]interface{}{"status": "failed", "error": "任务已取消", "email": r.Email}
			}
			if err := s.fn(); err != nil {
				friendlyErr := r.formatError(s.name, err)
				log.Printf("%s %s", prefix, friendlyErr)
				return map[string]interface{}{"status": "failed", "error": friendlyErr, "email": r.Email}
			}
		}

		otp, err := r.Step10GetOTP()
		if err != nil {
			friendlyErr := r.formatError("GetOTP", err)
			log.Printf("%s %s", prefix, friendlyErr)
			return map[string]interface{}{"status": "failed", "error": friendlyErr, "email": r.Email}
		}

		// Step 11: 创建身份
		if r.ctxCancelled() {
			return map[string]interface{}{"status": "failed", "error": "任务已取消", "email": r.Email}
		}
		if err := r.Step11CreateIdentity(otp); err != nil {
			friendlyErr := r.formatError("CreateIdentity", err)
			log.Printf("%s %s", prefix, friendlyErr)
			return map[string]interface{}{"status": "failed", "error": friendlyErr, "email": r.Email}
		}

		// Step 12: 设置密码（只有这一步成功后，邮箱才算真正被消耗）
		if r.ctxCancelled() {
			return map[string]interface{}{"status": "failed", "error": "任务已取消", "email": r.Email}
		}
		if err := r.Step12SetPassword(); err != nil {
			friendlyErr := r.formatError("SetPassword", err)
			log.Printf("%s %s", prefix, friendlyErr)
			return map[string]interface{}{"status": "failed", "error": friendlyErr, "email": r.Email}
		}
	} else {
		if r.Cfg.UseOutlook {
			log.Printf("%s 邮箱已被注册", prefix)
			return map[string]interface{}{"status": "failed", "error": "邮箱已注册过，跳过", "email": r.Email}
		}
		log.Printf("%s 邮箱状态异常", prefix)
		return map[string]interface{}{"status": "failed", "error": "临时邮箱不可能已存在", "email": r.Email}
	}

	// ========== 到达此处说明 SetPassword 已成功，邮箱已被消耗 ==========

	if r.ctxCancelled() {
		return map[string]interface{}{"status": "failed", "error": "任务已取消", "email": r.Email, "passwordSet": true}
	}

	finalSteps := []struct {
		name string
		fn   func() error
	}{
		{"SSOWorkflow", r.Step12_8SSOWorkflow},
	}
	for _, s := range finalSteps {
		if err := s.fn(); err != nil {
			friendlyErr := r.formatError(s.name, err)
			log.Printf("%s %s", prefix, friendlyErr)
			return map[string]interface{}{"status": "failed", "error": friendlyErr, "email": r.Email, "passwordSet": true}
		}
	}

	if err := r.wait(2 * time.Second); err != nil {
		return map[string]interface{}{"status": "failed", "error": "任务已取消", "email": r.Email, "passwordSet": true}
	}

	// 步骤13-15: 获取令牌（账号已注册，失败时重试而不是重新注册）
	var awsToken map[string]interface{}
	var kiroCode string
	var kiroTokens map[string]interface{}

	// Step13: 获取 SSO Token（最多重试3次）
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			log.Printf("%s [13] 重试获取登录凭证 (%d/3)", prefix, attempt)
			if err := r.wait(2 * time.Second); err != nil {
				return map[string]interface{}{"status": "failed", "error": "任务已取消", "email": r.Email, "passwordSet": true}
			}
		}
		var err error
		awsToken, err = r.Step13SSOToken()
		if err == nil {
			break
		}
		if attempt == 2 {
			friendlyErr := r.formatError("SSOToken", err)
			log.Printf("%s %s", prefix, friendlyErr)
			return map[string]interface{}{"status": "failed", "error": friendlyErr, "email": r.Email, "passwordSet": true}
		}
	}

	if r.ctxCancelled() {
		return map[string]interface{}{"status": "failed", "error": "任务已取消", "email": r.Email, "passwordSet": true}
	}

	// Step14: Kiro 授权（最多重试3次）
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			log.Printf("%s [14] 重试授权Kiro访问 (%d/3)", prefix, attempt)
			if err := r.wait(2 * time.Second); err != nil {
				return map[string]interface{}{"status": "failed", "error": "任务已取消", "email": r.Email, "passwordSet": true}
			}
		}
		var err error
		kiroCode, err = r.Step14KiroAuthorize()
		if err == nil {
			break
		}
		if attempt == 2 {
			friendlyErr := r.formatError("KiroAuthorize", err)
			log.Printf("%s %s", prefix, friendlyErr)
			return map[string]interface{}{"status": "failed", "error": friendlyErr, "email": r.Email, "passwordSet": true}
		}
	}

	// Step15: 交换令牌（最多重试3次）
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			log.Printf("%s [15] 重试获取访问令牌 (%d/3)", prefix, attempt)
			if err := r.wait(2 * time.Second); err != nil {
				return map[string]interface{}{"status": "failed", "error": "任务已取消", "email": r.Email, "passwordSet": true}
			}
		}
		var err error
		kiroTokens, err = r.Step15KiroExchange(kiroCode)
		if err == nil {
			break
		}
		if attempt == 2 {
			friendlyErr := r.formatError("KiroExchange", err)
			log.Printf("%s %s", prefix, friendlyErr)
			return map[string]interface{}{"status": "failed", "error": friendlyErr, "email": r.Email, "passwordSet": true}
		}
	}

	verify := r.VerifyAlive(awsToken)
	if r.ctxCancelled() {
		return map[string]interface{}{"status": "failed", "error": "任务已取消", "email": r.Email, "passwordSet": true}
	}
	if suspended, _ := verify["suspended"].(bool); suspended {
		log.Printf("%s 账号已被封禁", prefix)
		return map[string]interface{}{"status": "failed", "error": "suspended", "email": r.Email, "passwordSet": true}
	}

	alive, _ := verify["alive"].(bool)
	if alive {
		log.Printf("%s 注册成功", prefix)
	} else {
		log.Printf("%s 注册完成", prefix)
	}

	return map[string]interface{}{
		"email":         r.Email,
		"password":      r.Cfg.Password,
		"status":        "success",
		"passwordSet":   true,
		"client_id":     r.ClientID,
		"client_secret": r.ClientSecret,
		"device_code":   r.DeviceCode,
		"aws_token":     awsToken,
		"kiro_tokens":   kiroTokens,
		"verify":        verify,
	}
}
