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

package trace // import "go.opentelemetry.io/collector/receiver/otlpreceiver/internal/trace"

import (
	"context"
	"fmt"
	"google.golang.org/grpc/metadata"

	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/obsreport"
	"go.opentelemetry.io/collector/pdata/ptrace/ptraceotlp"
)

const (
	dataFormatProtobuf = "protobuf"
	Tenant             = "tenant"
	AttributeInstance  = "service.instance.name"
	AttributeHostName  = "host.name"
)

// Receiver is the type used to handle spans from OpenTelemetry exporters.
type Receiver struct {
	ptraceotlp.UnimplementedGRPCServer
	nextConsumer consumer.Traces
	obsrecv      *obsreport.Receiver
}

// New creates a new Receiver reference.
func New(nextConsumer consumer.Traces, obsrecv *obsreport.Receiver) *Receiver {
	return &Receiver{
		nextConsumer: nextConsumer,
		obsrecv:      obsrecv,
	}
}

// Export implements the service Export traces func.
func (r *Receiver) Export(ctx context.Context, req ptraceotlp.ExportRequest) (ptraceotlp.ExportResponse, error) {
	td := req.Traces()
	// We need to ensure that it propagates the receiver name as a tag
	numSpans := td.SpanCount()
	if numSpans == 0 {
		return ptraceotlp.NewExportResponse(), nil
	}

	// get tenant
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		fmt.Printf("Grpc from metadata error!")
	}
	var tenant string
	if vs, ok := md["tenant"]; ok && len(vs) > 0 {
		tenant = vs[0]
	}
	rss := td.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		rs := rss.At(i)
		rs.Resource().Attributes().PutStr(Tenant, tenant)
		if hostName, ok := rs.Resource().Attributes().Get(AttributeHostName); ok {
			rs.Resource().Attributes().PutStr(AttributeInstance, hostName.Str())
		}
	}

	ctx = r.obsrecv.StartTracesOp(ctx)
	err := r.nextConsumer.ConsumeTraces(ctx, td)
	r.obsrecv.EndTracesOp(ctx, dataFormatProtobuf, numSpans, err)

	return ptraceotlp.NewExportResponse(), err
}
