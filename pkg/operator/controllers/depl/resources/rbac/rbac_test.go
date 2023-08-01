package rbac

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRoleFromJSON(t *testing.T) {
	rules := rulesOrPanic()
	t.Logf("role %v", CLUSTER_RULE_YAML)
	t.Logf("role %+v", rules)
	assert.Equal(t, len(rules) != 0, true)
	t.Logf("group %+v", rules[4])
	assert.Equal(t, rules[4].APIGroups[0], "discovery.k8s.io")
}
