// Code generated by "go.opentelemetry.io/collector/cmd/builder". DO NOT EDIT.

// Program otelcontribcol is an OpenTelemetry Collector binary.
package main

import (
	"go.opentelemetry.io/collector/exporter"
	"log"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/otelcol"
)

func main() {
	factories, err := components()
	if err != nil {
		log.Fatalf("failed to build components: %v", err)
	}

	exporter.MakeFactoryMap()
	info := component.BuildInfo{
		Command:     "otelcontribcol",
		Description: "Local OpenTelemetry Collector Contrib binary, testing only.",
		Version:     "0.75.0-dev",
	}

	if err := run(otelcol.CollectorSettings{BuildInfo: info, Factories: factories}); err != nil {
		log.Fatal(err)
	}
}

func runInteractive(params otelcol.CollectorSettings) error {
	cmd := otelcol.NewCommand(params)
	if err := cmd.Execute(); err != nil {
		log.Fatalf("collector server run finished with error: %v", err)
	}

	return nil
}
