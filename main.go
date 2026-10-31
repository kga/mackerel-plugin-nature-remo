// Command mackerel-plugin-nature-remo posts Nature Remo sensor values to
// Mackerel as custom metrics.
package main

import (
	"flag"
	"fmt"
	"os"

	mp "github.com/mackerelio/go-mackerel-plugin"
	"github.com/tenntenn/natureremo"
)

const accessTokenEnv = "NATURE_REMO_ACCESS_TOKEN"

func main() {
	token := flag.String("access-token", os.Getenv(accessTokenEnv), "Nature API access token (defaults to $"+accessTokenEnv+")")
	prefix := flag.String("metric-key-prefix", "NatureRemo", "metric key prefix")
	deviceKey := flag.String("device-key", deviceKeyName, `what identifies a device in metric names: "name" or "id"`)
	tempfile := flag.String("tempfile", "", "temp file name")
	flag.Parse()

	if *token == "" {
		exit("access token is required: set $" + accessTokenEnv + " or -access-token")
	}
	if *deviceKey != deviceKeyName && *deviceKey != deviceKeyID {
		exit(fmt.Sprintf("invalid -device-key %q: must be %q or %q", *deviceKey, deviceKeyName, deviceKeyID))
	}

	p := mp.NewMackerelPlugin(&plugin{
		devices:   natureremo.NewClient(*token).DeviceService,
		prefix:    *prefix,
		deviceKey: *deviceKey,
		warnf: func(format string, args ...any) {
			fmt.Fprintf(os.Stderr, format+"\n", args...)
		},
	})
	p.Tempfile = *tempfile
	p.Run()
}

func exit(msg string) {
	fmt.Fprintln(os.Stderr, msg)
	os.Exit(1)
}
