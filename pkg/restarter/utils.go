package restarter

import (
	"io/ioutil"
	"time"

	"github.com/ghodss/yaml"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func LoadServiceDependants(file string) (*ServiceDependants, error) {
	data, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}
	return decodeConfigFile(data)
}

func decodeConfigFile(data []byte) (*ServiceDependants, error) {
	dependants := new(ServiceDependants)
	err := yaml.Unmarshal(data, dependants)
	if err != nil {
		return nil, err
	}
	return dependants, nil
}

// IsPodAvailable returns true if a pod is available; false otherwise.
// Precondition for an available pod is that it must be ready. On top
// of that, there are two cases when a pod can be considered available:
// 1. minReadySeconds == 0, or
// 2. LastTransitionTime (is set) + minReadySeconds < current time
func IsPodAvailable(pod *v1.Pod, minReadySeconds int32, now metav1.Time) bool {
	if !IsPodReady(pod) {
		return false
	}

	c := GetPodReadyCondition(pod.Status)
	minReadySecondsDuration := time.Duration(minReadySeconds) * time.Second
	if minReadySeconds == 0 || !c.LastTransitionTime.IsZero() && c.LastTransitionTime.Add(minReadySecondsDuration).Before(now.Time) {
		return true
	}
	return false
}

// IsPodReady returns true if a pod is ready; false otherwise.
func IsPodReady(pod *v1.Pod) bool {
	return IsPodReadyConditionTrue(pod.Status)
}

// IsPodReady returns true if a pod is ready; false otherwise.
func IsPodDeleted(pod *v1.Pod) bool {
	return pod.DeletionTimestamp != nil
}

// IsPodReadyConditionTrue returns true if a pod is ready; false otherwise.
func IsPodReadyConditionTrue(status v1.PodStatus) bool {
	condition := GetPodReadyCondition(status)
	return condition != nil && condition.Status == v1.ConditionTrue
}

// GetPodReadyCondition extracts the pod ready condition from the given status and returns that.
// Returns nil if the condition is not present.
func GetPodReadyCondition(status v1.PodStatus) *v1.PodCondition {
	_, condition := GetPodCondition(&status, v1.PodReady)
	return condition
}

// GetPodCondition extracts the provided condition from the given status and returns that.
// Returns nil and -1 if the condition is not present, and the index of the located condition.
func GetPodCondition(status *v1.PodStatus, conditionType v1.PodConditionType) (int, *v1.PodCondition) {
	if status == nil {
		return -1, nil
	}
	return GetPodConditionFromList(status.Conditions, conditionType)
}

// GetPodConditionFromList extracts the provided condition from the given list of condition and
// returns the index of the condition and the condition. Returns -1 and nil if the condition is not present.
func GetPodConditionFromList(conditions []v1.PodCondition, conditionType v1.PodConditionType) (int, *v1.PodCondition) {
	if conditions == nil {
		return -1, nil
	}
	for i := range conditions {
		if conditions[i].Type == conditionType {
			return i, &conditions[i]
		}
	}
	return -1, nil
}

func ShouldDeletePod(pod *v1.Pod) bool {
	return !IsPodDeleted(pod) && IsPodInCrashloopBackoff(pod.Status)
}

func IsPodInCrashloopBackoff(status v1.PodStatus) bool {
	for _, containerStatus := range status.ContainerStatuses {
		if IsContainerInCrashLoopBackOff(containerStatus.State) {
			return true
		}
	}
	return false
}

func IsContainerInCrashLoopBackOff(containerState v1.ContainerState) bool {
	if containerState.Waiting != nil {
		return containerState.Waiting.Reason == CrashLoopBackOff
	}
	return false
}

func IsReadyEndpointPresentInSubsets(subsets []v1.EndpointSubset) bool {
	for _, subset := range subsets {
		if len(subset.Addresses) != 0 {
			return true
		}
	}
	return false
}
