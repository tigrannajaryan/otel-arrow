package otlpconvert

import (
	"log"

	otlpmetrics "go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/tigrannajaryan/stef/stef-go/anyvalue"
	"github.com/tigrannajaryan/stef/stef-go/types"
)

type TreeConverter struct {
	recordCount         int
	emptyDataPointCount int
	tempAttrs           types.AttrList
	encoder             anyvalue.Encoder
}

func NewTreeConverter() *TreeConverter {
	return &TreeConverter{}
}

func (c *TreeConverter) Otlp2Tree(rms otlpmetrics.ResourceMetricsSlice) *SortedMetrics {
	sm := NewSortedMetrics()

	for i := 0; i < rms.Len(); i++ {
		rm := rms.At(i)

		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			sms := rm.ScopeMetrics().At(j)
			for k := 0; k < sms.Metrics().Len(); k++ {
				metric := sms.Metrics().At(k)

				switch metric.Type() {
				case otlpmetrics.MetricTypeGauge:
					c.covertNumberDataPoints(sm, rm, sms, metric, metric.Gauge().DataPoints(), 0)
				case otlpmetrics.MetricTypeSum:
					c.covertNumberDataPoints(
						sm, rm, sms, metric, metric.Sum().DataPoints(),
						calcMetricFlags(metric.Sum().IsMonotonic(), metric.Sum().AggregationTemporality()),
					)
				case otlpmetrics.MetricTypeHistogram:
					c.covertHistogramDataPoints(sm, rm, sms, metric, metric.Histogram())
				//case otlpmetrics.MetricTypeExponentialHistogram:
				//	log.Printf("Exponential histograms are not yet supported")
				default:
					log.Fatalf("Unsupported metric type: %v\n", metric.Type())
				}
			}
		}
	}

	sm.SortValues()

	return sm
}

func calcMetricFlags(monotonic bool, temporality otlpmetrics.AggregationTemporality) types.MetricFlags {
	var flags types.MetricFlags
	if monotonic {
		flags |= types.MetricMonotonic
	}
	if temporality == otlpmetrics.AggregationTemporalityCumulative {
		flags |= types.MetricCumulative
	}
	return flags
}

func (c *TreeConverter) numberDataPointToTimedValue(out *types.TimedValue, dp otlpmetrics.NumberDataPoint) {
	switch dp.ValueType() {
	case otlpmetrics.NumberDataPointValueTypeInt:
		out.Int64Vals = append(out.Int64Vals[:0], dp.IntValue())
	case otlpmetrics.NumberDataPointValueTypeDouble:
		out.Float64Vals = append(out.Float64Vals[:0], dp.DoubleValue())
	default:
		log.Fatalf("Unsupported value type: %v", dp)
	}
}

func (c *TreeConverter) covertNumberDataPoints(
	sm *SortedMetrics,
	rm otlpmetrics.ResourceMetrics,
	sms otlpmetrics.ScopeMetrics,
	metric otlpmetrics.Metric,
	dps otlpmetrics.NumberDataPointSlice,
	flags types.MetricFlags,
) {
	var metricType types.MetricType
	var byMetric *ByMetric
	var byScope *ByScope
	for l := 0; l < dps.Len(); l++ {
		dp := dps.At(l)

		if dp.ValueType() == otlpmetrics.NumberDataPointValueTypeEmpty {
			c.emptyDataPointCount++
			continue
		}

		c.recordCount++

		mt := calcNumericMetricType(metric, dp)
		if mt != metricType || byMetric == nil {
			metricType = mt
			byMetric = sm.ByMetric(metric, metricType, flags, nil)
			byResource := byMetric.ByResource(rm.Resource(), rm.SchemaUrl())
			byScope = byResource.ByScope(sms.Scope(), sms.SchemaUrl())
		}

		MapToSortedAttrs(dp.Attributes(), &c.encoder, &c.tempAttrs)
		values := byScope.ByAttrs(c.tempAttrs)
		value := values.Append()
		value.TimestampUnixNano = uint64(dp.Timestamp())
		value.StartTimestampUnixNano = uint64(dp.StartTimestamp())
		value.Exemplars = c.exemplarsToExemplars(dp.Exemplars())

		c.numberDataPointToTimedValue(value, dp)
	}
}

