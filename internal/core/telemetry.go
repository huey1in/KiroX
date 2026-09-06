package core

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	mrand "math/rand"
	"net/url"
	"strings"
	"time"
)

// ──────────────────────────────────────────────────────────────
// AWS signin / profile 前端遥测 (手动浏览器流程必发, 风控相关性高)。
// 对照 app.kiro.dev.har entries 21/27/101 (/metrics/fingerprint)、
// 97 (/platform/user-event/send-event)、37/66 (d2c collector)、
// 34/63 (vs.aws.amazon.com/token WebVisor JWT)。
// 这些调用全部 best-effort: 失败只记日志不中断注册主流程。
// ──────────────────────────────────────────────────────────────

// PostFingerprintMetric 上报指纹指标 (application/x-www-form-urlencoded)。
// metricName 形如 "IsFingerprintGenerated:Success"; operation 形如
// "AWSSignin:FingerprintMetrics:start"。
func (r *Registrar) PostFingerprintMetric(metricName, value, operation string) error {
	ref := fmt.Sprintf("%s/platform/%s/login?workflowStateHandle=%s",
		r.Cfg.SigninBase, r.Cfg.DirectoryID, r.WorkflowHandle)
	return r.postFingerprintMetricAt(metricName, value, operation, ref)
}

func (r *Registrar) postFingerprintMetricAt(metricName, value, operation, ref string) error {
	if !r.Cfg.TelemetryEnabled {
		return nil
	}
	api := r.Cfg.SigninBase + "/metrics/fingerprint"

	form := url.Values{}
	form.Set("name", metricName)
	form.Set("value", value)
	form.Set("operation", operation)

	h := r.BuildHeaders(ref, r.Cfg.SigninBase)
	h["Content-Type"] = "application/x-www-form-urlencoded;charset=UTF-8"

	body, status, _, err := r.DoPostBodyRaw(api, form.Encode(), h)
	if err != nil {
		return err
	}
	if status != 200 {
		return fmt.Errorf("metrics/fingerprint HTTP %d: %s", status, truncateBytes(body, 120))
	}
	return nil
}

// PostFingerprintMetricSafe 上报指纹指标, 失败仅记录日志。
func (r *Registrar) PostFingerprintMetricSafe(metricName, value, operation string) {
	if err := r.PostFingerprintMetric(metricName, value, operation); err != nil {
		log.Printf("[遥测] %s 上报失败: %v", metricName, err)
	}
}

// SendUserEvent 上报页面事件 (signin.aws 页面加载/提交行为)。
// 对照 HAR entry 97: PAGE_LOAD / CREDENTIAL_COLLECTION。
func (r *Registrar) SendUserEvent(eventType, pageName string, timeSpentOnPage int64) error {
	return r.sendUserEvent(r.Cfg.DirectoryID, eventType, pageName, timeSpentOnPage)
}

func (r *Registrar) sendUserEvent(directoryID, eventType, pageName string, timeSpentOnPage int64) error {
	if !r.Cfg.TelemetryEnabled {
		return nil
	}
	api := r.Cfg.SigninBase + "/platform/user-event/send-event"
	ref := fmt.Sprintf("%s/platform/%s/signup?registrationCode=%s&state=%s",
		r.Cfg.SigninBase, r.Cfg.DirectoryID, r.RegCode, r.SignState)
	fp := r.GenFP("signup", "PageLoad", 0, "")

	userEvent := map[string]interface{}{
		"input_type": "UserEvent",
		"eventType":  eventType,
		"pageName":   pageName,
	}
	if timeSpentOnPage > 0 {
		userEvent["timeSpentOnPage"] = timeSpentOnPage
	}

	rid := NewUUID()
	h := r.BuildHeaders(ref, r.Cfg.SigninBase)
	h["x-amzn-requestid"] = rid
	h["x-amz-date"] = GmtDate()
	h["priority"] = "u=1, i"

	body, status, _, err := r.DoPostRaw(api, map[string]interface{}{
		"inputs": []interface{}{
			map[string]interface{}{
				"input_type":  "UserEventRequestInput",
				"directoryId": directoryID,
				"userName":    r.Email,
				"userEvents":  []interface{}{userEvent},
			},
			map[string]string{"input_type": "FingerPrintRequestInput", "fingerPrint": fp},
		},
		"requestId": rid,
	}, h)
	if err != nil {
		return err
	}
	if status != 200 {
		return fmt.Errorf("user-event/send-event HTTP %d: %s", status, truncateBytes(body, 120))
	}
	return nil
}

