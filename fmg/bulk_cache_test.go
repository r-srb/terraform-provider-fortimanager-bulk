package fortimanager

import (
	"fmt"
	"sync"
	"testing"
)

func freshCache() *BulkCache {
	return &BulkCache{store: make(map[string]map[string]interface{})}
}

func TestGetOrLoad_LoadsOnFirstCall(t *testing.T) {
	c := freshCache()
	calls := 0
	loader := func() ([]interface{}, error) {
		calls++
		return []interface{}{
			map[string]interface{}{"name": "addr1", "subnet": "10.0.0.1/32"},
			map[string]interface{}{"name": "addr2", "subnet": "10.0.0.2/32"},
		}, nil
	}

	result, err := c.GetOrLoad("test-endpoint", loader)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 1 {
		t.Errorf("expected loader called once, got %d", calls)
	}
	if len(result) != 2 {
		t.Errorf("expected 2 entries, got %d", len(result))
	}
	if _, ok := result["addr1"]; !ok {
		t.Error("expected addr1 in result")
	}
}

func TestGetOrLoad_CachesAfterFirstCall(t *testing.T) {
	c := freshCache()
	calls := 0
	loader := func() ([]interface{}, error) {
		calls++
		return []interface{}{map[string]interface{}{"name": "x"}}, nil
	}

	c.GetOrLoad("ep", loader)
	c.GetOrLoad("ep", loader)
	c.GetOrLoad("ep", loader)

	if calls != 1 {
		t.Errorf("expected loader called exactly once across 3 GetOrLoad calls, got %d", calls)
	}
}

func TestGetOrLoad_LoaderError_NotCached(t *testing.T) {
	c := freshCache()
	calls := 0
	loader := func() ([]interface{}, error) {
		calls++
		if calls == 1 {
			return nil, fmt.Errorf("transient failure")
		}
		return []interface{}{map[string]interface{}{"name": "ok"}}, nil
	}

	_, err := c.GetOrLoad("ep", loader)
	if err == nil {
		t.Fatal("expected error on first call")
	}

	result, err := c.GetOrLoad("ep", loader)
	if err != nil {
		t.Fatalf("unexpected error on second call: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("expected 1 item after retry, got %d", len(result))
	}
	if calls != 2 {
		t.Errorf("expected loader called twice (first failed, second succeeded), got %d", calls)
	}
}

func TestGetOrLoad_ItemsMissingNameKey_Skipped(t *testing.T) {
	c := freshCache()
	loader := func() ([]interface{}, error) {
		return []interface{}{
			map[string]interface{}{"name": "valid"},
			map[string]interface{}{"other_key": "no_name"}, // skipped
			"not a map",                                     // skipped
		}, nil
	}

	result, err := c.GetOrLoad("ep", loader)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("expected 1 valid entry, got %d", len(result))
	}
}

func TestGetOrLoad_ConcurrentCallsSingleLoad(t *testing.T) {
	c := freshCache()
	var calls int
	var mu sync.Mutex
	loader := func() ([]interface{}, error) {
		mu.Lock()
		calls++
		mu.Unlock()
		return []interface{}{map[string]interface{}{"name": "x"}}, nil
	}

	results := make([]map[string]interface{}, 20)
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, _ := c.GetOrLoad("ep", loader)
			mu.Lock()
			results[i] = r
			mu.Unlock()
		}()
	}
	wg.Wait()

	// The loader is invoked inside the write lock, so only one goroutine can
	// execute it at a time. The double-check ensures it is called exactly once.
	if calls != 1 {
		t.Errorf("expected loader called exactly once, got %d", calls)
	}
	// All 20 goroutines must receive the populated cache with the same entry.
	for i, r := range results {
		if len(r) != 1 {
			t.Errorf("goroutine %d: expected 1 entry in result, got %d", i, len(r))
		}
		if _, ok := r["x"]; !ok {
			t.Errorf("goroutine %d: expected key 'x' in result", i)
		}
	}
}

