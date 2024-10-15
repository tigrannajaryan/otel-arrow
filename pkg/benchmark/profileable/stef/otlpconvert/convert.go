package otlpconvert

import (
	"bytes"
	"io"

	otlpconvert "github.com/tigrannajaryan/stef/stef-otlp"
	"github.com/tigrannajaryan/stef/stef-otlp/sortedbymetric"
	metricspb "go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/tigrannajaryan/stef/stef-go/metrics"
	"github.com/tigrannajaryan/stef/stef-go/types"
	_ "github.com/tigrannajaryan/stef/stef-otlp"
)

type STEFEncoding struct {
	Opts metrics.WriterOptions
}

func (d *STEFEncoding) FromOTLP(data metricspb.Metrics) (*sortedbymetric.SortedTree, error) {
	converter := otlpconvert.NewOtlpToSortedTree()
	return converter.FromOtlp(data.ResourceMetrics())
}

func (d *STEFEncoding) Encode(sorted *sortedbymetric.SortedTree, writer *metrics.Writer) error {
	if err := sorted.ToStef(writer); err != nil {
		return err
	}
	return writer.Flush()
}

func (d *STEFEncoding) Decode(b []byte) (any, error) {
	buf := bytes.NewBuffer(b)
	r, err := metrics.NewReader(buf)
	if err != nil {
		return nil, err
	}

	for {
		readRecord, err := r.Read()
		if err == io.EOF {
			break
		}
		if readRecord == nil {
			panic("nil record")
		}
		if err != nil {
			return nil, err
		}
	}

	return nil, nil
}

func (e *STEFEncoding) Name() string {
	str := "STEF"
	if e.Opts.Compression != types.CompressionNone {
		str += "Z"
	}
	return str
}
