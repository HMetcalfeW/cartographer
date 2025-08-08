package dependency

import (
	"fmt"
	"strings"
	"encoding/json"

	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Edge represents a single dependency from one Kubernetes resource (the parent)
// to another resource (the child), along with the reason describing how or why
// the parent references the child.
type Edge struct {
	// ChildID is the unique identifier of the child resource, in the form "Kind/Name".
	ChildID string

	// Reason describes the nature of this dependency, e.g., "ownerRef", "secretRef", "selector".
	Reason string
}

// Analyzer defines the interface for a dependency analysis component.
type Analyzer interface {
	Analyze(obj *unstructured.Unstructured, allObjs []*unstructured.Unstructured, dependencies map[string][]Edge)
}

// OwnerRefAnalyzer analyzes owner references.
type OwnerRefAnalyzer struct{}

// Analyze processes owner references for a given object.
func (a *OwnerRefAnalyzer) Analyze(obj *unstructured.Unstructured, _ []*unstructured.Unstructured, dependencies map[string][]Edge) {
	childID := ResourceID(obj)
	for _, owner := range obj.GetOwnerReferences() {
		ownerID := fmt.Sprintf("%s/%s", owner.Kind, owner.Name)
		edge := Edge{ChildID: childID, Reason: "ownerRef"}
		dependencies[ownerID] = append(dependencies[ownerID], edge)

		log.WithFields(log.Fields{
			"func":    "OwnerRefAnalyzer.Analyze",
			"ownerID": ownerID,
			"childID": childID,
		}).Debug("Added owner->child dependency")
	}
}

// BuildDependencies analyzes a slice of unstructured Kubernetes objects and
// identifies their interdependencies. It returns a map where each key is a
// "parent" resource identifier ("Kind/Name"), and each value is a slice of
// Edge structures describing the child resource and the reason for the link.
//
// Example:
//
//	"Deployment/foo" -> Edge{ChildID: "Secret/bar", Reason: "secretRef"}.
func BuildDependencies(objs []*unstructured.Unstructured) map[string][]Edge {
	mainLogger := log.WithFields(log.Fields{
		"func":  "BuildDependencies",
		"count": len(objs),
	})
	mainLogger.Info("Starting dependency analysis")

	dependencies := make(map[string][]Edge)

	// 1. Create an empty slice for every resource upfront, so loners appear in the final map.
	for _, obj := range objs {
		parentKey := ResourceID(obj)
		dependencies[parentKey] = []Edge{} // ensures each resource is present
	}

	// Define the order of analyzers.
	analyzers := []Analyzer{
		&OwnerRefAnalyzer{},
		&LabelSelectorAnalyzer{},
		&IngressAnalyzer{},
		&HPAAnalyzer{},
		&PodSpecAnalyzer{},
	}

	// Run each analyzer.
	for _, analyzer := range analyzers {
		for _, obj := range objs {
			analyzer.Analyze(obj, objs, dependencies)
		}
	}

	mainLogger.WithField("dependencies_count", len(dependencies)).Info("Finished building dependencies")
	return dependencies
}

// PrintDependencies logs each parent and its dependencies (Edges) at the Info level.
// It prints both the child resource identifiers and the reason for each dependency.
func PrintDependencies(deps map[string][]Edge) {
	logger := log.WithField("func", "PrintDependencies")
	logger.Info("Printing dependency relationships")

	for parent, edges := range deps {
		if len(edges) == 0 {
			continue
		}
		childStrings := make([]string, 0, len(edges))
		for _, e := range edges {
			childStrings = append(childStrings, fmt.Sprintf("%s(%s)", e.ChildID, e.Reason))
		}
		logger.WithFields(log.Fields{
			"parent": parent,
			"edges":  childStrings,
		}).Info("Dependency relationship")
	}
}

// GenerateDOT produces a DOT graph where each parent node has directed edges
// to its child nodes, labeled with the Reason describing why the relationship exists.
//
// Example:
//
//	"Deployment/my-deploy" -> "Secret/my-secret" [label="secretRef"];
func GenerateDOT(deps map[string][]Edge) string {
	var sb strings.Builder
	sb.WriteString("digraph G {\n")
	sb.WriteString("  rankdir=\"LR\";\n")
	sb.WriteString("  node [shape=box];\n\n")

	for parent, edges := range deps {
		for _, edge := range edges {
			sb.WriteString(fmt.Sprintf("  \"%s\" -> \"%s\" [label=\"%s\"];\n", parent, edge.ChildID, edge.Reason))
		}
	}
	sb.WriteString("}\n")
	return sb.String()
}

// GenerateMermaid produces a Mermaid graph where each parent node has directed edges
// to its child nodes, labeled with the Reason describing why the relationship exists.
func GenerateMermaid(deps map[string][]Edge) string {
	var sb strings.Builder
	sb.WriteString("graph LR\n")

	for parent, edges := range deps {
		for _, edge := range edges {
			// Sanitize IDs for Mermaid (replace / with _)
			sanitizedParent := strings.ReplaceAll(parent, "/", "_")
			sanitizedChild := strings.ReplaceAll(edge.ChildID, "/", "_")
			sb.WriteString(fmt.Sprintf("  %s --> |%s| %s\n", sanitizedParent, edge.Reason, sanitizedChild))
		}
	}
	return sb.String()
}

// GenerateJSON produces a JSON representation of the dependency graph.
func GenerateJSON(deps map[string][]Edge) (string, error) {
	// For simplicity, we'll convert the map to a slice of structs for JSON marshaling.
	type Node struct {
		ID    string `json:"id"`
		Edges []Edge `json:"edges"`
	}

	var nodes []Node
	for parentID, edges := range deps {
		nodes = append(nodes, Node{ID: parentID, Edges: edges})
	}
	jsonBytes, err := json.MarshalIndent(nodes, "", "  ")
	if err != nil {
		return "", err
	}
	return string(jsonBytes), nil
}

// IsPodOrController returns true if the object is a Pod or a common controller
// type that embeds a Pod spec (.spec.template.spec or .spec.jobTemplate...).
func IsPodOrController(obj *unstructured.Unstructured) bool {
	switch obj.GetKind() {
	case "Pod", "Deployment", "DaemonSet", "StatefulSet", "ReplicaSet", "Job", "CronJob":
		return true
	default:
		return false
	}
}

// ResourceID builds a string "Kind/Name" from the object's kind and metadata.name.
func ResourceID(obj *unstructured.Unstructured) string {
	return fmt.Sprintf("%s/%s", obj.GetKind(), obj.GetName())
}

// LabelsMatch returns true if all key-value pairs in 'selector' are present in 'labels'.
func LabelsMatch(selector, labels map[string]string) bool {
	for k, v := range selector {
		if lv, found := labels[k]; !found || lv != v {
			return false
		}
	}
	return true
}

// MapInterfaceToStringMap attempts to cast an interface{} to map[string]interface{},
// then converts each value to a string if possible. Useful for label selectors
// or other fields that store data as map[string]interface{}.
func MapInterfaceToStringMap(in interface{}) map[string]string {
	out := make(map[string]string)
	if inMap, ok := in.(map[string]interface{}); ok {
		for k, v := range inMap {
			if vs, isStr := v.(string); isStr {
				out[k] = vs
			}
		}
	}
	return out
}

// GetPodSpec attempts to read .spec or .spec.template.spec for known controllers.
// If successful, it returns (podSpec, found=true, err=nil). Otherwise, found will
// be false or err will be non-nil, indicating an error or no pod spec.
func GetPodSpec(obj *unstructured.Unstructured) (map[string]interface{}, bool, error) {
	switch obj.GetKind() {
	case "Pod":
		return unstructured.NestedMap(obj.Object, "spec")
	case "Deployment", "DaemonSet", "StatefulSet", "ReplicaSet", "Job":
		return unstructured.NestedMap(obj.Object, "spec", "template", "spec")
	case "CronJob":
		return unstructured.NestedMap(obj.Object, "spec", "jobTemplate", "spec", "template", "spec")
	default:
		return nil, false, fmt.Errorf("kind %s does not have a standard pod template", obj.GetKind())
	}
}

// PodSpecAnalyzer analyzes Pod spec references.
type PodSpecAnalyzer struct{}

// Analyze processes Pod spec references for a given object.
func (a *PodSpecAnalyzer) Analyze(obj *unstructured.Unstructured, _ []*unstructured.Unstructured, dependencies map[string][]Edge) {
	if IsPodOrController(obj) {
		podSpec, found, err := GetPodSpec(obj)
		if err != nil {
			log.WithFields(log.Fields{
				"func":  "PodSpecAnalyzer.Analyze",
				"error": err,
				"kind":  obj.GetKind(),
				"name":  obj.GetName(),
			}).Warn("Error retrieving podSpec")
			return
		}
		if !found || podSpec == nil {
			return
		}

		parentID := ResourceID(obj)
		secrets, configMaps, pvcs, serviceAccounts := GatherPodSpecReferences(podSpec)

		for _, child := range secrets {
			dependencies[parentID] = append(dependencies[parentID], Edge{
				ChildID: child,
				Reason:  "secretRef",
			})
		}
		for _, child := range configMaps {
			dependencies[parentID] = append(dependencies[parentID], Edge{
				ChildID: child,
				Reason:  "configMapRef",
			})
		}
		for _, child := range pvcs {
			dependencies[parentID] = append(dependencies[parentID], Edge{
				ChildID: child,
				Reason:  "pvcRef",
			})
		}
		for _, child := range serviceAccounts {
			dependencies[parentID] = append(dependencies[parentID], Edge{
				ChildID: child,
				Reason:  "serviceAccountName",
			})
		}
	}
}

// GatherPodSpecReferences scans a Pod spec (including volumes, env, envFrom,
// serviceAccountName, and imagePullSecrets) and returns slices of references
// for secrets, configmaps, PVCs, and service accounts.
func GatherPodSpecReferences(
	podSpec map[string]interface{},
) (secretRefs, configMapRefs, pvcRefs, serviceAccounts []string) {
	// Volumes
	if volSlice, foundVol, _ := unstructured.NestedSlice(podSpec, "volumes"); foundVol && len(volSlice) > 0 {
		for _, vol := range volSlice {
			if volMap, ok := vol.(map[string]interface{}); ok {
				switch {
				case volMap["secret"] != nil:
					sObj := volMap["secret"].(map[string]interface{})
					if sName, ok := sObj["secretName"].(string); ok {
						secretRefs = append(secretRefs, "Secret/"+sName)
					}
				case volMap["configMap"] != nil:
					cmObj := volMap["configMap"].(map[string]interface{})
					if cmName, ok := cmObj["name"].(string); ok {
						configMapRefs = append(configMapRefs, "ConfigMap/"+cmName)
					}
				case volMap["persistentVolumeClaim"] != nil:
					pvcObj := volMap["persistentVolumeClaim"].(map[string]interface{})
					if pvcName, ok := pvcObj["claimName"].(string); ok {
						pvcRefs = append(pvcRefs, "PersistentVolumeClaim/"+pvcName)
					}
				}
			}
		}
	}

	// serviceAccountName
	if saName, foundSA, _ := unstructured.NestedString(podSpec, "serviceAccountName"); foundSA && saName != "" {
		serviceAccounts = append(serviceAccounts, "ServiceAccount/"+saName)
	}

	// imagePullSecrets
	if ipsList, foundIPS, _ := unstructured.NestedSlice(podSpec, "imagePullSecrets"); foundIPS && len(ipsList) > 0 {
		for _, ips := range ipsList {
			if ipsMap, ok := ips.(map[string]interface{}); ok {
				if secretName, ok := ipsMap["name"].(string); ok && secretName != "" {
					secretRefs = append(secretRefs, "Secret/"+secretName)
				}
			}
		}
	}

	// containers, initContainers, ephemeralContainers
	cKeys := []string{"containers", "initContainers", "ephemeralContainers"}
	for _, cKey := range cKeys {
		if cList, foundC, _ := unstructured.NestedSlice(podSpec, cKey); foundC && len(cList) > 0 {
			for _, cVal := range cList {
				if cMap, ok := cVal.(map[string]interface{}); ok {
					// env
					if envList, foundEnv, _ := unstructured.NestedSlice(cMap, "env"); foundEnv && len(envList) > 0 {
						for _, envVal := range envList {
							if envMap, ok := envVal.(map[string]interface{}); ok {
								if valueFrom, ok := envMap["valueFrom"].(map[string]interface{}); ok {
									ParseEnvValueFrom(valueFrom, &secretRefs, &configMapRefs)
								}
							}
						}
					}
					// envFrom
					if envFromList, foundEF, _ := unstructured.NestedSlice(cMap, "envFrom"); foundEF && len(envFromList) > 0 {
						for _, envFromVal := range envFromList {
							if envFromMap, ok := envFromVal.(map[string]interface{}); ok {
								ParseEnvFrom(envFromMap, &secretRefs, &configMapRefs)
							}
						}
					}
				}
			}
		}
	}

	return
}

// ParseEnvValueFrom examines env[].valueFrom for references to secrets/configmaps.
func ParseEnvValueFrom(valueFrom map[string]interface{}, secretRefs, configMapRefs *[]string) {
	if sRef, ok := valueFrom["secretKeyRef"].(map[string]interface{}); ok {
		if name, ok := sRef["name"].(string); ok {
			*secretRefs = append(*secretRefs, "Secret/"+name)
		}
	}
	if cmRef, ok := valueFrom["configMapKeyRef"].(map[string]interface{}); ok {
		if name, ok := cmRef["name"].(string); ok {
			*configMapRefs = append(*configMapRefs, "ConfigMap/"+name)
		}
	}
}

// ParseEnvFrom examines envFrom[].secretRef or envFrom[].configMapRef for references.
func ParseEnvFrom(envFrom map[string]interface{}, secretRefs, configMapRefs *[]string) {
	if sRef, ok := envFrom["secretRef"].(map[string]interface{}); ok {
		if name, ok := sRef["name"].(string); ok {
			*secretRefs = append(*secretRefs, "Secret/"+name)
		}
	}
	if cmRef, ok := envFrom["configMapRef"].(map[string]interface{}); ok {
		if name, ok := cmRef["name"].(string); ok {
			*configMapRefs = append(*configMapRefs, "ConfigMap/"+name)
		}
	}
}

// LabelSelectorAnalyzer analyzes label selectors for Services, NetworkPolicies, and PodDisruptionBudgets.
type LabelSelectorAnalyzer struct{}

// Analyze processes label selectors for a given object.
func (a *LabelSelectorAnalyzer) Analyze(obj *unstructured.Unstructured, allObjs []*unstructured.Unstructured, dependencies map[string][]Edge) {
	switch obj.GetKind() {
	case "Service":
		a.handleServiceLabelSelector(obj, allObjs, dependencies)
	case "NetworkPolicy":
		a.handleNetworkPolicy(obj, allObjs, dependencies)
	case "PodDisruptionBudget":
		a.handlePodDisruptionBudget(obj, allObjs, dependencies)
	}
}

// handleServiceLabelSelector finds Pods or higher-level controllers whose labels match
// the Service's .spec.selector, and records each matching resource as a child with Reason="selector".
func (a *LabelSelectorAnalyzer) handleServiceLabelSelector(
	svc *unstructured.Unstructured,
	allObjs []*unstructured.Unstructured,
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

	for _, target := range allObjs {
		if LabelsMatch(selectorMap, target.GetLabels()) {
			tgtID := ResourceID(target)
			deps[svcID] = append(deps[svcID], Edge{ChildID: tgtID, Reason: "selector"})
			localLogger.WithFields(log.Fields{
				"serviceID": svcID,
				"targetID":  tgtID,
			}).Debug("Added service->target dependency")
		}
	}
}

// handleNetworkPolicy finds Pods or controllers whose labels match
// .spec.podSelector.matchLabels, and records each link as Reason="podSelector".
func (a *LabelSelectorAnalyzer) handleNetworkPolicy(
	np *unstructured.Unstructured,
	allObjs []*unstructured.Unstructured,
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
		for _, obj := range allObjs {
			if LabelsMatch(selectorMap, obj.GetLabels()) {
				tgtID := ResourceID(obj)
				deps[npID] = append(deps[npID], Edge{ChildID: tgtID, Reason: "podSelector"})
				localLogger.WithFields(log.Fields{
					"networkPolicy": npID,
					"targetID":      tgtID,
				}).Debug("Added networkpolicy->pod dependency")
			}
		}
	}
}

