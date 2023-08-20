package app

import (
	"context"
	"flag"
	"fmt"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"os"
	"spotinfo/pkg/known"
	"spotinfo/pkg/models"
	"spotinfo/pkg/options"
	"spotinfo/pkg/spot_analyze/aws"
)

const (
	azColumn           = "Availability Zone"
	regionColumn       = "Region"
	instanceTypeColumn = "Instance Info"
	vCPUColumn         = "vCPU"
	memoryColumn       = "Memory GiB"
	savingsColumn      = "Savings over On-Demand"
	interruptionColumn = "Frequency of interruption"
	scoreColumn        = "Spot Market Score"
	priceColumn        = "USD/Hour"
)

func NewSpotinstCommand(ctx context.Context) *cobra.Command {
	opts := options.NewSpotinstOptions()
	cmd := &cobra.Command{
		Use:                   "spotinst",
		Long:                  "collect spotinst instance price",
		DisableFlagsInUseLine: true,
		SilenceUsage:          true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, ok := os.LookupEnv("SpotinstAccessToken"); !ok {
				return errors.New("env: SpotinstAccessToken not exist")
			}
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
	advices, err := aws.GetSpotSavings(ctx, opts)
	if err != nil {
		return err
	}
	printRegion := len(opts.Region) > 1 || (len(opts.Region) == 1 && opts.Region[0] == "all")
	printAdvicesTable(advices, printRegion, opts.Mode)
	return nil
}

func printAdvicesTable(advices []models.Advice, region bool, mode string) {
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	var header table.Row
	var tableColumnConfigs []table.ColumnConfig
	switch mode {
	case known.ScoreMode:
		header = table.Row{azColumn, instanceTypeColumn, vCPUColumn, memoryColumn, savingsColumn, interruptionColumn, scoreColumn, priceColumn}
	default:
		header = table.Row{instanceTypeColumn, vCPUColumn, memoryColumn, savingsColumn, interruptionColumn, priceColumn}

	}
	if region {
		header = append(table.Row{regionColumn}, header...)
	}
	t.AppendHeader(header)
	for _, advice := range advices {
		row := table.Row{}
		switch mode {
		case known.ScoreMode:
			tableColumnConfigs = append(tableColumnConfigs, table.ColumnConfig{
				Name:        "REGION",
				Number:      1,
				AutoMerge:   true,
				Align:       text.AlignLeft,
				AlignHeader: text.AlignCenter,
				AlignFooter: text.AlignCenter,
			})
			tableColumnConfigs = append(tableColumnConfigs, table.ColumnConfig{
				Name:   scoreColumn,
				Number: 8,
				Transformer: func(val interface{}) string {
					var color text.Color
					fmt.Println("score", val)
					score := val.(int)

					if score < 100 && score > 75 {
						color = text.FgHiGreen
					} else if score < 75 && score > 50 {
						color = text.FgHiYellow
					} else if score < 50 && score > 25 {
						color = text.FgHiMagenta
					} else if score < 25 && score > 0 {
						color = text.FgHiRed
					} else {
						color = text.FgWhite
					}

					return text.Colors{color}.Sprint(val)
				},
			})
			for az, score := range advice.Score {
				row = table.Row{az, advice.Instance, advice.Info.Cores, advice.Info.RAM, advice.Savings,
					advice.Range.Label, score, advice.Price}
				if region {
					row = append(table.Row{advice.Region}, row...)
				}
				t.AppendRow(row)
			}
		default:
			row = table.Row{advice.Instance, advice.Info.Cores, advice.Info.RAM, advice.Savings, advice.Range.Label, advice.Price}
			if region {
				row = append(table.Row{advice.Region}, row...)
			}
			t.AppendRow(row)
		}
	}
	// render as pretty table
	tableColumnConfigs = append(tableColumnConfigs, table.ColumnConfig{
		Name:        savingsColumn,
		Transformer: text.NewNumberTransformer("%d%%"),
	})
	t.Style().Title.Align = text.AlignCenter
	t.SetColumnConfigs(tableColumnConfigs)
	t.SetStyle(table.StyleLight)
	t.Style().Options.SeparateRows = true
	t.SortBy([]table.SortBy{
		{
			Name:   azColumn,
			Number: 2,
			Mode:   table.Asc,
		},
		{
			Name:   scoreColumn,
			Number: 8,
			Mode:   table.Asc,
		},
		{
			Name:   priceColumn,
			Number: 9,
			Mode:   table.Dsc,
		},
	})
	t.Render()

}
