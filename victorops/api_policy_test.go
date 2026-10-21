package victorops

import (
	"reflect"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/victorops/go-victorops/victorops"
)

func TestUpdateEscalationPolicy(t *testing.T) {
	c, calls := recorder(t)
	steps := []victorops.EscalationPolicySteps{{Timeout: 5, Entries: []victorops.EscalationPolicyStepEntry{{ExecutionType: "user", User: map[string]string{"username": "a"}}}}}
	if err := c.UpdateEscalationPolicy("pol-1", true, steps); err != nil {
		t.Fatal(err)
	}
	got := calls()
	if len(got) != 1 || got[0][:len("PUT /api-public/v1/policies/pol-1 ")] != "PUT /api-public/v1/policies/pol-1 " {
		t.Fatalf("got %q", got)
	}
}

func TestPolicyStepsRoundTrip(t *testing.T) {
	raw := map[string]interface{}{
		"name":    "p",
		"team_id": "team-1",
		"step": []interface{}{
			map[string]interface{}{"timeout": 0, "entries": []interface{}{
				map[string]interface{}{"type": "user", "username": "bob"},
				map[string]interface{}{"type": "email", "address": "a@b"},
				map[string]interface{}{"type": "rotationGroup", "slug": "rtg-1"},
				map[string]interface{}{"type": "rotationGroupNext", "slug": "rtg-1"},
				map[string]interface{}{"type": "rotationGroupPrevious", "slug": "rtg-1"},
			}},
			map[string]interface{}{"timeout": 15, "entries": []interface{}{
				map[string]interface{}{"type": "webhook", "slug": "wh-1"},
				map[string]interface{}{"type": "targetPolicy", "slug": "pol-2"},
			}},
		},
	}
	d := schema.TestResourceDataRaw(t, resourceEscalationPolicy().Schema, raw)
	ep, err := generateEscalationPolicyFromResourceData(d)
	if err != nil {
		t.Fatal(err)
	}
	got := flattenEscalationPolicySteps(ep.Steps)
	if !reflect.DeepEqual(got, raw["step"]) {
		t.Fatalf("got %#v\nwant %#v", got, raw["step"])
	}
}

func TestPolicyUnsupportedEntryType(t *testing.T) {
	d := schema.TestResourceDataRaw(t, resourceEscalationPolicy().Schema, map[string]interface{}{
		"name": "p", "team_id": "t",
		"step": []interface{}{map[string]interface{}{"entries": []interface{}{map[string]interface{}{"type": "team_page"}}}},
	})
	if _, err := generateEscalationPolicyFromResourceData(d); err == nil {
		t.Fatal("expected an error for an unsupported type")
	}
}

func TestPolicyWithoutSteps(t *testing.T) {
	d := schema.TestResourceDataRaw(t, resourceEscalationPolicy().Schema, map[string]interface{}{"name": "p", "team_id": "t"})
	ep, err := generateEscalationPolicyFromResourceData(d)
	if err != nil {
		t.Fatal(err)
	}
	// Must marshal as [], not null, which the API rejects
	if ep.Steps == nil || len(ep.Steps) != 0 {
		t.Fatalf("got %#v", ep.Steps)
	}
	if got := flattenEscalationPolicySteps(nil); len(got) != 0 {
		t.Fatalf("got %#v", got)
	}

	c, calls := recorder(t)
	if err := c.UpdateEscalationPolicy("pol-1", false, ep.Steps); err != nil {
		t.Fatal(err)
	}
	if got := calls(); len(got) != 1 || got[0] != "PUT /api-public/v1/policies/pol-1 map[ignoreCustomPagingPolicies:false steps:[]]" {
		t.Fatalf("got %q", got)
	}
}
