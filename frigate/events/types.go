package events

// MQTT BEGIN
type MQTTFrigateEvent struct {
	Type   string `json:"type,omitempty"`
	Before Before `json:"before,omitempty"`
	After  After  `json:"after,omitempty"`
}
type Snapshot struct {
	FrameTime  float64 `json:"frame_time,omitempty"`
	Box        []int   `json:"box,omitempty"`
	Area       int     `json:"area,omitempty"`
	Region     []int   `json:"region,omitempty"`
	Score      float64 `json:"score,omitempty"`
	Attributes []any   `json:"attributes,omitempty"`
}
type Attributes struct {
	Face float64 `json:"face,omitempty"`
}
type Before struct {
	ID                          string     `json:"id,omitempty"`
	Camera                      string     `json:"camera,omitempty"`
	FrameTime                   float64    `json:"frame_time,omitempty"`
	Snapshot                    Snapshot   `json:"snapshot,omitempty"`
	Label                       string     `json:"label,omitempty"`
	SubLabel                    any        `json:"sub_label,omitempty"`
	TopScore                    float64    `json:"top_score,omitempty"`
	FalsePositive               bool       `json:"false_positive,omitempty"`
	StartTime                   float64    `json:"start_time,omitempty"`
	EndTime                     any        `json:"end_time,omitempty"`
	Score                       float64    `json:"score,omitempty"`
	Box                         []int      `json:"box,omitempty"`
	Area                        int        `json:"area,omitempty"`
	Ratio                       float64    `json:"ratio,omitempty"`
	Region                      []int      `json:"region,omitempty"`
	CurrentZones                []string   `json:"current_zones,omitempty"`
	EnteredZones                []string   `json:"entered_zones,omitempty"`
	Thumbnail                   any        `json:"thumbnail,omitempty"`
	HasSnapshot                 bool       `json:"has_snapshot,omitempty"`
	HasClip                     bool       `json:"has_clip,omitempty"`
	Active                      bool       `json:"active,omitempty"`
	Stationary                  bool       `json:"stationary,omitempty"`
	MotionlessCount             int        `json:"motionless_count,omitempty"`
	PositionChanges             int        `json:"position_changes,omitempty"`
	Attributes                  Attributes `json:"attributes,omitempty"`
	CurrentAttributes           []any      `json:"current_attributes,omitempty"`
	CurrentEstimatedSpeed       float64    `json:"current_estimated_speed,omitempty"`
	AverageEstimatedSpeed       float64    `json:"average_estimated_speed,omitempty"`
	VelocityAngle               int        `json:"velocity_angle,omitempty"`
	RecognizedLicensePlate      string     `json:"recognized_license_plate,omitempty"`
	RecognizedLicensePlateScore float64    `json:"recognized_license_plate_score,omitempty"`
}
type CurrentAttributes struct {
	Label string  `json:"label,omitempty"`
	Box   []int   `json:"box,omitempty"`
	Score float64 `json:"score,omitempty"`
}
type After struct {
	ID                          string              `json:"id,omitempty"`
	Camera                      string              `json:"camera,omitempty"`
	FrameTime                   float64             `json:"frame_time,omitempty"`
	Snapshot                    Snapshot            `json:"snapshot,omitempty"`
	Label                       string              `json:"label,omitempty"`
	SubLabel                    []any               `json:"sub_label,omitempty"`
	TopScore                    float64             `json:"top_score,omitempty"`
	FalsePositive               bool                `json:"false_positive,omitempty"`
	StartTime                   float64             `json:"start_time,omitempty"`
	EndTime                     any                 `json:"end_time,omitempty"`
	Score                       float64             `json:"score,omitempty"`
	Box                         []int               `json:"box,omitempty"`
	Area                        int                 `json:"area,omitempty"`
	Ratio                       float64             `json:"ratio,omitempty"`
	Region                      []int               `json:"region,omitempty"`
	CurrentZones                []string            `json:"current_zones,omitempty"`
	EnteredZones                []string            `json:"entered_zones,omitempty"`
	Thumbnail                   any                 `json:"thumbnail,omitempty"`
	HasSnapshot                 bool                `json:"has_snapshot,omitempty"`
	HasClip                     bool                `json:"has_clip,omitempty"`
	Active                      bool                `json:"active,omitempty"`
	Stationary                  bool                `json:"stationary,omitempty"`
	MotionlessCount             int                 `json:"motionless_count,omitempty"`
	PositionChanges             int                 `json:"position_changes,omitempty"`
	Attributes                  Attributes          `json:"attributes,omitempty"`
	CurrentAttributes           []CurrentAttributes `json:"current_attributes,omitempty"`
	CurrentEstimatedSpeed       float64             `json:"current_estimated_speed,omitempty"`
	AverageEstimatedSpeed       float64             `json:"average_estimated_speed,omitempty"`
	VelocityAngle               int                 `json:"velocity_angle,omitempty"`
	RecognizedLicensePlate      string              `json:"recognized_license_plate,omitempty"`
	RecognizedLicensePlateScore float64             `json:"recognized_license_plate_score,omitempty"`
}

// MQTT END

// API :EVENT_ID
type FriateEventID struct {
	ID                 string      `json:"id,omitempty"`
	Label              string      `json:"label,omitempty"`
	SubLabel           string      `json:"sub_label,omitempty"`
	Camera             string      `json:"camera,omitempty"`
	StartTime          int         `json:"start_time,omitempty"`
	EndTime            int         `json:"end_time,omitempty"`
	FalsePositive      bool        `json:"false_positive,omitempty"`
	Zones              []string    `json:"zones,omitempty"`
	Thumbnail          string      `json:"thumbnail,omitempty"`
	HasClip            bool        `json:"has_clip,omitempty"`
	HasSnapshot        bool        `json:"has_snapshot,omitempty"`
	RetainIndefinitely bool        `json:"retain_indefinitely,omitempty"`
	PlusID             string      `json:"plus_id,omitempty"`
	ModelHash          string      `json:"model_hash,omitempty"`
	DetectorType       string      `json:"detector_type,omitempty"`
	ModelType          string      `json:"model_type,omitempty"`
	Data               interface{} `json:"data,omitempty"`
}

// API END
