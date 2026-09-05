package core

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"regexp"
	"strings"
	"time"

	"reg_go/internal/email"
	httputil "reg_go/internal/http"
)

// Step6SubmitEmail 提交邮箱
func (r *Registrar) Step6SubmitEmail() (string, error) {
	log.Printf("[6] 提交邮箱 %s", r.Email)
	api := fmt.Sprintf("%s/platform/%s/api/execute", r.Cfg.SigninBase, r.Cfg.DirectoryID)
	ref := fmt.Sprintf("%s/platform/%s/login?workflowStateHandle=%s",
		r.Cfg.SigninBase, r.Cfg.DirectoryID, r.WorkflowHandle)
	fp := r.GenFP("signin", "PageSubmit", len(r.Email), r.Email)

	rid := NewUUID()
	h := r.BuildHeaders(ref, r.Cfg.SigninBase)
	h["x-amzn-requestid"] = rid
	h["x-amz-date"] = GmtDate()
	h["priority"] = "u=1, i"

	body, status, respH, err := r.DoPostRaw(api, map[string]interface{}{
		"stepId":              "get-identity-user",
		"workflowStateHandle": r.WorkflowHandle,
		"actionId":            "SUBMIT",
		"inputs": []interface{}{
			map[string]string{"input_type": "UserRequestInput", "username": r.Email},
			map[string]string{"input_type": "ApplicationTypeRequestInput", "applicationType": "SSO_INDIVIDUAL_ID"},
			map[string]interface{}{
				"input_type":  "UserEventRequestInput",
				"directoryId": r.Cfg.DirectoryID,
				"userName":    r.Email,
				"userEvents": []map[string]interface{}{{
					"input_type":      "UserEvent",
					"eventType":       "PAGE_SUBMIT",
					"pageName":        "IDENTIFICATION",
					"timeSpentOnPage": 5000,
				}},
			},
			map[string]string{"input_type": "FingerPrintRequestInput", "fingerPrint": fp},
		},
		"visitorId": r.VisitorID,
		"requestId": rid,
	}, h)
	if err != nil {
		return "", err
	}
	httputil.SaveCookies(r.Cookies, respH)

	var data map[string]interface{}
	json.Unmarshal(body, &data)
	if wh, ok := data["workflowStateHandle"].(string); ok {
		r.WorkflowHandle = wh
	}

	if status == 400 {
		return "signup", nil
	} else if status == 200 {
		return "login", nil
	}
	return "", fmt.Errorf("提交邮箱失败: %d - %s", status, string(body)[:min(200, len(body))])
}

// Step7Signup 注册
func (r *Registrar) Step7Signup() error {
	log.Println("[7] 注册 (SIGNUP)")
	api := fmt.Sprintf("%s/platform/%s/api/execute", r.Cfg.SigninBase, r.Cfg.DirectoryID)
	ref := fmt.Sprintf("%s/platform/%s/login?workflowStateHandle=%s",
		r.Cfg.SigninBase, r.Cfg.DirectoryID, r.WorkflowHandle)
	fp := r.GenFP("signup", "PageSubmit", 0, "")

	rid := NewUUID()
	h := r.BuildHeaders(ref, r.Cfg.SigninBase)
	h["x-amzn-requestid"] = rid
	h["x-amz-date"] = GmtDate()
	h["priority"] = "u=1, i"

	body, _, respH, err := r.DoPostRaw(api, map[string]interface{}{
		"stepId":              "get-identity-user",
		"workflowStateHandle": r.WorkflowHandle,
		"actionId":            "SIGNUP",
		"inputs": []interface{}{
			map[string]string{"input_type": "UserRequestInput", "username": r.Email},
			map[string]string{"input_type": "FingerPrintRequestInput", "fingerPrint": fp},
		},
		"visitorId": r.VisitorID,
		"requestId": rid,
	}, h)
	if err != nil {
		return err
	}
	httputil.SaveCookies(r.Cookies, respH)

	var data map[string]interface{}
	json.Unmarshal(body, &data)
	if redir, ok := data["redirect"].(map[string]interface{}); ok {
		if rurl, ok := redir["url"].(string); ok && strings.Contains(rurl, "workflowStateHandle=") {
			r.WorkflowHandle = httputil.SplitAfter(rurl, "workflowStateHandle=")
		}
	}
	return nil
}

