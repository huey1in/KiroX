package core

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/url"
	"strings"
	"time"

	http "github.com/bogdanfinn/fhttp"
	tls_client "github.com/bogdanfinn/tls-client"

	"reg_go/internal/browser"
	"reg_go/internal/captcha"
	"reg_go/internal/crypto"
	"reg_go/internal/email"
	httputil "reg_go/internal/http"
)

// Registrar 完整的注册流程
type Registrar struct {
	Cfg      *Config
	Client   tls_client.HttpClient
	Cookies  map[string]string
	Identity *browser.BrowserIdentity
	FPCtx    *browser.FingerprintContext

	VisitorID        string
	Email            string
	EmailSvc         email.TempEmailService // 临时邮箱服务
	ClientID         string
	ClientSecret     string
	DeviceCode       string
	UserCode         string
	WorkflowHandle   string
	WorkflowID       string
	WorkflowState    string
	Ubid             string
	RegCode          string
	SignState        string
	AuthCode         string
	SSOState         string
	WdcCSRFToken     string
	SSOToken         string
	KiroCodeVerifier string
	KiroState        string
	KiroClientID     string
	KiroClientSecret string
	KiroRedirectPort int

	// 客户端扩展: 任务上下文
	Ctx       context.Context
	TaskLabel string

	// 本地加密
	JWE *crypto.JWEEncryptor

	// Outlook 模式: 发送验证码前的收件箱和垃圾邮件数量
	OutlookMailCounts email.OutlookMailboxCounts

	// 遥测/指纹计时: 记录各页面开始时间, 用于生成真实 timeSpentOnPage 与 D2C/katal 上报
	SigninPageStartedAt          time.Time
	SigninPageURL                string
	LastD2CFetchDuration         time.Duration
	ProfilePageStartedAt         time.Time
	ProfileEmailStartedAt        time.Time
	ProfileVerificationStartedAt time.Time
	PasswordPageStartedAt        time.Time

	solveAWSWAF func(context.Context, captcha.AWSWAFOptions) (captcha.AWSWAFSolution, error)
	solveImage  func(context.Context, []byte) (string, error)
}

// NewRegistrar 创建注册器
func NewRegistrar(cfg *Config) *Registrar {
	// 每个曲线点控制一个指纹域相对缓存身份的重采样概率。
	identity := browser.IdentityForOffsets(cfg.Proxy, cfg.FingerprintOffsets, cfg.FingerprintCurvePositions)
	log.Printf("[指纹] Chrome: %s | GPU: %s | 内存: %dGB | 核心: %d | 分辨率: %dx%d (%d-bit)",
		identity.ChromeVer, identity.GPUModel, identity.DeviceMemory, identity.HardwareConcurrency,
		identity.Screen.Width, identity.Screen.Height, identity.Screen.ColorDepth)

	client := httputil.NewTLSClient(cfg.Proxy, true, identity.ChromeVer)
	cookies := make(map[string]string)
	if token := strings.TrimSpace(cfg.WAFToken); token != "" {
		cookies["aws-waf-token"] = token
		for _, rawURL := range []string{cfg.SigninBase, cfg.ProfileBase} {
			target, err := url.Parse(rawURL)
			if err == nil && target.Hostname() != "" {
				client.SetCookies(target, []*http.Cookie{{
					Name:   "aws-waf-token",
					Value:  token,
					Path:   "/",
					Secure: true,
				}})
			}
		}
	}
	return &Registrar{
		Cfg:       cfg,
		Client:    client,
		Cookies:   cookies,
		Identity:  identity,
		FPCtx:     browser.NewFPContext(identity),
		VisitorID: httputil.VisitorID(),
		JWE:       &crypto.JWEEncryptor{},
		OutlookMailCounts: email.OutlookMailboxCounts{
			Junk: -1,
		},
	}
}

// isRetryableError 判断是否为可重试的瞬态网络错误（EOF、连接重置等）
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}
	errMsg := err.Error()
	return strings.Contains(errMsg, "EOF") ||
		strings.Contains(errMsg, "connection reset") ||
		strings.Contains(errMsg, "broken pipe") ||
		strings.Contains(errMsg, "connection refused") ||
		strings.Contains(errMsg, "i/o timeout") ||
		strings.Contains(errMsg, "TLS handshake timeout") ||
		strings.Contains(errMsg, "unexpected EOF")
}

// retryBackoff 计算重试退避时间（1-2秒 + 随机抖动）
func retryBackoff(attempt int) time.Duration {
	base := time.Duration(1000+attempt*500) * time.Millisecond
	jitter := time.Duration(rand.Intn(500)) * time.Millisecond
	return base + jitter
}

