package core

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/url"
	"strings"
	"time"

	fhttp "github.com/bogdanfinn/fhttp"
	httputil "reg_go/internal/http"
)

// Step12_8SSOWorkflow SSO 工作流
func (r *Registrar) Step12_8SSOWorkflow() error {
	log.Println("[12.8] SSO 工作流")

	redirectURL := url.QueryEscape(r.Cfg.ViewBase + "/start/#/")
	loginURL := fmt.Sprintf("%s/login?directory_id=view&redirect_url=%s", r.Cfg.PortalBase, redirectURL)

	h := map[string]string{
		"Accept":             "*/*",
		"User-Agent":         r.Identity.UA,
		"Origin":             r.Cfg.ViewBase,
		"Referer":            r.Cfg.ViewBase + "/",
		"sec-ch-ua":          r.Identity.SecUA,
		"sec-ch-ua-mobile":   "?0",
		"sec-ch-ua-platform": `"Windows"`,
		"sec-fetch-dest":     "empty",
		"sec-fetch-mode":     "cors",
		"sec-fetch-site":     "cross-site",
		"priority":           "u=1, i",
	}
	if v, ok := r.Cookies["awsccc"]; ok {
		h["Cookie"] = "awsccc=" + v
	}

	body, _, respH, err := r.DoGet(loginURL, h)
	if err != nil {
		return err
	}
	httputil.SaveCookies(r.Cookies, respH)

	var data map[string]interface{}
	json.Unmarshal(body, &data)
	if csrf, ok := data["csrfToken"].(string); ok {
		r.Cookies["loginCsrfToken"] = csrf
	}

	rurl, _ := data["redirectUrl"].(string)
	var wh string
	if strings.Contains(rurl, "workflowStateHandle=") {
		wh = httputil.SplitAfter(rurl, "workflowStateHandle=")
	}
	if wh == "" {
		return fmt.Errorf("SSO 无法获取 workflowStateHandle")
	}

	return r.completeSSOWorkflow(wh)
}

// completeSSOWorkflow 完成 SSO 工作流
func (r *Registrar) completeSSOWorkflow(wh string) error {
	api := fmt.Sprintf("%s/platform/%s/api/execute", r.Cfg.SigninBase, r.Cfg.DirectoryID)
	ref := fmt.Sprintf("%s/platform/%s/login?workflowStateHandle=%s",
		r.Cfg.SigninBase, r.Cfg.DirectoryID, wh)
	fp := r.GenFP("signin", "PageLoad", 0, "")

	rid := NewUUID()
	h := r.BuildHeaders(ref, r.Cfg.SigninBase)
	h["x-amzn-requestid"] = rid
	h["x-amz-date"] = GmtDate()
	h["priority"] = "u=1, i"

	body, _, respH, err := r.DoPostRaw(api, map[string]interface{}{
		"stepId": "", "workflowStateHandle": wh,
		"inputs":    []interface{}{map[string]string{"input_type": "FingerPrintRequestInput", "fingerPrint": fp}},
		"requestId": rid,
	}, h)
	if err != nil {
		return err
	}
	httputil.SaveCookies(r.Cookies, respH)

	var data map[string]interface{}
	json.Unmarshal(body, &data)
	newWH, _ := data["workflowStateHandle"].(string)
	if newWH == "" {
		newWH = wh
	}

	if data["stepId"] == "start" {
		fp = r.GenFP("signin", "PageLoad", 0, "")
		rid = NewUUID()
		h = r.BuildHeaders(ref, r.Cfg.SigninBase)
		h["x-amzn-requestid"] = rid
		h["x-amz-date"] = GmtDate()
		h["priority"] = "u=1, i"

		body, _, respH, err = r.DoPostRaw(api, map[string]interface{}{
			"stepId": "start", "workflowStateHandle": newWH,
			"inputs":    []interface{}{map[string]string{"input_type": "FingerPrintRequestInput", "fingerPrint": fp}},
			"requestId": rid,
		}, h)
		if err != nil {
			return err
		}
		httputil.SaveCookies(r.Cookies, respH)
		json.Unmarshal(body, &data)
	}

	if data["stepId"] == "end-of-workflow-success" {
		if redir, ok := data["redirect"].(map[string]interface{}); ok {
			if rurl, ok := redir["url"].(string); ok {
				r.AuthCode = httputil.ExtractParam(rurl, "workflowResultHandle")
				r.SSOState = httputil.ExtractParam(rurl, "state")
				r.WdcCSRFToken = httputil.ExtractParam(rurl, "wdc_csrf_token")
			}
		}
	}

	// 访问 start 页面
	params := url.Values{}
	if r.SSOState != "" {
		params.Set("state", r.SSOState)
	}
	params.Set("workflowResultHandle", r.AuthCode)
	if r.WdcCSRFToken != "" {
		params.Set("wdc_csrf_token", r.WdcCSRFToken)
	}
	startURL := r.Cfg.ViewBase + "/start/?" + params.Encode()

	h2 := map[string]string{
		"Accept":         "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
		"User-Agent":     r.Identity.UA,
		"Referer":        r.Cfg.SigninBase + "/",
		"sec-fetch-dest": "document",
		"sec-fetch-mode": "navigate",
	}
	var cookieParts []string
	if v, ok := r.Cookies["loginCsrfToken"]; ok {
		cookieParts = append(cookieParts, "loginCsrfToken="+v)
	}
	if v, ok := r.Cookies["awsccc"]; ok {
		cookieParts = append(cookieParts, "awsccc="+v)
	}
	if len(cookieParts) > 0 {
		h2["Cookie"] = strings.Join(cookieParts, "; ")
	}
	r.DoGet(startURL, h2)
	return nil
}

