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

package dockerobserver // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/dockerobserver"

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	dtypes "github.com/docker/docker/api/types"
	"github.com/docker/go-connections/nat"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
	dcommon "github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/docker"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/docker"
)

const (
	defaultDockerAPIVersion         = 1.22
	minimalRequiredDockerAPIVersion = 1.22
)

var _ extension.Extension = (*dockerObserver)(nil)
var _ observer.EndpointsLister = (*dockerObserver)(nil)
var _ observer.Observable = (*dockerObserver)(nil)

type dockerObserver struct {
	*observer.EndpointsWatcher
	logger  *zap.Logger
	config  *Config
	cancel  func()
	once    *sync.Once
	ctx     context.Context
	dClient *docker.Client
}

// newObserver creates a new docker observer extension.
func newObserver(logger *zap.Logger, config *Config) (extension.Extension, error) {
	d := &dockerObserver{
		logger: logger, config: config,
		once: &sync.Once{},
	}
	d.EndpointsWatcher = observer.NewEndpointsWatcher(d, time.Second, logger)
	return d, nil
}

// Start will instantiate required components needed by the Docker observer
func (d *dockerObserver) Start(ctx context.Context, host component.Host) error {
	dCtx, cancel := context.WithCancel(context.Background())
	d.cancel = cancel
	d.ctx = dCtx

	// Create new Docker client
	dConfig, err := docker.NewConfig(d.config.Endpoint, d.config.Timeout, d.config.ExcludedImages, d.config.DockerAPIVersion)
	if err != nil {
		return err
	}

	d.dClient, err = docker.NewDockerClient(dConfig, d.logger)
	if err != nil {
		return fmt.Errorf("could not create docker client: %w", err)
	}

	if err = d.dClient.LoadContainerList(ctx); err != nil {
		return err
	}

	d.once.Do(
		func() {
			go func() {
				cacheRefreshTicker := time.NewTicker(d.config.CacheSyncInterval)
				defer cacheRefreshTicker.Stop()

				clientCtx, clientCancel := context.WithCancel(d.ctx)

				go d.dClient.ContainerEventLoop(clientCtx)

				for {
					select {
					case <-d.ctx.Done():
						clientCancel()
						return
					case <-cacheRefreshTicker.C:
						err = d.dClient.LoadContainerList(clientCtx)
						if err != nil {
							d.logger.Error("Could not sync container cache", zap.Error(err))
						}
					}
				}
			}()
		},
	)

	return nil
}

func (d *dockerObserver) Shutdown(ctx context.Context) error {
	d.cancel()
	return nil
}

func (d *dockerObserver) ListEndpoints() []observer.Endpoint {
	var endpoints []observer.Endpoint
	for _, container := range d.dClient.Containers() {
		endpoints = append(endpoints, d.containerEndpoints(container.ContainerJSON)...)
	}
	return endpoints
}

// containerEndpoints generates a list of observer.Endpoint given a Docker ContainerJSON.
// This function will only generate endpoints if a container is in the Running state and not Paused.
func (d *dockerObserver) containerEndpoints(c *dtypes.ContainerJSON) []observer.Endpoint {
	var endpoints []observer.Endpoint

	if !c.State.Running || c.State.Running && c.State.Paused {
		return endpoints
	}

	knownPorts := map[nat.Port]bool{}
	for k := range c.Config.ExposedPorts {
		knownPorts[k] = true
	}

	// iterate over exposed ports and try to create endpoints
	for portObj := range knownPorts {
		endpoint := d.endpointForPort(portObj, c)
		// the endpoint was not set, so we'll drop it
		if endpoint == nil {
			continue
		}
		endpoints = append(endpoints, *endpoint)
	}

	return endpoints
}

// endpointForPort creates an observer.Endpoint for a given port that is exposed in a Docker container.
// Each endpoint has a unique ID generated by the combination of the container.ID, container.Name,
// underlying host name, and the port number.
// Uses the user provided config settings to override certain fields.
func (d *dockerObserver) endpointForPort(portObj nat.Port, c *dtypes.ContainerJSON) *observer.Endpoint {
	endpoint := observer.Endpoint{}
	port := uint16(portObj.Int())
	proto := portObj.Proto()

	mappedPort, mappedIP := findHostMappedPort(c, portObj)
	if d.config.IgnoreNonHostBindings && mappedPort == 0 && mappedIP == "" {
		return nil
	}

	// unique ID per containerID:port
	var id observer.EndpointID
	if mappedPort != 0 {
		id = observer.EndpointID(fmt.Sprintf("%s:%d", c.ID, mappedPort))
	} else {
		id = observer.EndpointID(fmt.Sprintf("%s:%d", c.ID, port))
	}

	imageRef, err := dcommon.ParseImageName(c.Config.Image)
	if err != nil {
		d.logger.Error("could not parse container image name", zap.Error(err))
	}

	details := &observer.Container{
		Name:        strings.TrimPrefix(c.Name, "/"),
		Image:       imageRef.Repository,
		Tag:         imageRef.Tag,
		Command:     strings.Join(c.Config.Cmd, " "),
		ContainerID: c.ID,
		Transport:   portProtoToTransport(proto),
		Labels:      c.Config.Labels,
	}

	// Set our hostname based on config settings
	if d.config.UseHostnameIfPresent && c.Config.Hostname != "" {
		details.Host = c.Config.Hostname
	} else {
		// Use the IP Address of the first network we iterate over.
		// This can be made configurable if so desired.
		for _, n := range c.NetworkSettings.Networks {
			details.Host = n.IPAddress
			break
		}

		// If we still haven't gotten a host at this point and we are using
		// host bindings, just make it localhost.
		if details.Host == "" && d.config.UseHostBindings {
			details.Host = "127.0.0.1"
		}
	}

	// If we are using HostBindings & port and IP are set, use those
	if d.config.UseHostBindings && mappedPort != 0 && mappedIP != "" {
		details.Host = mappedIP
		details.Port = mappedPort
		details.AlternatePort = port
		if details.Host == "0.0.0.0" {
			details.Host = "127.0.0.1"
		}
	} else {
		details.Port = port
		details.AlternatePort = mappedPort
	}

	endpoint = observer.Endpoint{
		ID:      id,
		Target:  fmt.Sprintf("%s:%d", details.Host, details.Port),
		Details: details,
	}

	return &endpoint
}

// FindHostMappedPort returns the port number of the docker port binding to the
// underlying host, or 0 if none exists.  It also returns the mapped ip that the
// port is bound to on the underlying host, or "" if none exists.
func findHostMappedPort(c *dtypes.ContainerJSON, exposedPort nat.Port) (uint16, string) {
	bindings := c.NetworkSettings.Ports[exposedPort]

	for _, binding := range bindings {
		if port, err := nat.ParsePort(binding.HostPort); err == nil {
			return uint16(port), binding.HostIP
		}
	}
	return 0, ""
}

// Valid proto for docker containers should be tcp, udp, sctp
// https://github.com/docker/go-connections/blob/v0.4.0/nat/nat.go#L116
func portProtoToTransport(proto string) observer.Transport {
	switch proto {
	case "tcp":
		return observer.ProtocolTCP
	case "udp":
		return observer.ProtocolUDP
	}
	return observer.ProtocolUnknown
}
