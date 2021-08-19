package spawn

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"
)

// This will fail if the yaml in base.yaml is invalid or if the structure is
// unexpected. If you're having problems with this test, look there.
func TestApplicationContainerIsValid(t *testing.T) {
	values, err := dealbotValues(randomDaemon())
	if err != nil {
		t.Fatalf("failed to decode helm values; probably bad yaml symtax. %v", err)
	}

	maybeContainer, ok := values["application"].(map[string]interface{})["container"]
	if !ok {
		t.Fatalf("values file doesn't have container defined.")
	}

	kcontainer := new(corev1.Container)
	buf, _ := yaml.Marshal(maybeContainer)
	if err := yaml.Unmarshal(buf, kcontainer); err != nil {
		t.Log("expected kubernetes container, instead found whatever this is")
		t.Log(string(buf))
		t.Fatalf("incorrect container schema")
	}
}
