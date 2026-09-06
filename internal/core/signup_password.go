package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"reg_go/internal/captcha"
	httputil "reg_go/internal/http"
)

// Step11CreateIdentity 创建身份
func (r *Registrar) Step11CreateIdentity(otp string) error {
	log.Println("[11] 创建身份")
	ref := fmt.Sprintf("%s/?workflowID=%s", r.Cfg.ProfileBase, r.WorkflowID)
	fp := r.GenFP("profile", "EmailVerification", 0, "")

	body, _, _, err := r.DoPostRaw(r.Cfg.ProfileBase+"/api/create-identity", map[string]interface{}{
		"workflowState": r.WorkflowState,
		"userData":      map[string]string{"email": r.Email, "fullName": r.Cfg.FullName},
		"otpCode":       otp,
		"browserData": map[string]interface{}{
			"attributes": map[string]interface{}{
				"fingerprint":     fp,
				"eventTimestamp":  time.Now().UTC().Format("2006-01-02T15:04:05.000Z"),
				"timeSpentOnPage": "45000",
				"pageName":        "EMAIL_VERIFICATION",
				"eventType":       "EmailVerification",
				"ubid":            r.Ubid,
				"visitorId":       r.VisitorID,
			},
			"cookies": map[string]interface{}{},
		},
	}, r.BuildProfileHeaders(ref))
	if err != nil {
		return err
	}

	var data map[string]interface{}
	json.Unmarshal(body, &data)
	r.RegCode, _ = data["registrationCode"].(string)
	r.SignState, _ = data["signInState"].(string)
	if r.RegCode == "" {
		return unexpectedServiceResponse("create-identity 未返回 registrationCode", body)
	}
	if len(r.RegCode) > 20 {
		log.Printf("regCode=%s...", r.RegCode[:20])
	}

	// katal 遥测 (HAR entry 72): CreateIdentity 完成后浏览器必发。
	verificationMs := float64(elapsedMillisSince(r.ProfileVerificationStartedAt, time.Now()))
	createIdentityMs := float64(elapsedMillisSince(r.ProfileEmailStartedAt, time.Now()))
	if verificationMs <= 0 {
		verificationMs = 15539 // HAR SignUpEmailVerificationStep 15539ms 兜底
	}
	if createIdentityMs <= 0 {
		createIdentityMs = 603
	}
	r.PostKatalVerificationBatch(verificationMs, createIdentityMs)
	return nil
}

