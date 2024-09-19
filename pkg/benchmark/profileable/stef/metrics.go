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

package stef

import (
	"io"
	"log"

	"github.com/tigrannajaryan/stef/types"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/otel-arrow/pkg/benchmark"
	"github.com/open-telemetry/otel-arrow/pkg/benchmark/dataset"
	"github.com/open-telemetry/otel-arrow/pkg/benchmark/profileable/stef/otlpconvert"

	"github.com/tigrannajaryan/stef/metrics"
)

type MetricsProfileable struct {
	compression benchmark.CompressionAlgorithm
	dataset     dataset.MetricsDataset
	//metrics     []pmetric.Metrics

	writer *metrics.Writer

	// Next batch to encode. The result goes to nextBatchToSerialize.
	nextBatchToEncode []pmetric.Metrics

	// Next batch to serialize. The result goes to byte buffers.
	nextBatchToSerialize []*otlpconvert.SortedMetrics

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
	chunkWrter *chunkWriter
}

// chunkWriter is a ChunkWriter that accumulates chunks in a memory buffer.
type chunkWriter struct {
	chunks [][]byte
}

func (m *chunkWriter) WriteChunk(header []byte, content []byte) error {
	all := append(header, content...)
	m.chunks = append(m.chunks, all)
	return nil
}

func NewMetricsProfileable(
	dataset dataset.MetricsDataset, compression benchmark.CompressionAlgorithm,
) *MetricsProfileable {
	return &MetricsProfileable{dataset: dataset, compression: compression}
}

func (s *MetricsProfileable) Name() string {
	return "STEF"
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

	s.chunkWrter = &chunkWriter{}
	opts := metrics.WriterOptions{}
	if _, ok := s.compression.(*benchmark.ZstdCompressionAlgo); ok {
		opts.Compression = types.CompressionZstd
	}

	var err error
	s.writer, err = metrics.NewWriter(s.chunkWrter, opts)
	if err != nil {
		log.Fatalln(err)
	}
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

	stef := otlpconvert.STEFEncoding{}

	s.nextBatchToSerialize = nil
	for _, metricReq := range s.nextBatchToEncode {
		s.nextBatchToSerialize = append(s.nextBatchToSerialize, stef.FromOTLP(metricReq))
	}
}

func (s *MetricsProfileable) Process(io.Writer) string {
	// Not used in this benchmark
	return ""
}

func (s *MetricsProfileable) Serialize(io.Writer) ([][]byte, error) {
	stef := otlpconvert.STEFEncoding{}
	buffers := make([][]byte, len(s.nextBatchToSerialize))
	for i, sorted := range s.nextBatchToSerialize {
		err := stef.Encode(sorted, s.writer)
		if err != nil {
			return nil, err
		}

		var bytes []byte
		for _, chunk := range s.chunkWrter.chunks {
			bytes = append(bytes, chunk...)
		}
		s.chunkWrter.chunks = nil

		buffers[i] = bytes
	}

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