func (r *Registrar) sendUserEventSafe(directoryID, eventType, pageName string, timeSpentOnPage int64) {
	if err := r.sendUserEvent(directoryID, eventType, pageName, timeSpentOnPage); err != nil {
		log.Printf("[遥测] %s/%s 上报失败: %v", pageName, eventType, err)
	}
}

func (r *Registrar) SendUserEventSafe(eventType, pageName string, timeSpentOnPage int64) {
	r.sendUserEventSafe(r.Cfg.DirectoryID, eventType, pageName, timeSpentOnPage)
}

// PostD2CEvent 上报 D2C 遥测事件 (timeTakenToFetchVID)。
// 对照 HAR entries 37/66。
func (r *Registrar) PostD2CEvent(pageURL string, fetchDuration time.Duration) error {
	if !r.Cfg.TelemetryEnabled {
		return nil
	}
	const api = "https://d2c.aws.amazon.com/csds/collector/v1/events/batch"

	origin := r.Cfg.SigninBase
	if strings.HasPrefix(pageURL, r.Cfg.ProfileBase) {
		origin = r.Cfg.ProfileBase
	}
	h := map[string]string{
		"Accept":             "application/json",
		"Accept-Language":    "zh-CN,zh;q=0.9,en;q=0.8",
		"Content-Type":       "application/json",
		"User-Agent":         r.Identity.UA,
		"Origin":             origin,
		"Referer":            pageURL,
		"sec-ch-ua":          r.Identity.SecUA,
		"sec-ch-ua-mobile":   "?0",
		"sec-ch-ua-platform": `"Windows"`,
		"sec-fetch-dest":     "empty",
		"sec-fetch-mode":     "cors",
		"sec-fetch-site":     "cross-site",
		"priority":           "u=1, i",
	}

	payload := map[string]interface{}{
		"batchId":       "D2CLogger",
		"schemaVersion": "1.0.0",
		"batchEvents": []interface{}{
			map[string]interface{}{
				"pageURL":        pageURL,
				"eventType":      "logEvent",
				"eventTimestamp": time.Now().UnixMilli(),
				"customData": map[string]interface{}{
					"timeTakenToFetchVID": fmt.Sprintf("%.1f", float64(fetchDuration.Microseconds())/1000.0),
					"logLevel":            "info",
				},
				"orgId": "awsme_scode",
			},
		},
	}

	body, status, _, err := r.DoPostRaw(api, payload, h)
	if err != nil {
		return err
	}
	if status != 200 && status != 202 {
		return fmt.Errorf("d2c events/batch HTTP %d: %s", status, truncateBytes(body, 120))
	}
	return nil
}

// PostD2CEventSafe 上报 D2C 遥测, 失败仅记录日志。
func (r *Registrar) PostD2CEventSafe(pageURL string, fetchDuration time.Duration) {
	if err := r.PostD2CEvent(pageURL, fetchDuration); err != nil {
		log.Printf("[遥测] d2c events 上报失败: %v", err)
	}
}

// webVisorJWT 生成 WebVisor 客户端自签 ES256 JWT。
// 对照 HAR entries 34/63: {"kid":<uuid>,"alg":"ES256"}.
// {"vid":<uuid>,"iss":"s_p","exp":<unix>}.
func webVisorJWT(vid string) (string, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return "", err
	}

	header := map[string]interface{}{"kid": NewUUID(), "alg": "ES256"}
	payload := map[string]interface{}{
		"vid": vid,
		"iss": "s_p",
		"exp": time.Now().Add(time.Hour).Unix(),
	}
	hb, _ := json.Marshal(header)
	pb, _ := json.Marshal(payload)
	signingInput := base64.RawURLEncoding.EncodeToString(hb) + "." + base64.RawURLEncoding.EncodeToString(pb)

	digest := sha256.Sum256([]byte(signingInput))
	rr, ss, err := ecdsa.Sign(rand.Reader, key, digest[:])
	if err != nil {
		return "", err
	}
	// 与 WebCrypto ES256 一致: 签名值固定 32 字节大端拼接。
	sig := make([]byte, 64)
	rr.FillBytes(sig[:32])
	ss.FillBytes(sig[32:])

	return signingInput + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}

