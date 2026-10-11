package models

import "testing"

func TestNormalizeTaskTargetAliases(t *testing.T) {
	for _, tc := range []struct {
		legacy, target, want string
		refused              bool
	}{
		{"", "", "", false}, {" ", " ", "", false}, {" a ", "", "a", false}, {"", " a ", "a", false},
		{" a ", "a", "a", false}, {"a", "b", "", true},
	} {
		req := RunSkillRequest{DeviceID: tc.legacy, TargetDeviceID: tc.target}
		err := req.NormalizeTargetDevice()
		if (err != nil) != tc.refused || err == nil && req.DeviceID != tc.want {
			t.Fatalf("%+v: target=%q error=%v", tc, req.DeviceID, err)
		}
	}
}
