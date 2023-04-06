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

package metrics

import (
	"testing"

	"github.com/lightstep/go-expohisto/structure"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func TestConnector_ExpoHistToExponentialDataPoint(t *testing.T) {
	tests := []struct {
		name  string
		input *structure.Histogram[float64]
		want  pmetric.ExponentialHistogramDataPoint
	}{
		{
			name:  "max bucket size - 4",
			input: structure.NewFloat64(structure.NewConfig(structure.WithMaxSize(4)), 2, 4),
			want: func() pmetric.ExponentialHistogramDataPoint {
				dp := pmetric.NewExponentialHistogramDataPoint()
				dp.SetCount(2)
				dp.SetSum(6)
				dp.SetMin(2)
				dp.SetMax(4)
				dp.SetZeroCount(0)
				dp.SetScale(1)
				dp.Positive().SetOffset(1)
				dp.Positive().BucketCounts().FromRaw([]uint64{
					1, 0, 1,
				})
				return dp
			}(),
		},
		{
			name:  "max bucket size - default",
			input: structure.NewFloat64(structure.NewConfig(), 2, 4),
			want: func() pmetric.ExponentialHistogramDataPoint {
				dp := pmetric.NewExponentialHistogramDataPoint()
				dp.SetCount(2)
				dp.SetSum(6)
				dp.SetMin(2)
				dp.SetMax(4)
				dp.SetZeroCount(0)
				dp.SetScale(7)
				dp.Positive().SetOffset(127)
				buckets := make([]uint64, 129)
				buckets[0] = 1
				buckets[128] = 1
				dp.Positive().BucketCounts().FromRaw(buckets)
				return dp
			}(),
		},
		{
			name:  "max bucket size - 4, negative observations",
			input: structure.NewFloat64(structure.NewConfig(structure.WithMaxSize(4)), -2, -4),
			want: func() pmetric.ExponentialHistogramDataPoint {
				dp := pmetric.NewExponentialHistogramDataPoint()
				dp.SetCount(2)
				dp.SetSum(-6)
				dp.SetMin(-4)
				dp.SetMax(-2)
				dp.SetZeroCount(0)
				dp.SetScale(1)
				dp.Negative().SetOffset(1)
				dp.Negative().BucketCounts().FromRaw([]uint64{
					1, 0, 1,
				})
				return dp
			}(),
		},
		{
			name:  "max bucket size - 4, negative and positive observations",
			input: structure.NewFloat64(structure.NewConfig(structure.WithMaxSize(4)), 2, 4, -2, -4),
			want: func() pmetric.ExponentialHistogramDataPoint {
				dp := pmetric.NewExponentialHistogramDataPoint()
				dp.SetCount(4)
				dp.SetSum(0)
				dp.SetMin(-4)
				dp.SetMax(4)
				dp.SetZeroCount(0)
				dp.SetScale(1)
				dp.Positive().SetOffset(1)
				dp.Positive().BucketCounts().FromRaw([]uint64{
					1, 0, 1,
				})
				dp.Negative().SetOffset(1)
				dp.Negative().BucketCounts().FromRaw([]uint64{
					1, 0, 1,
				})
				return dp
			}(),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pmetric.NewExponentialHistogramDataPoint()
			expoHistToExponentialDataPoint(tt.input, got)
			assert.Equal(t, tt.want, got)
		})
	}
}
