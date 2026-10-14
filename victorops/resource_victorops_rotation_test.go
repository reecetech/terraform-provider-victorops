package victorops

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"regexp"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"golang.org/x/time/rate"
)

// recorder serves every request with {} and records "METHOD path body"
func recorder(t *testing.T) (*APIClient, func() []string) {
	t.Helper()
	old := GlobalRateLimiter.Limit()
	GlobalRateLimiter.SetLimit(rate.Inf)
	t.Cleanup(func() { GlobalRateLimiter.SetLimit(old) })

	var mu sync.Mutex
	var calls []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		mu.Lock()
		calls = append(calls, fmt.Sprintf("%s %s %v", r.Method, r.URL.Path, body))
		mu.Unlock()
		_, _ = w.Write([]byte(`{"rot_id": 7}`))
	}))
	t.Cleanup(ts.Close)
	return NewAPIClient(ts.URL, "id", "key"), func() []string { return calls }
}

func TestParseRotationID(t *testing.T) {
	team, id, err := parseRotationID("team-abc/42")
	if err != nil || team != "team-abc" || id != 42 {
		t.Fatalf("got %q %d %v", team, id, err)
	}
	for _, bad := range []string{"", "team-abc", "team-abc/x", "/42"} {
		if _, _, err := parseRotationID(bad); err == nil {
			t.Errorf("%q: expected error", bad)
		}
	}
}

func TestRotationGroupItemGroupID(t *testing.T) {
	var items []RotationGroupItem
	if err := json.Unmarshal([]byte(`[{"slug":"rtg-1","groupId":5},{"slug":"rtg-2","groupId":"6"}]`), &items); err != nil {
		t.Fatal(err)
	}
	if items[0].GroupID != 5 || items[1].GroupID != 6 {
		t.Fatalf("got %+v", items)
	}
}

func TestRotationShiftRoundTrip(t *testing.T) {
	weekdays := []interface{}{map[string]interface{}{
		"sunday": false, "monday": true, "tuesday": true, "wednesday": true, "thursday": true, "friday": true, "saturday": false,
		"time_range": []interface{}{map[string]interface{}{"start_hour": 9, "start_minute": 0, "end_hour": 17, "end_minute": 30}},
	}}
	in := map[string]interface{}{
		"shift_id": 3, "label": "Days", "timezone": "Australia/Sydney", "start": "2024-01-01T11:00:00+11:00", "duration": 7, "shift_type": "std",
		"usernames": []interface{}{"a", "b"}, "mask": weekdays, "mask2": []interface{}{}, "mask3": []interface{}{},
	}

	got := expandShift(in)
	if got.Mask == nil || !got.Mask.Day.M || got.Mask.Day.Sa || got.Mask2 != nil || !reflect.DeepEqual(got.Usernames, []string{"a", "b"}) {
		t.Fatalf("expand: %+v", got)
	}

	flat := flattenShifts([]RotationShift{{
		RotID: 3, Label: "Days", Timezone: "Australia/Sydney", Start: 1704067200000, Duration: 7, Shifttype: "std",
		Usernames: []string{"a", "b"}, Mask: got.Mask, Mask2: &ShiftMask{},
	}})
	if !reflect.DeepEqual(flat[0], in) {
		t.Fatalf("flatten:\n got %#v\nwant %#v", flat[0], in)
	}
}

func TestValidateShiftStart(t *testing.T) {
	if _, errs := validateShiftStart("2024-01-01T09:00:00+11:00", "start"); len(errs) != 0 {
		t.Errorf("past start rejected: %v", errs)
	}
	if _, errs := validateShiftStart(time.Now().Add(24*time.Hour).Format(time.RFC3339), "start"); len(errs) != 1 {
		t.Errorf("future start accepted")
	}
	if _, errs := validateShiftStart("1704067200000", "start"); len(errs) != 1 {
		t.Errorf("epoch accepted")
	}
}

func TestSameInstant(t *testing.T) {
	if !sameInstant("", "2024-01-01T00:00:00Z", "2024-01-01T11:00:00+11:00", nil) {
		t.Error("same instant not equal")
	}
	if sameInstant("", "2024-01-01T00:00:00Z", "2024-01-02T00:00:00Z", nil) {
		t.Error("different instants equal")
	}
}

func TestFormatShiftStart(t *testing.T) {
	if got := formatShiftStart(1704067200000, "Australia/Sydney"); got != "2024-01-01T11:00:00+11:00" {
		t.Errorf("got %s", got)
	}
	if got := formatShiftStart(1704067200000, "not/a-zone"); got != "2024-01-01T00:00:00Z" {
		t.Errorf("got %s", got)
	}
}

func TestFlattenMaskEndOfDay(t *testing.T) {
	mask := &ShiftMask{Day: ShiftMaskDay{M: true}, Time: []ShiftMaskRange{{Start: ShiftMaskTime{}, End: ShiftMaskTime{}}}}
	r := flattenMask(mask)[0].(map[string]interface{})["time_range"].([]interface{})[0].(map[string]interface{})
	if r["end_hour"] != 23 || r["end_minute"] != 59 {
		t.Fatalf("got %v", r)
	}
}

func TestSyncMembers(t *testing.T) {
	c, calls := recorder(t)
	if err := syncMembers(c, "team-1", 2, 3, []string{"a", "b", "c"}, []string{"c", "d", "a"}); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"DELETE /api-public/v1/teams/team-1/rotations/2/3/members map[username:b]",
		"POST /api-public/v1/teams/team-1/rotations/2/3/members map[username:d]",
		"PUT /api-public/v1/teams/team-1/rotations/2/3/members map[position:0 username:c]",
		"PUT /api-public/v1/teams/team-1/rotations/2/3/members map[position:1 username:d]",
	}
	if !reflect.DeepEqual(calls(), want) {
		t.Fatalf("got %q", calls())
	}
}

