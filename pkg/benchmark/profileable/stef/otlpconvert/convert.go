package otlpconvert

import (
	"bytes"
	"io"

	metricspb "go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/tigrannajaryan/stef/metrics"
	"github.com/tigrannajaryan/stef/types"
)

type STEFEncoding struct {
	Opts metrics.WriterOptions
}

func (d *STEFEncoding) FromOTLP(data metricspb.Metrics) *SortedMetrics {
	converter := NewTreeConverter()
	return converter.Otlp2Tree(data.ResourceMetrics())
}

func (d *STEFEncoding) Encode(sorted *SortedMetrics, writer *metrics.Writer) error {
	sorted.Iter(
		func(metric *types.Metric, byMetric *ByMetric) error {
			if err := writer.WriteMetric(metric); err != nil {
				return err
			}
			byMetric.Iter(
				func(resource *types.Resource, byResource *ByResource) error {
					if err := writer.WriteResource(resource); err != nil {
						return err
					}
					byResource.Iter(
						func(scope *types.Scope, byScope *ByScope) error {
							if err := writer.WriteScope(scope); err != nil {
								return err
							}
							byScope.Iter(
								func(attrs types.AttrList, values *types.TimedValues) error {
									if err := writer.StartAttrs(attrs); err != nil {
										return err
									}
									for _, value := range values.Values() {
										if err := writer.WriteValue(&value); err != nil {
											return err
										}
									}

									//return writer.endAttrs()
									return nil
								},
							)
							return nil
						},
					)
					return nil
				},
			)
			return nil
		},
	)
	writer.Flush()
	return nil
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