// handlePodDisruptionBudget processes .spec.selector.matchLabels to find
// target objects (Pods, controllers) and creates an edge with Reason="pdbSelector".
func (a *LabelSelectorAnalyzer) handlePodDisruptionBudget(
	pdb *unstructured.Unstructured,
	allObjs []*unstructured.Unstructured,
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
		for _, obj := range allObjs {
			if LabelsMatch(selMap, obj.GetLabels()) {
				tgtID := ResourceID(obj)
				deps[pdbID] = append(deps[pdbID], Edge{ChildID: tgtID, Reason: "pdbSelector"})
				localLogger.WithFields(log.Fields{
					"pdb":    pdbID,
					"target": tgtID,
				}).Debug("Added pdb->pod/controller dependency")
			}
		}
	}
}

// IngressAnalyzer analyzes Ingress references.
type IngressAnalyzer struct{}

// Analyze processes Ingress references for a given object.
func (a *IngressAnalyzer) Analyze(obj *unstructured.Unstructured, _ []*unstructured.Unstructured, dependencies map[string][]Edge) {
	if obj.GetKind() == "Ingress" {
		a.handleIngressReferences(obj, dependencies)
	}
}

// handleIngressReferences inspects an Ingress's .spec.rules[].http.paths[].backend
// (both newer and older styles) and .spec.tls[].secretName, creating edges with
// Reason="ingressBackend" or Reason="tlsSecret", respectively.
func (a *IngressAnalyzer) handleIngressReferences(
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

// HPAAnalyzer analyzes HorizontalPodAutoscaler references.
type HPAAnalyzer struct{}

// Analyze processes HPA references for a given object.
func (a *HPAAnalyzer) Analyze(obj *unstructured.Unstructured, _ []*unstructured.Unstructured, dependencies map[string][]Edge) {
	if obj.GetKind() == "HorizontalPodAutoscaler" {
		a.handleHPAReferences(obj, dependencies)
	}
}

// handleHPAReferences checks .spec.scaleTargetRef for HPA objects, creating an
// edge with Reason="scaleTargetRef".
func (a *HPAAnalyzer) handleHPAReferences(
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
