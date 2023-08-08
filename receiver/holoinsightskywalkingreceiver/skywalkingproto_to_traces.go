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

package holoinsightskywalkingreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/skywalkingreceiver"

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"google.golang.org/grpc/metadata"
	"reflect"
	"strconv"
	"strings"
	"time"
	"unsafe"

	"github.com/google/uuid"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	conventions "go.opentelemetry.io/collector/semconv/v1.8.0"
	common "skywalking.apache.org/repo/goapi/collect/common/v3"
	agentV3 "skywalking.apache.org/repo/goapi/collect/language/agent/v3"
)

const (
	Tenant                             = "tenant"
	AttributeRefType                   = "refType"
	AttributeParentService             = "parent.service"
	AttributeParentInstance            = "parent.service.instance.id"
	AttributeParentInstanceName        = "parent.service.instance.name"
	AttributeInstance                  = "service.instance.name"
	AttributeParentEndpoint            = "parent.endpoint"
	AttributeSkywalkingSpanID          = "sw8.span_id"
	AttributeSkywalkingTraceID         = "sw8.trace_id"
	AttributeSkywalkingSegmentID       = "sw8.segment_id"
	AttributeSkywalkingParentSpanID    = "sw8.parent_span_id"
	AttributeSkywalkingParentSegmentID = "sw8.parent_segment_id"
	AttributeSkywalkingComponentName   = "sw8.component"
	AttributeNetworkAddressUsedAtPeer  = "network.AddressUsedAtPeer"
	AttributeSpanLayer                 = "spanLayer"
	ExtendTags                         = "extend_tags"
)

var otSpanTagsMapping = map[string]string{
	"url":             conventions.AttributeHTTPURL,
	"status_code":     conventions.AttributeHTTPStatusCode,
	"rpc.status_code": conventions.AttributeRPCGRPCStatusCode,
	"db.instance":     conventions.AttributeDBName,
	"mq.broker":       conventions.AttributeMessagingURL,
	"mq.topic":        conventions.AttributeMessagingDestination,
}

func SkywalkingToTraces(segment *agentV3.SegmentObject, md metadata.MD) ptrace.Traces {
	traceData := ptrace.NewTraces()

	swSpans := segment.Spans
	if swSpans == nil && len(swSpans) == 0 {
		return traceData
	}

	resourceSpan := traceData.ResourceSpans().AppendEmpty()
	rs := resourceSpan.Resource()

	// for _, span := range swSpans {
	//	swTagsToInternalResource(span, rs)
	//}

	if vs, ok := md[Tenant]; ok && len(vs) > 0 {
		rs.Attributes().PutStr(Tenant, vs[0])
		delete(md, Tenant)
	}
	rs.Attributes().PutStr(conventions.AttributeServiceName, segment.GetService())
	rs.Attributes().PutStr(conventions.AttributeServiceInstanceID, segment.GetServiceInstance())
	rs.Attributes().PutStr(AttributeSkywalkingTraceID, segment.GetTraceId())
	rs.Attributes().PutStr(conventions.AttributeNetHostIP, swServiceInstanceToIP(segment.GetServiceInstance()))
	rs.Attributes().PutStr(AttributeInstance, swServiceInstanceToIP(segment.GetServiceInstance()))

	il := resourceSpan.ScopeSpans().AppendEmpty()
	swSpansToSpanSlice(segment.GetTraceId(), segment.GetTraceSegmentId(), swSpans, il.Spans(), md)

	return traceData
}

func swServiceInstanceToIP(serviceInstance string) string {
	if strings.Contains(serviceInstance, "@") {
		return strings.Split(serviceInstance, "@")[1]
	}
	return serviceInstance
}

// func swTagsToInternalResource(span *agentV3.SpanObject, dest pcommon.Resource) {
//	if span == nil {
//		return
//	}
//
//	attrs := dest.Attributes()
//	attrs.Clear()
//
//	tags := span.Tags
//	if tags == nil {
//		return
//	}
//
//	for _, tag := range tags {
//		otKey, ok := otSpanTagsMapping[tag.Key]
//		if ok {
//			attrs.PutStr(otKey, tag.Value)
//		}
//	}
//}

