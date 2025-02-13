package dependency

import (
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// BuildDependencies analyzes a list of unstructured Kubernetes objects and
// returns a map representing resource dependencies. The map’s key is the
// "parent" resource (e.g., "Deployment/my-app"), and the values are a slice
// of "child" resources that depend on it.
func BuildDependencies(objs []*unstructured.Unstructured) map[string][]string {
	// Create a top-level logger for this function
	logger := log.WithFields(log.Fields{
		"func":  "BuildDependencies",
		"count": len(objs),
	})
	logger.Info("Starting dependency analysis")

	dependencies := make(map[string][]string)

	// 1. Process ownerReferences (Owner -> Child).
	for _, obj := range objs {
		childID := resourceID(obj)
		for _, owner := range obj.GetOwnerReferences() {
			ownerID := fmt.Sprintf("%s/%s", owner.Kind, owner.Name)
			dependencies[ownerID] = append(dependencies[ownerID], childID)

			log.WithFields(log.Fields{
				"func":    "BuildDependencies",
				"ownerID": ownerID,
				"childID": childID,
			}).Debug("Added owner->child dependency")
		}
	}

	// 2. Process label selectors in certain resource kinds:
	//    - Service
	//    - NetworkPolicy
	//    - PodDisruptionBudget
	for _, obj := range objs {
		switch obj.GetKind() {
		case "Service":
			handleServiceLabelSelector(obj, objs, dependencies)
		case "NetworkPolicy":
			handleNetworkPolicy(obj, objs, dependencies)
		case "PodDisruptionBudget":
			handlePodDisruptionBudget(obj, objs, dependencies)
		}
	}

	// 3. Ingress references:
	//    - Ingress -> Services (spec.rules[].http.paths[].backend)
	//    - Ingress -> Secrets (TLS)
	for _, obj := range objs {
		if obj.GetKind() == "Ingress" {
			handleIngressReferences(obj, dependencies)
		}
	}

	// 4. HorizontalPodAutoscaler references:
	//    - HPA -> scaleTargetRef
	for _, obj := range objs {
		if obj.GetKind() == "HorizontalPodAutoscaler" {
			handleHPAReferences(obj, dependencies)
		}
	}

	// 5. Pod spec references in Pods, Deployments, DaemonSets, etc.
	for _, obj := range objs {
		if isPodOrController(obj) {
			podSpec, found, err := getPodSpec(obj)
			if err != nil {
				log.WithFields(log.Fields{
					"func":  "BuildDependencies",
					"error": err,
					"kind":  obj.GetKind(),
					"name":  obj.GetName(),
				}).Warn("Error retrieving podSpec")
				continue
			}
			if !found || podSpec == nil {
				continue
			}

			parentID := resourceID(obj)
			secrets, configMaps, pvcs, serviceAccounts := gatherPodSpecReferences(podSpec)

			// Append the references. No S1011 lint issues:
			dependencies[parentID] = append(dependencies[parentID], secrets...)
			dependencies[parentID] = append(dependencies[parentID], configMaps...)
			dependencies[parentID] = append(dependencies[parentID], pvcs...)
			dependencies[parentID] = append(dependencies[parentID], serviceAccounts...)
		}
	}

	logger.WithField("dependencies_count", len(dependencies)).Info("Finished building dependencies")
	return dependencies
}

// PrintDependencies logs the dependency map at Info level.
func PrintDependencies(deps map[string][]string) {
	logger := log.WithField("func", "PrintDependencies")
	logger.Info("Printing dependency relationships")

	for parent, children := range deps {
		if len(children) == 0 {
			continue
		}
		logger.WithFields(log.Fields{
			"parent":   parent,
			"children": children,
		}).Info("Dependency relationship")
	}
}

// GenerateDOT creates a DOT graph from the given dependency map.
func GenerateDOT(deps map[string][]string) string {
	var sb strings.Builder
	sb.WriteString("digraph G {\n")
	sb.WriteString("  rankdir=\"LR\";\n")
	sb.WriteString("  node [shape=box];\n\n")

	for parent, children := range deps {
		for _, child := range children {
			sb.WriteString(fmt.Sprintf("  \"%s\" -> \"%s\";\n", parent, child))
		}
	}
	sb.WriteString("}\n")
	return sb.String()
}

//--------------------------------------------------------------------------------
// Additional handlers

func handleServiceLabelSelector(
	svc *unstructured.Unstructured,
	allObjs []*unstructured.Unstructured,
	deps map[string][]string,
) {
	localLogger := log.WithField("func", "handleServiceLabelSelector")
	svcID := resourceID(svc)
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
	selectorMap := mapInterfaceToStringMap(selObj)

	for _, target := range allObjs {
		if labelsMatch(selectorMap, target.GetLabels()) {
			tgtID := resourceID(target)
			deps[svcID] = append(deps[svcID], tgtID)
			localLogger.WithFields(log.Fields{
				"serviceID": svcID,
				"targetID":  tgtID,
			}).Debug("Added service->target dependency")
		}
	}
}

func handleNetworkPolicy(
	np *unstructured.Unstructured,
	allObjs []*unstructured.Unstructured,
	deps map[string][]string,
) {
	localLogger := log.WithField("func", "handleNetworkPolicy")
	npID := resourceID(np)
	spec, found, err := unstructured.NestedMap(np.Object, "spec")
	if err != nil {
		localLogger.WithError(err).Warn("Could not retrieve .spec from NetworkPolicy")
		return
	}
	if !found {
		return
	}
	podSel, selFound, _ := unstructured.NestedMap(spec, "podSelector", "matchLabels")
	selectorMap := mapInterfaceToStringMap(podSel)

	if selFound && len(selectorMap) > 0 {
		for _, obj := range allObjs {
			if labelsMatch(selectorMap, obj.GetLabels()) {
				tgtID := resourceID(obj)
				deps[npID] = append(deps[npID], tgtID)
				localLogger.WithFields(log.Fields{
					"networkPolicy": npID,
					"targetID":      tgtID,
				}).Debug("Added networkpolicy->pod dependency")
			}
		}
	}
}

func handlePodDisruptionBudget(
	pdb *unstructured.Unstructured,
	allObjs []*unstructured.Unstructured,
	deps map[string][]string,
) {
	localLogger := log.WithField("func", "handlePodDisruptionBudget")
	pdbID := resourceID(pdb)
	spec, found, err := unstructured.NestedMap(pdb.Object, "spec")
	if err != nil {
		localLogger.WithError(err).Warn("Could not retrieve .spec from PDB")
		return
	}
	if !found {
		return
	}

	selMapObj, selFound, _ := unstructured.NestedMap(spec, "selector", "matchLabels")
	selMap := mapInterfaceToStringMap(selMapObj)

	if selFound && len(selMap) > 0 {
		for _, obj := range allObjs {
			if labelsMatch(selMap, obj.GetLabels()) {
				tgtID := resourceID(obj)
				deps[pdbID] = append(deps[pdbID], tgtID)
				localLogger.WithFields(log.Fields{
					"pdb":    pdbID,
					"target": tgtID,
				}).Debug("Added pdb->pod/controller dependency")
			}
		}
	}
}

func handleIngressReferences(
	ingress *unstructured.Unstructured,
	deps map[string][]string,
) {
	localLogger := log.WithField("func", "handleIngressReferences")
	ingID := resourceID(ingress)

	// 1. Ingress -> Services in spec.rules[].http.paths[].backend
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
				// newer style: .backend.service.name
				backendSvc, foundB, _ := unstructured.NestedMap(pathMap, "backend", "service")
				if foundB && backendSvc != nil {
					if svcName, ok := backendSvc["name"].(string); ok && svcName != "" {
						svcID := "Service/" + svcName
						deps[ingID] = append(deps[ingID], svcID)
						localLogger.WithFields(log.Fields{
							"ingress": ingID,
							"service": svcID,
						}).Debug("Added ingress->service dependency")
					}
				}
				// older style: .backend.serviceName
				if oldSvcName, oldFound, _ := unstructured.NestedString(pathMap, "backend", "serviceName"); oldFound && oldSvcName != "" {
					svcID := "Service/" + oldSvcName
					deps[ingID] = append(deps[ingID], svcID)
					localLogger.WithFields(log.Fields{
						"ingress": ingID,
						"service": svcID,
					}).Debug("Added older style ingress->service dependency")
				}
			}
		}
	}

	// 2. Ingress -> Secrets in spec.tls[].secretName
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
				secID := "Secret/" + secName
				deps[ingID] = append(deps[ingID], secID)
				localLogger.WithFields(log.Fields{
					"ingress": ingID,
					"secret":  secID,
				}).Debug("Added ingress->secret dependency for TLS")
			}
		}
	}
}