func (c *TreeConverter) exemplarsToExemplars(exemplars otlpmetrics.ExemplarSlice) []types.Exemplar {
	ret := make([]types.Exemplar, 0, exemplars.Len())
	for i := 0; i < exemplars.Len(); i++ {
		src := exemplars.At(i)
		MapToSortedAttrs(src.FilteredAttributes(), &c.encoder, &c.tempAttrs)
		ret = append(
			ret, types.Exemplar{
				TimestampUnixNano: uint64(src.Timestamp()),
				FilteredAttrs:     c.tempAttrs.Clone(),
				SpanId:            types.AttrValue(src.TraceID().String()),
				TraceId:           types.AttrValue(src.SpanID().String()),
			},
		)
		dst := &ret[len(ret)-1]
		switch src.ValueType() {
		case otlpmetrics.ExemplarValueTypeInt:
			dst.Int64Val = src.IntValue()
			dst.ValueType = types.ExemplarValueTypeInt64
		case otlpmetrics.ExemplarValueTypeDouble:
			dst.Float64Val = src.DoubleValue()
			dst.ValueType = types.ExemplarValueTypeFloat64
		case otlpmetrics.ExemplarValueTypeEmpty:
			dst.ValueType = types.ExemplarValueTypeEmpty
		default:
			panic("unknown exemplar value type")
		}
	}
	return ret
}

func calcNumericMetricType(metric otlpmetrics.Metric, dp otlpmetrics.NumberDataPoint) types.MetricType {
	switch metric.Type() {
	case otlpmetrics.MetricTypeGauge:
		switch dp.ValueType() {
		case otlpmetrics.NumberDataPointValueTypeInt:
			return types.GaugeInt64
		case otlpmetrics.NumberDataPointValueTypeDouble:
			return types.GaugeFloat64
		default:
			log.Fatalf("Unsupported value type: %v", dp)
		}
	case otlpmetrics.MetricTypeSum:
		switch dp.ValueType() {
		case otlpmetrics.NumberDataPointValueTypeInt:
			return types.SumInt64
		case otlpmetrics.NumberDataPointValueTypeDouble:
			return types.SumFloat64
		default:
			log.Fatalf("Unsupported value type: %v", dp)
		}
	default:
		log.Fatalf("Unsupported value type: %v", metric.Type())
	}
	return 0
}

func (c *TreeConverter) covertHistogramDataPoints(
	sm *SortedMetrics,
	rm otlpmetrics.ResourceMetrics,
	sms otlpmetrics.ScopeMetrics,
	metric otlpmetrics.Metric,
	hist otlpmetrics.Histogram,
) {
	var byMetric *ByMetric
	var byScope *ByScope
	flags := calcMetricFlags(false, hist.AggregationTemporality())
	dps := hist.DataPoints()

	// We keep track of the last float64 encoded. When a value is missing (e.g. Sum is missing)
	// we put the last value instead. The value will be ignored by reader by looking at the
	// presenceMask, however by using the last value we improve compression ratio since
	// repeated values compress very well (they use only 1 bit).
	//lastFloat64Val := float64(0)

	for l := 0; l < dps.Len(); l++ {
		dp := dps.At(l)

		c.recordCount++

		byMetric = sm.ByMetric(metric, types.Histogram, flags, dp.ExplicitBounds().AsRaw())
		byResource := byMetric.ByResource(rm.Resource(), rm.SchemaUrl())
		byScope = byResource.ByScope(sms.Scope(), sms.SchemaUrl())
		MapToSortedAttrs(dp.Attributes(), &c.encoder, &c.tempAttrs)
		values := byScope.ByAttrs(c.tempAttrs)
		value := values.Append()
		value.TimestampUnixNano = uint64(dp.Timestamp())
		value.StartTimestampUnixNano = uint64(dp.StartTimestamp())
		value.Exemplars = c.exemplarsToExemplars(dp.Exemplars())

		// Calculate field presence bits and compose Float64Vals
		value.Float64Vals = []float64{}
		var presenceMask types.HistogramFieldPresenceMask
		if dp.HasSum() {
			presenceMask |= types.HistogramHasSum
			value.Float64Vals = append(value.Float64Vals, dp.Sum())
			//lastFloat64Val = *dp.Sum
		} else {
			value.Float64Vals = append(value.Float64Vals, 0)
		}
		if dp.HasMin() {
			presenceMask |= types.HistogramHasMin
			value.Float64Vals = append(value.Float64Vals, dp.Min())
			//lastFloat64Val = *dp.Min
		} else {
			value.Float64Vals = append(value.Float64Vals, 0)
		}
		if dp.HasMax() {
			presenceMask |= types.HistogramHasMax
			value.Float64Vals = append(value.Float64Vals, dp.Max())
			//lastFloat64Val = *dp.Max
		} else {
			value.Float64Vals = append(value.Float64Vals, 0)
		}

		// Compose Int64Vals
		value.Int64Vals = []int64{
			int64(presenceMask),
			int64(dp.Count()),
		}
		for i := 0; i < dp.BucketCounts().Len(); i++ {
			value.Int64Vals = append(value.Int64Vals, int64(dp.BucketCounts().At(i)))
		}
	}
}
