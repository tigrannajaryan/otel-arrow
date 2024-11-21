package otlpconvert

import (
	"bytes"
	"io"

	"github.com/tigrannajaryan/stef/tef-otlp/sortedbymetric"
	"github.com/tigrannajaryan/stef/tef/pkg"
	"github.com/tigrannajaryan/stef/tef/tefgen/example"
	metricspb "go.opentelemetry.io/collector/pdata/pmetric"

	otlpconvert "github.com/tigrannajaryan/stef/tef-otlp"
	"github.com/tigrannajaryan/stef/tef/types"
)

type STEFEncoding struct {
	Opts pkg.WriterOptions
}

func (d *STEFEncoding) FromOTLP(data metricspb.Metrics) (*sortedbymetric.SortedTree, error) {
	converter := otlpconvert.NewOtlpToSortedTree()
	return converter.FromOtlp(data.ResourceMetrics())
}

func (d *STEFEncoding) Encode(sorted *sortedbymetric.SortedTree, writer *example.Writer) error {
	if err := sorted.ToTef(writer); err != nil {
		return err
	}
	return writer.Flush()
}

func (d *STEFEncoding) Decode(b []byte) (any, error) {
	buf := bytes.NewBuffer(b)
	r, err := example.NewReader(buf)
	if err != nil {
		return nil, err
	}

	for {
		readRecord, err := r.Next()
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