func swSpansToSpanSlice(traceID string, segmentID string, spans []*agentV3.SpanObject, dest ptrace.SpanSlice, md metadata.MD) {
	if len(spans) == 0 {
		return
	}

	dest.EnsureCapacity(len(spans))
	for _, span := range spans {
		if span == nil {
			continue
		}
		swSpanToSpan(traceID, segmentID, span, dest.AppendEmpty(), md)
	}
}

func swSpanToSpan(traceID string, segmentID string, span *agentV3.SpanObject, dest ptrace.Span, md metadata.MD) {
	dest.SetTraceID(swTraceIDToTraceID(traceID))
	// skywalking defines segmentId + spanId as unique identifier
	// so use segmentId to convert to an unique otel-span
	dest.SetSpanID(segmentIDToSpanID(segmentID, uint32(span.GetSpanId())))

	// parent spanid = -1, means(root span) no parent span in skywalking,so just make otlp's parent span id empty.
	if span.ParentSpanId != -1 {
		dest.SetParentSpanID(segmentIDToSpanID(segmentID, uint32(span.GetParentSpanId())))
	}

	dest.SetName(span.OperationName)
	dest.SetStartTimestamp(microsecondsToTimestamp(span.GetStartTime()))
	dest.SetEndTimestamp(microsecondsToTimestamp(span.GetEndTime()))

	attrs := dest.Attributes()
	attrs.EnsureCapacity(len(span.Tags))
	swKvPairsToInternalAttributes(span.Tags, attrs)

	// drop the attributes slice if all of them were replaced during translation
	if attrs.Len() == 0 {
		attrs.Clear()
	}

	// {"custom_tag1":"xx", "custom_tag2":"xx"}
	// custom tags will be added to span tags
	if vs, ok := md[ExtendTags]; ok && len(vs) > 0 {
		m := make(map[string]string)
		json.Unmarshal([]byte(vs[0]), &m) //nolint
		for k, v := range m {
			attrs.PutStr(k, v)
		}
	}

	attrs.PutStr(AttributeSpanLayer, span.SpanLayer.String())
	attrs.PutStr(conventions.AttributeNetPeerName, span.GetPeer())
	attrs.PutStr(AttributeSkywalkingSegmentID, segmentID)
	setSwSpanIDToAttributes(span, attrs)
	setInternalSpanStatus(span, dest.Status())
	componentName := getServerNameBasedOnComponent(span.GetComponentId())
	attrs.PutStr(AttributeSkywalkingComponentName, componentName)

	switch {
	case span.SpanLayer == agentV3.SpanLayer_MQ:
		attrs.PutStr(conventions.AttributeMessagingSystem, componentName)
		if span.SpanType == agentV3.SpanType_Entry {
			dest.SetKind(ptrace.SpanKindConsumer)
		} else if span.SpanType == agentV3.SpanType_Exit {
			dest.SetKind(ptrace.SpanKindProducer)
		}
	case span.SpanLayer == agentV3.SpanLayer_Cache:
		attrs.PutStr(conventions.AttributeDBSystem, componentName)
		fallthrough
	case span.SpanLayer == agentV3.SpanLayer_Database:
		attrs.PutStr(conventions.AttributeDBSystem, componentName)
		fallthrough
	case span.GetSpanType() == agentV3.SpanType_Exit:
		dest.SetKind(ptrace.SpanKindClient)
	case span.GetSpanType() == agentV3.SpanType_Entry:
		dest.SetKind(ptrace.SpanKindServer)
	case span.GetSpanType() == agentV3.SpanType_Local:
		dest.SetKind(ptrace.SpanKindInternal)
	default:
		dest.SetKind(ptrace.SpanKindUnspecified)
	}

	swLogsToSpanEvents(span.GetLogs(), dest.Events())
	// skywalking: In the across thread and across processes, these references target the parent segments.
	swReferencesToSpanLinks(span.Refs, dest)
}

