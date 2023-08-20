package aws

import (
	"context"
	"crypto/tls"
	"fmt"
	"github.com/bytedance/sonic"
	"github.com/cloudwego/hertz/pkg/app/client"
	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"os"
	"spotinfo/pkg/known"
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
	Linux   float64 `json:"linux"`
	Windows float64 `json:"windows"`
}
type regionPrice struct {
	Instance map[string]instancePrice `json:"instance"`
}
type spotPriceData struct {
	Region map[string]regionPrice `json:"region"`
}

func pricingLazyLoad(url string, timeout time.Duration) (result *rawPriceData, err error) {
	result = &rawPriceData{}
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
	err = sonic.UnmarshalString(bodyString, &result)
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
	pricing.Region = make(map[string]regionPrice)

	for _, region := range raw.Config.Regions {
		var rp regionPrice
		rp.Instance = make(map[string]instancePrice)

		for _, it := range region.InstanceTypes {
			for _, size := range it.Sizes {
				var ip instancePrice

				for _, os := range size.ValueColumns {
					price, err := strconv.ParseFloat(os.Prices.USD, 64)
					if err != nil {
						price = 0
					}

					if os.Name == "mswin" {
						ip.Windows = price
					} else {
						ip.Linux = price
					}
				}

				rp.Instance[size.Size] = ip
			}
		}

		pricing.Region[region.Region] = rp
	}

	return &pricing
}

func getSpotInstancePrice(instance, region, instanceOs string) (float64, error) {
	var (
		err  error
		data *rawPriceData
	)
	loadPriceOnce.Do(func() {
		// 先从本地加载数据
		awsPriceJsonPath := "/tmp/price.json"
		if f, err := os.Stat(awsPriceJsonPath); err == nil && f.Size() >= 100 {
			// 判断文件时间
			if time.Now().Sub(f.ModTime()) < 24*time.Hour {
				// 从文件中加载数据
				fmt.Println("load price data from cache...")
				content, _ := os.ReadFile(awsPriceJsonPath)
				err = sonic.Unmarshal(content, &spotPrice)
				return
			}
		} else {
			os.Create(awsPriceJsonPath)
		}
		fmt.Println("missing price cache, load from remote...")
		data, err = pricingLazyLoad(known.SpotPriceJsURL, 1*time.Minute)
		if err != nil {
			fmt.Println("pricingLazyLoad failed, detail: ", err.Error())
			return
		}
		spotPrice = convertRawData(data)

		contentByte, err := sonic.Marshal(spotPrice)
		if err != nil {
			fmt.Println(err)
		}
		os.WriteFile(awsPriceJsonPath, contentByte, 0644)
	})

	if err != nil {
		return 0, errors.Wrap(err, "failed to load spot instance pricing")
	}

	rp, ok := spotPrice.Region[region]
	if !ok {
		return 0, errors.Errorf("no pricind fata for region: %v", region)
	}

	price, ok := rp.Instance[instance]
	if !ok {
		return 0, errors.Errorf("no pricind fata for instance: %v", instance)
	}

	if instanceOs == "windows" {
		return price.Windows, nil
	}

	return price.Linux, nil
}
