package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	mp "github.com/mackerelio/go-mackerel-plugin"
	"github.com/tenntenn/natureremo"
)

const (
	deviceKeyName = "name"
	deviceKeyID   = "id"

	fetchTimeout = 20 * time.Second
)

// sensorGraphs maps each exported sensor to the graph its values belong to.
var sensorGraphs = map[natureremo.SensorType]string{
	natureremo.SensorTypeTemperature:  "temperature",
	natureremo.SensorTypeHumidity:     "humidity",
	natureremo.SensorTypeIllumination: "illuminance",
}

// deviceLister is the part of natureremo.DeviceService this plugin needs.
type deviceLister interface {
	GetAll(ctx context.Context) ([]*natureremo.Device, error)
}

type plugin struct {
	devices   deviceLister
	prefix    string
	deviceKey string
	warnf     func(format string, args ...any)
}

func (p *plugin) MetricKeyPrefix() string {
	return p.prefix
}

func (p *plugin) GraphDefinition() map[string]mp.Graphs {
	wildcard := []mp.Metrics{{Name: "*", Label: "%1"}}
	return map[string]mp.Graphs{
		"temperature": {Label: "Temperature", Unit: mp.UnitFloat, Metrics: wildcard},
		"humidity":    {Label: "Humidity", Unit: mp.UnitInteger, Metrics: wildcard},
		"illuminance": {Label: "Illuminance", Unit: mp.UnitFloat, Metrics: wildcard},
	}
}

func (p *plugin) FetchMetrics() (map[string]float64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), fetchTimeout)
	defer cancel()

	devices, err := p.devices.GetAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetch devices: %w", err)
	}

	metrics := make(map[string]float64)
	owners := make(map[string]string, len(devices))
	for _, d := range devices {
		key := p.metricName(d)
		if owner, taken := owners[key]; taken {
			p.warnf("skip device %q: its metric name %q is already used by %q; use -device-key=id", d.Name, key, owner)
			continue
		}
		owners[key] = d.Name

		for sensor, graph := range sensorGraphs {
			if ev, ok := d.NewestEvents[sensor]; ok {
				metrics[graph+"."+key] = ev.Value
			}
		}
	}
	return metrics, nil
}

func (p *plugin) metricName(d *natureremo.Device) string {
	if p.deviceKey == deviceKeyID {
		return sanitize(d.ID)
	}
	return sanitize(d.Name)
}

// sanitize makes s usable in a metric name. Each disallowed rune becomes one
// underscore, so names of only non-ASCII characters can end up identical.
func sanitize(s string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case 'a' <= r && r <= 'z', 'A' <= r && r <= 'Z', '0' <= r && r <= '9', r == '-', r == '_':
			return r
		}
		return '_'
	}, strings.TrimSpace(s))
}
