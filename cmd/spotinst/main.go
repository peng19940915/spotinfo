package main

import (
	"os"
	"spotinfo/cmd/spotinst/app"
	"spotinfo/pkg/signals"
)

func main() {
	ctx := signals.SetupSignalHandler()
	if err := app.NewSpotinstCommand(ctx).Execute(); err != nil {
		//fmt.Fprintf(os.Stdout, "%v\n", err)
		os.Exit(1)
	}
}
