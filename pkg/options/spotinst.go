package options

import (
	"github.com/spf13/pflag"
)

type SpotinstOptions struct {
	UserName  string
	Password  string
	Type      string
	Region    []string
	Mode      string
	MaxCpu    int
	MaxMemory int
	Sort      string
	Order     string
	Os        string
}

var defaultAzs = []string{
	"us-east-1b",
	"us-east-1c",
	"us-east-1d",
	"us-east-1f",
}

func NewSpotinstOptions() *SpotinstOptions {
	return &SpotinstOptions{}
}

func (o *SpotinstOptions) AddFlags(flags *pflag.FlagSet) {
	flags.StringVarP(&o.Type, "instance_type", "i", "", "EC2 instance type (can be RE2 regexp patten)")
	flags.StringSliceVarP(&o.Region, "region", "r", []string{"all"}, "set one or more AWS regions, use \"all\" for all AWS regions")
	flags.IntVarP(&o.MaxCpu, "cpu", "c", 0, "filter: minimal vCPU cores")
	flags.IntVarP(&o.MaxMemory, "memory", "m", 0, "filter: minimal memory GiB")
	flags.StringVarP(&o.Sort, "sort", "s", "s", "sort results by interruption|type|savings|price|region|score")
	flags.StringVarP(&o.Order, "order", "o", "desc", "sort order asc|desc")
	flags.StringVar(&o.Os, "os", "Linux", "os type")
	flags.StringVar(&o.Mode, "mode", "score", "score|normal")
}