func (r *Registrar) maxHTTPRetries() int {
	if r.Cfg == nil {
		return 2
	}
	if r.Cfg.HTTPRetries < 0 || r.Cfg.HTTPRetries > 4 {
		return 2
	}
	return r.Cfg.HTTPRetries
}

func (r *Registrar) context() context.Context {
	if r.Ctx != nil {
		return r.Ctx
	}
	return context.Background()
}

func (r *Registrar) wait(delay time.Duration) error {
	ctx := r.context()
	if err := ctx.Err(); err != nil {
		return err
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return ctx.Err()
	}
}

// DoPost 发送 POST 请求（带自动重试）
func (r *Registrar) DoPost(url string, payload interface{}, headers map[string]string) ([]byte, map[string][]string, error) {
	maxRetries := r.maxHTTPRetries()
	var lastErr error
	var payloadBytes []byte
	if payload != nil {
		payloadBytes, _ = json.Marshal(payload)
	}

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if err := r.context().Err(); err != nil {
			return nil, nil, err
		}
		if attempt > 0 {
			log.Printf("[HTTP] POST 重试 (%d/%d), 等待退避...", attempt, maxRetries)
			if err := r.wait(retryBackoff(attempt)); err != nil {
				return nil, nil, err
			}
		}

		var body io.Reader
		if payloadBytes != nil {
			body = bytes.NewReader(payloadBytes)
		}
		req, err := http.NewRequestWithContext(r.context(), "POST", url, body)
		if err != nil {
			return nil, nil, err
		}
		httputil.SetHeaders(req, headers)
		resp, err := r.Client.Do(req)
		if err != nil {
			lastErr = err
			if isRetryableError(err) {
				continue
			}
			return nil, nil, err
		}
		if amsTraceEnabled() {
			if scopes := safeSetCookieScopes(resp.Header); len(scopes) > 0 {
				log.Printf("[WAF TRACE] cookies from %s: %v", safeResponseRoute(url), scopes)
			}
		}
		defer resp.Body.Close()
		data, err := io.ReadAll(resp.Body)
		return data, resp.Header, err
	}
	return nil, nil, lastErr
}

// DoGet 发送 GET 请求，返回完整信息（带自动重试）
func (r *Registrar) DoGet(url string, headers map[string]string) ([]byte, int, map[string][]string, error) {
	maxRetries := r.maxHTTPRetries()
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if err := r.context().Err(); err != nil {
			return nil, 0, nil, err
		}
		if attempt > 0 {
			log.Printf("[HTTP] GET 重试 (%d/%d), 等待退避...", attempt, maxRetries)
			if err := r.wait(retryBackoff(attempt)); err != nil {
				return nil, 0, nil, err
			}
		}

		req, err := http.NewRequestWithContext(r.context(), "GET", url, nil)
		if err != nil {
			return nil, 0, nil, err
		}
		httputil.SetHeaders(req, headers)
		resp, err := r.Client.Do(req)
		if err != nil {
			lastErr = err
			if isRetryableError(err) {
				continue
			}
			return nil, 0, nil, err
		}
		if amsTraceEnabled() {
			if scopes := safeSetCookieScopes(resp.Header); len(scopes) > 0 {
				log.Printf("[WAF TRACE] cookies from %s: %v", safeResponseRoute(url), scopes)
			}
		}
		defer resp.Body.Close()
		data, err := io.ReadAll(resp.Body)
		return data, resp.StatusCode, resp.Header, err
	}
	return nil, 0, nil, lastErr
}