// Step12SetPassword 设置密码
func (r *Registrar) Step12SetPassword() error {
	log.Println("[12] 设置密码")
	r.FPCtx.ResetPerfTiming()
	api := fmt.Sprintf("%s/platform/%s/signup/api/execute", r.Cfg.SigninBase, r.Cfg.DirectoryID)
	ref := fmt.Sprintf("%s/platform/%s/signup?registrationCode=%s&state=%s",
		r.Cfg.SigninBase, r.Cfg.DirectoryID, r.RegCode, r.SignState)
	profileRef := fmt.Sprintf("%s/?workflowID=%s", r.Cfg.ProfileBase, r.WorkflowID)
	genPasswordFP := func() string {
		return r.GenFPAt(ref, profileRef, "signup", "PageSubmit", 0, 0, "")
	}
	fp := genPasswordFP()

	// 12a: 获取加密公钥
	rid := NewUUID()
	h := r.BuildHeaders(ref, r.Cfg.SigninBase)
	h["x-amzn-requestid"] = rid
	h["x-amz-date"] = GmtDate()
	h["priority"] = "u=1, i"

	body, _, respH, err := r.DoPostRaw(api, map[string]interface{}{
		"stepId": "", "state": r.SignState,
		"inputs": []interface{}{
			map[string]string{
				"input_type":       "UserRegistrationRequestInput",
				"registrationCode": r.RegCode, "state": r.SignState,
			},
			map[string]string{"input_type": "FingerPrintRequestInput", "fingerPrint": fp},
		},
		"requestId": rid,
	}, h)
	if err != nil {
		return err
	}
	httputil.SaveCookies(r.Cookies, respH)

	var data map[string]interface{}
	json.Unmarshal(body, &data)
	r.WorkflowHandle, _ = data["workflowStateHandle"].(string)
	passwordStepID := passwordCreationStepID(data)
	presentationContext := httputil.GetNestedMap(data, "presentationContext")
	builderIDSourceDirectory, _ := presentationContext["builderIdSourceDirectory"].(string)
	identityPoolID, _ := presentationContext["identityPoolId"].(string)
	if strings.TrimSpace(identityPoolID) == "" {
		identityPoolID = r.Cfg.DirectoryID
	}
	applicationType, _ := presentationContext["applicationType"].(string)
	if amsTraceEnabled() {
		log.Printf("[WAF TRACE] password page stepId=%q identityPoolId=%q applicationType=%q builderIdSourceDirectory=%q responseKeys=%v",
			passwordStepID, identityPoolID, applicationType, builderIDSourceDirectory, safeJSONResponseKeys(body))
	}

	encCtx := httputil.GetNestedMap(data, "workflowResponseData", "encryptionContextResponse")
	pubKeyMap := httputil.GetNestedStringMap(encCtx, "publicKey")
	if pubKeyMap == nil || pubKeyMap["n"] == "" {
		return unexpectedServiceResponse("未获取到加密公钥", body)
	}

	issuer, _ := encCtx["issuer"].(string)
	if issuer == "" {
		issuer = "signin"
	}
	audience, _ := encCtx["audience"].(string)
	if audience == "" {
		audience = "AWSPasswordService"
	}
	region, _ := encCtx["region"].(string)
	if region == "" {
		region = "us-east-1"
	}
	if amsTraceEnabled() {
		log.Printf("[WAF TRACE] encryption context alg=%q issuer=%q audience=%q region=%q keyBits=%d",
			pubKeyMap["alg"], issuer, audience, region, decodedModulusBits(pubKeyMap["n"]))
	}

	// CreatePasswordPage sends these two page-load signals before it accepts a
	// password submission. They also establish the server-side fingerprint
	// context that the later CAPTCHA access code is redeemed against.
	r.sendUserEventSafe(identityPoolID, "PAGE_LOAD", "CREDENTIAL_COLLECTION", 0)
	if err := r.postFingerprintMetricAt(
		"IsFingerprintFileLoaded:Success",
		"1",
		"AWSSignin:FingerprintMetrics:OnLoad_Password_Page",
		ref,
	); err != nil {
		log.Printf("[遥测] 密码页指纹加载指标上报失败: %v", err)
	}

	encrypted, err := r.JWE.Encrypt(r.Cfg.Password, pubKeyMap, issuer, audience, region)
	if err != nil {
		return fmt.Errorf("JWE 加密失败: %w", err)
	}

	// 12b: 提交密码
	fp = genPasswordFP()
	passwordInput := map[string]interface{}{
		"input_type":            "PasswordRequestInput",
		"password":              encrypted,
		"successfullyEncrypted": "SUCCESSFUL",
		"errorLog":              nil,
	}
	userEvent := map[string]interface{}{
		"input_type":      "UserEvent",
		"eventType":       "PAGE_SUBMIT",
		"pageName":        "CREDENTIAL_COLLECTION",
		"timeSpentOnPage": 5000,
	}
	userEventInput := map[string]interface{}{
		"input_type":  "UserEventRequestInput",
		"directoryId": identityPoolID,
		"userName":    r.Email,
		"userEvents":  []interface{}{userEvent},
	}
	fingerprintInput := map[string]string{"input_type": "FingerPrintRequestInput", "fingerPrint": fp}
	r.PasswordPageStartedAt = time.Now().Add(-5 * time.Second)
	passwordPayload := map[string]interface{}{
		"stepId":              passwordStepID,
		"workflowStateHandle": r.WorkflowHandle,
		"actionId":            "SUBMIT",
		"inputs": []interface{}{
			passwordInput,
			userEventInput,
			map[string]string{"input_type": "UserRequestInput", "username": r.Email},
			fingerprintInput,
		},
		"visitorId": r.VisitorID,
	}
	if strings.TrimSpace(builderIDSourceDirectory) != "" {
		builderIDSession, readErr := r.readBuilderIDSession(builderIDSourceDirectory, ref)
		if readErr != nil {
			log.Printf("[WAF] 读取 Builder ID 会话失败: %v", readErr)
		} else {
			passwordPayload["builderIdSession"] = builderIDSession
			if amsTraceEnabled() {
				log.Printf("[WAF TRACE] builderIdSession present=%t length=%d", builderIDSession != "", len(builderIDSession))
			}
		}
	}
	passwordSubmitAttempt := 0
	submitPassword := func() ([]byte, map[string][]string, error) {
		passwordSubmitAttempt++
		rid = NewUUID()
		passwordPayload["requestId"] = rid
		h = r.BuildHeaders(ref, r.Cfg.SigninBase)
		h["x-amzn-requestid"] = rid
		h["x-amz-date"] = GmtDate()
		h["priority"] = "u=1, i"
		if amsTraceEnabled() {
			shape, _ := json.Marshal(safeJSONShape(passwordPayload))
			log.Printf("[WAF TRACE] password request=%s jarCookies=%v legacyCookies=%v", shape, r.safeClientCookieNames(api), safeCookieNames(r.Cookies))
			_, hasCaptchaAccessCode := passwordPayload["captchaRequest"]
			log.Printf("[WAF TRACE] password fingerprint attempt=%d pageHasCaptcha=%t captchaAccessCodePresent=%t encryptedLength=%d chrome=%q platform=%q plugins=%d screen=%dx%d/%d-bit memory=%d cores=%d canvasHash=%d",
				passwordSubmitAttempt,
				r.FPCtx != nil && r.FPCtx.PageHasCaptcha,
				hasCaptchaAccessCode,
				len(fingerprintInput["fingerPrint"]),
				r.Identity.ChromeVer,
				r.Identity.Platform,
				len(r.Identity.Plugins),
				r.Identity.Screen.Width,
				r.Identity.Screen.Height,
				r.Identity.Screen.ColorDepth,
				r.Identity.DeviceMemory,
				r.Identity.HardwareConcurrency,
				r.Identity.CanvasHash,
			)
		}
		responseBody, _, responseHeaders, submitErr := r.DoPostRaw(api, passwordPayload, h)
		if amsTraceEnabled() {
			log.Printf("[WAF TRACE] password response keys=%v error=%v headers=%v", safeJSONResponseKeys(responseBody), parseServiceError(responseBody), sortedHeaderNames(responseHeaders))
		}
		return responseBody, responseHeaders, submitErr
	}

	body, respH, err = submitPassword()
	if err != nil {
		return err
	}
	httputil.SaveCookies(r.Cookies, respH)

	rurl := passwordRedirect(body)
	initialChallenge, hadInitialChallenge := parseAWSWAFChallenge(body)
	if rurl == "" && r.Cfg.WAFEnabled {
		if hadInitialChallenge {
			r.FPCtx.PageHasCaptcha = true
		}
		handled, solveErr := r.solvePasswordWAFChallenge(body, ref, passwordPayload)
		if solveErr != nil {
			return solveErr
		}
		if handled {
			var challengeData map[string]interface{}
			if json.Unmarshal(body, &challengeData) == nil {
				encCtxUpdate := httputil.GetNestedMap(challengeData, "workflowResponseData", "encryptionContextResponse")
				if updatedKey := httputil.GetNestedStringMap(encCtxUpdate, "publicKey"); updatedKey != nil && updatedKey["n"] != "" {
					pubKeyMap = updatedKey
					if value, ok := encCtxUpdate["issuer"].(string); ok && value != "" {
						issuer = value
					}
					if value, ok := encCtxUpdate["audience"].(string); ok && value != "" {
						audience = value
					}
					if value, ok := encCtxUpdate["region"].(string); ok && value != "" {
						region = value
					}
					log.Println("[WAF] 挑战响应更新了密码加密上下文")
				}
			}
			// The browser encrypts the password again after AMS completes. JWE uses
			// fresh randomness, and AWS rejects a ciphertext reused from the request
			// that originally triggered the challenge.
			encrypted, err = r.JWE.Encrypt(r.Cfg.Password, pubKeyMap, issuer, audience, region)
			if err != nil {
				return fmt.Errorf("JWE 重新加密失败: %w", err)
			}
			passwordInput["password"] = encrypted
			userEvent["timeSpentOnPage"] = elapsedMillisSince(r.PasswordPageStartedAt, time.Now())
			fingerprintInput["fingerPrint"] = genPasswordFP()
			// In the browser the AMS callback closes the challenge before the user
			// submits the password form again. Give the redeemed code the same brief
			// propagation window instead of consuming it in the same instant.
			if err := r.wait(time.Second); err != nil {
				return err
			}
			log.Println("[WAF] 动态验证完成，正在重新提交密码")
			body, respH, err = submitPassword()
			if err != nil {
				return err
			}
			httputil.SaveCookies(r.Cookies, respH)
			rurl = passwordRedirect(body)
			if rurl == "" && hadInitialChallenge {
				if nextChallenge, ok := parseAWSWAFChallenge(body); ok {
					log.Printf("[WAF] 密码重试仍返回挑战: tokenRotated=%t, stateUpdated=%t, stepUpdated=%t",
						nextChallenge.RedemptionToken != initialChallenge.RedemptionToken,
						nextChallenge.WorkflowStateHandle != "" && nextChallenge.WorkflowStateHandle != initialChallenge.WorkflowStateHandle,
						nextChallenge.StepID != "" && nextChallenge.StepID != initialChallenge.StepID)
				}
			}
		}
	}
	if rurl == "" {
		return unexpectedServiceResponse("密码设置未返回 redirect", body)
	}

	wh := httputil.ExtractParam(rurl, "workflowStateHandle")
	st := httputil.ExtractParam(rurl, "state")
	rh := httputil.ExtractParam(rurl, "workflowResultHandle")
	return r.completeSignup(wh, st, rh)
}

