package dependency

import (
	"fmt"

	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// handleServiceLabelSelector finds Pods or higher-level controllers whose labels match
// the Service's .spec.selector, and records each matching resource as a child with Reason="selector".
func handleServiceLabelSelector(
	svc *unstructured.Unstructured,
	labelIdx LabelIndex,
	deps map[string][]Edge,
) {
	localLogger := log.WithField("func", "handleServiceLabelSelector")
	svcID := ResourceID(svc)
	spec, found, err := unstructured.NestedMap(svc.Object, "spec")
	if err != nil {
		localLogger.WithError(err).Warn("Could not retrieve .spec from Service")
		return
	}
	if !found {
		return
	}
	selObj, selFound, _ := unstructured.NestedFieldCopy(spec, "selector")
	if !selFound {
		return
	}
	selectorMap := MapInterfaceToStringMap(selObj)

	for _, target := range labelIdx.Match(selectorMap) {
		tgtID := ResourceID(target)
		deps[svcID] = append(deps[svcID], Edge{ChildID: tgtID, Reason: "selector"})
		localLogger.WithFields(log.Fields{
			"serviceID": svcID,
			"targetID":  tgtID,
		}).Debug("Added service->target dependency")
	}
}

// handleNetworkPolicy finds Pods or controllers whose labels match
// .spec.podSelector.matchLabels, and records each link as Reason="podSelector".
func handleNetworkPolicy(
	np *unstructured.Unstructured,
	labelIdx LabelIndex,
	deps map[string][]Edge,
) {
	localLogger := log.WithField("func", "handleNetworkPolicy")
	npID := ResourceID(np)
	spec, found, err := unstructured.NestedMap(np.Object, "spec")
	if err != nil {
		localLogger.WithError(err).Warn("Could not retrieve .spec from NetworkPolicy")
		return
	}
	if !found {
		return
	}
	podSel, selFound, _ := unstructured.NestedMap(spec, "podSelector", "matchLabels")
	selectorMap := MapInterfaceToStringMap(podSel)

	if selFound && len(selectorMap) > 0 {
		for _, obj := range labelIdx.Match(selectorMap) {
			tgtID := ResourceID(obj)
			deps[npID] = append(deps[npID], Edge{ChildID: tgtID, Reason: "podSelector"})
			localLogger.WithFields(log.Fields{
				"networkPolicy": npID,
				"targetID":      tgtID,
			}).Debug("Added networkpolicy->pod dependency")
		}
	}
}

// handlePodDisruptionBudget processes .spec.selector.matchLabels to find
// target objects (Pods, controllers) and creates an edge with Reason="pdbSelector".
func handlePodDisruptionBudget(
	pdb *unstructured.Unstructured,
	labelIdx LabelIndex,
	deps map[string][]Edge,
) {
	localLogger := log.WithField("func", "handlePodDisruptionBudget")
	pdbID := ResourceID(pdb)
	spec, found, err := unstructured.NestedMap(pdb.Object, "spec")
	if err != nil {
		localLogger.WithError(err).Warn("Could not retrieve .spec from PDB")
		return
	}
	if !found {
		return
	}
	selMapObj, selFound, _ := unstructured.NestedMap(spec, "selector", "matchLabels")
	selMap := MapInterfaceToStringMap(selMapObj)

	if selFound && len(selMap) > 0 {
		for _, obj := range labelIdx.Match(selMap) {
			tgtID := ResourceID(obj)
			deps[pdbID] = append(deps[pdbID], Edge{ChildID: tgtID, Reason: "pdbSelector"})
			localLogger.WithFields(log.Fields{
				"pdb":    pdbID,
				"target": tgtID,
			}).Debug("Added pdb->pod/controller dependency")
		}
	}
}

// handleIngressReferences inspects an Ingress's .spec.rules[].http.paths[].backend
// (both newer and older styles) and .spec.tls[].secretName, creating edges with
// Reason="ingressBackend" or Reason="tlsSecret", respectively.
func handleIngressReferences(
	ingress *unstructured.Unstructured,
	deps map[string][]Edge,
) {
	localLogger := log.WithField("func", "handleIngressReferences")
	ingID := ResourceID(ingress)

	// 1. Ingress -> Services in .spec.rules[].http.paths[].backend
	rules, foundRules, errRules := unstructured.NestedSlice(ingress.Object, "spec", "rules")
	if errRules != nil {
		localLogger.WithError(errRules).Warn("Error retrieving .spec.rules from Ingress")
	}
	if foundRules {
		for _, rule := range rules {
			rMap, ok := rule.(map[string]interface{})
			if !ok {
				continue
			}
			httpVal, foundHTTP, _ := unstructured.NestedMap(rMap, "http")
			if !foundHTTP || httpVal == nil {
				continue
			}
			paths, foundPaths, _ := unstructured.NestedSlice(httpVal, "paths")
			if !foundPaths {
				continue
			}
			for _, p := range paths {
				pathMap, ok := p.(map[string]interface{})
				if !ok {
					continue
				}
				// Newer style: .backend.service.name
				backendSvc, foundB, _ := unstructured.NestedMap(pathMap, "backend", "service")
				if foundB && backendSvc != nil {
					if svcName, ok := backendSvc["name"].(string); ok && svcName != "" {
						deps[ingID] = append(deps[ingID], Edge{
							ChildID: "Service/" + svcName, Reason: "ingressBackend",
						})
					}
				}
				// Older style: .backend.serviceName
				if oldSvcName, oldFound, _ := unstructured.NestedString(pathMap, "backend", "serviceName"); oldFound && oldSvcName != "" {
					deps[ingID] = append(deps[ingID], Edge{
						ChildID: "Service/" + oldSvcName, Reason: "ingressBackend",
					})
				}
			}
		}
	}

	// 2. Ingress -> Secrets in .spec.tls[].secretName
	tlsSlice, foundTls, errTls := unstructured.NestedSlice(ingress.Object, "spec", "tls")
	if errTls != nil {
		localLogger.WithError(errTls).Warn("Error retrieving .spec.tls from Ingress")
	}
	if foundTls {
		for _, tVal := range tlsSlice {
			tMap, ok := tVal.(map[string]interface{})
			if !ok {
				continue
			}
			if secName, ok := tMap["secretName"].(string); ok && secName != "" {
				deps[ingID] = append(deps[ingID], Edge{
					ChildID: "Secret/" + secName, Reason: "tlsSecret",
				})
			}
		}
	}
}

// handleHPAReferences checks .spec.scaleTargetRef for HPA objects, creating an
// edge with Reason="scaleTargetRef".
func handleHPAReferences(
	hpa *unstructured.Unstructured,
	deps map[string][]Edge,
) {
	localLogger := log.WithField("func", "handleHPAReferences")
	hpaID := ResourceID(hpa)
	scaleTarget, found, err := unstructured.NestedMap(hpa.Object, "spec", "scaleTargetRef")
	if err != nil {
		localLogger.WithError(err).Warn("Could not retrieve .spec.scaleTargetRef from HPA")
		return
	}
	if !found || len(scaleTarget) == 0 {
		return
	}
	if kind, ok := scaleTarget["kind"].(string); ok && kind != "" {
		if name, ok := scaleTarget["name"].(string); ok && name != "" {
			targetID := fmt.Sprintf("%s/%s", kind, name)
			deps[hpaID] = append(deps[hpaID], Edge{ChildID: targetID, Reason: "scaleTargetRef"})
		}
	}
}
