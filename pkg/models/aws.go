package models

// Advice - spot price advice: interruption range and savings
type Advice struct {
	Region    string
	Instance  string
	Range     InterruptionRange
	Savings   int
	Info      TypeInfo
	Price     float64
	Score     map[string]int
	ZonePrice map[string]float64
}

// InterruptionRange range
type InterruptionRange struct {
	Label string `json:"label"`
	Min   int    `json:"min"`
	Max   int    `json:"max"`
}

// TypeInfo instance type details: vCPU cores, memory, cam  run in EMR
type TypeInfo InstanceType

type InstanceType struct {
	Cores int     `json:"cores"`
	Emr   bool    `json:"emr"`
	RAM   float32 `json:"ram_gb"` //nolint:tagliatelle
}

type AdvisorData struct {
	Ranges        []interruptionRange     `json:"ranges"`
	InstanceTypes map[string]instanceType `json:"instance_types"` //nolint:tagliatelle
	Regions       map[string]osTypes      `json:"spot_advisor"`   //nolint:tagliatelle
}

type interruptionRange struct {
	Label string `json:"label"`
	Index int    `json:"index"`
	Dots  int    `json:"dots"`
	Max   int    `json:"max"`
}

type instanceType struct {
	Cores int     `json:"cores"`
	Emr   bool    `json:"emr"`
	RAM   float32 `json:"ram_gb"` //nolint:tagliatelle
}

type SpotInfo struct {
	Range   int `json:"r"`
	Savings int `json:"s"`
}

type osTypes struct {
	Windows map[string]SpotInfo `json:"Windows"` //nolint:tagliatelle
	Linux   map[string]SpotInfo `json:"Linux"`   //nolint:tagliatelle
}
