package k8shandler

import (
	"reflect"
	"testing"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func TestGetReadinessProbe(t *testing.T) {
	goodProbe := v1.Probe{
		TimeoutSeconds:      30,
		InitialDelaySeconds: 10,
		FailureThreshold:    15,
		Handler: v1.Handler{
			TCPSocket: &v1.TCPSocketAction{
				Port: intstr.FromInt(9300),
			},
		},
	}
	if !reflect.DeepEqual(goodProbe, getReadinessProbe()) {
		t.Errorf("Probe was incorrect: %v", getReadinessProbe())
	}
}

func TestGetAffinity(t *testing.T) {
	roles := []string{"master", "clientdatamaster", "clientdata", "data", "client"}
	goodAffinities := []v1.Affinity{}
	for _, role := range roles {
		aff := v1.Affinity{
			PodAntiAffinity: &v1.PodAntiAffinity{
				PreferredDuringSchedulingIgnoredDuringExecution: []v1.WeightedPodAffinityTerm{
					{
						Weight: 100,
						PodAffinityTerm: v1.PodAffinityTerm{
							LabelSelector: &metav1.LabelSelector{
								MatchExpressions: []metav1.LabelSelectorRequirement{
									{
										Key:      "role",
										Operator: metav1.LabelSelectorOpIn,
										Values:   []string{role},
									},
								},
							},
							TopologyKey: "kubernetes.io/hostname",
						},
					},
				},
			},
		}
		goodAffinities = append(goodAffinities, aff)
	}

	for i, role := range roles {
		cfg := elasticsearchNode{
			NodeType: role,
		}
		if !reflect.DeepEqual(goodAffinities[i], cfg.getAffinity()) {
			t.Errorf("Incorrect v1.Affinity constructed for role: %v", role)

		}
	}
}
