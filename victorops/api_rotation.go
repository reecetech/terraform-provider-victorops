package victorops

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// flexInt decodes a JSON number or numeric string; the API documents groupId as a string but returns an integer
type flexInt int

func (f *flexInt) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		n, err := strconv.Atoi(s)
		if err != nil {
			return err
		}
		*f = flexInt(n)
		return nil
	}
	var n int
	if err := json.Unmarshal(b, &n); err != nil {
		return err
	}
	*f = flexInt(n)
	return nil
}

// RotationGroupItem is one entry of the list rotation groups response
type RotationGroupItem struct {
	TeamSlug string  `json:"teamSlug"`
	Slug     string  `json:"slug"`
	Label    string  `json:"label"`
	GroupID  flexInt `json:"groupId"`
}

// ShiftMask is the days and time ranges a shift is on call
type ShiftMask struct {
	Day  ShiftMaskDay     `json:"day"`
	Time []ShiftMaskRange `json:"time,omitempty"`
}

// ShiftMaskDay flags the days of the week a mask applies to
type ShiftMaskDay struct {
	Su bool `json:"su"`
	M  bool `json:"m"`
	T  bool `json:"t"`
	W  bool `json:"w"`
	Th bool `json:"th"`
	F  bool `json:"f"`
	Sa bool `json:"sa"`
}

// ShiftMaskRange is a start/end time of day
type ShiftMaskRange struct {
	Start ShiftMaskTime `json:"start"`
	End   ShiftMaskTime `json:"end"`
}

// ShiftMaskTime is a time of day
type ShiftMaskTime struct {
	Hour   float64 `json:"hour"`
	Minute float64 `json:"minute"`
}

// RotationShiftPayload is the request body for creating or updating a shift
type RotationShiftPayload struct {
	Label     string     `json:"label"`
	Timezone  string     `json:"timezone"`
	Start     int64      `json:"start"`
	Duration  int        `json:"duration"`
	Shifttype string     `json:"shifttype"`
	Mask      *ShiftMask `json:"mask"`
	Mask2     *ShiftMask `json:"mask2,omitempty"`
	Mask3     *ShiftMask `json:"mask3,omitempty"`
	Usernames []string   `json:"usernames,omitempty"` // only honoured when creating the group
}

// RotationShift is a shift as returned by the API
type RotationShift struct {
	RotID     int        `json:"rot_id"`
	Label     string     `json:"label"`
	Timezone  string     `json:"timezone"`
	Start     int64      `json:"start"`
	Duration  int        `json:"duration"`
	Shifttype string     `json:"shifttype"`
	Mask      *ShiftMask `json:"mask"`
	Mask2     *ShiftMask `json:"mask2"`
	Mask3     *ShiftMask `json:"mask3"`
	Usernames []string   `json:"usernames"`
}

// RotationGroup is a rotation group as returned by create and get
type RotationGroup struct {
	ID     int             `json:"id"`
	Label  string          `json:"label"`
	Shifts []RotationShift `json:"shifts"`
}

type rotationGroupCreateRequest struct {
	Label  string                 `json:"label"`
	Shifts []RotationShiftPayload `json:"shifts,omitempty"`
}

func rotationsPath(team string) string {
	return fmt.Sprintf("/api-public/v1/teams/%s/rotations", team)
}

func rotationGroupPath(team string, groupID int) string {
	return fmt.Sprintf("%s/%d", rotationsPath(team), groupID)
}

func rotationShiftPath(team string, groupID, shiftID int) string {
	return fmt.Sprintf("%s/%d", rotationGroupPath(team, groupID), shiftID)
}

