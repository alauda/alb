package framework

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestJsonEq(t *testing.T) {
	const jsonStr = `
	{
	"port_map": {
		"80": [
			{
				"rule": "alb-dev-00080-mzm6",
				"internal_dsl": [
					"AND",
					[
						"STARTS_WITH",
						"URL",
						"/a"
					],
					[
						"EQ",
						"HOST",
						"host-a"
					]
				]
			}
		]
	},
	"backend_group": [
		{
			"name": "alb-dev-00080-mzm6",
			"backends": [
				{
					"address": "192.168.1.0",
					"port": 80,
					"weight": 50
				},
				{
					"address": "192.168.1.2",
					"port": 80,
					"weight": 50
				}
			]
		}
	]
}`
	hasRule := PolicyHasRule(jsonStr, 80, "alb-dev-00080-mzm6")
	assert.True(t, hasRule)
	hasPod := PolicyHasBackEnds(jsonStr, "alb-dev-00080-mzm6", `[map[address:192.168.1.0 port:80 weight:50] map[address:192.168.1.2 port:80 weight:50]]`)
	assert.True(t, hasPod)
}
