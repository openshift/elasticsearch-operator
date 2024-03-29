package comparators

import (
	v1 "k8s.io/api/core/v1"
)

// AreTolerationsSame compares two lists of tolerations for equality
func AreTolerationsSame(lhs, rhs []v1.Toleration) bool {
	if len(lhs) != len(rhs) {
		return false
	}

	for _, lhsToleration := range lhs {
		if !containsToleration(lhsToleration, rhs) {
			return false
		}
	}

	return true
}

// containsSameTolerations checks that the tolerations in rhs are all contained within lhs
// this follows our other patterns of "current, desired"
func ContainsSameTolerations(lhs, rhs []v1.Toleration) bool {
	for _, rhsToleration := range rhs {
		if !containsToleration(rhsToleration, lhs) {
			return false
		}
	}

	return true
}

func containsToleration(toleration v1.Toleration, tolerations []v1.Toleration) bool {
	for _, t := range tolerations {
		if isTolerationSame(t, toleration) {
			return true
		}
	}

	return false
}

func isTolerationSame(lhs, rhs v1.Toleration) bool {
	tolerationSecondsBool := false
	// check that both are either null or not null
	if (lhs.TolerationSeconds == nil) == (rhs.TolerationSeconds == nil) {
		if lhs.TolerationSeconds != nil {
			// only compare values (attempt to dereference) if pointers aren't nil
			tolerationSecondsBool = *lhs.TolerationSeconds == *rhs.TolerationSeconds
		} else {
			tolerationSecondsBool = true
		}
	}

	tolerationEffectBool := lhs.Effect == rhs.Effect
	if lhs.Effect == "" || rhs.Effect == "" {
		tolerationEffectBool = true
	}

	// A toleration with the exists operator can leave the key empty to tolerate everything
	if (lhs.Operator == rhs.Operator) && (lhs.Operator == v1.TolerationOpExists) {
		if lhs.Key == "" || rhs.Key == "" {
			return true
		}
	}

	return (lhs.Key == rhs.Key) &&
		(lhs.Operator == rhs.Operator) &&
		(lhs.Value == rhs.Value) &&
		tolerationEffectBool &&
		tolerationSecondsBool
}