// call sends the request and decodes the response into out (if not nil). A 404 returns found=false.
func (c *APIClient) call(method, path string, body, out interface{}) (found bool, err error) {
	if err := WaitForRateLimitGlobal(); err != nil {
		return false, err
	}
	respBody, status, err := c.doRequest(method, path, body)
	if err != nil {
		return false, err
	}
	if status == 404 {
		return false, nil
	}
	if status != 200 && status != 204 {
		sent, _ := json.Marshal(body)
		return false, fmt.Errorf("%s %s: API error (%d): %s (request: %s)", method, path, status, string(respBody), sent)
	}
	if out != nil {
		if err := json.Unmarshal(respBody, out); err != nil {
			return false, fmt.Errorf("%s %s: %w", method, path, err)
		}
	}
	return true, nil
}

// ListRotationGroups lists a team's rotation groups, including their rtg- slugs
func (c *APIClient) ListRotationGroups(team string) ([]RotationGroupItem, error) {
	var out struct {
		RotationGroups []RotationGroupItem `json:"rotationGroups"`
	}
	if _, err := c.call("GET", rotationsPath(team), nil, &out); err != nil {
		return nil, err
	}
	return out.RotationGroups, nil
}

// GetRotationGroup gets a group and its shifts; nil if it no longer exists
func (c *APIClient) GetRotationGroup(team string, groupID int) (*RotationGroup, error) {
	var out RotationGroup
	found, err := c.call("GET", rotationGroupPath(team, groupID), nil, &out)
	if err != nil || !found {
		return nil, err
	}
	return &out, nil
}

// CreateRotationGroup creates a group, with its shifts and their members
func (c *APIClient) CreateRotationGroup(team, label string, shifts []RotationShiftPayload) (*RotationGroup, error) {
	var out RotationGroup
	if _, err := c.call("POST", rotationsPath(team), rotationGroupCreateRequest{Label: label, Shifts: shifts}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateRotationGroupLabel renames a group
func (c *APIClient) UpdateRotationGroupLabel(team string, groupID int, label string) error {
	_, err := c.call("PUT", rotationGroupPath(team, groupID), map[string]string{"label": label}, nil)
	return err
}

// DeleteRotationGroup deletes a group
func (c *APIClient) DeleteRotationGroup(team string, groupID int) error {
	_, err := c.call("DELETE", rotationGroupPath(team, groupID), nil, nil)
	return err
}

// CreateRotationShift adds a shift to a group; members are added separately
func (c *APIClient) CreateRotationShift(team string, groupID int, s RotationShiftPayload) (*RotationShift, error) {
	var out RotationShift
	if _, err := c.call("POST", rotationGroupPath(team, groupID), s, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateRotationShift updates a shift in place; members are updated separately
func (c *APIClient) UpdateRotationShift(team string, groupID, shiftID int, s RotationShiftPayload) error {
	_, err := c.call("PUT", rotationShiftPath(team, groupID, shiftID), s, nil)
	return err
}

// DeleteRotationShift deletes a shift
func (c *APIClient) DeleteRotationShift(team string, groupID, shiftID int) error {
	_, err := c.call("DELETE", rotationShiftPath(team, groupID, shiftID), nil, nil)
	return err
}

// AddRotationShiftMember appends a member to a shift
func (c *APIClient) AddRotationShiftMember(team string, groupID, shiftID int, username string) error {
	_, err := c.call("POST", rotationShiftPath(team, groupID, shiftID)+"/members", map[string]string{"username": username}, nil)
	return err
}

// RemoveRotationShiftMember removes a member from a shift
func (c *APIClient) RemoveRotationShiftMember(team string, groupID, shiftID int, username string) error {
	_, err := c.call("DELETE", rotationShiftPath(team, groupID, shiftID)+"/members", map[string]string{"username": username}, nil)
	return err
}

// MoveRotationShiftMember sets a member's position in a shift
func (c *APIClient) MoveRotationShiftMember(team string, groupID, shiftID int, username string, position int) error {
	_, err := c.call("PUT", rotationShiftPath(team, groupID, shiftID)+"/members", map[string]interface{}{"username": username, "position": position}, nil)
	return err
}
