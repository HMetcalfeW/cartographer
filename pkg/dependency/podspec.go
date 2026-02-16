package dependency

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

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

// GatherPodSpecReferences scans a Pod spec (including volumes, env, envFrom,
// serviceAccountName, and imagePullSecrets) and returns slices of references
// for secrets, configmaps, PVCs, and service accounts.
func GatherPodSpecReferences(
	podSpec map[string]interface{},
) (secretRefs, configMapRefs, pvcRefs, serviceAccounts []string) {
	gatherVolumeRefs(podSpec, &secretRefs, &configMapRefs, &pvcRefs)
	gatherServiceAccountRefs(podSpec, &serviceAccounts)
	gatherImagePullSecretRefs(podSpec, &secretRefs)
	gatherContainerEnvRefs(podSpec, &secretRefs, &configMapRefs)
	return
}

// gatherVolumeRefs extracts secret, configMap, and PVC references from .spec.volumes.
func gatherVolumeRefs(podSpec map[string]interface{}, secretRefs, configMapRefs, pvcRefs *[]string) {
	volSlice, found, _ := unstructured.NestedSlice(podSpec, "volumes")
	if !found {
		return
	}
	for _, vol := range volSlice {
		volMap, ok := vol.(map[string]interface{})
		if !ok {
			continue
		}
		if sObj, ok := volMap["secret"].(map[string]interface{}); ok {
			if sName, ok := sObj["secretName"].(string); ok {
				*secretRefs = append(*secretRefs, "Secret/"+sName)
			}
		} else if cmObj, ok := volMap["configMap"].(map[string]interface{}); ok {
			if cmName, ok := cmObj["name"].(string); ok {
				*configMapRefs = append(*configMapRefs, "ConfigMap/"+cmName)
			}
		} else if pvcObj, ok := volMap["persistentVolumeClaim"].(map[string]interface{}); ok {
			if pvcName, ok := pvcObj["claimName"].(string); ok {
				*pvcRefs = append(*pvcRefs, "PersistentVolumeClaim/"+pvcName)
			}
		}
	}
}

// gatherServiceAccountRefs extracts .spec.serviceAccountName.
func gatherServiceAccountRefs(podSpec map[string]interface{}, serviceAccounts *[]string) {
	if saName, found, _ := unstructured.NestedString(podSpec, "serviceAccountName"); found && saName != "" {
		*serviceAccounts = append(*serviceAccounts, "ServiceAccount/"+saName)
	}
}

// gatherImagePullSecretRefs extracts secret names from .spec.imagePullSecrets.
func gatherImagePullSecretRefs(podSpec map[string]interface{}, secretRefs *[]string) {
	ipsList, found, _ := unstructured.NestedSlice(podSpec, "imagePullSecrets")
	if !found {
		return
	}
	for _, ips := range ipsList {
		if ipsMap, ok := ips.(map[string]interface{}); ok {
			if secretName, ok := ipsMap["name"].(string); ok && secretName != "" {
				*secretRefs = append(*secretRefs, "Secret/"+secretName)
			}
		}
	}
}

// gatherContainerEnvRefs extracts secret/configMap references from env and envFrom
// across containers, initContainers, and ephemeralContainers.
func gatherContainerEnvRefs(podSpec map[string]interface{}, secretRefs, configMapRefs *[]string) {
	for _, cKey := range []string{"containers", "initContainers", "ephemeralContainers"} {
		cList, found, _ := unstructured.NestedSlice(podSpec, cKey)
		if !found {
			continue
		}
		for _, cVal := range cList {
			cMap, ok := cVal.(map[string]interface{})
			if !ok {
				continue
			}
			if envList, foundEnv, _ := unstructured.NestedSlice(cMap, "env"); foundEnv {
				for _, envVal := range envList {
					if envMap, ok := envVal.(map[string]interface{}); ok {
						if valueFrom, ok := envMap["valueFrom"].(map[string]interface{}); ok {
							ParseEnvValueFrom(valueFrom, secretRefs, configMapRefs)
						}
					}
				}
			}
			if envFromList, foundEF, _ := unstructured.NestedSlice(cMap, "envFrom"); foundEF {
				for _, envFromVal := range envFromList {
					if envFromMap, ok := envFromVal.(map[string]interface{}); ok {
						ParseEnvFrom(envFromMap, secretRefs, configMapRefs)
					}
				}
			}
		}
	}
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
