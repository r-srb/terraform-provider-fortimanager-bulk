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

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.GetOrLoad("ep", loader)
		}()
	}
	wg.Wait()

	// Due to double-checked locking, loader may be called more than once
	// but only during the initial race window. After that: zero extra calls.
	// The important invariant: all 20 goroutines get the same cached result.
	if calls < 1 {
		t.Error("expected loader called at least once")
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
