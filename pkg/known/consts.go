package known

const (
	SpotHost           = "https://console.spotinst.com"
	SpotAccountId      = "act-4321e68e"
	SpotSignUri        = "/api/auth/signIn"
	SpotMarketScoreUri = "/api/aws/ec2/market/score"
)

const (
	NormalMode = "normal"
	ScoreMode  = "score"
)

const (
	SpotAdvisorJSONURL = "https://spot-bid-advisor.s3.amazonaws.com/spot-advisor-data.json"
	SpotPriceJsURL     = "https://spot-price.s3.amazonaws.com/spot.js"
)

var (
	AvailablespotinstAzs = map[string][]string{"us-east-1": {
		"us-east-1a",
		"us-east-1b",
		"us-east-1c",
		"us-east-1d",
		"us-east-1f",
	},
	}
)