func swReferencesToSpanLinks(refs []*agentV3.SegmentReference, dest ptrace.Span) {
	if len(refs) == 0 {
		return
	}
	links := dest.Links()
	links.EnsureCapacity(len(refs))

	for _, ref := range refs {
		link := links.AppendEmpty()
		link.SetTraceID(swTraceIDToTraceID(ref.TraceId))
		if dest.ParentSpanID().IsEmpty() {
			dest.SetParentSpanID(segmentIDToSpanID(ref.ParentTraceSegmentId, uint32(ref.ParentSpanId)))
		}
		link.SetSpanID(segmentIDToSpanID(ref.ParentTraceSegmentId, uint32(ref.ParentSpanId)))
		link.TraceState().FromRaw("")
		kvParis := []*common.KeyStringValuePair{
			{
				Key:   AttributeParentService,
				Value: ref.ParentService,
			},
			{
				Key:   AttributeParentInstance,
				Value: ref.ParentServiceInstance,
			},
			{
				Key:   AttributeParentEndpoint,
				Value: ref.ParentEndpoint,
			},
			{
				Key:   AttributeNetworkAddressUsedAtPeer,
				Value: ref.NetworkAddressUsedAtPeer,
			},
			{
				Key:   AttributeRefType,
				Value: ref.RefType.String(),
			},
			{
				Key:   AttributeSkywalkingTraceID,
				Value: ref.TraceId,
			},
			{
				Key:   AttributeSkywalkingParentSegmentID,
				Value: ref.ParentTraceSegmentId,
			},
			{
				Key:   AttributeSkywalkingParentSpanID,
				Value: strconv.Itoa(int(ref.ParentSpanId)),
			},
			{
				Key:   AttributeParentInstanceName,
				Value: swServiceInstanceToIP(ref.ParentServiceInstance),
			},
		}
		swKvPairsToInternalAttributes(kvParis, link.Attributes())
	}
}

func setInternalSpanStatus(span *agentV3.SpanObject, dest ptrace.Status) {
	if span.GetIsError() {
		dest.SetCode(ptrace.StatusCodeError)
		dest.SetMessage("ERROR")
	} else {
		dest.SetCode(ptrace.StatusCodeOk)
		dest.SetMessage("SUCCESS")
	}
}

func setSwSpanIDToAttributes(span *agentV3.SpanObject, dest pcommon.Map) {
	dest.PutInt(AttributeSkywalkingSpanID, int64(span.GetSpanId()))
	if span.ParentSpanId != -1 {
		dest.PutInt(AttributeSkywalkingParentSpanID, int64(span.GetParentSpanId()))
	}
}

func swLogsToSpanEvents(logs []*agentV3.Log, dest ptrace.SpanEventSlice) {
	if len(logs) == 0 {
		return
	}
	dest.EnsureCapacity(len(logs))

	for i, log := range logs {
		var event ptrace.SpanEvent
		if dest.Len() > i {
			event = dest.At(i)
		} else {
			event = dest.AppendEmpty()
		}

		event.SetName("logs")
		event.SetTimestamp(microsecondsToTimestamp(log.GetTime()))
		if len(log.GetData()) == 0 {
			continue
		}

		attrs := event.Attributes()
		attrs.Clear()
		attrs.EnsureCapacity(len(log.GetData()))
		swKvPairsToInternalAttributes(log.GetData(), attrs)
	}
}

func swKvPairsToInternalAttributes(pairs []*common.KeyStringValuePair, dest pcommon.Map) {
	if pairs == nil {
		return
	}

	for _, pair := range pairs {
		otKey, ok := otSpanTagsMapping[pair.Key]
		if ok {
			dest.PutStr(otKey, pair.Value)
		} else {
			dest.PutStr(pair.Key, pair.Value)
		}
	}
}