// newVisitorUUID 生成 UUID v4 风格的 visitor id (vid)。
func newVisitorUUID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// 极端情况下回退到时间随机数, 保持格式合法。
		now := time.Now().UnixNano()
		for i := range b {
			b[i] = byte(now >> (8 * (i % 8)))
		}
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// truncateBytes 截断响应体用于错误信息。
func truncateBytes(b []byte, n int) string {
	s := strings.TrimSpace(string(b))
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}

// ──────────────────────────────────────────────────────────────
// katal / unagi 遥测 (手动浏览器在 profile 页必经, 风控环境一致性)。
// 对照 HAR entries 69/72:
//   POST https://unagi-na.amazon.com/1/events/com.amazon.eel.katal.metrics.core.nexus
//   Content-Type: text/plain;charset=UTF-8, Origin: https://profile.aws.amazon.com,
//   sec-fetch-mode: no-cors, body = {"cs":{"dct":{...}},"events":[...]}
// ──────────────────────────────────────────────────────────────

const katalNexusURL = "https://unagi-na.amazon.com/1/events/com.amazon.eel.katal.metrics.core.nexus"

// katalEntry 一条 katal 指标 (metricKey + schema + value)。
type katalEntry struct {
	Key    string
	Schema string
	Value  float64
}

// katalGroup 一组同 producer 的指标 (对应 HAR dct 中的一个 actionId 分组)。
type katalGroup struct {
	Producer string
	Metrics  []katalEntry
}

// PostKatalNexus 上报一批 katal 指标 (失败仅日志)。
func (r *Registrar) PostKatalNexus(groups []katalGroup) {
	if !r.Cfg.TelemetryEnabled {
		return
	}
	if len(groups) == 0 {
		return
	}
	now := time.Now()
	dct := map[string]interface{}{
		"#0": "site", "#1": "AWSUserProfileFrontEnd", "#2": "serviceName",
		"#3": "actionId", "#5": "cloudWatchDimensions", "#6": "methodName",
		"#8": "metricKey", "#10": "value", "#11": "isMonitor",
		"#12": "producerId", "#13": "katal", "#14": "schemaId",
		"#16": "timestamp", "#17": "messageId",
	}
	var events []interface{}
	slot := 18
	for _, g := range groups {
		if len(g.Metrics) == 0 {
			continue
		}
		actionID := NewUUID()
		dct["#4"] = actionID
		dct["#7"] = g.Producer
		first := g.Metrics[0]
		dct["#9"] = first.Key
		dct["#15"] = first.Schema
		metricSlots := make([]struct{ key, schema string }, len(g.Metrics))
		metricSlots[0] = struct{ key, schema string }{"#9", "#15"}
		for i := 1; i < len(g.Metrics); i++ {
			keySlot := fmt.Sprintf("#%d", slot)
			slot++
			schemaSlot := fmt.Sprintf("#%d", slot)
			slot++
			dct[keySlot] = g.Metrics[i].Key
			dct[schemaSlot] = g.Metrics[i].Schema
			metricSlots[i] = struct{ key, schema string }{keySlot, schemaSlot}
		}
		for i, m := range g.Metrics {
			events = append(events, map[string]interface{}{
				"data": map[string]interface{}{
					"#0": "#1", "#2": "#1", "#3": "#4", "#6": "#7",
					"#8": metricSlots[i].key, "#10": m.Value, "#11": true,
					"#12": "#13", "#14": metricSlots[i].schema,
					"#16": now.UTC().Format("2006-01-02T15:04:05.000Z"),
					"#17": fmt.Sprintf("1-%d-%d", now.UnixMilli(), 1000000000+mrand.Int63n(9000000000)),
				},
			})
		}
	}

	payload, err := json.Marshal(map[string]interface{}{
		"cs":     map[string]interface{}{"dct": dct},
		"events": events,
	})
	if err != nil {
		log.Printf("[遥测] katal 序列化失败: %v", err)
		return
	}

	h := map[string]string{
		"Accept":             "*/*",
		"Content-Type":       "text/plain;charset=UTF-8",
		"User-Agent":         r.Identity.UA,
		"Origin":             r.Cfg.ProfileBase,
		"sec-ch-ua":          r.Identity.SecUA,
		"sec-ch-ua-mobile":   "?0",
		"sec-ch-ua-platform": `"Windows"`,
		"sec-fetch-dest":     "empty",
		"sec-fetch-mode":     "no-cors",
		"sec-fetch-site":     "cross-site",
		"priority":           "u=1, i",
	}
	body, status, _, err := r.DoPostRaw(katalNexusURL, json.RawMessage(payload), h)
	if err != nil {
		log.Printf("[遥测] katal 上报失败: %v", err)
		return
	}
	if status != 200 && status != 202 && status != 204 {
		log.Printf("[遥测] katal 上报 HTTP %d: %s", status, truncateBytes(body, 120))
	}
}