// DoPostBodyRaw 发送 POST 请求 (原始字符串 body, 不做 JSON 序列化)
func (r *Registrar) DoPostBodyRaw(url string, rawBody string, headers map[string]string) ([]byte, int, map[string][]string, error) {
	if err := r.context().Err(); err != nil {
		return nil, 0, nil, err
	}
	body := strings.NewReader(rawBody)
	req, err := http.NewRequestWithContext(r.context(), "POST", url, body)
	if err != nil {
		return nil, 0, nil, err
	}
	httputil.SetHeaders(req, headers)
	resp, err := r.Client.Do(req)
	if err != nil {
		return nil, 0, nil, err
	}
	if amsTraceEnabled() {
		if scopes := safeSetCookieScopes(resp.Header); len(scopes) > 0 {
			log.Printf("[WAF TRACE] cookies from %s: %v", safeResponseRoute(url), scopes)
		}
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	return data, resp.StatusCode, resp.Header, err
}

// DoPostRaw 发送 POST 请求，返回状态码（带自动重试）
func (r *Registrar) DoPostRaw(url string, payload interface{}, headers map[string]string) ([]byte, int, map[string][]string, error) {
	maxRetries := r.maxHTTPRetries()
	var lastErr error
	var payloadBytes []byte
	if payload != nil {
		payloadBytes, _ = json.Marshal(payload)
	}

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if err := r.context().Err(); err != nil {
			return nil, 0, nil, err
		}
		if attempt > 0 {
			log.Printf("[HTTP] POST 重试 (%d/%d), 等待退避...", attempt, maxRetries)
			if err := r.wait(retryBackoff(attempt)); err != nil {
				return nil, 0, nil, err
			}
		}

		var body io.Reader
		if payloadBytes != nil {
			body = bytes.NewReader(payloadBytes)
		}
		req, err := http.NewRequestWithContext(r.context(), "POST", url, body)
		if err != nil {
			return nil, 0, nil, err
		}
		httputil.SetHeaders(req, headers)
		resp, err := r.Client.Do(req)
		if err != nil {
			lastErr = err
			if isRetryableError(err) {
				continue
			}
			return nil, 0, nil, err
		}
		if amsTraceEnabled() {
			if scopes := safeSetCookieScopes(resp.Header); len(scopes) > 0 {
				log.Printf("[WAF TRACE] cookies from %s: %v", safeResponseRoute(url), scopes)
			}
		}
		defer resp.Body.Close()
		data, err := io.ReadAll(resp.Body)
		return data, resp.StatusCode, resp.Header, err
	}
	return nil, 0, nil, lastErr
}

// GenFP 生成指纹
func (r *Registrar) GenFP(pageType, eventType string, emailLen int, emailAddr string) string {
	return r.GenFPWithTime(pageType, eventType, 0, emailLen, emailAddr)
}

// GenFPWithTime 生成指纹（指定页面停留时间）
func (r *Registrar) GenFPWithTime(pageType, eventType string, timeOnPage, emailLen int, emailAddr string) string {
	did := r.Cfg.DirectoryID
	var loc, ref string

	switch pageType {
	case "signin":
		loc = fmt.Sprintf("%s/platform/%s/login?workflowStateHandle=%s", r.Cfg.SigninBase, did, r.WorkflowHandle)
	case "signup":
		loc = fmt.Sprintf("%s/platform/%s/signup?workflowStateHandle=%s", r.Cfg.SigninBase, did, r.WorkflowHandle)
	default: // profile
		if eventType == "PageSubmit" {
			loc = fmt.Sprintf("%s/?workflowID=%s#/signup/enter-email", r.Cfg.ProfileBase, r.WorkflowID)
		} else {
			loc = fmt.Sprintf("%s/?workflowID=%s#/signup/start", r.Cfg.ProfileBase, r.WorkflowID)
		}
		if r.WorkflowID == "" {
			loc = r.Cfg.ProfileBase + "/"
		}
	}

	if pageType == "profile" {
		ref = fmt.Sprintf("%s/platform/%s/signup?workflowStateHandle=%s", r.Cfg.SigninBase, did, r.WorkflowHandle)
	} else {
		ref = r.Cfg.ViewBase + "/"
	}

	return r.GenFPAt(loc, ref, pageType, eventType, timeOnPage, emailLen, emailAddr)
}

func (r *Registrar) GenFPAt(locationURL, referrer, pageType, eventType string, timeOnPage, emailLen int, emailAddr string) string {
	fpJSON := browser.GenerateFingerprintJSON(r.Identity, locationURL, referrer, r.FPCtx, pageType, eventType, timeOnPage, emailLen, emailAddr)
	return crypto.EncryptFingerprint(fpJSON)
}

// Step1OIDC OIDC 注册
func (r *Registrar) Step1OIDC() error {
	log.Println("[1] OIDC 注册")
	body, _, err := r.DoPost(r.Cfg.OIDCBase+"/client/register", map[string]interface{}{
		"clientName": "Amazon Q Developer for command line",
		"clientType": "public",
		"scopes":     []string{"codewhisperer:completions", "codewhisperer:analysis", "codewhisperer:conversations", "codewhisperer:transformations", "codewhisperer:taskassist"},
	}, map[string]string{"Content-Type": "application/json"})
	if err != nil {
		return err
	}
	var data map[string]interface{}
	json.Unmarshal(body, &data)
	r.ClientID, _ = data["clientId"].(string)
	r.ClientSecret, _ = data["clientSecret"].(string)
	if r.ClientID == "" {
		return fmt.Errorf("OIDC 注册失败: %s", string(body))
	}
	return nil
}