// microsecondsToTimestamp converts epoch microseconds to pcommon.Timestamp
func microsecondsToTimestamp(ms int64) pcommon.Timestamp {
	return pcommon.NewTimestampFromTime(time.UnixMilli(ms))
}

func swTraceIDToTraceID(traceID string) pcommon.TraceID {
	// skywalking traceid format:
	// de5980b8-fce3-4a37-aab9-b4ac3af7eedd: from browser/js-sdk/envoy/nginx-lua sdk/py-agent
	// 56a5e1c519ae4c76a2b8b11d92cead7f.12.16563474296430001: from java-agent

	// just converts to [16]byte
	if len(traceID) == 32 {
		t := unsafeGetBytes(traceID)
		var bTraceID [16]byte
		_, err := hex.Decode(bTraceID[:], t)
		if err != nil {
			return bTraceID
		}
		return bTraceID
	}

	if len(traceID) <= 36 { // 36: uuid length (rfc4122)
		uid, err := uuid.Parse(traceID)
		if err != nil {
			return pcommon.NewTraceIDEmpty()
		}
		return pcommon.TraceID(uid)
	}
	return swStringToUUID(traceID, 0)
}

func segmentIDToSpanID(segmentID string, spanID uint32) pcommon.SpanID {
	// skywalking segmentid format:
	// 56a5e1c519ae4c76a2b8b11d92cead7f.12.16563474296430001: from TraceSegmentId
	// 56a5e1c519ae4c76a2b8b11d92cead7f: from ParentTraceSegmentId

	if len(segmentID) < 32 {
		return pcommon.NewSpanIDEmpty()
	}
	return uuidTo8Bytes(swStringToUUID(segmentID, spanID))
}

func swStringToUUID(s string, extra uint32) (dst [16]byte) {
	// there are 2 possible formats for 's':
	// s format = 56a5e1c519ae4c76a2b8b11d92cead7f.0000000000.000000000000000000
	//            ^ start(length=32)               ^ mid(u32) ^ last(u64)
	// uid = UUID(start) XOR ([4]byte(extra) . [4]byte(uint32(mid)) . [8]byte(uint64(last)))

	// s format = 56a5e1c519ae4c76a2b8b11d92cead7f
	//            ^ start(length=32)
	// uid = UUID(start) XOR [4]byte(extra)

	if len(s) < 32 {
		return
	}

	t := unsafeGetBytes(s)
	var uid [16]byte
	_, err := hex.Decode(uid[:], t[:32])
	if err != nil {
		return uid
	}

	for i := 0; i < 4; i++ {
		uid[i] ^= byte(extra)
		extra >>= 8
	}

	if len(s) == 32 {
		return uid
	}

	index1 := bytes.IndexByte(t, '.')
	index2 := bytes.LastIndexByte(t, '.')
	if index1 != 32 || index2 < 0 {
		return
	}

	mid, err := strconv.Atoi(s[index1+1 : index2])
	// fix spanId repeat
	mid += int(extra)
	if err != nil {
		return
	}

	last, err := strconv.Atoi(s[index2+1:])
	if err != nil {
		return
	}

	for i := 4; i < 8; i++ {
		uid[i] ^= byte(mid)
		mid >>= 8
	}

	for i := 8; i < 16; i++ {
		uid[i] ^= byte(last)
		last >>= 8
	}

	return uid
}

func uuidTo8Bytes(uuid [16]byte) [8]byte {
	// high bit XOR low bit
	// XOR has a probability to generate the same id
	//var dst [8]byte
	//for i := 0; i < 8; i++ {
	//	dst[i] = uuid[i] ^ uuid[i+8]
	//}

	var dst [8]byte
	hash := sha256.Sum256(uuid[:])
	copy(dst[:], hash[:8])
	return dst
}

func unsafeGetBytes(s string) []byte {
	return (*[0x7fff0000]byte)(unsafe.Pointer(
		(*reflect.StringHeader)(unsafe.Pointer(&s)).Data),
	)[:len(s):len(s)]
}
