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

package otlpdict

import (
	"io"

	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/otel-arrow/pkg/benchmark"
	"github.com/open-telemetry/otel-arrow/pkg/benchmark/dataset"

	"github.com/tigrannajaryan/stef/metrics"
)

type MetricsProfileable struct {
	compression benchmark.CompressionAlgorithm
	dataset     dataset.MetricsDataset
	//metrics     []pmetric.Metrics

	// Next batch to encode. The result goes to nextBatchToSerialize.
	nextBatchToEncode []pmetric.Metrics

	// Next batch to serialize. The result goes to byte buffers.
	nextBatchToSerialize []metrics.Records

	// Keep all sent traces for verification after delivery.
	allSentMetrics [][]pmetric.Metrics

	// Counts the number of traces received. Indexes into allSentMetrics so that we can
	// compare sent against received.
	rcvMetricIdx int

	// Unary or streaming mode.
	unaryRpcMode bool

	// Sender's dictionaries.
	//sKeyDict        sendDict
	//sMetricNameDict sendDict
	//sValDict        sendDict

	// Receiver's cumulative dictionaries.
	//rKeyDict        []string
	//rMetricNameDict []string
	//rValDict        []string

	// A flag to compare sent and received data.
	verifyDelivery bool

	// Stores deserialized data that needs to be decoded.
	rcvMetrics []metrics.Records
}

func NewMetricsProfileable(
	dataset dataset.MetricsDataset, compression benchmark.CompressionAlgorithm,
) *MetricsProfileable {
	return &MetricsProfileable{dataset: dataset, compression: compression}
}

func (s *MetricsProfileable) Name() string {
	return "OTLP DICT"
}

func (s *MetricsProfileable) Tags() []string {
	modeStr := "unary rpc"
	if !s.unaryRpcMode {
		modeStr = "stream mode"
	}
	return []string{s.compression.String(), modeStr}
}

func (s *MetricsProfileable) DatasetSize() int { return s.dataset.Len() }

func (s *MetricsProfileable) CompressionAlgorithm() benchmark.CompressionAlgorithm {
	return s.compression
}

func (s *MetricsProfileable) StartProfiling(io.Writer) {
	s.resetCumulativeDicts()
}

func (s *MetricsProfileable) EndProfiling(io.Writer) {}

func (s *MetricsProfileable) InitBatchSize(_ io.Writer, _ int) {}

func (s *MetricsProfileable) PrepareBatch(_ io.Writer, startAt, size int) {
	s.nextBatchToEncode = s.dataset.Metrics(startAt, size)
	s.allSentMetrics = append(s.allSentMetrics, s.nextBatchToEncode)
}

func (s *MetricsProfileable) resetCumulativeDicts() {
	// Note: we don't use string with ref index 0, so we initialize maps on both ends to
	// avoid using the 0 index in the payload.

	//s.sKeyDict.cum = map[string]uint64{"": 0}
	//s.sMetricNameDict.cum = map[string]uint64{"": 0}
	//s.sValDict.cum = map[string]uint64{"": 0}
	//s.rKeyDict = []string{""}
	//s.rValDict = []string{""}
	//s.rMetricNameDict = []string{""}
}

