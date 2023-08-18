package aws

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"github.com/bytedance/sonic"
	"github.com/cloudwego/hertz/pkg/app/client"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"net/url"
	"os"
	"spotinfo/pkg/known"
	"spotinfo/pkg/models"
	"sync"
	"time"
)

var (
	loadScoreOnce sync.Once
	spotCores     *spotScoreData
)

type instanceScore struct {
	instance map[string]float64
}
type spotScoreData struct {
	region map[string]instanceScore
}

func getSpotinstCore(ctx context.Context, azs, instances []string) (scs []models.SpotinstScore, err error) {
	SpotinstAccessToken, exist := os.LookupEnv("SpotinstAccessToken")
	if !exist {
		err = errors.New("env: SpotinstAccessToken not exist")
		return
	}
	uri, _ := url.JoinPath(known.SpotHost, known.SpotMarketScoreUri)
	req, resp := protocol.AcquireRequest(), protocol.AcquireResponse()
	defer func() {
		protocol.ReleaseRequest(req)
		protocol.ReleaseResponse(resp)
	}()
	req.SetMethod(consts.MethodPost)
	req.SetRequestURI(uri)
	req.SetQueryString(fmt.Sprintf("accountId=%s", known.SpotAccountId))

	var bodyMap = make(map[string]interface{}, 0)
	bodyMap["availabilityZones"] = azs
	bodyMap["instanceTypes"] = instances
	bodyMap["product"] = "Linux/UNIX"
	bodyMap["minimumInstanceLifetime"] = []int{1}

	requestBody, _ := sonic.Marshal(bodyMap)
	req.SetBody(requestBody)

	req.SetHeaders(map[string]string{
		"User-Agent": "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/115.0.0.0 Safari/537.36",
		"Accept":     "application/json, text/plain, */*",
		//"Accept-Encoding": "gzip, deflate, br",
		"Accept-Language": "en,zh-CN;q=0.9,zh;q=0.8",
		"Content-Type":    "application/json;charset=UTF-8",
	})
	req.SetAuthToken(SpotinstAccessToken)
	hClient, _ := client.NewClient(client.WithTLSConfig(&tls.Config{
		InsecureSkipVerify: true,
	}))

	err = hClient.DoTimeout(ctx, req, resp, 6*time.Second)
	if err != nil {
		return
	}
	var ssResp = &models.SpotinstScoreResp{}
	err = sonic.Unmarshal(resp.Body(), ssResp)
	if err != nil {
		err = errors.New(fmt.Sprintf("parse spotinst score data failed %s ", string(resp.Body())))
		return
	}
	for _, item := range ssResp.Items {
		for _, marketScore := range item.MarketsScore {
			var ss = models.SpotinstScore{
				InstanceType: marketScore.InstanceType,
				Az:           marketScore.AvailabilityZone,
				Score:        marketScore.Score,
			}
			scs = append(scs, ss)
		}
	}

	return
}

func getSpotinstCores(ctx context.Context, azs, instances []string) (scs []models.SpotinstScore, err error) {
	batch := 8
	for i := 0; i < len(instances); i++ {

		if i%batch == 0 && i > 0 || i == len(instances)-1 {
			start := ((i / batch) - 1) * batch
			end := start + batch
			if i == len(instances)-1 {
				start = (i / batch) * batch
				end = len(instances)
			}
			tmpScs, err := getSpotinstCore(ctx, azs, instances[start:end])
			if err != nil {
				hlog.Error(err.Error())
				continue
			}
			scs = append(scs, tmpScs...)
		}
	}
	return
}

func getSpotInstanceScore(ctx context.Context, instance, region string, data *advisorData) (score float64, err error) {

	loadScoreOnce.Do(func() {
		// 获取所有region/instance
		var allRegion, allInstance []string
		for k := range data.Regions {
			allRegion = append(allRegion, k)
		}
		for k := range data.InstanceTypes {
			allInstance = append(allInstance, k)
		}
		scs, err := getSpotinstCores(ctx, allRegion, allInstance)
		if err != nil {
			return
		}
		spotCores.region = make(map[string]instanceScore, 0)
		for _, sc := range scs {
			//score := make(map[string]float64, 0)
			if _, ok := spotCores.region[sc.Az]; !ok {
				spotCores.region[sc.Az] = instanceScore{
					instance: make(map[string]float64, 0),
				}
			}
			spotCores.region[sc.Az].instance[sc.InstanceType] = sc.Score
		}
	})
	score = spotCores.region[region].instance[instance]
	return
}