// Step13SSOToken 获取 SSO Token
func (r *Registrar) Step13SSOToken() (map[string]interface{}, error) {
	log.Println("[13] 获取 SSO Token")
	csrf := r.Cookies["loginCsrfToken"]
	if csrf == "" {
		return nil, fmt.Errorf("缺少 loginCsrfToken")
	}

	h := map[string]string{
		"Accept":               "application/json, text/plain, */*",
		"Content-Type":         "application/x-www-form-urlencoded",
		"User-Agent":           r.Identity.UA,
		"Origin":               r.Cfg.ViewBase,
		"Referer":              r.Cfg.ViewBase + "/",
		"x-amz-sso-csrf-token": csrf,
		"sec-ch-ua":            r.Identity.SecUA,
		"sec-ch-ua-mobile":     "?0",
		"sec-ch-ua-platform":   `"Windows"`,
		"sec-fetch-dest":       "empty",
		"sec-fetch-mode":       "cors",
		"sec-fetch-site":       "cross-site",
		"priority":             "u=1, i",
	}

	formData := url.Values{
		"authCode": {r.AuthCode},
		"state":    {r.SSOState},
		"orgId":    {"view"},
	}

	client := httputil.NewTLSClient(r.Cfg.Proxy, true, r.Identity.ChromeVer)
	defer client.CloseIdleConnections()

	for retry := 0; retry < 5; retry++ {
		req, err := fhttp.NewRequestWithContext(r.context(), "POST", r.Cfg.PortalBase+"/auth/sso-token",
			strings.NewReader(formData.Encode()))
		if err != nil {
			return nil, err
		}
		httputil.SetHeaders(req, h)
		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, err
		}

		var data map[string]interface{}
		json.Unmarshal(body, &data)

		if tok, ok := data["token"].(string); ok && tok != "" {
			r.SSOToken = tok
			break
		}
		if errMsg, _ := data["errorMessage"].(string); strings.Contains(strings.ToLower(errMsg), "not authorized") {
			if err := r.wait(3 * time.Second); err != nil {
				return nil, err
			}
			continue
		}
		return nil, fmt.Errorf("SSO Token 失败: %s", string(body))
	}
	if r.SSOToken == "" {
		return nil, fmt.Errorf("SSO Token 重试 5 次仍失败")
	}

	// Accept device + Associate token
	body, _, err := r.DoPost(r.Cfg.OIDCBase+"/device_authorization/accept_user_code",
		map[string]interface{}{"userCode": r.UserCode, "userSessionId": r.SSOToken},
		map[string]string{"Content-Type": "application/json"})
	if err != nil {
		return nil, err
	}
	var dcData map[string]interface{}
	json.Unmarshal(body, &dcData)
	dc := dcData["deviceContext"]

	r.DoPost(r.Cfg.OIDCBase+"/device_authorization/associate_token",
		map[string]interface{}{"deviceContext": dc, "userSessionId": r.SSOToken},
		map[string]string{"Content-Type": "application/json"})

	// 轮询 token
	for i := 0; i < 30; i++ {
		body, status, _, err := r.DoPostRaw(r.Cfg.OIDCBase+"/token", map[string]interface{}{
			"clientId": r.ClientID, "clientSecret": r.ClientSecret,
			"deviceCode": r.DeviceCode,
			"grantType":  "urn:ietf:params:oauth:grant-type:device_code",
		}, map[string]string{"Content-Type": "application/json"})
		if err != nil {
			return nil, err
		}
		if status == 200 {
			var result map[string]interface{}
			json.Unmarshal(body, &result)
			return result, nil
		}
		if err := r.wait(2 * time.Second); err != nil {
			return nil, err
		}
	}
	return nil, fmt.Errorf("Token 轮询超时")
}