// Step7_5SignupInit Signup API 初始化
func (r *Registrar) Step7_5SignupInit() error {
	log.Println("[7.5] Signup API 初始化")
	api := fmt.Sprintf("%s/platform/%s/signup/api/execute", r.Cfg.SigninBase, r.Cfg.DirectoryID)
	ref := fmt.Sprintf("%s/platform/%s/signup?workflowStateHandle=%s",
		r.Cfg.SigninBase, r.Cfg.DirectoryID, r.WorkflowHandle)

	fp := r.GenFP("signup", "first_load", 0, "")
	rid := NewUUID()
	h := r.BuildHeaders(ref, r.Cfg.SigninBase)
	h["x-amzn-requestid"] = rid
	h["x-amz-date"] = GmtDate()
	h["priority"] = "u=1, i"

	body, _, respH, err := r.DoPostRaw(api, map[string]interface{}{
		"stepId": "", "workflowStateHandle": r.WorkflowHandle,
		"inputs": []interface{}{
			map[string]string{"input_type": "UserRequestInput", "username": r.Email},
			map[string]string{"input_type": "FingerPrintRequestInput", "fingerPrint": fp},
		},
		"visitorId": r.VisitorID, "requestId": rid,
	}, h)
	if err != nil {
		return err
	}
	httputil.SaveCookies(r.Cookies, respH)

	var data map[string]interface{}
	json.Unmarshal(body, &data)
	if wh, ok := data["workflowStateHandle"].(string); ok {
		r.WorkflowHandle = wh
	}
	if data["stepId"] != "start" {
		return fmt.Errorf("Signup init 返回意外 stepId: %v", data["stepId"])
	}

	// 第二次请求
	fp = r.GenFP("signup", "PageLoad", 0, "")
	rid = NewUUID()
	h = r.BuildHeaders(ref, r.Cfg.SigninBase)
	h["x-amzn-requestid"] = rid
	h["x-amz-date"] = GmtDate()
	h["priority"] = "u=1, i"

	body, _, respH, err = r.DoPostRaw(api, map[string]interface{}{
		"stepId": "start", "workflowStateHandle": r.WorkflowHandle,
		"inputs": []interface{}{
			map[string]string{"input_type": "UserRequestInput", "username": r.Email},
			map[string]string{"input_type": "FingerPrintRequestInput", "fingerPrint": fp},
		},
		"visitorId": r.VisitorID, "requestId": rid,
	}, h)
	if err != nil {
		return err
	}
	httputil.SaveCookies(r.Cookies, respH)

	json.Unmarshal(body, &data)
	if wh, ok := data["workflowStateHandle"].(string); ok {
		r.WorkflowHandle = wh
	}
	if redir, ok := data["redirect"].(map[string]interface{}); ok {
		if rurl, ok := redir["url"].(string); ok && strings.Contains(rurl, "workflowID=") {
			wid := httputil.SplitAfter(rurl, "workflowID=")
			if i := strings.IndexByte(wid, '#'); i >= 0 {
				wid = wid[:i]
			}
			r.WorkflowID = wid
		}
	}
	if r.WorkflowID == "" {
		return fmt.Errorf("Signup init 未返回 workflowID")
	}
	return nil
}

