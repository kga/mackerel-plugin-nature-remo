package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	mp "github.com/mackerelio/go-mackerel-plugin"
	"github.com/tenntenn/natureremo"
)

type fakeDevices struct {
	devices []*natureremo.Device
	err     error
}

func (f fakeDevices) GetAll(context.Context) ([]*natureremo.Device, error) {
	return f.devices, f.err
}

func device(id, name string, events map[natureremo.SensorType]float64) *natureremo.Device {
	d := &natureremo.Device{NewestEvents: map[natureremo.SensorType]natureremo.SensorValue{}}
	d.ID, d.Name = id, name
	for sensor, v := range events {
		d.NewestEvents[sensor] = natureremo.SensorValue{Value: v}
	}
	return d
}

func TestFetchMetrics(t *testing.T) {
	remo := device("id-1", "Living Room", map[natureremo.SensorType]float64{
		natureremo.SensorTypeTemperature:  21.5,
		natureremo.SensorTypeHumidity:     40,
		natureremo.SensorTypeIllumination: 120,
		natureremo.SensorTypeMovement:     1,
	})
	mini := device("id-2", "mini", map[natureremo.SensorType]float64{
		natureremo.SensorTypeTemperature: 19,
	})

	tests := []struct {
		name      string
		deviceKey string
		want      map[string]float64
	}{
		{
			name:      "by name",
			deviceKey: deviceKeyName,
			want: map[string]float64{
				"temperature.Living_Room": 21.5,
				"humidity.Living_Room":    40,
				"illuminance.Living_Room": 120,
				"temperature.mini":        19,
			},
		},
		{
			name:      "by id",
			deviceKey: deviceKeyID,
			want: map[string]float64{
				"temperature.id-1": 21.5,
				"humidity.id-1":    40,
				"illuminance.id-1": 120,
				"temperature.id-2": 19,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &plugin{devices: fakeDevices{devices: []*natureremo.Device{remo, mini}}, deviceKey: tt.deviceKey}
			got, err := p.FetchMetrics()
			if err != nil {
				t.Fatal(err)
			}
			if !maps.Equal(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFetchMetricsNameCollision(t *testing.T) {
	devices := []*natureremo.Device{
		device("id-1", "寝室", map[natureremo.SensorType]float64{natureremo.SensorTypeTemperature: 18}),
		device("id-2", "居間", map[natureremo.SensorType]float64{natureremo.SensorTypeTemperature: 22}),
	}

	var warnings []string
	p := &plugin{
		devices:   fakeDevices{devices: devices},
		deviceKey: deviceKeyName,
		warnf: func(format string, args ...any) {
			warnings = append(warnings, fmt.Sprintf(format, args...))
		},
	}
	got, err := p.FetchMetrics()
	if err != nil {
		t.Fatal(err)
	}
	if want := map[string]float64{"temperature.__": 18}; !maps.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "居間") {
		t.Errorf("warnings = %q, want one naming the skipped device", warnings)
	}
}

func TestFetchMetricsError(t *testing.T) {
	want := errors.New("boom")
	p := &plugin{devices: fakeDevices{err: want}}
	if _, err := p.FetchMetrics(); !errors.Is(err, want) {
		t.Errorf("err = %v, want %v", err, want)
	}
}

func TestSanitize(t *testing.T) {
	tests := map[string]string{
		"remo":          "remo",
		"  Living Room": "Living_Room",
		"a-b_c.d":       "a-b_c_d",
		"リビング":          "____",
		"Remo 3":        "Remo_3",
	}
	for in, want := range tests {
		if got := sanitize(in); got != want {
			t.Errorf("sanitize(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestOutput goes through the real API client and the Mackerel plugin library,
// so it covers decoding a response through to the emitted metric keys.
func TestOutput(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.Header.Get("Authorization"), "Bearer token"; got != want {
			t.Errorf("Authorization = %q, want %q", got, want)
		}
		w.Header().Set("X-Rate-Limit-Limit", "30")
		w.Header().Set("X-Rate-Limit-Remaining", "29")
		w.Header().Set("X-Rate-Limit-Reset", "1700000000")
		_, err := fmt.Fprint(w, `[{
			"id": "id-1",
			"name": "Remo",
			"newest_events": {
				"te": {"val": 21.5, "created_at": "2026-10-31T00:00:00Z"},
				"hu": {"val": 40, "created_at": "2026-10-31T00:00:00Z"}
			}
		}]`)
		if err != nil {
			t.Error(err)
		}
	}))
	defer srv.Close()

	client := natureremo.NewClient("token")
	client.BaseURL = srv.URL

	p := mp.NewMackerelPlugin(&plugin{devices: client.DeviceService, prefix: "NatureRemo", deviceKey: deviceKeyName})
	p.Tempfile = filepath.Join(t.TempDir(), "tempfile")
	out := captureStdout(t, p.OutputValues)

	for _, want := range []string{"NatureRemo.temperature.Remo\t21.500000\t", "NatureRemo.humidity.Remo\t40\t"} {
		if !strings.Contains(out, want) {
			t.Errorf("output %q does not contain %q", out, want)
		}
	}
}

func captureStdout(t *testing.T, f func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = stdout }()

	f()

	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}
