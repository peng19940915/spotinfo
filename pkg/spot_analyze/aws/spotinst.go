package aws

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"github.com/bytedance/sonic"
	"github.com/cloudwego/hertz/pkg/app/client"
	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/panjf2000/ants/v2"
	"net/url"
	"os"
	"spotinfo/pkg/known"
	"spotinfo/pkg/models"
	"sync"
	"time"
)

var (
	loadScoreOnce sync.Once
	spotScores    *spotScoreData
)

type instanceScore struct {
	Instance map[string]int `json:"instance"`
}
type spotScoreData struct {
	Azs map[string]instanceScore `json:"azs"`
}

func getSpotinstScore(param interface{}) {
	ap := param.(*AntsParams)
	SpotinstAccessToken, _ := os.LookupEnv("SpotinstAccessToken")
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
	bodyMap["availabilityZones"] = ap.Azs
	bodyMap["instanceTypes"] = ap.Instances
	bodyMap["product"] = "Linux/UNIX (Amazon VPC)"
	bodyMap["minimumInstanceLifetime"] = []int{1}
	//https://console.spotinst.com/api/aws/ec2/availabilityZone?accountId=act-4321e68e&region=us-east-3
	//
	requestBody, _ := sonic.Marshal(bodyMap)
	req.SetBody(requestBody)
	fmt.Println(string(requestBody))
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

	err := hClient.DoTimeout(ap.Ctx, req, resp, 30*time.Second)
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
				Score:        int(marketScore.Score),
			}
			ap.Scs.Lock.Lock()
			ap.Scs.SS = append(ap.Scs.SS, ss)
			ap.Scs.Lock.Unlock()
		}
	}

	return
}

type AntsParams struct {
	Azs       []string
	Ctx       context.Context
	Instances []string
	Scs       *models.SpotinstScores
}

func getSpotinstScores(ctx context.Context, instances []string) (scs *models.SpotinstScores, err error) {
	batch := 50
	var wg sync.WaitGroup
	scs = &models.SpotinstScores{
		Lock: sync.RWMutex{},
		SS:   []models.SpotinstScore{},
	}
	var allAzs []string
	{
	}
	for _, azs := range known.AvailablespotinstAzs {
		allAzs = append(allAzs, azs...)
	}
	p, _ := ants.NewPoolWithFunc(10, func(params interface{}) {
		getSpotinstScore(params)
		wg.Done()
	})
	defer p.Release()

	for i := 0; i < len(instances); i++ {
		if i%batch == 0 && i > 0 || i == len(instances)-1 {
			start := ((i / batch) - 1) * batch
			end := start + batch
			if i == len(instances)-1 {
				start = (i / batch) * batch
				end = len(instances)
			}
			// 并发
			ap := &AntsParams{
				Ctx:       ctx,
				Azs:       allAzs,
				Instances: instances[start:end],
				Scs:       scs,
			}
			wg.Add(1)
			_ = p.Invoke(ap)
		}
	}

	wg.Wait()

	return
}

func getSpotInstanceScore(ctx context.Context, instance, az string, data *models.AdvisorData) (score int, err error) {
	loadScoreOnce.Do(func() {
		awsScoreJsonPath := "/tmp/score.json"
		if f, err := os.Stat(awsScoreJsonPath); err == nil && f.Size() >= 100 {
			// 判断文件时间
			if time.Now().Sub(f.ModTime()) < 24*time.Hour {
				// 从文件中加载数据
				fmt.Println("load score data from cache...")
				content, _ := os.ReadFile(awsScoreJsonPath)
				err = sonic.Unmarshal(content, &spotScores)
				return
			}
		} else {
			os.Create(awsScoreJsonPath)
		}
		fmt.Println("missing score cache, load from remote...")
		// 获取所有region/instance
		var allRegion, allInstance []string
		for k := range data.Regions {
			allRegion = append(allRegion, k)
		}
		for k := range data.InstanceTypes {
			allInstance = append(allInstance, k)
		}
		scs, err := getSpotinstScores(ctx, allInstance)
		if err != nil {
			return
		}
		spotScores = &spotScoreData{}
		spotScores.Azs = make(map[string]instanceScore, 0)
		for _, sc := range scs.SS {
			//score := make(map[string]float64, 0)
			if _, ok := spotScores.Azs[sc.Az]; !ok {
				spotScores.Azs[sc.Az] = instanceScore{
					Instance: make(map[string]int, 0),
				}
			}
			spotScores.Azs[sc.Az].Instance[sc.InstanceType] = sc.Score
		}
		contentByte, err := sonic.Marshal(spotScores)
		if err != nil {
			fmt.Println(err)
		}
		os.WriteFile(awsScoreJsonPath, contentByte, 0644)
	})
	score = spotScores.Azs[az].Instance[instance]
	return
}