// Step7_8ProfileInit Profile 页面初始化
func (r *Registrar) Step7_8ProfileInit() error {
	log.Println("[7.8] Profile 页面初始化")
	r.Ubid = httputil.UbidGen()
	r.Cookies["aws-user-profile-ubid"] = r.Ubid
	r.Cookies["i18next"] = "zh-CN"
	if _, ok := r.Cookies["awsccc"]; !ok {
		r.Cookies["awsccc"] = httputil.Awsccc()
	}

	url := fmt.Sprintf("%s/?workflowID=%s", r.Cfg.ProfileBase, r.WorkflowID)
	pageStartedAt := time.Now()
	referer := fmt.Sprintf("%s/platform/%s/signup?workflowStateHandle=%s", r.Cfg.SigninBase, r.Cfg.DirectoryID, r.WorkflowHandle)
	body, status, respH, err := r.DoGet(url, r.BuildProfileNavigationHeaders(referer))
	if err != nil {
		return err
	}
	httputil.SaveCookies(r.Cookies, respH)
	if status < 200 || status >= 300 {
		return profilePageResponseError(status, body)
	}
	if hash := extractProfileWebpackHash(body); hash != "" {
		r.Identity.WebpackHash = hash
		log.Printf("Profile bundle=%s", hash)
	}
	r.ProfilePageStartedAt = pageStartedAt
	r.FPCtx.ResetPerfTiming()

	// HAR entry 50: POST /api/get-config {} (浏览器加载 profile 应用后必发)
	if body, status, _, err := r.DoPostRaw(r.Cfg.ProfileBase+"/api/get-config",
		map[string]interface{}{}, r.BuildProfileHeaders(url)); err != nil {
		return err
	} else if status != 200 {
		log.Printf("[profile] get-config 非 200 (%d): %s", status, truncateBytes(body, 120))
	}

	d2cStart := time.Now()
	if err := r.FetchD2CToken(r.Cfg.ProfileBase, url); err != nil {
		return err
	}
	r.LastD2CFetchDuration = time.Since(d2cStart)
	return nil
}

var profileWebpackHashRE = regexp.MustCompile(`(?:^|[/_])app_([a-fA-F0-9]{10,64})\.min\.js(?:["'?]|$)`)

func extractProfileWebpackHash(body []byte) string {
	matches := profileWebpackHashRE.FindSubmatch(body)
	if len(matches) != 2 {
		return ""
	}
	return string(matches[1])
}

func profilePageResponseError(status int, body []byte) error {
	if status >= 400 {
		bodyText := strings.ToLower(string(body))
		if status == 403 && strings.Contains(bodyText, "cloudfront") {
			return fmt.Errorf("Profile page 被 CloudFront 拒绝 (HTTP %d)", status)
		}
		return fmt.Errorf("Profile page 请求失败 (HTTP %d)", status)
	}
	return fmt.Errorf("Profile page 未返回 HTML")
}

func elapsedMillisSince(start, now time.Time) int64 {
	if start.IsZero() || now.Before(start) {
		return 0
	}
	return now.Sub(start).Milliseconds()
}

func profileStartResponseError(status int, body []byte) error {
	if status >= 400 {
		bodyText := strings.ToLower(string(body))
		if status == 403 && strings.Contains(bodyText, "cloudfront") {
			return fmt.Errorf("Profile start 被 CloudFront 拒绝 (HTTP %d)", status)
		}
		return fmt.Errorf("Profile start 请求失败 (HTTP %d)", status)
	}
	return fmt.Errorf("Profile start 未返回 workflowState")
}

// Step8ProfileStart Profile 启动
func (r *Registrar) Step8ProfileStart() error {
	log.Println("[8] Profile 启动")
	ref := fmt.Sprintf("%s/?workflowID=%s", r.Cfg.ProfileBase, r.WorkflowID)
	fp := r.GenFP("profile", "PageLoad", 0, "")

	body, _, respH, err := r.DoPostRaw(r.Cfg.ProfileBase+"/api/start", map[string]interface{}{
		"workflowID": r.WorkflowID,
		"browserData": map[string]interface{}{
			"attributes": map[string]interface{}{
				"fingerprint":     fp,
				"eventTimestamp":  time.Now().UTC().Format("2006-01-02T15:04:05.000Z"),
				"timeSpentOnPage": fmt.Sprintf("%d", elapsedMillisSince(r.ProfilePageStartedAt, time.Now())),
				"eventType":       "PageLoad",
				"ubid":            r.Ubid,
				"visitorId":       r.VisitorID,
			},
			"cookies": map[string]interface{}{},
		},
	}, r.BuildProfileHeaders(ref))
	if err != nil {
		return err
	}
	httputil.SaveCookies(r.Cookies, respH)

	var data map[string]interface{}
	json.Unmarshal(body, &data)
	r.WorkflowState, _ = data["workflowState"].(string)
	if r.WorkflowState == "" {
		return fmt.Errorf("Profile start 未返回 workflowState: %s", string(body))
	}
	if len(r.WorkflowState) > 30 {
		log.Printf("workflowState=%s...", r.WorkflowState[:30])
	}
	r.ProfileEmailStartedAt = time.Now()

	// HAR entry 66: D2C 遥测 timeTakenToFetchVID (profile 页)
	profilePageURL := fmt.Sprintf("%s/?workflowID=%s#/signup/start?workflowID=%s",
		r.Cfg.ProfileBase, r.WorkflowID, r.WorkflowID)
	r.PostD2CEventSafe(profilePageURL, r.LastD2CFetchDuration)
	return nil
}

