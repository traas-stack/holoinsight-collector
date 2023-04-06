// Copyright  OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package holoinsightskywalkingreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/skywalkingreceiver"

// This file implements factory for skywalking receiver.

import (
	"context"
	"fmt"
	"net"
	"strconv"

	"github.com/traas-stack/holoinsight-collector/internal/sharedcomponent"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
)

const (
	typeStr   = "holoinsight_skywalking"
	stability = component.StabilityLevelBeta

	// Protocol values.
	protoGRPC = "grpc"
	protoHTTP = "http"

	// Default endpoints to bind to.
	defaultGRPCBindEndpoint = "0.0.0.0:11800"
	defaultHTTPBindEndpoint = "0.0.0.0:12800"
	defaultServerEndpoint   = "127.0.0.1:8080"
)

type skywalkingReceiverFactory struct {
	receivers *sharedcomponent.SharedComponents
}

// NewFactory creates a new Skywalking receiver factory.
func NewFactory() receiver.Factory {
	f := &skywalkingReceiverFactory{
		receivers: sharedcomponent.NewSharedComponents(),
	}
	return receiver.NewFactory(
		typeStr,
		f.createDefaultConfig,
		receiver.WithTraces(f.createTracesReceiver, stability),
		receiver.WithMetrics(f.createMetricsReceiver, stability))
}

// CreateDefaultConfig creates the default configuration for Skywalking receiver.
func (f *skywalkingReceiverFactory) createDefaultConfig() component.Config {
	return &Config{
		Protocols: Protocols{
			GRPC: &configgrpc.GRPCServerSettings{
				NetAddr: confignet.NetAddr{
					Endpoint:  defaultGRPCBindEndpoint,
					Transport: "tcp",
				},
			},
			HTTP: &confighttp.HTTPServerSettings{
				Endpoint: defaultHTTPBindEndpoint,
			},
		},
		HoloinsightServer: Protocols{
			HTTP: &confighttp.HTTPServerSettings{
				Endpoint: defaultServerEndpoint,
			},
		},
	}
}

func (f *skywalkingReceiverFactory) getReceiver(set receiver.CreateSettings, cfg component.Config) (component.Component, error) {
	var err error
	r := f.receivers.GetOrAdd(cfg, func() component.Component {
		// Convert settings in the source c to configuration struct
		// that Skywalking receiver understands.
		rCfg := cfg.(*Config)

		var c configuration
		// Set ports
		if rCfg.Protocols.GRPC != nil {
			c.CollectorGRPCServerSettings = *rCfg.Protocols.GRPC
			if c.CollectorGRPCPort, err = extractPortFromEndpoint(rCfg.Protocols.GRPC.NetAddr.Endpoint); err != nil {
				err = fmt.Errorf("unable to extract port for the gRPC endpoint: %w", err)
				return nil
			}
		}

		if rCfg.Protocols.HTTP != nil {
			c.CollectorHTTPSettings = *rCfg.Protocols.HTTP
			if c.CollectorHTTPPort, err = extractPortFromEndpoint(rCfg.Protocols.HTTP.Endpoint); err != nil {
				err = fmt.Errorf("unable to extract port for the HTTP endpoint: %w", err)
				return nil
			}
		}

		if rCfg.HoloinsightServer.HTTP != nil {
			c.GatewayHTTPSettings = *rCfg.HoloinsightServer.HTTP
			if c.GatewayHTTPPort, err = extractPortFromEndpoint(rCfg.HoloinsightServer.HTTP.Endpoint); err != nil {
				err = fmt.Errorf("unable to extract port for the HoloinsightServer HTTP endpoint: %w", err)
				return nil
			}
		}

		var skywalkingReceiver component.Component
		skywalkingReceiver, err = newSkywalkingReceiver(&c, set)
		return skywalkingReceiver
	})

	if err != nil {
		return nil, err
	}
	return r.Unwrap(), err
}

// createTracesReceiver creates a trace receiver based on provided config.
func (f *skywalkingReceiverFactory) createTracesReceiver(
	_ context.Context,
	set receiver.CreateSettings,
	cfg component.Config,
	nextConsumer consumer.Traces,
) (receiver.Traces, error) {
	receiver, err := f.getReceiver(set, cfg)
	if err != nil {
		set.Logger.Error(err.Error())
		return nil, err
	}
	receiver.(tracesDataConsumer).setNextTracesConsumer(nextConsumer)

	return receiver, nil
}

// createTracesReceiver creates a trace receiver based on provided config.
func (f *skywalkingReceiverFactory) createMetricsReceiver(
	_ context.Context,
	set receiver.CreateSettings,
	cfg component.Config,
	nextConsumer consumer.Metrics,
) (receiver.Metrics, error) {
	receiver, err := f.getReceiver(set, cfg)
	if err != nil {
		set.Logger.Error(err.Error())
		return nil, err
	}

	receiver.(metricDataConsumer).setNextMetricsConsumer(nextConsumer)

	return receiver, nil
}

// extract the port number from string in "address:port" format. If the
// port number cannot be extracted returns an error.
func extractPortFromEndpoint(endpoint string) (int, error) {
	_, portStr, err := net.SplitHostPort(endpoint)
	if err != nil {
		return 0, fmt.Errorf("endpoint is not formatted correctly: %w", err)
	}
	port, err := strconv.ParseInt(portStr, 10, 0)
	if err != nil {
		return 0, fmt.Errorf("endpoint port is not a number: %w", err)
	}
	if port < 1 || port > 65535 {
		return 0, fmt.Errorf("port number must be between 1 and 65535")
	}
	return int(port), nil
}
