package fortimanager

import (
	"fmt"
	"sync"
)

// BulkCache is a process-scoped in-memory cache of FortiManager list responses.
// It is populated on the first Read call for each resource type per endpoint and
// reused for all subsequent Read calls within the same terraform process invocation.
// This reduces N individual GETs (one per resource instance) to ~30 paginated GETs
// covering the entire ADOM object inventory.
type BulkCache struct {
	mu    sync.RWMutex
	store map[string]map[string]interface{} // endpoint path → object name → object data
}

var globalBulkCache = &BulkCache{
	store: make(map[string]map[string]interface{}),
}

// GetOrLoad returns the cached map for the given endpoint, calling loader to populate
// it on the first miss. The loader is not called again on subsequent calls for the
// same endpoint unless Invalidate has been called.
// Objects without a "name" field are silently skipped (they cannot be keyed).
// If the loader returns an error, the result is not cached and the next call retries.
func (bc *BulkCache) GetOrLoad(endpoint string, loader func() ([]interface{}, error)) (map[string]interface{}, error) {
	bc.mu.RLock()
	if cached, ok := bc.store[endpoint]; ok {
		bc.mu.RUnlock()
		return cached, nil
	}
	bc.mu.RUnlock()

	bc.mu.Lock()
	defer bc.mu.Unlock()
	// Double-checked: another goroutine may have populated while we waited for the write lock.
	if cached, ok := bc.store[endpoint]; ok {
		return cached, nil
	}

	items, err := loader()
	if err != nil {
		return nil, err
	}

	indexed := make(map[string]interface{}, len(items))
	for _, item := range items {
		obj, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		name, ok := obj["name"].(string)
		if !ok || name == "" {
			continue
		}
		indexed[name] = obj
	}
	bc.store[endpoint] = indexed
	return indexed, nil
}

// Invalidate removes the cached data for the given endpoint, forcing a reload
// on the next GetOrLoad call. Not needed during normal terraform plan/apply since
// the process lifecycle bounds the cache, but useful for testing.
func (bc *BulkCache) Invalidate(endpoint string) {
	bc.mu.Lock()
	delete(bc.store, endpoint)
	bc.mu.Unlock()
}

// GetOrLoadPolicyIndex is like GetOrLoad but indexes objects by their "policyid"
// field (integer) rather than "name". The store key is endpoint + "__policyid" to
// avoid collisions with any name-indexed cache for the same endpoint.
// policyid values of type float64 are formatted as decimal integers; string values
// are used as-is.
func (bc *BulkCache) GetOrLoadPolicyIndex(endpoint string, loader func() ([]interface{}, error)) (map[string]interface{}, error) {
	storeKey := endpoint + "__policyid"
	bc.mu.RLock()
	if cached, ok := bc.store[storeKey]; ok {
		bc.mu.RUnlock()
		return cached, nil
	}
	bc.mu.RUnlock()

	bc.mu.Lock()
	defer bc.mu.Unlock()
	if cached, ok := bc.store[storeKey]; ok {
		return cached, nil
	}

	items, err := loader()
	if err != nil {
		return nil, err
	}

	indexed := make(map[string]interface{}, len(items))
	for _, item := range items {
		obj, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		switch v := obj["policyid"].(type) {
		case float64:
			indexed[fmt.Sprintf("%d", int(v))] = obj
		case string:
			indexed[v] = obj
		}
	}
	bc.store[storeKey] = indexed
	return indexed, nil
}
