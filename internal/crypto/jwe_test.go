package crypto

import (
	"regexp"
	"testing"
)

func TestGenUUIDReturnsRFC4122Version4(t *testing.T) {
	pattern := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	for i := 0; i < 32; i++ {
		if value := genUUID(); !pattern.MatchString(value) {
			t.Fatalf("genUUID() = %q", value)
		}
	}
}
