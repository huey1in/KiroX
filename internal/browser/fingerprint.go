package browser

import (
	"fmt"
	"math/rand"
	"time"
)

// FingerprintContext 保持同一会话内硬件级指纹字段不变
type FingerprintContext struct {
	Identity       *BrowserIdentity
	CanvasHash     int32
	HistogramBins  [256]int
	LsUbidSignin   string
	LsUbidProfile  string
	PageHasCaptcha bool
	perfTiming     map[string]int64
	startTime      *int64
}

// NewFPContext 创建指纹上下文
func NewFPContext(identity *BrowserIdentity) *FingerprintContext {
	ts := time.Now().Unix()
	return &FingerprintContext{
		Identity:      identity,
		CanvasHash:    identity.CanvasHash,
		HistogramBins: identity.HistogramBase,
		LsUbidSignin: fmt.Sprintf("%s-%07d-%07d:%d",
			identity.LsubidPrefixSignin, rand.Intn(10000000), rand.Intn(10000000), ts),
	}
}

// GetLsUbid 获取对应域名的 lsUbid
func (c *FingerprintContext) GetLsUbid(pageType string) string {
	if pageType == "profile" {
		if c.LsUbidProfile == "" {
			var ts int64
			if c.perfTiming != nil {
				ts = c.perfTiming["loadEventEnd"] / 1000
			} else {
				ts = time.Now().Unix()
			}
			c.LsUbidProfile = fmt.Sprintf("%s-%07d-%07d:%d",
				c.Identity.LsubidPrefixProfile, rand.Intn(10000000), rand.Intn(10000000), ts)
		}
		return c.LsUbidProfile
	}
	return c.LsUbidSignin
}

// GetPerfTiming 获取 performance.timing
func (c *FingerprintContext) GetPerfTiming(nowMs int64) map[string]int64 {
	if c.perfTiming == nil {
		c.perfTiming = GenPerfTiming(nowMs)
	}
	return c.perfTiming
}

// GetStartTime 获取页面 start 时间戳
func (c *FingerprintContext) GetStartTime(nowMs int64) int64 {
	if c.startTime == nil {
		t := nowMs
		c.startTime = &t
	}
	return *c.startTime
}

// ResetPerfTiming 切换到新页面时重置 timing
func (c *FingerprintContext) ResetPerfTiming() {
	c.perfTiming = nil
	c.startTime = nil
}

// GenerateFingerprintJSON 生成指纹 JSON（不加密），供远程加密使用
func GenerateFingerprintJSON(
	identity *BrowserIdentity,
	locationURL, referrer string,
	ctx *FingerprintContext,
	pageType, eventType string,
	timeOnPage, emailLen int,
	email string,
) string {
	nowMs := time.Now().UnixMilli()
	fpData := BuildFingerprintData(identity, locationURL, referrer, nowMs, ctx,
		pageType, eventType, timeOnPage, emailLen, email)
	return MarshalOrdered(fpData)
}