func (s *MetricsProfileable) ConvertOtlpToOtlpArrow(_ io.Writer, _, _ int) {
	// In the standard OTLP exporter the incoming messages are already OTLP messages,
	// so we don't need to create or convert them.
	//s.sKeyDict.delta = map[string]uint64{}
	//s.sValDict.delta = map[string]uint64{}
	//s.sMetricNameDict.delta = map[string]uint64{}

	if s.unaryRpcMode {
		s.resetCumulativeDicts()
	}

	s.nextBatchToSerialize = nil
	//for _, metricReq := range s.nextBatchToEncode {
	//	// Make a copy of metricReq. In production implementation this is will be unnecessary
	//	// but we are forced to do it here since the profiler's source data is in the old OTLP format.
	//
	//	// First marshal it to bytes.
	//	r := pmetricotlp.NewExportRequestFromMetrics(metricReq)
	//	bytes, err := r.MarshalProto()
	//	if err != nil {
	//		panic(err)
	//	}
	//	// Then unmarshal from bytes to OTLP DICT's Protobuf message.
	//	destMetrics := &otlpdictmetrics.ExportMetricsServiceRequest{}
	//	err = proto.Unmarshal(bytes, destMetrics)
	//	if err != nil {
	//		panic(err)
	//	}
	//
	//	s.nextBatchToSerialize = append(s.nextBatchToSerialize, destMetrics)
	//
	//	// Now do the actual dictionary encoding. This is the part that needs to be benchmarked
	//	// separately in the future.
	//	rss := destMetrics.ResourceMetrics
	//	for i := 0; i < len(rss); i++ {
	//		rs := rss[i]
	//
	//		dictionizeAttrs(s.sKeyDict, s.sValDict, rs.Resource.Attributes)
	//
	//		ss := rs.ScopeMetrics
	//		for j := 0; j < len(ss); j++ {
	//			sps := ss[j].Metrics
	//			for k := 0; k < len(sps); k++ {
	//				metric := sps[k]
	//				dictionizeStr(s.sMetricNameDict, &metric.Name, &metric.NameRef)
	//				dictionizeStr(s.sMetricNameDict, &metric.Description, &metric.DescriptionRef)
	//
	//				switch v := metric.Data.(type) {
	//				case *v1.Metric_Sum:
	//					for _, dp := range v.Sum.DataPoints {
	//						dictionizeAttrs(s.sKeyDict, s.sValDict, dp.Attributes)
	//					}
	//				case *v1.Metric_Gauge:
	//					for _, dp := range v.Gauge.DataPoints {
	//						dictionizeAttrs(s.sKeyDict, s.sValDict, dp.Attributes)
	//					}
	//				case *v1.Metric_Histogram:
	//					for _, dp := range v.Histogram.DataPoints {
	//						dictionizeAttrs(s.sKeyDict, s.sValDict, dp.Attributes)
	//					}
	//				case *v1.Metric_ExponentialHistogram:
	//					for _, dp := range v.ExponentialHistogram.DataPoints {
	//						dictionizeAttrs(s.sKeyDict, s.sValDict, dp.Attributes)
	//					}
	//				default:
	//					panic("unimplemented metric type")
	//				}
	//			}
	//		}
	//	}
	//}
	//
	//// Include dictionaries in the first request of the batch.
	//s.nextBatchToSerialize[0].ValDict = dictToProto(s.sValDict)
	//s.nextBatchToSerialize[0].KeyDict = dictToProto(s.sKeyDict)
	//s.nextBatchToSerialize[0].MetricNameDict = dictToProto(s.sMetricNameDict)
}

func (s *MetricsProfileable) Process(io.Writer) string {
	// Not used in this benchmark
	return ""
}

func (s *MetricsProfileable) Serialize(io.Writer) ([][]byte, error) {
	buffers := make([][]byte, len(s.nextBatchToSerialize))
	//for i, t := range s.nextBatchToSerialize {
	//	bytes, err := proto.Marshal(t)
	//	if err != nil {
	//		return nil, err
	//	}
	//	buffers[i] = bytes
	//}

	return buffers, nil
}

func (s *MetricsProfileable) Deserialize(_ io.Writer, buffers [][]byte) {
	//s.rcvMetrics = make([]otlpdictmetrics.ExportMetricsServiceRequest, len(buffers))
	//
	//for i, b := range buffers {
	//	if err := proto.Unmarshal(b, &s.rcvMetrics[i]); err != nil {
	//		panic(err)
	//	}
	//}
}

func (s *MetricsProfileable) ConvertOtlpArrowToOtlp(_ io.Writer) {
	//for i := 0; i < len(s.rcvMetrics); i++ {
	//	deserializeDict(s.rcvMetrics[i].ValDict, &s.rValDict)
	//	deserializeDict(s.rcvMetrics[i].KeyDict, &s.rKeyDict)
	//	deserializeDict(s.rcvMetrics[i].MetricNameDict, &s.rMetricNameDict)
	//
	//	// Compare received data to sent to make sure the protocol works correctly.
	//	// This should be disabled in speed benchmarks since it is not part of normal
	//	// protocol operation.
	//	if s.verifyDelivery {
	//		//sentTraces := s.allSentMetrics[s.rcvMetricIdx]
	//		//s.rcvMetricIdx++
	//		//for _, srcTrace := range sentTraces {
	//		//if !s.equalTraces(srcTrace, s.rcvTraces[i]) {
	//		//	panic("sent and received traces are not equal")
	//		//}
	//		//}
	//	}
	//}
}

func (s *MetricsProfileable) Clear() {
	s.nextBatchToEncode = nil
}

func (s *MetricsProfileable) ShowStats() {}

func (s *MetricsProfileable) EnableUnaryRpcMode() {
	s.unaryRpcMode = true
}
