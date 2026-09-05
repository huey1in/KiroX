package browser

import (
	"encoding/json"
	"fmt"
	"hash/crc32"
	"math/rand"
	"strings"

	"reg_go/internal/crypto"
)

// GenPerfTiming 生成 performance.timing
func GenPerfTiming(nowMs int64) map[string]int64 {
	loadEventEnd := nowMs - int64(500+rand.Intn(1001))
	loadDuration := int64(2000 + rand.Intn(2001))
	base := loadEventEnd - loadDuration

	dnsOffset := int64(2 + rand.Intn(8))
	connectEndOffset := int64(300 + rand.Intn(300))
	responseOffset := connectEndOffset + int64(200+rand.Intn(400))
	domInteractiveOffset := loadDuration - int64(5+rand.Intn(11))
	domContentLoadedStart := domInteractiveOffset + int64(rand.Intn(3))

	return map[string]int64{
		"connectStart":             base + dnsOffset + 1 + int64(rand.Intn(3)),
		"secureConnectionStart":    base + dnsOffset + 3 + int64(rand.Intn(5)),
		"unloadEventEnd":           0,
		"domainLookupStart":        base + dnsOffset,
		"domainLookupEnd":          base + dnsOffset + int64(rand.Intn(2)),
		"responseStart":            base + responseOffset,
		"connectEnd":               base + connectEndOffset,
		"responseEnd":              base + responseOffset + int64(rand.Intn(5)),
		"requestStart":             base + connectEndOffset,
		"domLoading":               base + responseOffset + 2 + int64(rand.Intn(5)),
		"redirectStart":            0,
		"loadEventEnd":             loadEventEnd,
		"domComplete":              loadEventEnd,
		"navigationStart":          base,
		"loadEventStart":           loadEventEnd,
		"domContentLoadedEventEnd": loadEventEnd,
		"unloadEventStart":         0,
		"redirectEnd":              0,
		"domInteractive":           base + domInteractiveOffset,
		"fetchStart":               base + dnsOffset,
		"domContentLoadedEventStart": base + domContentLoadedStart,
	}
}

