package models

import "sync"

type SpotinstScores struct {
	Lock sync.RWMutex
	SS   []SpotinstScore
}
type SpotinstScore struct {
	Az           string
	Score        int
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