// Step2Device 设备授权
func (r *Registrar) Step2Device() error {
	log.Println("[2] 设备授权")
	body, _, err := r.DoPost(r.Cfg.OIDCBase+"/device_authorization", map[string]interface{}{
		"clientId": r.ClientID, "clientSecret": r.ClientSecret,
		"startUrl": r.Cfg.StartURL,
	}, map[string]string{"Content-Type": "application/json"})
	if err != nil {
		return err
	}
	var data map[string]interface{}
	json.Unmarshal(body, &data)
	r.DeviceCode, _ = data["deviceCode"].(string)
	r.UserCode, _ = data["userCode"].(string)
	log.Printf("user_code=%s", r.UserCode)
	return nil
}

// Step3Email 获取邮箱 (临时邮箱、Outlook)
func (r *Registrar) Step3Email() error {
	if r.Cfg.UseOutlook && r.Cfg.OutlookAccount != nil {
		log.Println("[3] 使用 Outlook 邮箱")
		r.Email = r.Cfg.OutlookAccount.Email
		log.Printf("email=%s", r.Email)
		return nil
	}
	if r.Cfg.UseCloudMail && r.Cfg.CloudMailProvider != nil {
		log.Println("[3] 使用 Cloud-Mail 邮箱")
		r.EmailSvc = email.NewCloudMailService(r.Cfg.CloudMailProvider)
		r.Email = r.EmailSvc.GetAddress()
		log.Printf("email=%s", r.Email)
		return nil
	}
	if r.Cfg.UseMoeMail && r.Cfg.MoeMailProvider != nil {
		log.Println("[3] 使用 MoeMail 邮箱（已创建）")
		r.EmailSvc = email.NewMoEmailServiceFromProvider(r.Cfg.MoeMailProvider)
		r.Email = r.EmailSvc.GetAddress()
		log.Printf("email=%s", r.Email)
		return nil
	}
	if r.Cfg.UseMailNest && r.Cfg.MailNestProvider != nil {
		log.Println("[3] 使用 MailNest 邮箱")
		r.EmailSvc = email.NeMailNestServiceFromProvider(r.Cfg.MailNestProvider)
		r.Email = r.EmailSvc.GetAddress()
		log.Printf("email=%s", r.Email)
		return nil
	}
	if r.Cfg.UseICloud && r.Cfg.ICloudAccount != nil {
		log.Println("[3] 使用 iCloud 邮箱")
		r.EmailSvc = email.NewICloudService(*r.Cfg.ICloudAccount, r.Cfg.Proxy, r.Identity.ChromeVer)
		r.Email = r.EmailSvc.Create()
		log.Printf("email=%s", r.Email)
		return nil
	}
	log.Println("[3] 创建临时邮箱")
	// 如果未配置 MoEmail URL，从已保存的 MoeMail 配置中自动读取
	baseURL := r.Cfg.MoEmailBaseURL
	apiKey := r.Cfg.MoEmailAPIKey
	if baseURL == "" {
		configs := email.GetMoeMailConfigs()
		if len(configs) > 0 {
			baseURL = configs[0].URL
			apiKey = configs[0].APIKey
			log.Printf("[MoEmail] 自动使用已保存配置: %s", configs[0].Name)
		}
	}
	r.EmailSvc = email.NewMoEmailService(baseURL, apiKey)
	r.Email = r.EmailSvc.Create()
	log.Printf("email=%s", r.Email)
	return nil
}

// Step4Portal Portal 初始化
func (r *Registrar) Step4Portal() error {
	log.Println("[4] Portal 初始化")
	r.Cookies["awsccc"] = httputil.Awsccc()

	redirect := fmt.Sprintf("%s/start/#/device?user_code=%s", r.Cfg.ViewBase, r.UserCode)
	url := fmt.Sprintf("%s/login?directory_id=view&redirect_url=%s", r.Cfg.PortalBase, redirect)

	h := map[string]string{
		"Accept":       "application/json, text/plain, */*",
		"Content-Type": "application/json",
		"Origin":       r.Cfg.ViewBase,
		"Referer":      r.Cfg.ViewBase + "/",
		"User-Agent":   r.Identity.UA,
	}

	body, _, respH, err := r.DoGet(url, h)
	if err != nil {
		return err
	}
	httputil.SaveCookies(r.Cookies, respH)

	var data map[string]interface{}
	json.Unmarshal(body, &data)

	rurl, _ := data["redirectUrl"].(string)
	if strings.Contains(rurl, "workflowStateHandle=") {
		r.WorkflowHandle = httputil.SplitAfter(rurl, "workflowStateHandle=")
	}
	if csrf, ok := data["csrfToken"].(string); ok {
		r.Cookies["loginCsrfToken"] = csrf
	}
	if r.WorkflowHandle == "" {
		return fmt.Errorf("Portal 未返回 workflow handle")
	}

	loginURL := fmt.Sprintf("%s/platform/%s/login?workflowStateHandle=%s",
		r.Cfg.SigninBase, r.Cfg.DirectoryID, r.WorkflowHandle)
	d2cStart := time.Now()
	if err := r.FetchD2CToken(r.Cfg.SigninBase, loginURL); err != nil {
		return err
	}
	r.LastD2CFetchDuration = time.Since(d2cStart)
	return nil
}

