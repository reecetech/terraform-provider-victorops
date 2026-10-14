package victorops

import (
	"reflect"
	"testing"
)

func TestRoutingKeyUpdateAndDelete(t *testing.T) {
	c, calls := recorder(t)
	if err := c.UpdateRoutingKey("old key", "new", []string{"pol-1"}); err != nil {
		t.Fatal(err)
	}
	if err := c.DeleteRoutingKey("new"); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"PUT /api-public/v1/org/routing-keys/old key map[routingKey:new targets:[pol-1]]",
		"DELETE /api-public/v1/org/routing-keys/new map[]",
	}
	if !reflect.DeepEqual(calls(), want) {
		t.Fatalf("got %q", calls())
	}
}
