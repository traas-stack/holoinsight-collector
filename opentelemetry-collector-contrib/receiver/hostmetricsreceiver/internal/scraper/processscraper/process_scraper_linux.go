// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:build linux
// +build linux

package processscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/processscraper"

import (
	"github.com/shirou/gopsutil/v3/cpu"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/processscraper/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/processscraper/ucal"
)

func (s *scraper) recordCPUTimeMetric(now pcommon.Timestamp, cpuTime *cpu.TimesStat) {
	s.mb.RecordProcessCPUTimeDataPoint(now, cpuTime.User, metadata.AttributeStateUser)
	s.mb.RecordProcessCPUTimeDataPoint(now, cpuTime.System, metadata.AttributeStateSystem)
	s.mb.RecordProcessCPUTimeDataPoint(now, cpuTime.Iowait, metadata.AttributeStateWait)
}

func (s *scraper) recordCPUUtilization(now pcommon.Timestamp, cpuUtilization ucal.CPUUtilization) {
	s.mb.RecordProcessCPUUtilizationDataPoint(now, cpuUtilization.User, metadata.AttributeStateUser)
	s.mb.RecordProcessCPUUtilizationDataPoint(now, cpuUtilization.System, metadata.AttributeStateSystem)
	s.mb.RecordProcessCPUUtilizationDataPoint(now, cpuUtilization.Iowait, metadata.AttributeStateWait)
}

func getProcessName(proc processHandle, _ string) (string, error) {
	name, err := proc.Name()
	if err != nil {
		return "", err
	}

	return name, err
}

func getProcessExecutable(proc processHandle) (string, error) {
	exe, err := proc.Exe()
	if err != nil {
		return "", err
	}

	return exe, nil
}

func getProcessCommand(proc processHandle) (*commandMetadata, error) {
	cmdline, err := proc.CmdlineSlice()
	if err != nil {
		return nil, err
	}

	var cmd string
	if len(cmdline) > 0 {
		cmd = cmdline[0]
	}

	command := &commandMetadata{command: cmd, commandLineSlice: cmdline}
	return command, nil
}
