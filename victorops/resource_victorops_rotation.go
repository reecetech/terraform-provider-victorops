package victorops

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

var maskDays = []string{"sunday", "monday", "tuesday", "wednesday", "thursday", "friday", "saturday"}

func maskSchema(description string, required bool) *schema.Schema {
	days := map[string]*schema.Schema{}
	for _, d := range maskDays {
		days[d] = &schema.Schema{
			Type:        schema.TypeBool,
			Optional:    true,
			Description: "Whether the mask applies on this day.",
		}
	}
	days["time_range"] = &schema.Schema{
		Type:        schema.TypeList,
		Required:    true,
		MinItems:    1,
		Description: "Times of day the mask is on call. Required by the API; use 00:00-23:59 for all day. An end of 00:00 is read back as 23:59, so use 23:59 for end of day.",
		Elem: &schema.Resource{Schema: map[string]*schema.Schema{
			"start_hour":   {Type: schema.TypeInt, Required: true, ValidateFunc: validation.IntBetween(0, 23)},
			"start_minute": {Type: schema.TypeInt, Required: true, ValidateFunc: validation.IntBetween(0, 59)},
			"end_hour":     {Type: schema.TypeInt, Required: true, ValidateFunc: validation.IntBetween(0, 23)},
			"end_minute":   {Type: schema.TypeInt, Required: true, ValidateFunc: validation.IntBetween(0, 59)},
		}},
	}
	s := &schema.Schema{
		Type:        schema.TypeList,
		MaxItems:    1,
		Description: description,
		Elem:        &schema.Resource{Schema: days},
	}
	if required {
		s.Required = true
	} else {
		s.Optional = true
	}
	return s
}

func resourceRotation() *schema.Resource {
	return &schema.Resource{
		Description:   "Manages a rotation group and its shifts. The group's `slug` is what escalation policy `rotationGroup` entries refer to.",
		CreateContext: resourceRotationCreate,
		ReadContext:   resourceRotationRead,
		UpdateContext: resourceRotationUpdate,
		DeleteContext: resourceRotationDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			"team_id": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "The slug/ID of the team that owns the rotation group.",
			},
			"label": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The name of the rotation group.",
			},
			"slug": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The rotation group slug (`rtg-...`), used in escalation policy entries.",
			},
			"group_id": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "The numeric rotation group ID.",
			},
			"shift": {
				Type:        schema.TypeList,
				Optional:    true,
				Description: "The shifts in the rotation group, in order. Shifts are matched to existing ones by position.",
				Elem: &schema.Resource{Schema: map[string]*schema.Schema{
					"shift_id": {
						Type:        schema.TypeInt,
						Computed:    true,
						Description: "The numeric shift ID.",
					},
					"label": {
						Type:        schema.TypeString,
						Required:    true,
						Description: "The name of the shift.",
					},
					"timezone": {
						Type:         schema.TypeString,
						Required:     true,
						ValidateFunc: validation.StringInSlice(TimeZoneList(), false),
						Description:  "The shift's timezone (e.g. 'Australia/Sydney').",
					},
					"start": {
						Type:             schema.TypeString,
						Required:         true,
						ValidateFunc:     validateShiftStart,
						DiffSuppressFunc: sameInstant,
						Description:      "When the shift began, as an RFC 3339 timestamp (e.g. '2024-01-01T09:00:00+11:00'). Each person's turn is counted from here. Must not be in the future: the API answers 500.",
					},
					"duration": {
						Type:         schema.TypeInt,
						Required:     true,
						ValidateFunc: validation.IntBetween(1, 90),
						Description:  "The length of each person's turn in days (90 or less).",
					},
					"shift_type": {
						Type:         schema.TypeString,
						Required:     true,
						ValidateFunc: validation.StringInSlice([]string{"std", "fts", "cstm", "pho"}, false),
						Description:  "One of `std` (Standard), `fts` (Follow The Sun), `cstm` (Custom), `pho` (Period Hand Off).",
					},
					"usernames": {
						Type:        schema.TypeList,
						Optional:    true,
						Description: "The members of the shift, in rotation order.",
						Elem:        &schema.Schema{Type: schema.TypeString},
					},
					"mask":  maskSchema("When the shift is on call.", true),
					"mask2": maskSchema("A second on-call window, for `fts` and `cstm` shifts.", false),
					"mask3": maskSchema("A third on-call window, for `cstm` shifts.", false),
				}},
			},
		},
	}
}

