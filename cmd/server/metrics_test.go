package main

import "testing"

func TestMetricsAddrFromEnv(t *testing.T) {
	cases := map[string]string{
		"":               defaultMetricsAddr,
		"  ":             defaultMetricsAddr,
		"off":            "",
		"OFF":            "",
		"false":          "",
		"9464":           ":9464",
		":9464":          ":9464",
		"127.0.0.1:9464": "127.0.0.1:9464",
	}
	for value, want := range cases {
		got := metricsAddrFromEnv(func(key string) string {
			if key == "SECTILE_METRICS_ADDR" {
				return value
			}
			return ""
		})
		if got != want {
			t.Errorf("SECTILE_METRICS_ADDR=%q gives %q, want %q", value, got, want)
		}
	}
}
