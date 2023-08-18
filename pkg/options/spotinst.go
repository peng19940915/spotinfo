package options

import "github.com/spf13/pflag"

type SpotinstOptions struct {
	UserName string
	Password string
	AZs      []string
	//InstanceTypes []string
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
	//flags.StringVarP(&o.Password, "password", "p", "", "password of spotinst")
	//flags.StringVarP(&o.UserName, "username", "u", "", "username of spotinst")
	//flags.StringSliceVarP(&o.AZs, "", "", defaultAzs, "")
	//flags.StringSliceVarP(&o.InstanceTypes, "", "", []string{}, "")
}