// validateShiftStart rejects unparseable and future starts, which the API fails with a 500
func validateShiftStart(v interface{}, key string) ([]string, []error) {
	t, err := time.Parse(time.RFC3339, v.(string))
	if err != nil {
		return nil, []error{fmt.Errorf("%s must be an RFC 3339 timestamp such as 2024-01-01T09:00:00+11:00: %s", key, err)}
	}
	if t.After(time.Now()) {
		return nil, []error{fmt.Errorf("%s must not be in the future (got %s); VictorOps returns a 500 for it", key, v)}
	}
	return nil, nil
}

// sameInstant treats two spellings of one moment (2024-01-01T00:00:00Z and ...+11:00) as equal
func sameInstant(_, old, new string, _ *schema.ResourceData) bool {
	o, err1 := time.Parse(time.RFC3339, old)
	n, err2 := time.Parse(time.RFC3339, new)
	return err1 == nil && err2 == nil && o.Equal(n)
}

// formatShiftStart renders epoch milliseconds in the shift's timezone
func formatShiftStart(ms int64, timezone string) string {
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		loc = time.UTC
	}
	layout := time.RFC3339
	if ms%1000 != 0 {
		layout = time.RFC3339Nano
	}
	return time.UnixMilli(ms).In(loc).Format(layout)
}

func parseRotationID(id string) (team string, groupID int, err error) {
	parts := strings.SplitN(id, "/", 2)
	if len(parts) == 2 {
		groupID, err = strconv.Atoi(parts[1])
	}
	if len(parts) != 2 || parts[0] == "" || err != nil {
		return "", 0, fmt.Errorf("invalid id %q specified, should be in format \"team_id/group_id\"", id)
	}
	return parts[0], groupID, nil
}

func expandMask(raw []interface{}) *ShiftMask {
	if len(raw) == 0 || raw[0] == nil {
		return nil
	}
	m := raw[0].(map[string]interface{})
	mask := &ShiftMask{Day: ShiftMaskDay{
		Su: m["sunday"].(bool), M: m["monday"].(bool), T: m["tuesday"].(bool), W: m["wednesday"].(bool),
		Th: m["thursday"].(bool), F: m["friday"].(bool), Sa: m["saturday"].(bool),
	}}
	for _, r := range m["time_range"].([]interface{}) {
		t := r.(map[string]interface{})
		mask.Time = append(mask.Time, ShiftMaskRange{
			Start: ShiftMaskTime{Hour: float64(t["start_hour"].(int)), Minute: float64(t["start_minute"].(int))},
			End:   ShiftMaskTime{Hour: float64(t["end_hour"].(int)), Minute: float64(t["end_minute"].(int))},
		})
	}
	return mask
}

func flattenMask(m *ShiftMask) []interface{} {
	d := m.dayMap()
	if m == nil || (len(m.Time) == 0 && !anyTrue(d)) {
		return []interface{}{}
	}
	out := map[string]interface{}{}
	for k, v := range d {
		out[k] = v
	}
	ranges := make([]interface{}, len(m.Time))
	for i, t := range m.Time {
		endHour, endMinute := int(t.End.Hour), int(t.End.Minute)
		if endHour == 0 && endMinute == 0 {
			endHour, endMinute = 23, 59 // the API stores an end of 23:59 as 00:00 (end of day)
		}
		ranges[i] = map[string]interface{}{
			"start_hour": int(t.Start.Hour), "start_minute": int(t.Start.Minute),
			"end_hour": endHour, "end_minute": endMinute,
		}
	}
	out["time_range"] = ranges
	return []interface{}{out}
}

