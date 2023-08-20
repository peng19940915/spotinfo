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
	"regexp"
	"sort"
	"spotinfo/pkg/known"
	"spotinfo/pkg/models"
	"spotinfo/pkg/options"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
)

var (
	loadDataOnce sync.Once
	// parsed json raw data
	data *models.AdvisorData
	// min ranges
	minRange = map[int]int{5: 0, 11: 6, 16: 12, 22: 17, 100: 23} //nolint:gomnd
)

const (
	// SortByRange sort by frequency of interruption
	SortByRange = iota
	// SortByInstance sort by instance type (lexicographical)
	SortByInstance = iota
	// SortBySavings sort by savings percentage
	SortBySavings = iota
	// SortByPrice sort by spot price
	SortByPrice = iota
	// SortByRegion sort by AWS region name
	SortByRegion = iota
	// SortByScore sort by Spotinst market score
	SortByScore = iota
)

//---- public types

// ByRange implements sort.Interface based on the Range.Min field
type ByRange []models.Advice

func (a ByRange) Len() int           { return len(a) }
func (a ByRange) Less(i, j int) bool { return a[i].Range.Min < a[j].Range.Min }
func (a ByRange) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }

// ByInstance implements sort.Interface based on the Instance field
type ByInstance []models.Advice

func (a ByInstance) Len() int           { return len(a) }
func (a ByInstance) Less(i, j int) bool { return strings.Compare(a[i].Instance, a[j].Instance) == -1 }
func (a ByInstance) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }

// BySavings implements sort.Interface based on the Savings field
type BySavings []models.Advice

func (a BySavings) Len() int           { return len(a) }
func (a BySavings) Less(i, j int) bool { return a[i].Savings < a[j].Savings }
func (a BySavings) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }

// ByPrice implements sort.Interface based on the Price field
type ByPrice []models.Advice

func (a ByPrice) Len() int           { return len(a) }
func (a ByPrice) Less(i, j int) bool { return a[i].Price < a[j].Price }
func (a ByPrice) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }

// ByRegion implements sort.Interface based on the Region field
type ByRegion []models.Advice

func (a ByRegion) Len() int           { return len(a) }
func (a ByRegion) Less(i, j int) bool { return strings.Compare(a[i].Region, a[j].Region) == -1 }
func (a ByRegion) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }

func dataLazyLoad(url string, timeout time.Duration) (result *models.AdvisorData, err error) {
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
	err = sonic.Unmarshal(resp.Body(), &result)

	return result, nil
}

// GetSpotSavings get spot saving advices
func GetSpotSavings(ctx context.Context, opts *options.SpotinstOptions) ([]models.Advice, error) {
	var err error
	loadDataOnce.Do(func() {
		const timeout = 10
		// 先从本地加载数据
		infoJsonPath := "/tmp/info.json"
		if f, err := os.Stat(infoJsonPath); err == nil && f.Size() >= 100 {
			// 判断文件时间
			if time.Now().Sub(f.ModTime()) < 24*time.Hour {
				// 从文件中加载数据
				fmt.Println("load info data from cache...")
				content, _ := os.ReadFile(infoJsonPath)
				err = sonic.Unmarshal(content, &data)
				return

			}
		} else {
			os.Create(infoJsonPath)
		}
		fmt.Println("missing info cache, load from remote...")
		data, err = dataLazyLoad(known.SpotAdvisorJSONURL, timeout*time.Second)
		contentByte, _ := sonic.Marshal(data)
		os.WriteFile(infoJsonPath, contentByte, 0644)
	})

	if err != nil {
		return nil, errors.Wrap(err, "failed to load spot data")
	}
	var regions = opts.Region
	// special case: "all" regions (slice with single element)
	if len(opts.Region) == 1 && opts.Region[0] == "all" {
		// replace regions with all available regions
		regions = make([]string, 0, len(data.Regions))
		for k := range data.Regions {
			regions = append(regions, k)
		}
	}

	// get advices for specified regions
	var result []models.Advice

	for _, region := range regions {
		r, ok := data.Regions[region]
		if !ok {
			return nil, errors.Errorf("no spot price for region %s", region)
		}

		var spotInfos map[string]models.SpotInfo
		if strings.EqualFold("windows", opts.Os) {
			spotInfos = r.Windows
		} else if strings.EqualFold("linux", opts.Os) {
			spotInfos = r.Linux
		} else {
			return nil, errors.New("invalid instance OS, must be windows/linux")
		}
		// construct advices result
		for instance, adv := range spotInfos {
			// match instance type name
			matched, err := regexp.MatchString(opts.Type, instance)
			if err != nil {
				return nil, errors.Wrap(err, "failed to match instance type")
			}
			if !matched { // skip not matched
				continue
			}
			// filter by min vCPU and memory
			info := data.InstanceTypes[instance]
			if (opts.MinCpu != 0 && info.Cores < opts.MinCpu) || (opts.MinMemory != 0 && info.RAM < float32(opts.MinMemory)) {
				continue
			}
			// get price details
			spotPriceDatas, err := getSpotInstancePrice(instance, region, opts.Os)

			var spotScoreMaps = make(map[string]int)
			// get spotinst score details

			if azs, ok := known.AvailablespotinstAzs[region]; ok && opts.Mode == known.ScoreMode {
				for _, az := range azs {
					score, err := getSpotInstanceScore(ctx, instance, az, data)
					if err != nil {
						fmt.Println("get spot instance score failed", err)
						continue
					}
					spotScoreMaps[az] = score
				}
			}

			// prepare record
			rng := models.InterruptionRange{
				Label: data.Ranges[adv.Range].Label,
				Max:   data.Ranges[adv.Range].Max,
				Min:   minRange[data.Ranges[adv.Range].Max],
			}
			result = append(result, models.Advice{
				Region:   region,
				Instance: instance,
				Range:    rng,
				Savings:  adv.Savings,
				Score:    spotScoreMaps,
				Info:     models.TypeInfo(info),
				Price:    spotPriceDatas,
			})
		}
	}

	// sort results by - range (default)
	var data sort.Interface

	switch opts.Sort {
	case "rage":
		data = ByRange(result)
	case "instance":
		data = ByInstance(result)
	case "saving":
		data = BySavings(result)
	case "price":
		data = ByPrice(result)
	case "region":
		data = ByRegion(result)
	default:
		data = ByRange(result)
	}

	if opts.Order == "desc" {
		data = sort.Reverse(data)
	}

	sort.Sort(data)

	return result, nil
}
