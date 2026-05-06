package forticlient

import (
	"fmt"
)

const bulkPageSize = 500

// decodeDataList extracts the full data array from a JSON-RPC list response.
// Unlike decodeData which returns only the first element, this returns all items.
func decodeDataList(result map[string]interface{}) ([]interface{}, error) {
	v, ok := result["result"]
	if !ok {
		return nil, fmt.Errorf("missing 'result' field in API response")
	}
	l, ok := v.([]interface{})
	if !ok || len(l) == 0 {
		return nil, fmt.Errorf("unexpected 'result' shape in API response")
	}
	item, ok := l[0].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected result[0] shape in API response")
	}

	// This status-code check defends against direct callers of decodeDataList.
	// When called via bulkReadPage, sendRequest already handles non-zero status
	// errors via fortiAPIErrorFormat before we reach this point.
	if status, ok := item["status"].(map[string]interface{}); ok {
		if code, ok := status["code"].(float64); ok && code != 0 {
			msg, _ := status["message"].(string)
			return nil, fmt.Errorf("API error %v: %s", code, msg)
		}
	}

	if item["data"] == nil {
		return []interface{}{}, nil
	}
	dataList, ok := item["data"].([]interface{})
	if !ok {
		return []interface{}{}, nil
	}
	return dataList, nil
}

// bulkReadPage fetches one page of a list resource at the given FMG path.
// offset is the zero-based start index; count is the maximum number of items to return.
// Returns the items in that page. If fewer than count items are returned, it's the last page.
func (c *FortiSDKClient) bulkReadPage(path string, offset, count int) ([]interface{}, error) {
	data := map[string]interface{}{
		"method":  "get",
		"verbose": 1,
		"params": []map[string]interface{}{{
			"url":   path,
			"range": []int{offset, count},
		}},
	}
	if c.Session != "" {
		data["session"] = c.Session
	}

	_, result, err := sendRequest(c, data)
	if err != nil {
		return nil, fmt.Errorf("bulk read page at %s (offset %d): %w", path, offset, err)
	}
	return decodeDataList(result)
}

// bulkReadAllWithPageSize fetches all items at path using the given page size.
// Unexported but accessible for testing within the same package; production code uses bulkReadAll.
func (c *FortiSDKClient) bulkReadAllWithPageSize(path string, pageSize int) ([]interface{}, error) {
	var all []interface{}
	for offset := 0; ; offset += pageSize {
		page, err := c.bulkReadPage(path, offset, pageSize)
		if err != nil {
			return nil, err
		}
		all = append(all, page...)
		if len(page) < pageSize {
			break
		}
	}
	return all, nil
}

// bulkReadAll fetches all items at path using the default bulkPageSize.
func (c *FortiSDKClient) bulkReadAll(path string) ([]interface{}, error) {
	return c.bulkReadAllWithPageSize(path, bulkPageSize)
}

// BulkGetObjectFirewallAddress fetches all IPv4 address objects for the given adom.
// adomv is the already-resolved path component, e.g. "adom/TEST" or "global".
func (c *FortiSDKClient) BulkGetObjectFirewallAddress(adomv string) ([]interface{}, error) {
	return c.bulkReadAll("/pm/config/" + adomv + "/obj/firewall/address")
}

// BulkGetObjectFirewallAddress6 fetches all IPv6 address objects for the given adom.
func (c *FortiSDKClient) BulkGetObjectFirewallAddress6(adomv string) ([]interface{}, error) {
	return c.bulkReadAll("/pm/config/" + adomv + "/obj/firewall/address6")
}

// BulkGetObjectFirewallAddrgrp fetches all IPv4 address group objects for the given adom.
func (c *FortiSDKClient) BulkGetObjectFirewallAddrgrp(adomv string) ([]interface{}, error) {
	return c.bulkReadAll("/pm/config/" + adomv + "/obj/firewall/addrgrp")
}

// BulkGetObjectFirewallAddrgrp6 fetches all IPv6 address group objects for the given adom.
func (c *FortiSDKClient) BulkGetObjectFirewallAddrgrp6(adomv string) ([]interface{}, error) {
	return c.bulkReadAll("/pm/config/" + adomv + "/obj/firewall/addrgrp6")
}

// BulkGetObjectFirewallServiceCustom fetches all custom service objects for the given adom.
func (c *FortiSDKClient) BulkGetObjectFirewallServiceCustom(adomv string) ([]interface{}, error) {
	return c.bulkReadAll("/pm/config/" + adomv + "/obj/firewall/service/custom")
}

// BulkGetObjectFirewallServiceGroup fetches all service group objects for the given adom.
func (c *FortiSDKClient) BulkGetObjectFirewallServiceGroup(adomv string) ([]interface{}, error) {
	return c.bulkReadAll("/pm/config/" + adomv + "/obj/firewall/service/group")
}

// BulkGetPackagesPblockFirewallPolicy fetches all firewall policies within a specific pblock.
// adomv: resolved path component e.g. "adom/TEST"; pblock: pblock name e.g. "block_alpha".
func (c *FortiSDKClient) BulkGetPackagesPblockFirewallPolicy(adomv, pblock string) ([]interface{}, error) {
	return c.bulkReadAll("/pm/config/" + adomv + "/pblock/" + escapeURLString(pblock) + "/firewall/policy")
}
