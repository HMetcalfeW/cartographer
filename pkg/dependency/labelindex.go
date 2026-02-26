package dependency

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// LabelIndex maps "key=value" strings to the set of pod/controller objects
// carrying that label. Built once in BuildDependencies and used by
// selector-based handlers for O(n) lookups instead of O(n²) scans.
type LabelIndex map[string][]*unstructured.Unstructured

// BuildLabelIndex creates a LabelIndex from a slice of objects, indexing only
// Pods and controller types (Deployment, DaemonSet, etc.).
func BuildLabelIndex(objs []*unstructured.Unstructured) LabelIndex {
	idx := make(LabelIndex)
	for _, obj := range objs {
		if !IsPodOrController(obj) {
			continue
		}
		for k, v := range obj.GetLabels() {
			key := k + "=" + v
			idx[key] = append(idx[key], obj)
		}
	}
	return idx
}

// Match returns all pod/controller objects whose labels satisfy every key-value
// pair in the selector. For a single-label selector this is a direct lookup;
// for multi-label selectors it intersects the per-label sets.
func (idx LabelIndex) Match(selector map[string]string) []*unstructured.Unstructured {
	if len(selector) == 0 {
		return nil
	}

	// Find the smallest candidate set to minimize intersection work.
	var smallest []*unstructured.Unstructured
	first := true
	for k, v := range selector {
		key := k + "=" + v
		candidates := idx[key]
		if len(candidates) == 0 {
			return nil // no objects have this label — empty intersection
		}
		if first || len(candidates) < len(smallest) {
			smallest = candidates
			first = false
		}
	}

	// Single-label fast path.
	if len(selector) == 1 {
		return smallest
	}

	// Multi-label: filter the smallest set to those matching all labels.
	var result []*unstructured.Unstructured
	for _, obj := range smallest {
		if LabelsMatch(selector, obj.GetLabels()) {
			result = append(result, obj)
		}
	}
	return result
}

// MatchSelector returns all pod/controller objects whose labels satisfy both
// the matchLabels map AND every matchExpressions requirement. If matchLabels
// is non-empty it narrows candidates via the index first; if only expressions
// are provided it scans all indexed objects.
func (idx LabelIndex) MatchSelector(matchLabels map[string]string, exprs []LabelSelectorRequirement) []*unstructured.Unstructured {
	if len(matchLabels) == 0 && len(exprs) == 0 {
		return nil
	}

	var candidates []*unstructured.Unstructured

	if len(matchLabels) > 0 {
		candidates = idx.Match(matchLabels)
		if len(candidates) == 0 {
			return nil
		}
	} else {
		// No matchLabels — collect all unique indexed objects as candidates.
		seen := make(map[string]struct{})
		for _, objs := range idx {
			for _, obj := range objs {
				id := ResourceID(obj)
				if _, exists := seen[id]; !exists {
					seen[id] = struct{}{}
					candidates = append(candidates, obj)
				}
			}
		}
	}

	if len(exprs) == 0 {
		return candidates
	}

	var result []*unstructured.Unstructured
	for _, obj := range candidates {
		if MatchesExpressions(exprs, obj.GetLabels()) {
			result = append(result, obj)
		}
	}
	return result
}

// MatchesExpressions returns true if the given labels satisfy every requirement.
// An empty expression list is vacuously true. All expressions are ANDed together
// per the Kubernetes LabelSelector spec.
func MatchesExpressions(exprs []LabelSelectorRequirement, labels map[string]string) bool {
	for _, expr := range exprs {
		val, exists := labels[expr.Key]
		switch expr.Operator {
		case "In":
			if !exists || !stringInSlice(val, expr.Values) {
				return false
			}
		case "NotIn":
			if exists && stringInSlice(val, expr.Values) {
				return false
			}
		case "Exists":
			if !exists {
				return false
			}
		case "DoesNotExist":
			if exists {
				return false
			}
		}
	}
	return true
}

func stringInSlice(s string, list []string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// ExtractMatchExpressions reads the "matchExpressions" field from an
// unstructured selector map (e.g. the result of NestedMap for "podSelector"
// or "selector") and returns a typed slice. Malformed entries are skipped.
func ExtractMatchExpressions(selectorMap map[string]interface{}) []LabelSelectorRequirement {
	raw, ok := selectorMap["matchExpressions"]
	if !ok {
		return nil
	}
	items, ok := raw.([]interface{})
	if !ok {
		return nil
	}
	var result []LabelSelectorRequirement
	for _, item := range items {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		key, _ := m["key"].(string)
		operator, _ := m["operator"].(string)
		if key == "" || operator == "" {
			continue
		}
		var values []string
		if rawVals, ok := m["values"].([]interface{}); ok {
			for _, rv := range rawVals {
				if s, ok := rv.(string); ok {
					values = append(values, s)
				}
			}
		}
		result = append(result, LabelSelectorRequirement{
			Key:      key,
			Operator: operator,
			Values:   values,
		})
	}
	return result
}
