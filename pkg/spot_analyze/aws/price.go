package aws

import (
	"context"
	"crypto/tls"
	"fmt"
	"github.com/bytedance/sonic"
	"github.com/cloudwego/hertz/pkg/app/client"
	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
)

var (
	loadPriceOnce sync.Once
	// spot pricing data
	spotPrice *spotPriceData
	// aws region map: map between non-standard codes in spot pricing JS and AWS region code
	awsSpotPricingRegions = map[string]string{
		"us-east":    "us-east-1",
		"us-west":    "us-west-1",
		"eu-ireland": "eu-west-1",
		"apac-sin":   "ap-southeast-1",
		"apac-syd":   "ap-southeast-2",
		"apac-tokyo": "ap-northeast-1",
	}
)

const (
	responsePrefix = "callback("
	responseSuffix = ");"
	spotPriceJsURL = "https://spot-price.s3.amazonaws.com/spot.js"
)

type rawPriceData struct {
	Embedded bool // true if loaded from embedded copy
	Config   struct {
		Rate         string   `json:"rate"`
		ValueColumns []string `json:"valueColumns"`
		Currencies   []string `json:"currencies"`
		Regions      []struct {
			Region        string `json:"region"`
			InstanceTypes []struct {
				Type  string `json:"type"`
				Sizes []struct {
					Size         string `json:"size"`
					ValueColumns []struct {
						Name   string `json:"name"`
						Prices struct {
							USD string `json:"USD"` //nolint:tagliatelle
						} `json:"prices"`
					} `json:"valueColumns"`
				} `json:"sizes"`
			} `json:"instanceTypes"`
		} `json:"regions"`
	} `json:"config"`
}

type instancePrice struct {
	linux   float64
	windows float64
}
type regionPrice struct {
	instance map[string]instancePrice
}
type spotPriceData struct {
	region map[string]regionPrice
}

func pricingLazyLoad(url string, timeout time.Duration) (result *rawPriceData, err error) {
	req, resp := protocol.AcquireRequest(), protocol.AcquireResponse()
	defer func() {
		protocol.ReleaseRequest(req)
		protocol.ReleaseResponse(resp)
	}()
	req.SetMethod(consts.MethodGet)
	req.SetRequestURI(url)
	hClient, _ := client.NewClient(client.WithTLSConfig(&tls.Config{
		InsecureSkipVerify: true,
	}))

	err = hClient.DoTimeout(context.TODO(), req, resp, timeout)

	if err != nil {
		return
	}
	if resp.StatusCode() != consts.StatusOK {
		err = errors.New(fmt.Sprintf("url:%s code: %d, detail:%s", url, resp.StatusCode(), string(resp.Body())))
		return
	}
	bodyString := string(resp.Body())

	bodyString = strings.TrimPrefix(bodyString, responsePrefix)
	bodyString = strings.TrimSuffix(bodyString, responseSuffix)
	err = sonic.Unmarshal(resp.Body(), &result)
	if err != nil {
		return
	}

	for index, r := range result.Config.Regions {
		if awsRegion, ok := awsSpotPricingRegions[r.Region]; ok {
			result.Config.Regions[index].Region = awsRegion
		}
	}

	return
}

func convertRawData(raw *rawPriceData) *spotPriceData {
	// fill priceData from rawPriceData
	var pricing spotPriceData
	pricing.region = make(map[string]regionPrice)

	for _, region := range raw.Config.Regions {
		var rp regionPrice
		rp.instance = make(map[string]instancePrice)

		for _, it := range region.InstanceTypes {
			for _, size := range it.Sizes {
				var ip instancePrice

				for _, os := range size.ValueColumns {
					price, err := strconv.ParseFloat(os.Prices.USD, 64)
					if err != nil {
						price = 0
					}

					if os.Name == "mswin" {
						ip.windows = price
					} else {
						ip.linux = price
					}
				}

				rp.instance[size.Size] = ip
			}
		}

		pricing.region[region.Region] = rp
	}

	return &pricing
}

func getSpotInstancePrice(instance, region, os string) (float64, error) {
	var (
		err  error
		data *rawPriceData
	)

	loadPriceOnce.Do(func() {
		const timeout = 10
		data, err = pricingLazyLoad(spotPriceJsURL, timeout*time.Second)
		spotPrice = convertRawData(data)
	})

	if err != nil {
		return 0, errors.Wrap(err, "failed to load spot instance pricing")
	}

	rp, ok := spotPrice.region[region]
	if !ok {
		return 0, errors.Errorf("no pricind fata for region: %v", region)
	}

	price, ok := rp.instance[instance]
	if !ok {
		return 0, errors.Errorf("no pricind fata for instance: %v", instance)
	}

	if os == "windows" {
		return price.windows, nil
	}

	return price.linux, nil
}
