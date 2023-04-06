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

package holoinsightskywalkingreceiver

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
	common "skywalking.apache.org/repo/goapi/collect/common/v3"
	agent "skywalking.apache.org/repo/goapi/collect/language/agent/v3"
)

func SkywalkingToMetrics(
	jvmMetric *agent.JVMMetricCollection) pmetric.ResourceMetrics {
	rs := pmetric.NewResourceMetrics()
	rs.SetSchemaUrl(conventions.SchemaURL)
	resourceAttr := rs.Resource().Attributes()
	resourceAttr.PutStr("service.instance", jvmMetric.GetServiceInstance())
	resourceAttr.PutStr("service.name", jvmMetric.GetService())

	ils := rs.ScopeMetrics().AppendEmpty()
	appendJVMMetrics(ils.Metrics(), jvmMetric)

	return rs
}

func appendJVMMetrics(dest pmetric.MetricSlice, jvmMetric *agent.JVMMetricCollection) {
	for _, metric := range jvmMetric.GetMetrics() {
		ts := microsecondsToTimestamp(metric.GetTime())

		appendCpu(dest, metric.Cpu, ts)
		appendMemory(dest, metric.GetMemory(), ts)
		appendMemoryPool(dest, metric.GetMemoryPool(), ts)
		appendGc(dest, metric.GetGc(), ts)
		appendThread(dest, metric.GetThread(), ts)
	}
}

func appendThread(dest pmetric.MetricSlice, thread *agent.Thread, timestamp pcommon.Timestamp) {
	populateGauge(dest.AppendEmpty(), "thread.live.count", thread.GetLiveCount(), timestamp, nil, nil)
	populateGauge(dest.AppendEmpty(), "thread.daemon.count", thread.GetDaemonCount(), timestamp, nil, nil)
	populateGauge(dest.AppendEmpty(), "thread.peak.count", thread.GetPeakCount(), timestamp, nil, nil)
	populateGauge(dest.AppendEmpty(), "thread.runnablestate.count", thread.GetRunnableStateThreadCount(), timestamp, nil, nil)
	populateGauge(dest.AppendEmpty(), "thread.blockedstate.count", thread.GetBlockedStateThreadCount(), timestamp, nil, nil)
	populateGauge(dest.AppendEmpty(), "thread.waitingstate.count", thread.GetWaitingStateThreadCount(), timestamp, nil, nil)
	populateGauge(dest.AppendEmpty(), "thread.timedwaiting.count", thread.GetTimedWaitingStateThreadCount(), timestamp, nil, nil)
}

func appendCpu(dest pmetric.MetricSlice, cpu *common.CPU, timestamp pcommon.Timestamp) {
	populateGaugeF(dest.AppendEmpty(), "cpu.usagepercent", "By", cpu.UsagePercent, timestamp, nil, nil)
}

func appendGc(dest pmetric.MetricSlice, gcs []*agent.GC, timestamp pcommon.Timestamp) {
	for _, gc := range gcs {
		labelKeys := []string{"gc.phase"}
		labelValues := make([]string, 1)
		labelValues[0] = gc.GetPhase().Enum().String()
		populateGauge(dest.AppendEmpty(), "gc.time", gc.GetTime(), timestamp, labelKeys, labelValues)
		populateGauge(dest.AppendEmpty(), "gc.count", gc.GetCount(), timestamp, labelKeys, labelValues)
	}
}

func appendMemoryPool(dest pmetric.MetricSlice, memoryPools []*agent.MemoryPool, timestamp pcommon.Timestamp) {
	for _, memoryPool := range memoryPools {
		labelKeys := []string{"memorypool.type"}
		labelValues := make([]string, 1)
		labelValues[0] = memoryPool.GetType().Enum().String()

		populateGauge(dest.AppendEmpty(), "memorypool.init", memoryPool.GetInit(), timestamp, labelKeys, labelValues)
		populateGauge(dest.AppendEmpty(), "memorypool.max", memoryPool.GetMax(), timestamp, labelKeys, labelValues)
		populateGauge(dest.AppendEmpty(), "memorypool.used", memoryPool.GetUsed(), timestamp, labelKeys, labelValues)
		populateGauge(dest.AppendEmpty(), "memorypool.committed", memoryPool.GetCommitted(), timestamp, labelKeys, labelValues)
	}
}

func appendMemory(dest pmetric.MetricSlice, memorys []*agent.Memory, timestamp pcommon.Timestamp) {
	for _, memory := range memorys {
		labelKeys := []string{"memory.isheap"}
		var labelValues []string
		if memory.GetIsHeap() {
			labelValues = []string{"true"}
		} else {
			labelValues = []string{"false"}
		}
		populateGauge(dest.AppendEmpty(), "memory.init", memory.GetInit(), timestamp, labelKeys, labelValues)
		populateGauge(dest.AppendEmpty(), "memory.max", memory.GetMax(), timestamp, labelKeys, labelValues)
		populateGauge(dest.AppendEmpty(), "memory.used", memory.GetUsed(), timestamp, labelKeys, labelValues)
		populateGauge(dest.AppendEmpty(), "memory.committed", memory.GetCommitted(), timestamp, labelKeys, labelValues)
	}
}

func populateGauge(dest pmetric.Metric, name string, val int64, ts pcommon.Timestamp, labelKeys []string, labelValues []string) {
	// Unit, labelKeys, labelValues always constants, when that changes add them as argument to the func.
	populateMetricMetadata(dest, name, "By", pmetric.MetricTypeGauge)
	sum := dest.Gauge()
	dp := sum.DataPoints().AppendEmpty()

	dp.SetIntValue(val)
	dp.SetTimestamp(ts)
	populateAttributes(dp.Attributes(), labelKeys, labelValues)
}

func populateGaugeF(dest pmetric.Metric, name string, unit string, val float64, ts pcommon.Timestamp, labelKeys []string, labelValues []string) {
	populateMetricMetadata(dest, name, unit, pmetric.MetricTypeGauge)
	sum := dest.Gauge()
	dp := sum.DataPoints().AppendEmpty()
	dp.SetDoubleValue(val)
	dp.SetTimestamp(ts)
	populateAttributes(dp.Attributes(), labelKeys, labelValues)
}

func populateMetricMetadata(dest pmetric.Metric, name string, unit string, ty pmetric.MetricType) {
	dest.SetName(name)
	dest.SetUnit(unit)
	switch ty {
	case pmetric.MetricTypeGauge:
		dest.SetEmptyGauge()
	case pmetric.MetricTypeSum:
		dest.SetEmptySum()
	case pmetric.MetricTypeHistogram:
		dest.SetEmptyHistogram()
	case pmetric.MetricTypeExponentialHistogram:
		dest.SetEmptyExponentialHistogram()
	case pmetric.MetricTypeSummary:
		dest.SetEmptySummary()
	}
}

func populateAttributes(dest pcommon.Map, labelKeys []string, labelValues []string) {
	for i := range labelKeys {
		dest.PutStr(labelKeys[i], labelValues[i])
	}
}
