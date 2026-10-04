package data

import (
	"encoding/json"
	"strings"
	"testing"
)

func validConfig() *ValveConfig {
	return &ValveConfig{
		SerialNumber: "860470062586809",
		Mode:         ValveModeAuto,
		UTCOffsetMin: 180,
		Schedule:     []ScheduleEntry{{Days: []int{5, 1, 3, 3}, On: "06:00", Off: "06:30"}},
	}
}

func TestValidateAcceptsAndNormalises(t *testing.T) {
	c := validConfig()
	if err := c.Validate(); err != nil {
		t.Fatalf("valid config rejected: %v", err)
	}
	got := c.Schedule[0].Days
	if len(got) != 3 || got[0] != 1 || got[1] != 3 || got[2] != 5 {
		t.Fatalf("days should be sorted and unique, got %v", got)
	}
}

func TestValidateRejects(t *testing.T) {
	cases := map[string]func(c *ValveConfig){
		"unknown mode":      func(c *ValveConfig) { c.Mode = "manual" },
		"empty mode":        func(c *ValveConfig) { c.Mode = "" },
		"offset too high":   func(c *ValveConfig) { c.UTCOffsetMin = 841 },
		"offset too low":    func(c *ValveConfig) { c.UTCOffsetMin = -721 },
		"no days":           func(c *ValveConfig) { c.Schedule[0].Days = nil },
		"day zero":          func(c *ValveConfig) { c.Schedule[0].Days = []int{0} },
		"day eight":         func(c *ValveConfig) { c.Schedule[0].Days = []int{8} },
		"time without zero": func(c *ValveConfig) { c.Schedule[0].On = "6:00" },
		"hour 24":           func(c *ValveConfig) { c.Schedule[0].On = "24:00" },
		"minute 60":         func(c *ValveConfig) { c.Schedule[0].Off = "06:60" },
		"empty time":        func(c *ValveConfig) { c.Schedule[0].Off = "" },
		"seconds in time":   func(c *ValveConfig) { c.Schedule[0].Off = "06:30:00" },
		"on equals off":     func(c *ValveConfig) { c.Schedule[0].Off = c.Schedule[0].On },
		"too many entries": func(c *ValveConfig) {
			c.Schedule = nil
			for i := 0; i < MaxScheduleEntries+1; i++ {
				c.Schedule = append(c.Schedule, ScheduleEntry{Days: []int{1}, On: "06:00", Off: "07:00"})
			}
		},
	}
	for name, mutate := range cases {
		c := validConfig()
		mutate(c)
		if err := c.Validate(); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

func TestValidateAllowsPastMidnightAndManualModes(t *testing.T) {
	c := validConfig()
	c.Schedule = []ScheduleEntry{{Days: []int{5}, On: "22:00", Off: "02:00"}}
	if err := c.Validate(); err != nil {
		t.Fatalf("a window past midnight should be fine: %v", err)
	}
	for _, mode := range []string{ValveModeOn, ValveModeOff} {
		c := validConfig()
		c.Mode = mode
		c.Schedule = nil
		if err := c.Validate(); err != nil {
			t.Errorf("mode %s with no schedule should be fine: %v", mode, err)
		}
	}
}

// The payload is what the firmware parses, so pin its exact shape
func TestDevicePayloadShape(t *testing.T) {
	c := validConfig()
	c.Version = 7
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	b, err := c.DevicePayload()
	if err != nil {
		t.Fatal(err)
	}
	want := `{"version":7,"mode":"auto","utc_offset_min":180,"schedule":[{"days":[1,3,5],"on":"06:00","off":"06:30"}]}`
	if string(b) != want {
		t.Fatalf("payload\n got %s\nwant %s", b, want)
	}

	empty := &ValveConfig{Mode: ValveModeOff, Version: 2}
	b, _ = empty.DevicePayload()
	if !strings.Contains(string(b), `"schedule":[]`) {
		t.Fatalf("an empty schedule must be sent as [] not null: %s", b)
	}
	var back map[string]interface{}
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
}