func handleHPAReferences(
	hpa *unstructured.Unstructured,
	deps map[string][]string,
) {
	localLogger := log.WithField("func", "handleHPAReferences")
	hpaID := resourceID(hpa)
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
			deps[hpaID] = append(deps[hpaID], targetID)
			localLogger.WithFields(log.Fields{
				"hpa":      hpaID,
				"targetID": targetID,
			}).Debug("Added hpa->target dependency")
		}
	}
}

// gatherPodSpecReferences checks volumes, env, serviceAccountName, imagePullSecrets
// across containers, initContainers, ephemeralContainers. Returns slices of references.
func gatherPodSpecReferences(
	podSpec map[string]interface{},
) (secretRefs, configMapRefs, pvcRefs, serviceAccounts []string) {
	// 1. volumes
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

	// 2. serviceAccountName
	if saName, foundSA, _ := unstructured.NestedString(podSpec, "serviceAccountName"); foundSA && saName != "" {
		serviceAccounts = append(serviceAccounts, "ServiceAccount/"+saName)
	}

	// 3. imagePullSecrets
	if ipsList, foundIPS, _ := unstructured.NestedSlice(podSpec, "imagePullSecrets"); foundIPS && len(ipsList) > 0 {
		for _, ips := range ipsList {
			if ipsMap, ok := ips.(map[string]interface{}); ok {
				if secretName, ok := ipsMap["name"].(string); ok && secretName != "" {
					secretRefs = append(secretRefs, "Secret/"+secretName)
				}
			}
		}
	}

	// 4. containers, initContainers, ephemeralContainers
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
									parseEnvValueFrom(valueFrom, &secretRefs, &configMapRefs)
								}
							}
						}
					}
					// envFrom
					if envFromList, foundEF, _ := unstructured.NestedSlice(cMap, "envFrom"); foundEF && len(envFromList) > 0 {
						for _, envFromVal := range envFromList {
							if envFromMap, ok := envFromVal.(map[string]interface{}); ok {
								parseEnvFrom(envFromMap, &secretRefs, &configMapRefs)
							}
						}
					}
				}
			}
		}
	}

	return
}

func parseEnvValueFrom(valueFrom map[string]interface{}, secretRefs, configMapRefs *[]string) {
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

func parseEnvFrom(envFrom map[string]interface{}, secretRefs, configMapRefs *[]string) {
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

func isPodOrController(obj *unstructured.Unstructured) bool {
	switch obj.GetKind() {
	case "Pod", "Deployment", "DaemonSet", "StatefulSet", "ReplicaSet", "Job", "CronJob":
		return true
	default:
		return false
	}
}

// getPodSpec attempts to read .spec or .spec.template.spec for known controllers, returning (podSpec, found, error).
func getPodSpec(obj *unstructured.Unstructured) (map[string]interface{}, bool, error) {
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

func labelsMatch(selector, labels map[string]string) bool {
	for k, v := range selector {
		if lv, found := labels[k]; !found || lv != v {
			return false
		}
	}
	return true
}

func resourceID(obj *unstructured.Unstructured) string {
	return fmt.Sprintf("%s/%s", obj.GetKind(), obj.GetName())
}

// mapInterfaceToStringMap is a small helper to safely convert an interface{}
// (expected to be map[string]interface{}) into map[string]string.
func mapInterfaceToStringMap(in interface{}) map[string]string {
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