func passwordCreationStepID(response map[string]interface{}) string {
	if stepID, ok := response["stepId"].(string); ok && strings.TrimSpace(stepID) != "" {
		return stepID
	}
	return "get-new-password-for-password-creation"
}

func (r *Registrar) readBuilderIDSession(sourceDirectory, referer string) (string, error) {
	sourceDirectory = strings.Trim(strings.TrimSpace(sourceDirectory), "/")
	if sourceDirectory == "" || strings.ContainsAny(sourceDirectory, "?#") {
		return "", fmt.Errorf("Builder ID 源目录无效")
	}
	endpoint := fmt.Sprintf("%s/platform/%s/cookieread", r.Cfg.SigninBase, sourceDirectory)
	headers := map[string]string{
		"Accept":                    "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
		"Accept-Language":           "zh-CN,zh;q=0.9,en;q=0.8",
		"User-Agent":                r.Identity.UA,
		"Referer":                   referer,
		"sec-ch-ua":                 r.Identity.SecUA,
		"sec-ch-ua-mobile":          "?0",
		"sec-ch-ua-platform":        `"Windows"`,
		"sec-fetch-dest":            "iframe",
		"sec-fetch-mode":            "navigate",
		"sec-fetch-site":            "same-origin",
		"Upgrade-Insecure-Requests": "1",
	}
	body, status, _, err := r.DoGet(endpoint, headers)
	if err != nil {
		return "", err
	}
	if status < 200 || status >= 300 {
		return "", fmt.Errorf("Builder ID cookieread HTTP %d", status)
	}
	var response struct {
		CookieValue string `json:"cookieValue"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return "", fmt.Errorf("Builder ID cookieread 响应格式无效")
	}
	return response.CookieValue, nil
}

func (r *Registrar) solvePasswordWAFChallenge(body []byte, websiteURL string, passwordPayload map[string]interface{}) (bool, error) {
	challenge, ok := parseAWSWAFChallenge(body)
	if !ok {
		return false, nil
	}
	if challenge.StepID != "" {
		passwordPayload["stepId"] = challenge.StepID
	}
	if challenge.WorkflowStateHandle != "" {
		passwordPayload["workflowStateHandle"] = challenge.WorkflowStateHandle
		r.WorkflowHandle = challenge.WorkflowStateHandle
	}
	if amsInspectionEnabled() {
		reportPath, err := r.captureAMSScript(challenge.JSAPIScript, challenge.RedemptionToken, websiteURL)
		if err != nil {
			return false, fmt.Errorf("AMS 诊断采集失败: %w", err)
		}
		log.Printf("[WAF] AMS SDK 诊断已保存: %s", reportPath)
		return false, fmt.Errorf("AMS 诊断捕获完成")
	}
	log.Println("[WAF] AWS 要求 AMS 动态验证，正在读取挑战类型")
	tokenPayload, amsChallenge, err := r.loadAMSChallenge(challenge.RedemptionToken, websiteURL)
	if err != nil {
		return false, fmt.Errorf("AWS WAF 动态验证失败: %w", err)
	}
	solverProxy := captcha.RemoteWorkerProxy(r.Cfg.Proxy)
	if strings.TrimSpace(r.Cfg.Proxy) != "" && solverProxy == "" {
		log.Println("[WAF] 当前任务代理仅本机可访问，WAF_GRID 将使用 2Captcha 代理池")
	}
	solveCtx, cancelSolve := context.WithTimeout(r.context(), 3*time.Minute)
	var accessCode string
	switch strings.ToUpper(amsChallenge.ChallengeType) {
	case "AMCS":
		log.Println("[WAF] 挑战类型: AMCS，正在识别验证图片")
		accessCode, err = r.solveAMCSImage(solveCtx, tokenPayload, amsChallenge, challenge.RedemptionToken, websiteURL)
	case "WAF_GRID":
		log.Println("[WAF] 挑战类型: WAF_GRID，正在通过 2Captcha 处理")
		accessCode, err = r.solveAMSWAFGrid(solveCtx, tokenPayload, amsChallenge, challenge.RedemptionToken, websiteURL, solverProxy)
	default:
		err = fmt.Errorf("不支持的 AMS challengeType: %s", amsChallenge.ChallengeType)
	}
	cancelSolve()
	if err != nil {
		var apiErr *captcha.APIError
		if errors.As(err, &apiErr) && apiErr.Code == "ERROR_CAPTCHA_UNSOLVABLE" {
			return false, fmt.Errorf("AWS WAF 动态验证失败: %w", err)
		}
		return false, fmt.Errorf("AWS WAF 动态验证失败: %w", err)
	}
	passwordPayload["captchaRequest"] = map[string]string{
		"captchaAccessCode": accessCode,
	}
	return true, nil
}

func passwordRedirect(body []byte) string {
	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return ""
	}
	redirect, _ := data["redirect"].(map[string]interface{})
	url, _ := redirect["url"].(string)
	return url
}

// completeSignup 完成注册工作流
func (r *Registrar) completeSignup(wh, state, rh string) error {
	log.Println("[12.5] 完成注册工作流")
	api := fmt.Sprintf("%s/platform/%s/api/execute", r.Cfg.SigninBase, r.Cfg.DirectoryID)
	ref := fmt.Sprintf("%s/platform/%s/login?workflowStateHandle=%s&state=%s&workflowResultHandle=%s",
		r.Cfg.SigninBase, r.Cfg.DirectoryID, wh, state, rh)
	fp := r.GenFP("signin", "PageLoad", 0, "")

	rid := NewUUID()
	h := r.BuildHeaders(ref, r.Cfg.SigninBase)
	h["x-amzn-requestid"] = rid
	h["x-amz-date"] = GmtDate()
	h["priority"] = "u=1, i"

	body, _, respH, err := r.DoPostRaw(api, map[string]interface{}{
		"stepId": "", "workflowStateHandle": wh,
		"workflowResultHandle": rh, "state": state,
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
	if data["stepId"] != "end-of-workflow-success" {
		return fmt.Errorf("完成工作流失败: %v", data["stepId"])
	}

	if redir, ok := data["redirect"].(map[string]interface{}); ok {
		if rurl, ok := redir["url"].(string); ok {
			r.AuthCode = httputil.ExtractParam(rurl, "workflowResultHandle")
			r.SSOState = httputil.ExtractParam(rurl, "state")
			r.WdcCSRFToken = httputil.ExtractParam(rurl, "wdc_csrf_token")
		}
	}
	return nil
}
