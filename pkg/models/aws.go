package models

// Advice - spot price advice: interruption range and savings
type Advice struct {
	Region    string
	Instance  string
	Range     Range
	Savings   int
	Info      TypeInfo
	Price     float64
	ZonePrice map[string]float64
}

// Range interruption range
type Range struct {
	Label string `json:"label"`
	Min   int    `json:"min"`
	Max   int    `json:"max"`
}

// TypeInfo instance type details: vCPU cores, memory, cam  run in EMR
type TypeInfo instanceType

type instanceType struct {
	Cores int     `json:"cores"`
	Emr   bool    `json:"emr"`
	RAM   float32 `json:"ram_gb"` //nolint:tagliatelle
}
