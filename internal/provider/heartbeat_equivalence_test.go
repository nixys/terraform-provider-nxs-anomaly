package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func heartbeatObj(interval, grace attr.Value) types.Object {
	obj, _ := types.ObjectValue(heartbeatAttrTypes, map[string]attr.Value{"interval_seconds": interval, "grace_seconds": grace})
	return obj
}

func heartbeatValues(o types.Object) (int64, int64) {
	a := o.Attributes()
	return a["interval_seconds"].(types.Int64).ValueInt64(), a["grace_seconds"].(types.Int64).ValueInt64()
}

func TestKeepEquivalentHeartbeat(t *testing.T) {
	cases := []struct {
		name                    string
		plan, api               types.Object
		wantInterval, wantGrace int64
	}{
		{"grace 0 is the API's interval/3", heartbeatObj(types.Int64Value(300), types.Int64Value(0)),
			heartbeatObj(types.Int64Value(300), types.Int64Value(100)), 300, 0},
		{"interval below 60 is raised", heartbeatObj(types.Int64Value(30), types.Int64Value(0)),
			heartbeatObj(types.Int64Value(60), types.Int64Value(20)), 30, 0},
		{"unset grace takes the API's value", heartbeatObj(types.Int64Value(300), types.Int64Unknown()),
			heartbeatObj(types.Int64Value(300), types.Int64Value(100)), 300, 100},
		{"a different grace is drift", heartbeatObj(types.Int64Value(300), types.Int64Value(0)),
			heartbeatObj(types.Int64Value(300), types.Int64Value(45)), 300, 45},
		{"a different interval is drift", heartbeatObj(types.Int64Value(300), types.Int64Value(50)),
			heartbeatObj(types.Int64Value(600), types.Int64Value(50)), 600, 50},
		{"disabled stays disabled", heartbeatObj(types.Int64Value(0), types.Int64Value(0)),
			heartbeatObj(types.Int64Value(0), types.Int64Value(0)), 0, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			i, g := heartbeatValues(keepEquivalentHeartbeat(c.api, c.plan))
			if i != c.wantInterval || g != c.wantGrace {
				t.Errorf("got interval=%d grace=%d, want %d/%d", i, g, c.wantInterval, c.wantGrace)
			}
		})
	}
}
