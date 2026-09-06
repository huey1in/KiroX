package http

import (
	"net/url"
	"reflect"
	"testing"

	fhttp "github.com/bogdanfinn/fhttp"
	"github.com/bogdanfinn/tls-client/profiles"
)

func TestSaveCookiesOnlyReadsSetCookieHeaders(t *testing.T) {
	cookies := map[string]string{"existing": "kept"}
	headers := map[string][]string{
		"Set-Cookie": {
			"session=updated; Path=/; Secure; HttpOnly",
			"theme=dark; SameSite=Lax",
		},
		"Report-To": {`{"group":"network-errors","max_age":86400}`},
		"X-Debug":   {"accidental=value"},
	}

	SaveCookies(cookies, headers)

	want := map[string]string{
		"existing": "kept",
		"session":  "updated",
		"theme":    "dark",
	}
	if !reflect.DeepEqual(cookies, want) {
		t.Fatalf("cookies = %#v, want %#v", cookies, want)
	}
}

func TestNewTLSClientPreservesScopedCookiesWithSameName(t *testing.T) {
	client := NewTLSClient("", true)
	defer client.CloseIdleConnections()
	target, err := url.Parse("https://us-east-1.signin.aws/platform/directory/signup/api/execute")
	if err != nil {
		t.Fatal(err)
	}
	client.SetCookies(target, []*fhttp.Cookie{
		{Name: "aws-usi-authn", Value: "host", Path: "/platform/directory"},
		{Name: "aws-usi-authn", Value: "parent", Domain: ".signin.aws", Path: "/platform/directory"},
		{Name: "foreign", Value: "ignored", Domain: ".amazon.com", Path: "/"},
	})

	cookies := client.GetCookies(target)
	count := 0
	for _, cookie := range cookies {
		if cookie.Name == "aws-usi-authn" {
			count++
		}
		if cookie.Name == "foreign" {
			t.Fatal("cookie jar accepted a cookie for an unrelated domain")
		}
	}
	if count != 2 {
		t.Fatalf("same-name scoped cookie count = %d, cookies = %#v", count, cookies)
	}
	if cookies[0].Value != "host" || cookies[1].Value != "parent" {
		t.Fatalf("same-name cookie order = %#v", cookies)
	}
}

func TestTLSProfileMatchesGeneratedChromeVersions(t *testing.T) {
	cases := []struct {
		version string
		want    profiles.ClientProfile
	}{
		{version: "131.0.0.0", want: profiles.Chrome_131},
		{version: "133.0.0.0", want: profiles.Chrome_133},
		{version: "144.0.0.0", want: profiles.Chrome_144},
	}
	for _, tc := range cases {
		t.Run(tc.version, func(t *testing.T) {
			got := tlsProfileForChrome(tc.version)
			if got.GetClientHelloStr() != tc.want.GetClientHelloStr() {
				t.Fatalf("profile = %q, want %q", got.GetClientHelloStr(), tc.want.GetClientHelloStr())
			}
		})
	}
}

func TestSaveCookiesMatchesHeaderNameCaseInsensitively(t *testing.T) {
	cookies := map[string]string{}
	SaveCookies(cookies, map[string][]string{
		"set-cookie": {"token=value; Path=/"},
	})

	if cookies["token"] != "value" {
		t.Fatalf("cookies = %#v", cookies)
	}
}
