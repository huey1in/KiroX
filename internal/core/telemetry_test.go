package core

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	fhttp "github.com/bogdanfinn/fhttp"
	tls_client "github.com/bogdanfinn/tls-client"

	"reg_go/internal/browser"
	httputil "reg_go/internal/http"
)

// Route the registrar's actual request to a local server without contacting AWS.
type localKatalClient struct {
	tls_client.HttpClient
	endpoint *url.URL
}

func (c *localKatalClient) Do(req *fhttp.Request) (*fhttp.Response, error) {
	if req.URL.String() != katalNexusURL {
		return nil, fmt.Errorf("unexpected telemetry endpoint: %s", req.URL)
	}
	localReq := req.Clone(req.Context())
	localReq.URL = c.endpoint
	localReq.Host = c.endpoint.Host
	return c.HttpClient.Do(localReq)
}

func TestPostKatalNexusPreservesEveryGroupAndMetric(t *testing.T) {
	type capturedRequest struct {
		body        []byte
		contentType string
	}
	captured := make(chan capturedRequest, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Errorf("read telemetry request: %v", err)
		}
		captured <- capturedRequest{body: body, contentType: req.Header.Get("Content-Type")}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()
	endpoint, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	client := httputil.NewTLSClient("", true, "133.0.0.0")
	defer client.CloseIdleConnections()
	r := &Registrar{
		Cfg:      NewConfig(),
		Identity: &browser.BrowserIdentity{},
		Client:   &localKatalClient{HttpClient: client, endpoint: endpoint},
	}
	groups := []katalGroup{
		{Producer: "FirstProducer", Metrics: []katalEntry{
			{Key: "FirstCounter", Schema: "CounterSchema", Value: 1},
			{Key: "FirstTimer", Schema: "TimerSchema", Value: 125},
		}},
		{Producer: "EmptyProducer"},
		{Producer: "LastProducer", Metrics: []katalEntry{
			{Key: "LastTimer", Schema: "LastTimerSchema", Value: 250},
			{Key: "LastCounter", Schema: "LastCounterSchema", Value: 2},
		}},
	}
	r.PostKatalNexus(groups)
	var request capturedRequest
	select {
	case request = <-captured:
	default:
		t.Fatal("telemetry request did not reach the local server")
	}
	if request.contentType != "text/plain;charset=UTF-8" {
		t.Fatalf("Content-Type = %q", request.contentType)
	}
	var payload struct {
		CS struct {
			Dictionary map[string]string `json:"dct"`
		} `json:"cs"`
		Events []struct {
			Data map[string]interface{} `json:"data"`
		} `json:"events"`
	}
	if err := json.Unmarshal(request.body, &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Events) != 4 {
		t.Fatalf("metric count = %d, want 4", len(payload.Events))
	}
	index := 0
	actions := map[string]bool{}
	for _, group := range groups {
		var groupAction string
		for _, metric := range group.Metrics {
			data := payload.Events[index].Data
			index++
			resolve := func(field string) string {
				t.Helper()
				reference, ok := data[field].(string)
				if !ok {
					t.Fatalf("missing reference %s in %#v", field, data)
				}
				value, ok := payload.CS.Dictionary[reference]
				if !ok {
					t.Fatalf("missing dictionary entry %s", reference)
				}
				return value
			}
			if got := resolve("#6"); got != group.Producer {
				t.Errorf("producer = %q, want %q", got, group.Producer)
			}
			if got := resolve("#8"); got != metric.Key {
				t.Errorf("metric key = %q, want %q", got, metric.Key)
			}
			if got := resolve("#14"); got != metric.Schema {
				t.Errorf("metric schema = %q, want %q", got, metric.Schema)
			}
			if got := data["#10"]; got != metric.Value {
				t.Errorf("metric value = %v, want %v", got, metric.Value)
			}
			action := resolve("#3")
			if action == "" {
				t.Fatal("empty action ID")
			}
			if groupAction == "" {
				if actions[action] {
					t.Errorf("action ID %s reused across groups", action)
				}
				groupAction = action
				actions[action] = true
			} else if action != groupAction {
				t.Errorf("action ID changed within group %s", group.Producer)
			}
		}
	}
}
