// Copyright  OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License")
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

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/traas-stack/holoinsight-collector/internal/utils"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.uber.org/zap"
	v3c "skywalking.apache.org/repo/goapi/collect/agent/configuration/v3"
	common "skywalking.apache.org/repo/goapi/collect/common/v3"
	event "skywalking.apache.org/repo/goapi/collect/event/v3"
	agent "skywalking.apache.org/repo/goapi/collect/language/agent/v3"
	profile "skywalking.apache.org/repo/goapi/collect/language/profile/v3"
	management "skywalking.apache.org/repo/goapi/collect/management/v3"
	"strings"
)

const (
	GatewayAgentConfigURL             = "/internal/api/gateway/agent/configuration/query"
	GatewayAgentConfigTenantParam     = "tenant"
	GatewayAgentConfigServiceParam    = "service"
	GatewayAgentConfigExtendInfoParam = "extendInfo"
)

type dummyReportService struct {
	management.UnimplementedManagementServiceServer
	v3c.UnimplementedConfigurationDiscoveryServiceServer
	agent.UnimplementedJVMMetricReportServiceServer
	profile.UnimplementedProfileTaskServer
	agent.UnimplementedBrowserPerfServiceServer
	event.UnimplementedEventServiceServer

	GatewayHTTPPort     int
	GatewayHTTPSettings confighttp.HTTPServerSettings
	logger              *zap.Logger
}

type AgentConfiguration struct {
	Tenant        string
	Service       string
	Configuration map[string]string
	UUID          string
}

type AgentConfigurationRequest struct {
	Tenant     string `json:"tenant"`
	Service    string `json:"service"`
	ExtendInfo string `json:"extendInfo"`
}

// for sw InstanceProperties
func (d *dummyReportService) ReportInstanceProperties(ctx context.Context, in *management.InstanceProperties) (*common.Commands, error) {
	return &common.Commands{}, nil
}

// for sw InstancePingPkg
func (d *dummyReportService) KeepAlive(ctx context.Context, in *management.InstancePingPkg) (*common.Commands, error) {
	return &common.Commands{}, nil
}

// for sw JVMMetric
func (d *dummyReportService) Collect(_ context.Context, jvm *agent.JVMMetricCollection) (*common.Commands, error) {
	return &common.Commands{}, nil
}

// for sw agent cds
func (d *dummyReportService) FetchConfigurations(ctx context.Context, req *v3c.ConfigurationSyncRequest) (*common.Commands, error) {
	if d.GatewayHTTPSettings.Endpoint == "" {
		d.logger.Error("[fetchConfigurations] Holoinsight server http endpoint not set! ")
	}
	tenantValue := ctx.Value(Tenant)
	if tenantValue == nil {
		d.logger.Error("[fetchConfigurations] tenant cannot be empty!")
		return &common.Commands{}, nil
	}
	tenant := tenantValue.(string)
	service := req.GetService()

	var extendInfo string
	extendValue := ctx.Value(ExtendTags)
	if extendValue != nil {
		extendMap := extendValue.(map[string]string)
		extendMap["type"] = "skywalking"
		jsonStr, _ := json.Marshal(extendMap)
		extendInfo = string(jsonStr)
	}

	url := d.GatewayHTTPSettings.Endpoint
	if !strings.HasPrefix(d.GatewayHTTPSettings.Endpoint, "http://") {
		url = "http://" + d.GatewayHTTPSettings.Endpoint
	}
	request := &AgentConfigurationRequest{
		Tenant:     tenant,
		Service:    service,
		ExtendInfo: extendInfo,
	}
	requestBody, _ := json.Marshal(request)
	response, err := utils.HTTPPost(url+GatewayAgentConfigURL, string(requestBody))
	if err != nil {
		d.logger.Error("[fetchConfigurations] Get agent configurations from Holoinsight server error: ", zap.Error(err))
		return &common.Commands{}, nil
	}
	if len(response) == 0 {
		d.logger.Debug(fmt.Sprintf("[fetchConfigurations] tenant: %s, service: %s , extendInfo: %s, configurations is null!",
			tenant, service, extendInfo))
		return &common.Commands{}, nil
	}

	agentConfiguration := &AgentConfiguration{}
	err = json.Unmarshal(response, agentConfiguration)
	if err != nil {
		d.logger.Error("[fetchConfigurations] Agent configurations unmarshal error: ", zap.Error(err))
	}

	if len(response) != 0 && agentConfiguration.UUID != req.GetUuid() {
		configList := make([]*common.KeyStringValuePair, 0, 8)
		configList = append(configList, &common.KeyStringValuePair{Key: "UUID", Value: agentConfiguration.UUID})
		configList = append(configList, &common.KeyStringValuePair{Key: "SerialNumber", Value: uuid.New().String()})

		for key, value := range agentConfiguration.Configuration {
			configList = append(configList, &common.KeyStringValuePair{Key: key, Value: value})
		}

		c := &common.Command{Command: "ConfigurationDiscoveryCommand", Args: configList}
		d.logger.Info(fmt.Sprintf("[fetchConfigurations] tenant: %s, service: %s, extendInfo: %s, config: %s",
			tenant, service, extendInfo, agentConfiguration.Configuration))

		return &common.Commands{Commands: []*common.Command{c}}, nil
	}
	return &common.Commands{}, nil
}

// for sw profile
func (d *dummyReportService) GetProfileTaskCommands(_ context.Context, q *profile.ProfileTaskCommandQuery) (*common.Commands, error) {
	return &common.Commands{}, nil
}

func (d *dummyReportService) CollectSnapshot(stream profile.ProfileTask_CollectSnapshotServer) error {
	return nil
}

func (d *dummyReportService) ReportTaskFinish(_ context.Context, report *profile.ProfileTaskFinishReport) (*common.Commands, error) {
	return &common.Commands{}, nil
}
