package core

import (
	"encoding/json"
	"fmt"
	"strings"

	httputil "reg_go/internal/http"
)

// BuildHeaders 构建通用请求头
func (r *Registrar) BuildHeaders(referer, origin string) map[string]string {
	h := map[string]string{
		"Accept":             "application/json, text/plain, */*",
		"Accept-Language":    "zh-CN,zh;q=0.9,en;q=0.8",
		"Accept-Encoding":    "gzip, deflate, br",
		"Content-Type":       "application/json",
		"User-Agent":         r.Identity.UA,
		"sec-ch-ua":          r.Identity.SecUA,
		"sec-ch-ua-mobile":   "?0",
		"sec-ch-ua-platform": `"Windows"`,
		"sec-fetch-dest":     "empty",
		"sec-fetch-mode":     "cors",
		"sec-fetch-site":     "same-origin",
	}
	if referer != "" {
		h["Referer"] = referer
	}
	if origin != "" {
		h["Origin"] = origin
	}
	return h
}

// BuildProfileHeaders 构建 profile 页面请求头
func (r *Registrar) BuildProfileHeaders(referer string) map[string]string {
	h := map[string]string{
		"Accept":             "*/*",
		"Accept-Language":    "zh-CN,zh;q=0.9,en;q=0.8",
		"Content-Type":       "application/json;charset=UTF-8",
		"User-Agent":         r.Identity.UA,
		"Origin":             r.Cfg.ProfileBase,
		"Referer":            referer,
		"sec-ch-ua":          r.Identity.SecUA,
		"sec-ch-ua-mobile":   "?0",
		"sec-ch-ua-platform": `"Windows"`,
		"sec-fetch-dest":     "empty",
		"sec-fetch-mode":     "cors",
		"sec-fetch-site":     "same-origin",
		"priority":           "u=1, i",
	}
	keys := []string{"awsccc", "aws-user-profile-ubid", "i18next", "aws-waf-token"}
	if _, ok := r.Cookies["awsd2c-token"]; ok {
		keys = append(keys, "awsd2c-token", "awsd2c-token-c")
	}
	var parts []string
	for _, k := range keys {
		if v, ok := r.Cookies[k]; ok {
			parts = append(parts, fmt.Sprintf("%s=%s", k, v))
		}
	}
	if len(parts) > 0 {
		h["Cookie"] = strings.Join(parts, "; ")
	}
	return h
}

// CookieString 将 cookies 拼接为字符串
func (r *Registrar) CookieString() string {
	var parts []string
	for k, v := range r.Cookies {
		parts = append(parts, fmt.Sprintf("%s=%s", k, v))
	}
	return strings.Join(parts, "; ")
}

// BuildProfileNavigationHeaders mirrors the document request that loads the
// Profile app. The browser sends the current AWS cookies on this request too;
// keeping them on the initial document load makes the subsequent FWCIM and
// D2C-token state belong to the same session.
func (r *Registrar) BuildProfileNavigationHeaders(referer string) map[string]string {
	h := map[string]string{
		"Accept":                    "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
		"Accept-Language":           "zh-CN,zh;q=0.9,en;q=0.8",
		"User-Agent":                r.Identity.UA,
		"sec-ch-ua":                 r.Identity.SecUA,
		"sec-ch-ua-mobile":          "?0",
		"sec-ch-ua-platform":        `"Windows"`,
		"sec-fetch-dest":            "document",
		"sec-fetch-mode":            "navigate",
		"sec-fetch-site":            "cross-site",
		"Upgrade-Insecure-Requests": "1",
	}
	if referer != "" {
		h["Referer"] = referer
	}
	if cookie := r.profileCookieString(); cookie != "" {
		h["Cookie"] = cookie
	}
	return h
}

func (r *Registrar) profileCookieString() string {
	keys := []string{"awsccc", "aws-user-profile-ubid", "awsd2c-token", "awsd2c-token-c", "i18next", "aws-waf-token"}
	var parts []string
	for _, key := range keys {
		if value := strings.TrimSpace(r.Cookies[key]); value != "" {
			parts = append(parts, fmt.Sprintf("%s=%s", key, value))
		}
	}
	return strings.Join(parts, "; ")
}

// FetchD2CToken 获取 D2C Token
func (r *Registrar) FetchD2CToken(origin, referer string) error {
	headers := map[string]string{
		"Accept":             "*/*",
		"Content-Type":       "application/json",
		"User-Agent":         r.Identity.UA,
		"Origin":             origin,
		"Referer":            referer,
		"sec-ch-ua":          r.Identity.SecUA,
		"sec-ch-ua-mobile":   "?0",
		"sec-ch-ua-platform": `"Windows"`,
		"sec-fetch-dest":     "empty",
		"sec-fetch-mode":     "cors",
		"sec-fetch-site":     "cross-site",
		"priority":           "u=1, i",
	}
	var parts []string
	if v, ok := r.Cookies["awsccc"]; ok {
		parts = append(parts, "awsccc="+v)
	}
	if old, ok := r.Cookies["awsd2c-token"]; ok {
		parts = append(parts, "awsd2c-token="+old, "awsd2c-token-c="+old)
	}
	if len(parts) > 0 {
		headers["Cookie"] = strings.Join(parts, "; ")
	}

	// 手动浏览器流程 (HAR entries 34/63) 每次都本地生成一个 ES256 自签 JWT
	// {"vid":<uuid>,"iss":"s_p"} 提交, 并把 vid 作为后续 api/execute 的 visitorId。
	// 项目原先提交空 body, 与真实浏览器行为不一致。
	vid := newVisitorUUID()
	jwt, err := webVisorJWT(vid)
	if err != nil {
		return err
	}
	payload := map[string]interface{}{"token": jwt}

	body, respHeaders, err := r.DoPost("https://vs.aws.amazon.com/token", payload, headers)
	if err != nil {
		return err
	}
	httputil.SaveCookies(r.Cookies, respHeaders)

	var data map[string]interface{}
	json.Unmarshal(body, &data)
	if tok, ok := data["token"].(string); ok && tok != "" {
		r.Cookies["awsd2c-token"] = tok
		r.Cookies["awsd2c-token-c"] = tok
	}
	// visitorId 采用本地生成的 vid (与浏览器一致: 后续请求复用同一 vid)。
	r.VisitorID = vid
	return nil
}
