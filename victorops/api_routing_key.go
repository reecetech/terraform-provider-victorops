package victorops

import (
	"fmt"
	"net/url"
)

type routingKeyUpdateRequest struct {
	RoutingKey string   `json:"routingKey"`
	Targets    []string `json:"targets"`
}

// UpdateRoutingKey renames a routing key and replaces its targets (policy slugs)
func (c *APIClient) UpdateRoutingKey(key, newKey string, targets []string) error {
	found, err := c.call("PUT", "/api-public/v1/org/routing-keys/"+url.PathEscape(key), routingKeyUpdateRequest{newKey, targets}, nil)
	if err == nil && !found {
		err = fmt.Errorf("routing key %q not found", key)
	}
	return err
}

// DeleteRoutingKey deletes a routing key; a missing one counts as deleted
func (c *APIClient) DeleteRoutingKey(key string) error {
	_, err := c.call("DELETE", "/api-public/v1/org/routing-keys/"+url.PathEscape(key), nil, nil)
	return err
}