func copyPerfTiming(src map[string]int64) map[string]int64 {
	dst := make(map[string]int64, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func formatScreen(s ScreenInfo) string {
	return fmt.Sprintf("%d-%d-%d-%d-*-*-*", s.Width, s.Height, s.AvailHeight, s.ColorDepth)
}

func formatPlugins(plugins []map[string]string) string {
	names := make([]string, len(plugins))
	for i, p := range plugins {
		names[i] = p["name"]
	}
	return strings.Join(names, " ")
}

func genMetricsFirstLoad(pageType string) map[string]int {
	m := map[string]int{
		"el": 0, "script": 0, "h": 0, "batt": 0, "perf": 0, "auto": 0,
		"tz": 0, "fp2": 0, "lsubid": 0, "browser": 0, "capabilities": 0,
		"gpu": 0, "dnt": 0, "math": 0, "tts": 0, "input": 0, "canvas": 0,
		"captchainput": 0, "pow": 0,
	}
	switch pageType {
	case "profile":
		m["script"] = 1
		m["batt"] = 2 + rand.Intn(2)
		m["capabilities"] = 3 + rand.Intn(2)
		m["input"] = 10 + rand.Intn(5)
		m["canvas"] = 3 + rand.Intn(3)
	case "signup":
		m["script"] = 1
		m["batt"] = 1
		m["capabilities"] = 1
		m["gpu"] = 3 + rand.Intn(2)
		m["dnt"] = 1
	default: // signin
		m["script"] = 1
		m["gpu"] = 4 + rand.Intn(3)
	}
	return m
}

func genMetricsPageSubmit() map[string]int {
	return map[string]int{
		"el": 0, "script": 0, "h": 0, "batt": 0, "perf": rand.Intn(3),
		"auto": 0, "tz": 0, "fp2": 0, "lsubid": 0, "browser": 0,
		"capabilities": 0, "gpu": 0, "dnt": 0, "math": 0, "tts": 0,
		"input": 0, "canvas": 0, "captchainput": 0, "pow": 0,
	}
}

// genInteraction 生成交互数据
func genInteraction(eventType string) map[string]interface{} {
	return genInteractionSpec(eventType, 0, 0, 0)
}

// genInteractionSpec 按手动抓包的行为画像生成交互数据:
//   signin 提交邮箱/SIGNUP:  clicks=2, keyPresses=2, pastes=1 (粘贴邮箱)
//   signup 提交密码:         clicks=5, keyPresses=16, pastes=1
//   profile 验证码:          clicks=1, keyPresses=2, pastes=1 (粘贴验证码)
func genInteractionSpec(eventType string, clicks, keyPresses, pastes int) map[string]interface{} {
	if eventType == "PageLoad" || eventType == "first_load" {
		return map[string]interface{}{
			"clicks": 0, "touches": 0, "keyPresses": 0,
			"cuts": 0, "copies": 0, "pastes": 0,
			"keyPressTimeIntervals": []int{},
			"mouseClickPositions":   []string{},
			"keyCycles":             []int{}, "mouseCycles": []int{}, "touchCycles": []int{},
		}
	}
	if clicks <= 0 {
		clicks = 1 + rand.Intn(10) // 1~10 clicks
	}
	if keyPresses <= 0 {
		keyPresses = 3 + rand.Intn(20) // 3~22 keys
	}
	nKeys := keyPresses
	nClicks := clicks
	nIntervals := max(1, nKeys/3) + rand.Intn(max(1, nKeys/2-nKeys/3+1))
	nCycles := max(2, nKeys/2) + rand.Intn(max(1, nKeys*2/3-nKeys/2+1))

	intervals := make([]int, nIntervals)
	for i := range intervals {
		intervals[i] = 30 + rand.Intn(1500) // 30ms-1.5s
	}
	cycles := make([]int, nCycles)
	for i := range cycles {
		cycles[i] = 10 + rand.Intn(800)
	}
	positions := make([]string, nClicks)
	for i := range positions {
		positions[i] = fmt.Sprintf("%d,%d", 50+rand.Intn(1500), 50+rand.Intn(800))
	}
	mouseCycles := make([]int, nClicks)
	for i := range mouseCycles {
		mouseCycles[i] = 20 + rand.Intn(300)
	}

	return map[string]interface{}{
		"clicks": nClicks, "touches": 0, "keyPresses": nKeys,
		"cuts": 0, "copies": 0, "pastes": pastes,
		"keyPressTimeIntervals": intervals,
		"mouseClickPositions":   positions,
		"keyCycles":             cycles, "mouseCycles": mouseCycles, "touchCycles": []int{},
	}
}

// interactionFor 按页面/事件返回交互画像。
func interactionFor(pageType, eventType string) map[string]interface{} {
	switch {
	case pageType == "signin" && (eventType == "PageSubmit" || eventType == "SignupStart"):
		return genInteractionSpec(eventType, 2, 2, 1)
	case pageType == "signup" && (eventType == "PageSubmit" || eventType == "SignupStart"):
		return genInteractionSpec(eventType, 5, 16, 1)
	case pageType == "profile" && eventType == "EmailVerification":
		return genInteractionSpec(eventType, 1, 2, 1)
	case pageType == "profile" && eventType == "PageSubmit":
		return genInteractionSpec(eventType, 2, 2, 1)
	default:
		return genInteraction(eventType)
	}
}

// genFormField 生成表单字段追踪数据
func genFormField(startMs int64, emailLen int, email string, interaction map[string]interface{}) map[string]interface{} {
	fieldTs := startMs - int64(10+rand.Intn(41))
	fieldRand := 1000 + rand.Intn(9000)
	fieldID := 1 + rand.Intn(99)
	fieldName := fmt.Sprintf("formField%d-%d-%d", fieldID, fieldTs, fieldRand)

	nKeys := max(3, emailLen/3+rand.Intn(10)-3)
	intervals := make([]int, min(nKeys-1, 10))
	for i := range intervals {
		intervals[i] = 30 + rand.Intn(1500)
	}
	keyCycles := make([]int, min(nKeys, 10))
	for i := range keyCycles {
		keyCycles[i] = 10 + rand.Intn(500)
	}

	// 如果有 interaction 数据，复用
	if kp, ok := interaction["keyPresses"].(int); ok && kp > 0 {
		nKeys = kp
	}
	pastes := 0
	if ps, ok := interaction["pastes"].(int); ok && ps > 0 {
		pastes = ps
	}

	var cksum string
	if email != "" {
		cksum = fmt.Sprintf("%08X", crc32.ChecksumIEEE([]byte(email)))
	} else {
		cksum = fmt.Sprintf("%08X", crc32.ChecksumIEEE([]byte(fmt.Sprintf("user%d@example.com", 1000+rand.Intn(9000)))))
	}

	return map[string]interface{}{
		fieldName: map[string]interface{}{
			"clicks": 1, "touches": 0, "keyPresses": nKeys,
			"cuts": 0, "copies": 0, "pastes": pastes,
			"keyPressTimeIntervals": intervals,
			"mouseClickPositions":   []string{fmt.Sprintf("%d.5,%d.5", 100+rand.Intn(151), 10+rand.Intn(11))},
			"keyCycles": keyCycles, "mouseCycles": []int{80 + rand.Intn(71)}, "touchCycles": []int{},
			"width": 180, "height": 32, "totalFocusTime": 400 + rand.Intn(2100),
			"checksum": cksum, "autocomplete": false, "prefilled": false,
		},
	}
}

// OrderedMap 有序 map，用于保证 JSON 字段顺序
type OrderedMap struct {
	keys   []string
	values map[string]interface{}
}

// NewOrderedMap 创建有序 map
func NewOrderedMap() *OrderedMap {
	return &OrderedMap{values: make(map[string]interface{})}
}

// Set 设置键值对
func (o *OrderedMap) Set(key string, value interface{}) {
	if _, exists := o.values[key]; !exists {
		o.keys = append(o.keys, key)
	}
	o.values[key] = value
}

// MarshalJSON 序列化为有序 JSON
func (o *OrderedMap) MarshalJSON() ([]byte, error) {
	var sb strings.Builder
	sb.WriteByte('{')
	for i, k := range o.keys {
		if i > 0 {
			sb.WriteByte(',')
		}
		kb, _ := json.Marshal(k)
		sb.Write(kb)
		sb.WriteByte(':')
		vb, _ := json.Marshal(o.values[k])
		sb.Write(vb)
	}
	sb.WriteByte('}')
	return []byte(sb.String()), nil
}

// MarshalOrdered 将有序 map 序列化为紧凑 JSON
func MarshalOrdered(m *OrderedMap) string {
	b, _ := m.MarshalJSON()
	return string(b)
}

// BuildFingerprintData 构建完整的指纹 JSON 数据
func BuildFingerprintData(
	identity *BrowserIdentity,
	locationURL, referrer string,
	nowMs int64,
	ctx *FingerprintContext,
	pageType, eventType string,
	timeOnPage, emailLen int,
	email string,
) *OrderedMap {
	// 硬件级字段
	canvasHash := identity.CanvasHash
	histogram := identity.HistogramBase
	if ctx != nil {
		canvasHash = ctx.CanvasHash
		histogram = ctx.HistogramBins
	}

	// performance.timing
	var perfTiming map[string]int64
	if ctx != nil {
		perfTiming = ctx.GetPerfTiming(nowMs)
	} else {
		perfTiming = GenPerfTiming(nowMs)
	}
	// 手动流程 (HAR entry 15): 首个指纹在 DOMContentLoaded 之后、load 事件完成之前生成,
	// loadEventEnd/loadEventStart/domComplete 尚未填充。
	if eventType == "first_load" {
		perfTiming = copyPerfTiming(perfTiming)
		perfTiming["loadEventEnd"] = 0
		perfTiming["loadEventStart"] = 0
		perfTiming["domComplete"] = 0
	}

	// lsUbid
	var lsUbid string
	if ctx != nil {
		lsUbid = ctx.GetLsUbid(pageType)
	} else {
		lsUbid = fmt.Sprintf("%s-%07d-%07d:%d",
			identity.LsubidPrefixSignin, rand.Intn(10000000), rand.Intn(10000000), perfTiming["loadEventEnd"]/1000)
	}

	// 页面相关字段
	var dynamicURLs []string
	var scriptsElapsed int
	var historyLength int
	var isCompatible bool

	// 对齐手动抓包 (HAR): signin 页 history=5 且加载 cloudfront index.js;
	// profile 页 history=6 (首载) -> 8; signup 密码页 history=9, scripts.elapsed=0。
	switch pageType {
	case "profile":
		dynamicURLs = []string{fmt.Sprintf("/dist/main/app_%s.min.js", identity.WebpackHash)}
		scriptsElapsed = 1
		if eventType == "PageLoad" || eventType == "first_load" {
			historyLength = 6
		} else {
			historyLength = 8
		}
		isCompatible = true
	case "signup":
		dynamicURLs = []string{"/assets/js/app.js"}
		scriptsElapsed = 0
		historyLength = 9
		isCompatible = true
	default:
		dynamicURLs = []string{"/assets/js/app.js", "https://d35uxhjf90umnp.cloudfront.net/index.js"}
		scriptsElapsed = 1
		historyLength = 5
		isCompatible = false
	}

	// metrics
	var metrics map[string]int
	if eventType == "first_load" || (eventType == "PageLoad" && pageType == "profile") {
		metrics = genMetricsFirstLoad(pageType)
	} else {
		metrics = genMetricsPageSubmit()
	}

	// interaction
	interaction := interactionFor(pageType, eventType)

	// start / end 时间
	endMs := nowMs + int64(rand.Intn(51))
	var startTime int64
	if eventType != "PageLoad" && eventType != "first_load" && timeOnPage > 0 {
		startTime = endMs - int64(timeOnPage)
	} else if ctx != nil {
		if eventType == "first_load" {
			startTime = ctx.GetStartTime(nowMs - int64(500+rand.Intn(501)))
		} else if eventType == "PageLoad" && pageType == "profile" {
			startTime = ctx.GetStartTime(nowMs - int64(30+rand.Intn(51)))
		} else {
			startTime = ctx.GetStartTime(nowMs)
		}
	} else {
		startTime = nowMs
	}

	pluginsStr := formatPlugins(identity.Plugins)
	screenStr := formatScreen(identity.Screen)

	// 组装 (字段顺序严格按真实抓包)
	result := NewOrderedMap()
	result.Set("metrics", metrics)
	result.Set("start", startTime)
	result.Set("interaction", interaction)
	result.Set("scripts", map[string]interface{}{
		"dynamicUrls": dynamicURLs, "inlineHashes": []string{},
		"elapsed": scriptsElapsed, "dynamicUrlCount": len(dynamicURLs), "inlineHashesCount": 0,
	})
	result.Set("history", map[string]int{"length": historyLength})
	result.Set("battery", map[string]interface{}{})
	result.Set("performance", map[string]interface{}{"timing": perfTiming})
	result.Set("automation", map[string]interface{}{
		"wd": map[string]interface{}{
			"properties": map[string]interface{}{
				"document": []string{}, "window": []string{}, "navigator": []string{},
			},
		},
		"phantom": map[string]interface{}{
			"properties": map[string]interface{}{"window": []string{}},
		},
	})
	result.Set("end", endMs)
	result.Set("timeZone", identity.TimezoneHours)
	result.Set("flashVersion", nil)
	result.Set("plugins", pluginsStr+" ||"+screenStr)
	result.Set("dupedPlugins", pluginsStr+" ||"+screenStr)
	result.Set("screenInfo", screenStr)
	result.Set("lsUbid", lsUbid)
	result.Set("referrer", referrer)
	result.Set("userAgent", identity.UA)
	result.Set("deviceMemory", identity.DeviceMemory)
	result.Set("hardwareConcurrency", identity.HardwareConcurrency)
	result.Set("platform", identity.Platform)
	result.Set("location", locationURL)
	result.Set("webDriver", false)
	result.Set("capabilities", map[string]interface{}{
		"css": map[string]int{
			"textShadow": 1, "WebkitTextStroke": 1, "boxShadow": 1,
			"borderRadius": 1, "borderImage": 1, "opacity": 1,
			"transform": 1, "transition": 1,
		},
		"js": map[string]interface{}{
			"audio": true, "geolocation": true, "localStorage": "supported",
			"touch": false, "video": true, "webWorker": true,
		},
		"elapsed": 0,
	})
	result.Set("gpu", map[string]interface{}{
		"vendor": identity.GPUVendor, "model": identity.GPUModel,
		"extensions": identity.WebGLExts,
	})
	result.Set("dnt", nil)
	result.Set("math", map[string]string{
		"tan": identity.MathTan, "sin": identity.MathSin, "cos": identity.MathCos,
	})

	// profile 页面的 timeToSubmit
	if pageType == "profile" {
		if eventType == "PageLoad" || eventType == "first_load" {
			result.Set("timeToSubmit", 1+rand.Intn(5))
		} else if timeOnPage > 0 {
			result.Set("timeToSubmit", timeOnPage)
		} else {
			result.Set("timeToSubmit", 2000+rand.Intn(4001))
		}
	}

	// form 字段
	if pageType == "profile" && eventType != "PageLoad" && eventType != "first_load" && emailLen > 0 {
		result.Set("form", genFormField(nowMs, emailLen, email, interaction))
	} else {
		result.Set("form", map[string]interface{}{})
	}

	// canvas
	histSlice := make([]int, 256)
	copy(histSlice, histogram[:])
	result.Set("canvas", map[string]interface{}{
		"hash": canvasHash, "emailHash": nil, "histogramBins": histSlice,
	})
	result.Set("token", map[string]interface{}{"isCompatible": isCompatible, "pageHasCaptcha": 0})
	result.Set("auth", map[string]interface{}{"form": map[string]string{"method": "get"}})
	result.Set("errors", []interface{}{})
	result.Set("version", crypto.GetTESVersion())

	return result
}