// PostKatalSignupBatch 上报注册期批次 (对照 HAR entry 69): 在 SendOTP 成功后调用。
func (r *Registrar) PostKatalSignupBatch(getConfigMs, sendOTPMs float64) {
	r.PostKatalNexus([]katalGroup{
		{
			Producer: "AppContextProvider",
			Metrics: []katalEntry{
				{Key: "HTTPRequest.GetConfig.StatusCode.200", Schema: "katal.client.metrics.Counter.3", Value: 1},
				{Key: "HTTPRequest.GetConfig.StatusCode.2XX", Schema: "katal.client.metrics.Counter.3", Value: 1},
				{Key: "HTTPRequest.GetConfig.Latency", Schema: "katal.client.metrics.Timer.2", Value: getConfigMs},
				{Key: "HTTPRequest.GetConfig.Failure", Schema: "katal.client.metrics.Counter.3", Value: 0},
				{Key: "HTTPRequest.GetAppContext.StatusCode.404", Schema: "katal.client.metrics.Counter.3", Value: 1},
				{Key: "HTTPRequest.GetAppContext.StatusCode.4XX", Schema: "katal.client.metrics.Counter.3", Value: 1},
				{Key: "HTTPRequest.GetAppContext.Latency", Schema: "katal.client.metrics.Timer.2", Value: getConfigMs},
				{Key: "HTTPRequest.GetAppContext.Failure", Schema: "katal.client.metrics.Counter.3", Value: 1},
			},
		},
		{
			Producer: "Steps",
			Metrics: []katalEntry{
				{Key: "StartSignUp", Schema: "katal.client.metrics.Timer.2", Value: getConfigMs},
			},
		},
		{
			Producer: "SignUpContextProvider",
			Metrics: []katalEntry{
				{Key: "HTTPRequest.SendOTP.StatusCode.200", Schema: "katal.client.metrics.Counter.3", Value: 1},
				{Key: "HTTPRequest.SendOTP.StatusCode.2XX", Schema: "katal.client.metrics.Counter.3", Value: 1},
				{Key: "HTTPRequest.SendOTP.Latency", Schema: "katal.client.metrics.Timer.2", Value: sendOTPMs},
				{Key: "HTTPRequest.SendOTP.Failure", Schema: "katal.client.metrics.Counter.3", Value: 0},
			},
		},
	})
}

// PostKatalVerificationBatch 上报验证批次 (对照 HAR entry 72): 在 CreateIdentity 成功后调用。
func (r *Registrar) PostKatalVerificationBatch(verificationMs, createIdentityMs float64) {
	r.PostKatalNexus([]katalGroup{
		{
			Producer: "Steps",
			Metrics: []katalEntry{
				{Key: "SignUpEmailVerificationStep", Schema: "katal.client.metrics.Timer.2", Value: verificationMs},
			},
		},
		{
			Producer: "SignUpContextProvider",
			Metrics: []katalEntry{
				{Key: "HTTPRequest.CreateIdentity.StatusCode.200", Schema: "katal.client.metrics.Counter.3", Value: 1},
				{Key: "HTTPRequest.CreateIdentity.StatusCode.2XX", Schema: "katal.client.metrics.Counter.3", Value: 1},
				{Key: "HTTPRequest.CreateIdentity.Latency", Schema: "katal.client.metrics.Timer.2", Value: createIdentityMs},
				{Key: "HTTPRequest.CreateIdentity.Failure", Schema: "katal.client.metrics.Counter.3", Value: 0},
			},
		},
	})
}
