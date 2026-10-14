package victorops

import (
	"net/url"

	"github.com/victorops/go-victorops/victorops"
)

type escalationPolicyUpdateRequest struct {
	IgnoreCustomPagingPolicies bool                              `json:"ignoreCustomPagingPolicies"`
	Steps                      []victorops.EscalationPolicySteps `json:"steps"`
}

// UpdateEscalationPolicy replaces a policy's steps; its name and team cannot change
func (c *APIClient) UpdateEscalationPolicy(slug string, ignoreCustomPaging bool, steps []victorops.EscalationPolicySteps) error {
	_, err := c.call("PUT", "/api-public/v1/policies/"+url.PathEscape(slug), escalationPolicyUpdateRequest{ignoreCustomPaging, steps}, nil)
	return err
}