func (m *ShiftMask) dayMap() map[string]bool {
	if m == nil {
		return map[string]bool{}
	}
	d := m.Day
	return map[string]bool{
		"sunday": d.Su, "monday": d.M, "tuesday": d.T, "wednesday": d.W, "thursday": d.Th, "friday": d.F, "saturday": d.Sa,
	}
}

func anyTrue(m map[string]bool) bool {
	for _, v := range m {
		if v {
			return true
		}
	}
	return false
}

func expandUsernames(raw interface{}) []string {
	var out []string
	for _, u := range raw.([]interface{}) {
		out = append(out, u.(string))
	}
	return out
}

func expandShift(m map[string]interface{}) RotationShiftPayload {
	start, _ := time.Parse(time.RFC3339, m["start"].(string)) // checked by validateShiftStart
	return RotationShiftPayload{
		Label:     m["label"].(string),
		Timezone:  m["timezone"].(string),
		Start:     start.UnixMilli(),
		Duration:  m["duration"].(int),
		Shifttype: m["shift_type"].(string),
		Mask:      expandMask(m["mask"].([]interface{})),
		Mask2:     expandMask(m["mask2"].([]interface{})),
		Mask3:     expandMask(m["mask3"].([]interface{})),
		Usernames: expandUsernames(m["usernames"]),
	}
}

func expandShifts(raw []interface{}) []RotationShiftPayload {
	out := make([]RotationShiftPayload, len(raw))
	for i, s := range raw {
		out[i] = expandShift(s.(map[string]interface{}))
	}
	return out
}

func flattenShifts(shifts []RotationShift) []interface{} {
	out := make([]interface{}, len(shifts))
	for i, s := range shifts {
		usernames := make([]interface{}, len(s.Usernames))
		for j, u := range s.Usernames {
			usernames[j] = u
		}
		out[i] = map[string]interface{}{
			"shift_id":   s.RotID,
			"label":      s.Label,
			"timezone":   s.Timezone,
			"start":      formatShiftStart(s.Start, s.Timezone),
			"duration":   s.Duration,
			"shift_type": s.Shifttype,
			"usernames":  usernames,
			"mask":       flattenMask(s.Mask),
			"mask2":      flattenMask(s.Mask2),
			"mask3":      flattenMask(s.Mask3),
		}
	}
	return out
}

func resourceRotationCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	config := m.(Config)
	apiClient := NewAPIClient(config.BaseURL, config.APIId, config.APIKey)
	team := d.Get("team_id").(string)

	// Shifts and members go through the update path, so create and update behave alike
	group, err := apiClient.CreateRotationGroup(team, d.Get("label").(string), nil)
	if err != nil {
		return diag.FromErr(err)
	}
	if group.ID == 0 {
		return diag.Errorf("VictorOps returned a rotation group with no ID")
	}

	// Set early so a failed shift leaves the group in state (tainted) instead of orphaned
	d.SetId(fmt.Sprintf("%s/%d", team, group.ID))
	if err := syncShifts(apiClient, team, group.ID, nil, d.Get("shift").([]interface{})); err != nil {
		return diag.FromErr(err)
	}
	return resourceRotationRead(ctx, d, m)
}

func resourceRotationRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	config := m.(Config)
	apiClient := NewAPIClient(config.BaseURL, config.APIId, config.APIKey)

	team, groupID, err := parseRotationID(d.Id())
	if err != nil {
		return diag.FromErr(err)
	}

	group, err := apiClient.GetRotationGroup(team, groupID)
	if err != nil {
		return diag.FromErr(err)
	}
	if group == nil {
		d.SetId("")
		return nil
	}

	// The group endpoint doesn't return the slug
	groups, err := apiClient.ListRotationGroups(team)
	if err != nil {
		return diag.FromErr(err)
	}
	slug := ""
	for _, g := range groups {
		if int(g.GroupID) == groupID {
			slug = g.Slug
		}
	}

	for k, v := range map[string]interface{}{
		"team_id":  team,
		"label":    group.Label,
		"group_id": groupID,
		"slug":     slug,
		"shift":    flattenShifts(group.Shifts),
	} {
		if err := d.Set(k, v); err != nil {
			return diag.FromErr(err)
		}
	}
	return nil
}

func resourceRotationUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	config := m.(Config)
	apiClient := NewAPIClient(config.BaseURL, config.APIId, config.APIKey)

	team, groupID, err := parseRotationID(d.Id())
	if err != nil {
		return diag.FromErr(err)
	}

	if d.HasChange("label") {
		if err := apiClient.UpdateRotationGroupLabel(team, groupID, d.Get("label").(string)); err != nil {
			return diag.FromErr(err)
		}
	}
	if d.HasChange("shift") {
		old, new := d.GetChange("shift")
		if err := syncShifts(apiClient, team, groupID, old.([]interface{}), new.([]interface{})); err != nil {
			return diag.FromErr(err)
		}
	}
	return resourceRotationRead(ctx, d, m)
}

func resourceRotationDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	config := m.(Config)
	apiClient := NewAPIClient(config.BaseURL, config.APIId, config.APIKey)

	team, groupID, err := parseRotationID(d.Id())
	if err != nil {
		return diag.FromErr(err)
	}
	if err := apiClient.DeleteRotationGroup(team, groupID); err != nil {
		return diag.FromErr(err)
	}
	d.SetId("")
	return nil
}

// shiftSettings is a shift's payload without its members, for comparison
func shiftSettings(s RotationShiftPayload) string {
	s.Usernames = nil
	b, _ := json.Marshal(s)
	return string(b)
}

// syncShifts matches shifts by position: updates existing ones, adds new ones, deletes the extra old ones
func syncShifts(c *APIClient, team string, groupID int, old, new []interface{}) error {
	for i, raw := range new {
		n := raw.(map[string]interface{})
		want := expandShift(n)

		if i >= len(old) {
			created := want
			created.Usernames = nil // ignored here; members are added below
			s, err := c.CreateRotationShift(team, groupID, created)
			if err != nil {
				return err
			}
			if s.RotID == 0 {
				return fmt.Errorf("VictorOps returned shift %q with no ID", want.Label)
			}
			if err := syncMembers(c, team, groupID, s.RotID, nil, want.Usernames); err != nil {
				return err
			}
			continue
		}

		o := old[i].(map[string]interface{})
		shiftID := o["shift_id"].(int)
		have := expandShift(o)
		if shiftSettings(have) != shiftSettings(want) {
			if err := c.UpdateRotationShift(team, groupID, shiftID, want); err != nil {
				return err
			}
		}
		if err := syncMembers(c, team, groupID, shiftID, have.Usernames, want.Usernames); err != nil {
			return err
		}
	}

	for _, raw := range old[min(len(old), len(new)):] {
		if err := c.DeleteRotationShift(team, groupID, raw.(map[string]interface{})["shift_id"].(int)); err != nil {
			return err
		}
	}
	return nil
}

// syncMembers removes members that are gone, appends new ones, then moves members into the wanted order
func syncMembers(c *APIClient, team string, groupID, shiftID int, have, want []string) error {
	wanted := map[string]bool{}
	for _, u := range want {
		wanted[u] = true
	}
	if len(wanted) != len(want) {
		return fmt.Errorf("usernames must be unique within a shift: %v", want)
	}
	current := []string{}
	for _, u := range have {
		if wanted[u] {
			current = append(current, u)
			continue
		}
		if err := c.RemoveRotationShiftMember(team, groupID, shiftID, u); err != nil {
			return err
		}
	}

	present := map[string]bool{}
	for _, u := range current {
		present[u] = true
	}
	for _, u := range want {
		if present[u] {
			continue
		}
		if err := c.AddRotationShiftMember(team, groupID, shiftID, u); err != nil {
			return err
		}
		current = append(current, u)
	}

	for i, u := range want {
		if current[i] == u {
			continue
		}
		if err := c.MoveRotationShiftMember(team, groupID, shiftID, u, i); err != nil {
			return err
		}
		rest := []string{}
		for _, name := range current {
			if name != u {
				rest = append(rest, name)
			}
		}
		current = append(rest[:i], append([]string{u}, rest[i:]...)...)
	}
	return nil
}
