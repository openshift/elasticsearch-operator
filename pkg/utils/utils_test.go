package utils

import (
	"os"
	"testing"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

const (
	envKey   = "TEST"
	envValue = "value"
)

func TestLookupEnvWithDefaultDefined(t *testing.T) {
	os.Setenv(envKey, envValue)
	res := LookupEnvWithDefault(envKey, "should be ignored")
	if res != envValue {
		t.Errorf("Expected %s=%s but got %s=%s", envKey, envValue, envKey, res)
	}
}

func TestLookupEnvWithDefaultUndefined(t *testing.T) {
	expected := "defaulted"
	os.Unsetenv(envKey)
	res := LookupEnvWithDefault(envKey, expected)
	if res != expected {
		t.Errorf("Expected %s=%s but got %s=%s", envKey, expected, envKey, res)
	}
}

func TestCompareResources(t *testing.T) {
	cases := []struct {
		current  *v1.ResourceRequirements
		desired  *v1.ResourceRequirements
		expected bool
	}{
		{
			&v1.ResourceRequirements{},
			&v1.ResourceRequirements{},
			false,
		},
		{
			&v1.ResourceRequirements{
				Limits: map[v1.ResourceName]resource.Quantity{"cpu": *resource.NewMilliQuantity(1000, resource.DecimalSI)},
			},
			&v1.ResourceRequirements{
				Limits: map[v1.ResourceName]resource.Quantity{"cpu": *resource.NewMilliQuantity(1000, resource.DecimalSI)},
			},
			false,
		},
		{
			&v1.ResourceRequirements{},
			&v1.ResourceRequirements{
				Limits: map[v1.ResourceName]resource.Quantity{"cpu": *resource.NewMilliQuantity(1000, resource.DecimalSI)},
			},
			true,
		},
		{
			&v1.ResourceRequirements{
				Limits: map[v1.ResourceName]resource.Quantity{"cpu": *resource.NewMilliQuantity(1000, resource.DecimalSI)},
			},
			&v1.ResourceRequirements{},
			true,
		},
		{
			&v1.ResourceRequirements{},
			&v1.ResourceRequirements{
				Requests: map[v1.ResourceName]resource.Quantity{"cpu": *resource.NewMilliQuantity(1000, resource.DecimalSI)},
			},
			true,
		},
		{
			&v1.ResourceRequirements{
				Requests: map[v1.ResourceName]resource.Quantity{"cpu": *resource.NewMilliQuantity(1000, resource.DecimalSI)},
			},
			&v1.ResourceRequirements{},
			true,
		},
	}

	for i, c := range cases {
		changed, _ := CompareResources(*c.current, *c.desired)
		if changed != c.expected {
			t.Errorf("Case %d: Expected %v but got %v", i, c.expected, changed)
		}
	}
}
