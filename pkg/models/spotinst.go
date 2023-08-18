package models

type SpotinstScore struct {
	Az           string
	Score        float64
	InstanceType string
}

type SpotinstScoreResp struct {
	Kind  string `json:"kind"`
	Items []struct {
		LifetimePeriod int `json:"lifetimePeriod"`
		MarketsScore   []struct {
			AvailabilityZone string  `json:"availabilityZone"`
			InstanceType     string  `json:"instanceType"`
			Product          string  `json:"product"`
			Score            float64 `json:"score"`
		} `json:"marketsScore"`
	} `json:"items"`
	RegistrationState int `json:"registrationState"`
}