func TestSyncMembersDuplicates(t *testing.T) {
	c, calls := recorder(t)
	if err := syncMembers(c, "t", 1, 1, nil, []string{"a", "a"}); err == nil || len(calls()) != 0 {
		t.Fatalf("expected an error before any call, got %v %q", err, calls())
	}
}

func TestSyncShifts(t *testing.T) {
	c, calls := recorder(t)
	shift := func(id int, label string, users ...interface{}) interface{} {
		return map[string]interface{}{
			"shift_id": id, "label": label, "timezone": "UTC", "start": "2024-01-01T00:00:00Z", "duration": 7, "shift_type": "std",
			"usernames": users, "mask2": []interface{}{}, "mask3": []interface{}{},
			"mask": []interface{}{map[string]interface{}{
				"sunday": false, "monday": true, "tuesday": false, "wednesday": false, "thursday": false, "friday": false, "saturday": false,
				"time_range": []interface{}{map[string]interface{}{"start_hour": 0, "start_minute": 0, "end_hour": 23, "end_minute": 59}},
			}},
		}
	}
	// only the 2nd shift changed
	old := []interface{}{shift(1, "a"), shift(2, "b"), shift(3, "c")}
	new := []interface{}{shift(0, "a"), shift(0, "b2"), shift(0, "c")}
	if err := syncShifts(c, "t", 9, old, new); err != nil {
		t.Fatal(err)
	}
	got := calls()
	if len(got) != 1 || got[0][:len("PUT /api-public/v1/teams/t/rotations/9/2 ")] != "PUT /api-public/v1/teams/t/rotations/9/2 " {
		t.Fatalf("expected a single update of shift 2, got %q", got)
	}

	c, calls = recorder(t)
	if err := syncShifts(c, "t", 9, old[:1], []interface{}{shift(0, "a"), shift(0, "d", "u")}); err != nil {
		t.Fatal(err)
	}
	got = calls()
	if len(got) != 2 || got[0][:5] != "POST " || got[1] != "POST /api-public/v1/teams/t/rotations/9/7/members map[username:u]" {
		t.Fatalf("expected shift create then member add, got %q", got)
	}

	c, calls = recorder(t)
	if err := syncShifts(c, "t", 9, old, old[:1]); err != nil {
		t.Fatal(err)
	}
	if got := calls(); !reflect.DeepEqual(got, []string{
		"DELETE /api-public/v1/teams/t/rotations/9/2 map[]", "DELETE /api-public/v1/teams/t/rotations/9/3 map[]",
	}) {
		t.Fatalf("expected deletes of shifts 2 and 3, got %q", got)
	}
}

func TestAccRotationCreate(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testing in short mode")
	}
	username := os.Getenv("VO_REPLACEMENT_USERNAME")
	if username == "" {
		t.Skip("VO_REPLACEMENT_USERNAME not set")
	}

	tfResourceName := "victorops_rotation.test"
	resource.Test(t, resource.TestCase{
		PreCheck: func() { testAccPreCheck(t) },
		ProviderFactories: map[string]func() (*schema.Provider, error){
			"victorops": func() (*schema.Provider, error) { return testAccProvider, nil },
		},
		CheckDestroy: testAccRotationDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccRotationConfig("tf-acc-rotation", "Primary", username),
				Check: resource.ComposeTestCheckFunc(
					resource.TestMatchResourceAttr(tfResourceName, "slug", regexp.MustCompile(`^rtg-`)),
					resource.TestCheckResourceAttr(tfResourceName, "label", "Primary"),
					resource.TestCheckResourceAttr(tfResourceName, "shift.0.usernames.0", username),
				),
			},
			{
				Config: testAccRotationConfig("tf-acc-rotation", "Renamed", username),
				Check:  resource.TestCheckResourceAttr(tfResourceName, "label", "Renamed"),
			},
			{ResourceName: tfResourceName, ImportState: true, ImportStateVerify: true},
		},
	})
}

func testAccRotationConfig(team, label, username string) string {
	return fmt.Sprintf(`
resource "victorops_team" "test" {
  name = %q
}

resource "victorops_rotation" "test" {
  team_id = victorops_team.test.id
  label   = %q

  shift {
    label      = "Weekdays"
    timezone   = "UTC"
    start      = "2024-01-01T00:00:00Z"
    duration   = 7
    shift_type = "std"
    usernames  = [%q]

    mask {
      monday    = true
      tuesday   = true
      wednesday = true
      thursday  = true
      friday    = true

      time_range {
        start_hour   = 0
        start_minute = 0
        end_hour     = 23
        end_minute   = 59
      }
    }
  }
}
`, team, label, username)
}

func testAccRotationDestroy(s *terraform.State) error {
	config := testAccProvider.Meta().(Config)
	apiClient := NewAPIClient(config.BaseURL, config.APIId, config.APIKey)
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "victorops_rotation" {
			continue
		}
		team, groupID, err := parseRotationID(rs.Primary.ID)
		if err != nil {
			return err
		}
		if g, err := apiClient.GetRotationGroup(team, groupID); err == nil && g != nil {
			return fmt.Errorf("rotation group %s still exists", rs.Primary.ID)
		}
	}
	return nil
}
