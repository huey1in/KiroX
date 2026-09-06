package browser

import (
	"reflect"
	"sort"
	"testing"
)

func TestNormalizeIdentityOffsetsMigratesFiveDomains(t *testing.T) {
	got := normalizeIdentityOffsets([]int{-10, 20, 40, 60, 120})
	want := [10]int{0, 20, 0, 20, 20, 40, 40, 60, 60, 100}
	if got != want {
		t.Fatalf("normalizeIdentityOffsets() = %v, want %v", got, want)
	}
}

func TestResolveFingerprintOffsetsUsesHorizontalPositions(t *testing.T) {
	offsets := []int{0, 100, 100, 100, 100, 100, 100, 100, 100, 100}
	defaultCurve := resolveFingerprintOffsets(offsets, nil)
	shiftedCurve := resolveFingerprintOffsets(offsets, []int{0, 50, 56, 62, 68, 74, 80, 86, 92, 100})
	if defaultCurve[1] != 100 {
		t.Fatalf("default curve domain value = %d, want 100", defaultCurve[1])
	}
	if shiftedCurve[1] != 22 {
		t.Fatalf("shifted curve domain value = %d, want 22", shiftedCurve[1])
	}
}

func TestApplySelectedIdentityDomains(t *testing.T) {
	base := fingerprintIdentityFixture("base", 1)
	fresh := fingerprintIdentityFixture("fresh", 2)
	tests := []struct {
		index  int
		fields []string
	}{
		{0, []string{"ChromeVer", "SecUA", "UA"}},
		{1, []string{"Platform"}},
		{2, []string{"Plugins"}},
		{3, []string{"DeviceMemory", "HardwareConcurrency"}},
		{4, []string{"GPUModel", "GPUVendor", "WebGLExts"}},
		{5, []string{"Screen"}},
		{6, []string{"TimezoneHours"}},
		{7, []string{"CanvasHash", "HistogramBase"}},
		{8, []string{"MathCos", "MathSin", "MathTan"}},
		{9, []string{"LsubidPrefixProfile", "LsubidPrefixSignin", "WebpackHash"}},
	}
	for _, tt := range tests {
		selected := [10]bool{}
		selected[tt.index] = true
		got := cloneIdentity(base)
		applySelectedIdentityDomains(got, fresh, selected)
		changed := changedIdentityFields(base, got)
		if !reflect.DeepEqual(changed, tt.fields) {
			t.Errorf("domain %d changed %v, want %v", tt.index, changed, tt.fields)
		}
	}
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
