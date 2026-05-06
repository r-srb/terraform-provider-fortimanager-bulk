package forticlient

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/terraform-providers/terraform-provider-fortimanager/sdk/auth"
	"github.com/terraform-providers/terraform-provider-fortimanager/sdk/config"
)

func makeTestClient(t *testing.T, handler http.Handler) (*FortiSDKClient, func()) {
	ts := httptest.NewTLSServer(handler)
	addr := ts.Listener.Addr().String()
	a := &auth.Auth{Hostname: addr, Token: "test-token"}
	c := &FortiSDKClient{
		Token: "test-token",
		Config: config.Config{
			Auth:     a,
			HTTPCon:  ts.Client(),
			FwTarget: addr,
		},
	}
	return c, ts.Close
}

func jsonRPCListResponse(items []map[string]interface{}) []byte {
	resp := map[string]interface{}{
		"result": []map[string]interface{}{{
			"status": map[string]interface{}{"code": 0, "message": "OK"},
			"data":   items,
		}},
	}
	b, _ := json.Marshal(resp)
	return b
}

func TestBulkReadAll_SinglePage(t *testing.T) {
	items := []map[string]interface{}{
		{"name": "addr1", "subnet": "10.0.0.1/32"},
		{"name": "addr2", "subnet": "10.0.0.2/32"},
	}
	c, stop := makeTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(jsonRPCListResponse(items))
	}))
	defer stop()

	result, err := c.bulkReadAll("/pm/config/adom/TEST/obj/firewall/address")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 items, got %d", len(result))
	}
	first := result[0].(map[string]interface{})
	if first["name"] != "addr1" {
		t.Errorf("expected addr1, got %v", first["name"])
	}
}

func TestBulkReadAll_Pagination(t *testing.T) {
	// Server returns 3 items on first call (page size = 3) and 1 item on second call.
	pageSize := 3
	callCount := 0
	pages := [][]map[string]interface{}{
		{{"name": "a1"}, {"name": "a2"}, {"name": "a3"}},
		{{"name": "a4"}},
	}
	c, stop := makeTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		params := body["params"].([]interface{})
		p := params[0].(map[string]interface{})
		_ = p["range"] // verify range key is present
		w.Write(jsonRPCListResponse(pages[callCount]))
		callCount++
	}))
	defer stop()

	result, err := c.bulkReadAllWithPageSize("/pm/config/adom/TEST/obj/firewall/address", pageSize)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 4 {
		t.Fatalf("expected 4 items, got %d", len(result))
	}
	if callCount != 2 {
		t.Errorf("expected 2 HTTP calls, got %d", callCount)
	}
}

func TestDecodeDataList_Empty(t *testing.T) {
	result := map[string]interface{}{
		"result": []interface{}{
			map[string]interface{}{
				"status": map[string]interface{}{"code": float64(0), "message": "OK"},
				"data":   []interface{}{},
			},
		},
	}
	items, err := decodeDataList(result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected empty slice, got %v", items)
	}
}

func TestDecodeDataList_APIError(t *testing.T) {
	result := map[string]interface{}{
		"result": []interface{}{
			map[string]interface{}{
				"status": map[string]interface{}{"code": float64(-6), "message": "No permission"},
			},
		},
	}
	_, err := decodeDataList(result)
	if err == nil {
		t.Fatal("expected error for non-zero status code")
	}
}