// Step9SendOTP 发送验证码
func (r *Registrar) Step9SendOTP() error {
	log.Println("[9] 发送验证码")

	// Outlook 模式: 记录发送前的邮件数量
	if r.Cfg.UseOutlook && r.Cfg.OutlookAccount != nil {
		count, err := email.GetInboxCount(*r.Cfg.OutlookAccount)
		if err != nil {
			log.Printf("获取邮件数量失败: %v, 默认为0", err)
		} else {
			r.OutlookMailCount = count
			log.Printf("发送前邮件数: %d", count)
		}
	}

	ref := fmt.Sprintf("%s/?workflowID=%s", r.Cfg.ProfileBase, r.WorkflowID)
	timeOnPage := 5000 + rand.Intn(3001)
	fp := r.GenFPWithTime("profile", "PageSubmit", timeOnPage, len(r.Email), r.Email)
	tsp := fmt.Sprintf("%d", timeOnPage)

	reqPayload := map[string]interface{}{
		"workflowState": r.WorkflowState,
		"email":         r.Email,
		"browserData": map[string]interface{}{
			"attributes": map[string]interface{}{
				"fingerprint":     fp,
				"eventTimestamp":  time.Now().UTC().Format("2006-01-02T15:04:05.000Z"),
				"timeSpentOnPage": tsp,
				"pageName":        "EMAIL_COLLECTION",
				"eventType":       "PageSubmit",
				"ubid":            r.Ubid,
				"visitorId":       r.VisitorID,
			},
			"cookies": map[string]interface{}{},
		},
	}

	respBody, status, _, err := r.DoPostRaw(r.Cfg.ProfileBase+"/api/send-otp", reqPayload, r.BuildProfileHeaders(ref))
	if err != nil {
		return err
	}
	if status != 200 {
		if r.Cfg.Debug {
			log.Printf("[DEBUG] send-otp 失败: status=%d, body=%s, fp_len=%d", status, string(respBody), len(fp))
		}
		return fmt.Errorf("send-otp 失败 (%d)", status)
	}
	r.ProfileVerificationStartedAt = time.Now()
	log.Println("验证码已发送")

	// katal 遥测 (HAR entry 69): SendOTP 完成后浏览器必发。
	getConfigMs := float64(elapsedMillisSince(r.ProfilePageStartedAt, time.Now()))
	sendOTPMs := float64(elapsedMillisSince(r.ProfileEmailStartedAt, time.Now()))
	if getConfigMs <= 0 {
		getConfigMs = 577 // HAR GetConfig 577ms 的典型值兜底
	}
	if sendOTPMs <= 0 {
		sendOTPMs = 672
	}
	r.PostKatalSignupBatch(getConfigMs, sendOTPMs)
	return nil
}

// Step10GetOTP 等待验证码 (临时邮箱或 Outlook IMAP)
func (r *Registrar) Step10GetOTP() (string, error) {
	log.Println("[10] 等待验证码")
	if r.Cfg.UseOutlook && r.Cfg.OutlookAccount != nil {
		code, err := email.WaitForOTP(*r.Cfg.OutlookAccount, r.OutlookMailCount, 120, 5)
		if err != nil {
			return "", err
		}
		log.Printf("验证码: %s", code)
		return code, nil
	}
	code, err := r.EmailSvc.WaitForCode(120, 3)
	if err != nil {
		return "", err
	}
	log.Printf("验证码: %s", code)
	return code, nil
}
