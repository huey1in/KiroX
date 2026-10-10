package browser

import (
	"reflect"
	"sort"
	"testing"
)

func TestRegistrationIdentityPreservesHardwareDomains(t *testing.T) {
	base := fingerprintIdentityFixture("base", 1)
	fresh := fingerprintIdentityFixture("fresh", 2)
	for _, tt := range []struct {
		canvas, mathRuntime bool
		fields              []string
	}{
		{false, false, []string{}},
		{true, false, []string{"CanvasHash", "HistogramBase"}},
		{false, true, []string{"MathCos", "MathSin", "MathTan"}},
		{true, true, []string{"CanvasHash", "HistogramBase", "MathCos", "MathSin", "MathTan"}},
	} {
		got := cloneIdentity(base)
		applyRegistrationIdentityDomains(got, fresh, tt.canvas, tt.mathRuntime)
		if changed := changedIdentityFields(base, got); !reflect.DeepEqual(changed, tt.fields) {
			t.Errorf("canvas=%t math=%t changed %v, want %v", tt.canvas, tt.mathRuntime, changed, tt.fields)
		}
	}
}

func TestIdentityMatchesTLSProfiles(t *testing.T) {
	valid := RandomIdentity()
	valid.ChromeVer = "133.0.0.0"
	valid.UA = "Mozilla/5.0 Chrome/133.0.0.0 Safari/537.36"
	valid.SecUA = `"Not_A Brand";v="24", "Chromium";v="133", "Google Chrome";v="133"`
	if !identityMatchesTLSProfiles(valid) {
		t.Fatal("rejected a supported consistent identity")
	}
	for name, identity := range map[string]*BrowserIdentity{
		"stale version": cloneWith(valid, func(identity *BrowserIdentity) {
			identity.ChromeVer = "139.0.0.0"
			identity.UA = "Chrome/139.0.0.0"
			identity.SecUA = `"Chromium";v="139", "Google Chrome";v="139"`
		}),
		"UA mismatch": cloneWith(valid, func(identity *BrowserIdentity) {
			identity.UA = "Chrome/139.0.0.0"
		}),
		"SecUA mismatch": cloneWith(valid, func(identity *BrowserIdentity) {
			identity.SecUA = `"Chromium";v="139", "Google Chrome";v="139"`
		}),
		"invalid memory": cloneWith(valid, func(identity *BrowserIdentity) {
			identity.DeviceMemory = 6
		}),
		"invalid color depth": cloneWith(valid, func(identity *BrowserIdentity) {
			identity.Screen.ColorDepth = 30
		}),
		"randomized math": cloneWith(valid, func(identity *BrowserIdentity) {
			identity.MathTan = "-1.42144882387472099"
		}),
		"shuffled plugins": cloneWith(valid, func(identity *BrowserIdentity) {
			identity.Plugins[0], identity.Plugins[1] = identity.Plugins[1], identity.Plugins[0]
		}),
	} {
		t.Run(name, func(t *testing.T) {
			if identityMatchesTLSProfiles(identity) {
				t.Fatalf("accepted inconsistent identity: %#v", identity)
			}
		})
	}
}

func cloneWith(base *BrowserIdentity, mutate func(*BrowserIdentity)) *BrowserIdentity {
	clone := cloneIdentity(base)
	clone.Plugins = append([]map[string]string(nil), base.Plugins...)
	mutate(clone)
	return clone
}

func fingerprintIdentityFixture(prefix string, number int) *BrowserIdentity {
	identity := &BrowserIdentity{
		ChromeVer: prefix + "-chrome", UA: prefix + "-ua", SecUA: prefix + "-secua",
		GPUVendor: prefix + "-vendor", GPUModel: prefix + "-model", WebGLExts: []string{prefix + "-ext"},
		CanvasHash: int32(number), MathTan: prefix + "-tan", MathSin: prefix + "-sin", MathCos: prefix + "-cos",
		Plugins:      []map[string]string{{"name": prefix + "-plugin"}},
		Screen:       ScreenInfo{Width: number, Height: number, AvailWidth: number, AvailHeight: number, ColorDepth: number},
		DeviceMemory: number, HardwareConcurrency: number, Platform: prefix + "-platform",
		LsubidPrefixSignin: prefix + "-signin", LsubidPrefixProfile: prefix + "-profile",
		WebpackHash: prefix + "-webpack", TimezoneHours: number,
	}
	identity.HistogramBase[0] = number
	return identity
}

func changedIdentityFields(before, after *BrowserIdentity) []string {
	beforeValue := reflect.ValueOf(before).Elem()
	afterValue := reflect.ValueOf(after).Elem()
	typeInfo := beforeValue.Type()
	changed := make([]string, 0)
	for i := 0; i < beforeValue.NumField(); i++ {
		if !reflect.DeepEqual(beforeValue.Field(i).Interface(), afterValue.Field(i).Interface()) {
			changed = append(changed, typeInfo.Field(i).Name)
		}
	}
	sort.Strings(changed)
	return changed
}
