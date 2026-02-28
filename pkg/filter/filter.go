package filter

import (
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Apply removes objects whose Kind matches any entry in excludeKinds
// (case-insensitive) or whose metadata.name matches any entry in
// excludeNames (exact match). Returns the filtered slice.
// If both lists are empty, the input is returned unchanged.
func Apply(
	objs []*unstructured.Unstructured,
	excludeKinds []string,
	excludeNames []string,
) []*unstructured.Unstructured {
	if len(excludeKinds) == 0 && len(excludeNames) == 0 {
		return objs
	}

	logger := log.WithFields(log.Fields{
		"func":         "filter.Apply",
		"input":        len(objs),
		"excludeKinds": len(excludeKinds),
		"excludeNames": len(excludeNames),
	})
	logger.Debug("Applying exclusion filters")

	kindSet := make(map[string]bool, len(excludeKinds))
	for _, k := range excludeKinds {
		kindSet[strings.ToLower(k)] = true
	}

	nameSet := make(map[string]bool, len(excludeNames))
	for _, n := range excludeNames {
		nameSet[n] = true
	}

	result := make([]*unstructured.Unstructured, 0, len(objs))
	for _, obj := range objs {
		if kindSet[strings.ToLower(obj.GetKind())] {
			log.WithFields(log.Fields{
				"func":   "filter.Apply",
				"id":     fmt.Sprintf("%s/%s", obj.GetKind(), obj.GetName()),
				"reason": "kind",
			}).Debug("Excluded resource")
			continue
		}
		if nameSet[obj.GetName()] {
			log.WithFields(log.Fields{
				"func":   "filter.Apply",
				"id":     fmt.Sprintf("%s/%s", obj.GetKind(), obj.GetName()),
				"reason": "name",
			}).Debug("Excluded resource")
			continue
		}
		result = append(result, obj)
	}

	logger.WithFields(log.Fields{
		"excluded":  len(objs) - len(result),
		"remaining": len(result),
	}).Debug("Filter complete")

	return result
}
