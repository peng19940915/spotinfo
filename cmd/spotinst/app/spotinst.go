package app

import (
	"context"
	"flag"
	"fmt"
	"github.com/spf13/cobra"
	"spotinfo/pkg/options"
	"spotinfo/pkg/spotinst"
)

func NewSpotinstCommand(ctx context.Context) *cobra.Command {
	opts := options.NewSpotinstOptions()
	cmd := &cobra.Command{
		Use:                   "spotinst",
		Long:                  "collect spotinst instance price",
		DisableFlagsInUseLine: true,
		SilenceUsage:          true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := Run(ctx, opts); err != nil {
				return err
			}
			return nil
		},
	}
	cmd.Flags().AddGoFlagSet(flag.CommandLine)
	opts.AddFlags(cmd.Flags())
	return cmd
}

func Run(ctx context.Context, opts *options.SpotinstOptions) error {
	data, err := spotinst.GetSpotinstCore(ctx, []string{
		"us-east-1b",
		"us-east-1c",
		"us-east-1d",
		"us-east-1f",
	}, []string{
		"c4.4xlarge",
		"c4.8xlarge",
		"c5.4xlarge",
		"c6gn.8xlarge",
		"c6i.4xlarge"})
	if err != nil {
		return err
	}
	fmt.Println(data)
	return nil
}