func TestInvalidate_ForcesReload(t *testing.T) {
	c := freshCache()
	calls := 0
	loader := func() ([]interface{}, error) {
		calls++
		return []interface{}{map[string]interface{}{"name": "x"}}, nil
	}

	c.GetOrLoad("ep", loader)
	c.Invalidate("ep")
	c.GetOrLoad("ep", loader)

	if calls != 2 {
		t.Errorf("expected loader called twice (after invalidation), got %d", calls)
	}
}

func TestGetOrLoadPolicyIndex_LoadsOnFirstCall(t *testing.T) {
	c := freshCache()
	calls := 0
	loader := func() ([]interface{}, error) {
		calls++
		return []interface{}{
			map[string]interface{}{"policyid": float64(1), "name": "allow-all"},
			map[string]interface{}{"policyid": float64(2), "name": "deny-all"},
		}, nil
	}

	result, err := c.GetOrLoadPolicyIndex("test-ep", loader)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 1 {
		t.Errorf("expected loader called once, got %d", calls)
	}
	if len(result) != 2 {
		t.Errorf("expected 2 entries, got %d", len(result))
	}
	if _, ok := result["1"]; !ok {
		t.Error("expected key '1' in result")
	}
	if _, ok := result["2"]; !ok {
		t.Error("expected key '2' in result")
	}
}

func TestGetOrLoadPolicyIndex_CachesAfterFirstCall(t *testing.T) {
	c := freshCache()
	calls := 0
	loader := func() ([]interface{}, error) {
		calls++
		return []interface{}{map[string]interface{}{"policyid": float64(10)}}, nil
	}

	c.GetOrLoadPolicyIndex("ep", loader)
	c.GetOrLoadPolicyIndex("ep", loader)
	c.GetOrLoadPolicyIndex("ep", loader)

	if calls != 1 {
		t.Errorf("expected loader called exactly once across 3 calls, got %d", calls)
	}
}

func TestGetOrLoadPolicyIndex_IsolatedFromNameIndex(t *testing.T) {
	c := freshCache()
	nameLoader := func() ([]interface{}, error) {
		return []interface{}{map[string]interface{}{"name": "policy-a"}}, nil
	}
	policyLoader := func() ([]interface{}, error) {
		return []interface{}{map[string]interface{}{"policyid": float64(42)}}, nil
	}

	nameResult, _ := c.GetOrLoad("ep", nameLoader)
	policyResult, _ := c.GetOrLoadPolicyIndex("ep", policyLoader)

	if _, ok := nameResult["policy-a"]; !ok {
		t.Error("expected 'policy-a' in name-indexed cache")
	}
	if _, ok := policyResult["42"]; !ok {
		t.Error("expected '42' in policy-indexed cache")
	}
	// The two indexes must not bleed into each other
	if _, ok := nameResult["42"]; ok {
		t.Error("policyid key '42' should not appear in name-indexed cache")
	}
	if _, ok := policyResult["policy-a"]; ok {
		t.Error("name key 'policy-a' should not appear in policy-indexed cache")
	}
}

func TestGetOrLoadPolicyIndex_MissingPolicyID_Skipped(t *testing.T) {
	c := freshCache()
	loader := func() ([]interface{}, error) {
		return []interface{}{
			map[string]interface{}{"policyid": float64(1)},
			map[string]interface{}{"name": "no-policyid"},   // skipped
			"not a map",                                      // skipped
			map[string]interface{}{"policyid": "stringid"},  // accepted as string
		}, nil
	}

	result, err := c.GetOrLoadPolicyIndex("ep", loader)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("expected 2 valid entries, got %d", len(result))
	}
	if _, ok := result["1"]; !ok {
		t.Error("expected key '1' in result")
	}
	if _, ok := result["stringid"]; !ok {
		t.Error("expected key 'stringid' in result")
	}
}
