// Copyright 2020 OpenTelemetry Authors
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

package holoinsightskywalkingreceiver

import (
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"os"
	agent "skywalking.apache.org/repo/goapi/collect/language/agent/v3"
	"testing"
)

func TestMetricValueStats(t *testing.T) {
	buf, err := os.ReadFile("./testdata/metric.data")
	println(string(buf))

	require.NoError(t, err)
	var jvmMetric agent.JVMMetricCollection
	err = json.Unmarshal(buf, &jvmMetric)
	require.NoError(t, err)

	rs := SkywalkingToMetrics(&jvmMetric)
	metricValueMap := map[string]any{"thread.live.count": int64(31), "thread.daemon.count": int64(30),
		"thread.runnablestate.threadcount": int64(14), "thread.blockedstate.threadcount": int64(0),
		"thread.waitingstate.threadcount": int64(3), "thread.timedwaiting.threadcount": int64(14),
		"cpu.usage": float64(0.5502263652584543),
	}
	AssertDataEqual(t, rs, metricValueMap)
}

func AssertDataEqual(t *testing.T, pmetric pmetric.ResourceMetrics, metricValueMap map[string]any) {
	println(pmetric.ScopeMetrics().Len())
	for i := 0; i < pmetric.ScopeMetrics().Len(); i++ {
		scopeMetric := pmetric.ScopeMetrics().At(i)

		for j := 0; j < scopeMetric.Metrics().Len(); j++ {
			metric := scopeMetric.Metrics().At(j)

			value := metricValueMap[metric.Name()]
			if value != nil {
				switch value.(type) {
				case int64:
					assert.Equal(t, value, metric.Gauge().DataPoints().At(0).IntValue())
				case float64:
					assert.Equal(t, value, metric.Gauge().DataPoints().At(0).DoubleValue())
				}
			}
		}
	}
}
