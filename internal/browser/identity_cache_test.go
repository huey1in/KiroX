package browser

import (
	"reflect"
	"sort"
	"testing"
	"time"
)

func TestRegistrationIdentityPreservesHardwareDomains(t *testing.T) {
	base := RandomIdentity()
	idCacheMu.Lock()
	previousCache := idCache
	idCache = map[string]cachedIdentity{
		"registration.example:8080": {Identity: base, CreatedAt: time.Now().Unix()},
	}
	idCacheMu.Unlock()
	t.Cleanup(func() {
		idCacheMu.Lock()
		idCache = previousCache
		idCacheMu.Unlock()
	})
	allowed := map[string]bool{
		"CanvasHash": true, "HistogramBase": true,
		"LsubidPrefixSignin": true, "LsubidPrefixProfile": true, "WebpackHash": true,
	}
	for i := 0; i < 50; i++ {
		got := IdentityForRegistration("http://registration.example:8080")
		if got == base {
			t.Fatal("registration returned the shared cached identity")
		}
		for _, field := range changedIdentityFields(base, got) {
			if !allowed[field] {
				t.Fatalf("registration changed the cached hardware field %s", field)
			}
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
	clone := *base
	clone.Plugins = append([]map[string]string(nil), base.Plugins...)
	mutate(&clone)
	return &clone
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
