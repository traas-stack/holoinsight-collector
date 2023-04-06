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

import (
	"context"
	"go.opentelemetry.io/collector/pdata/pmetric"
	common "skywalking.apache.org/repo/goapi/collect/common/v3"
	agent "skywalking.apache.org/repo/goapi/collect/language/agent/v3"
)

type metricsReportService struct {
	sr *swReceiver
	agent.UnimplementedJVMMetricReportServiceServer
}

func (s *metricsReportService) Collect(ctx context.Context, jvmMetric *agent.JVMMetricCollection) (*common.Commands, error) {
	rs := SkywalkingToMetrics(jvmMetric)
	md := pmetric.NewMetrics()
	rs.MoveTo(md.ResourceMetrics().AppendEmpty())

	err := s.sr.nextMetricsConsumer.ConsumeMetrics(ctx, md)
	return &common.Commands{}, err
}