// Step5WorkflowInit 工作流初始化
func (r *Registrar) Step5WorkflowInit() error {
	log.Println("[5] 工作流初始化")
	r.SigninPageStartedAt = time.Now()
	api := fmt.Sprintf("%s/platform/%s/api/execute", r.Cfg.SigninBase, r.Cfg.DirectoryID)
	ref := fmt.Sprintf("%s/platform/%s/login?workflowStateHandle=%s",
		r.Cfg.SigninBase, r.Cfg.DirectoryID, r.WorkflowHandle)
	r.SigninPageURL = ref

	fp := r.GenFP("signin", "first_load", 0, "")
	rid := NewUUID()
	h := r.BuildHeaders(ref, r.Cfg.SigninBase)
	h["x-amzn-requestid"] = rid
	h["x-amz-date"] = GmtDate()
	h["priority"] = "u=1, i"

	body, _, respH, err := r.DoPostRaw(api, map[string]interface{}{
		"stepId": "", "workflowStateHandle": r.WorkflowHandle,
		"inputs":    []interface{}{map[string]string{"input_type": "FingerPrintRequestInput", "fingerPrint": fp}},
		"requestId": rid,
	}, h)
	if err != nil {
		return err
	}
	httputil.SaveCookies(r.Cookies, respH)

	// HAR entry 21: /metrics/fingerprint IsFingerprintGenerated:Success (operation=start)
	metricFP := r.GenFP("signin", "first_load", 0, "")
	r.PostFingerprintMetricSafe("IsFingerprintGenerated:Success", metricFP, "AWSSignin:FingerprintMetrics:start")

	var data map[string]interface{}
	json.Unmarshal(body, &data)
	if wh, ok := data["workflowStateHandle"].(string); ok {
		r.WorkflowHandle = wh
	}

	if data["stepId"] == "start" {
		fp = r.GenFP("signin", "PageLoad", 0, "")
		rid = NewUUID()
		h = r.BuildHeaders(ref, r.Cfg.SigninBase)
		h["x-amzn-requestid"] = rid
		h["x-amz-date"] = GmtDate()
		h["priority"] = "u=1, i"

		body, _, respH, err = r.DoPostRaw(api, map[string]interface{}{
			"stepId": "start", "workflowStateHandle": r.WorkflowHandle,
			"inputs":    []interface{}{map[string]string{"input_type": "FingerPrintRequestInput", "fingerPrint": fp}},
			"requestId": rid,
		}, h)
		if err != nil {
			return err
		}
		httputil.SaveCookies(r.Cookies, respH)
		json.Unmarshal(body, &data)
		if wh, ok := data["workflowStateHandle"].(string); ok {
			r.WorkflowHandle = wh
		}
	}

	// HAR entry 27: /metrics/fingerprint IsFingerprintFileLoaded:Success (operation=OnLoad_Username_Page)
	r.PostFingerprintMetricSafe("IsFingerprintFileLoaded:Success", "1", "AWSSignin:FingerprintMetrics:OnLoad_Username_Page")

	// D2C/WebVisor visitor token 与耗时遥测 (HAR entries 34/35/37)。
	// Step4Portal 已调 FetchD2CToken, 这里仅上报耗时事件。
	loginURL := fmt.Sprintf("%s/platform/%s/login?workflowStateHandle=%s",
		r.Cfg.SigninBase, r.Cfg.DirectoryID, r.WorkflowHandle)
	r.PostD2CEventSafe(loginURL, r.LastD2CFetchDuration)
	return nil
}

// NewUUID 生成 UUID
func NewUUID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// GmtDate 生成 GMT 日期字符串
func GmtDate() string {
	return time.Now().UTC().Format("Mon, 02 Jan 2006 15:04:05 GMT")
}
