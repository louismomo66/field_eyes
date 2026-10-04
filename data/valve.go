package data

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"time"

	"gorm.io/gorm"
)

// Valve modes. "auto" follows the schedule; "on"/"off" are the manual override.
const (
	ValveModeAuto = "auto"
	ValveModeOn   = "on"
	ValveModeOff  = "off"

	MaxScheduleEntries = 8
)

var hhmm = regexp.MustCompile(`^([01][0-9]|2[0-3]):[0-5][0-9]$`)

// ScheduleEntry opens the valve at On and closes it at Off (local time) on the given
// days, 1 = Monday ... 7 = Sunday. If Off is earlier than On the window runs past midnight.
type ScheduleEntry struct {
	Days []int  `json:"days"`
	On   string `json:"on"`
	Off  string `json:"off"`
}

// ValveConfig is what a user sets in the dashboard for one valve device. It is published
// (retained) to the device, which keeps it and runs the schedule on its own clock.
type ValveConfig struct {
	ID           uint            `gorm:"primaryKey" json:"id"`
	SerialNumber string          `gorm:"type:varchar(100);uniqueIndex;not null" json:"serial_number"`
	Mode         string          `gorm:"type:varchar(10);not null;default:auto" json:"mode"`
	UTCOffsetMin int             `json:"utc_offset_min"` // the schedule's local time = UTC + this
	Schedule     []ScheduleEntry `gorm:"serializer:json;type:text" json:"schedule"`
	Version      uint            `json:"version"` // bumped on every save
	UpdatedByID  uint            `json:"updated_by_id"`

	// Reported back by the device
	ReportedState  string     `gorm:"type:varchar(10)" json:"reported_state"` // OPEN, CLOSED or UNKNOWN
	ReportedMode   string     `gorm:"type:varchar(10)" json:"reported_mode"`
	AppliedVersion uint       `json:"applied_version"` // config version the device is running
	ReportedAt     *time.Time `json:"reported_at"`     // when the report reached the server
	ChangedAt      *time.Time `json:"changed_at"`      // when the device last moved the valve

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Validate checks the config and puts it in canonical form (days sorted and unique,
// schedule never nil so it is sent as []).
func (c *ValveConfig) Validate() error {
	switch c.Mode {
	case ValveModeAuto, ValveModeOn, ValveModeOff:
	default:
		return fmt.Errorf("mode must be %q, %q or %q", ValveModeAuto, ValveModeOn, ValveModeOff)
	}
	if c.UTCOffsetMin < -720 || c.UTCOffsetMin > 840 {
		return errors.New("utc_offset_min must be between -720 and 840")
	}
	if len(c.Schedule) > MaxScheduleEntries {
		return fmt.Errorf("a schedule can have at most %d entries", MaxScheduleEntries)
	}
	if c.Schedule == nil {
		c.Schedule = []ScheduleEntry{}
	}
	for i := range c.Schedule {
		e := &c.Schedule[i]
		if len(e.Days) == 0 {
			return fmt.Errorf("schedule entry %d: choose at least one day", i+1)
		}
		seen := map[int]bool{}
		days := make([]int, 0, len(e.Days))
		for _, d := range e.Days {
			if d < 1 || d > 7 {
				return fmt.Errorf("schedule entry %d: days must be 1 (Monday) to 7 (Sunday)", i+1)
			}
			if !seen[d] {
				seen[d] = true
				days = append(days, d)
			}
		}
		sort.Ints(days)
		e.Days = days
		if !hhmm.MatchString(e.On) || !hhmm.MatchString(e.Off) {
			return fmt.Errorf("schedule entry %d: times must look like 06:30", i+1)
		}
		if e.On == e.Off {
			return fmt.Errorf("schedule entry %d: the on and off times must differ", i+1)
		}
	}
	return nil
}

// DevicePayload is the message the device receives on <root>/<serial>/valve/config.
func (c *ValveConfig) DevicePayload() ([]byte, error) {
	schedule := c.Schedule
	if schedule == nil {
		schedule = []ScheduleEntry{}
	}
	return json.Marshal(struct {
		Version      uint            `json:"version"`
		Mode         string          `json:"mode"`
		UTCOffsetMin int             `json:"utc_offset_min"`
		Schedule     []ScheduleEntry `json:"schedule"`
	}{c.Version, c.Mode, c.UTCOffsetMin, schedule})
}

// ValveConfigInterface stores valve configs.
type ValveConfigInterface interface {
	// Get returns nil, nil when the device has no config yet.
	Get(serialNumber string) (*ValveConfig, error)
	// Save creates or updates the config for cfg.SerialNumber and bumps its version.
	Save(cfg *ValveConfig) error
	// UpdateReport records what the device reported. Unknown devices are ignored.
	UpdateReport(serialNumber, state, mode string, appliedVersion uint, reportedAt time.Time, changedAt *time.Time) error
	Delete(serialNumber string) error
}

type ValveConfigRepository struct {
	db *gorm.DB
}

func NewValveConfigRepository(db *gorm.DB) ValveConfigInterface {
	return &ValveConfigRepository{db: db}
}

func (r *ValveConfigRepository) Get(serialNumber string) (*ValveConfig, error) {
	var cfg ValveConfig
	err := r.db.Where("serial_number = ?", serialNumber).First(&cfg).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (r *ValveConfigRepository) Save(cfg *ValveConfig) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var existing ValveConfig
		err := tx.Where("serial_number = ?", cfg.SerialNumber).First(&existing).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			cfg.ID = 0
			cfg.Version = 1
			return tx.Create(cfg).Error
		}
		if err != nil {
			return err
		}
		existing.Mode = cfg.Mode
		existing.UTCOffsetMin = cfg.UTCOffsetMin
		existing.Schedule = cfg.Schedule
		existing.UpdatedByID = cfg.UpdatedByID
		existing.Version++
		if err := tx.Save(&existing).Error; err != nil {
			return err
		}
		*cfg = existing
		return nil
	})
}

func (r *ValveConfigRepository) UpdateReport(serialNumber, state, mode string, appliedVersion uint, reportedAt time.Time, changedAt *time.Time) error {
	return r.db.Model(&ValveConfig{}).Where("serial_number = ?", serialNumber).Updates(map[string]interface{}{
		"reported_state":  state,
		"reported_mode":   mode,
		"applied_version": appliedVersion,
		"reported_at":     reportedAt,
		"changed_at":      changedAt,
	}).Error
}

func (r *ValveConfigRepository) Delete(serialNumber string) error {
	return r.db.Where("serial_number = ?", serialNumber).Delete(&ValveConfig{}).Error
}
