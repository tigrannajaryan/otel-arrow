package otlpconvert

import (
	"io"
	"sort"
	"strings"

	otlpcommon "go.opentelemetry.io/collector/pdata/pcommon"
	otlpmetric "go.opentelemetry.io/collector/pdata/pmetric"
	//otlpresource "go.opentelemetry.io/collector/pdata/presource"
	"modernc.org/b/v2"

	"github.com/tigrannajaryan/stef/anyvalue"
	"github.com/tigrannajaryan/stef/types"
)

type SortedMetrics struct {
	encoder  anyvalue.Encoder
	byMetric *b.Tree[*types.Metric, *ByMetric]
}

type ByMetric struct {
	encoder    *anyvalue.Encoder
	byResource *b.Tree[*types.Resource, *ByResource]
}

type ByResource struct {
	encoder *anyvalue.Encoder
	byScope *b.Tree[*types.Scope, *ByScope]
}

type ByScope struct {
	byAttrs *b.Tree[types.AttrList, *types.TimedValues]
}

func NewSortedMetrics() *SortedMetrics {
	return &SortedMetrics{byMetric: b.TreeNew[*types.Metric, *ByMetric](cmpMetric)}
}

func cmpMetric(left, right *types.Metric) int {
	if left == nil {
		if right == nil {
			return 0
		}
		return -1
	}
	if right == nil {
		return 1
	}

	c := strings.Compare(left.Name, right.Name)
	if c != 0 {
		return c
	}
	c = strings.Compare(left.Description, right.Description)
	if c != 0 {
		return c
	}
	c = strings.Compare(left.Unit, right.Unit)
	if c != 0 {
		return c
	}
	c = int(left.Type) - int(right.Type)
	if c != 0 {
		return c
	}

	if left.Type == types.Histogram {
		c = cmpBounds(left.HistogramBounds, right.HistogramBounds)
		if c != 0 {
			return c
		}
	}

	c = int(left.Flags) - int(right.Flags)
	if c != 0 {
		return c
	}
	return cmpAttrs(left.Metadata, right.Metadata)
}

func cmpBounds(left []float64, right []float64) int {
	c := len(left) - len(right)
	if c != 0 {
		return c
	}
	for i := range left {
		fc := left[i] - right[i]
		if fc < 0 {
			return -1
		}
		if fc > 0 {
			return 1
		}
	}
	return 0
}

func (m *SortedMetrics) ByMetric(
	metric otlpmetric.Metric, metricType types.MetricType, flags types.MetricFlags,
	histogramBounds []float64,
) *ByMetric {
	metr := metric2metric(metric, metricType, flags, histogramBounds)
	elem, exists := m.byMetric.Get(metr)
	if !exists {
		elem = &ByMetric{encoder: &m.encoder, byResource: b.TreeNew[*types.Resource, *ByResource](types.CmpResource)}
		m.byMetric.Set(metr, elem)
	}
	return elem
}

func metric2metric(
	metric otlpmetric.Metric, metricType types.MetricType, flags types.MetricFlags, histogramBounds []float64,
) *types.Metric {
	return &types.Metric{
		Name:            metric.Name(),
		Description:     metric.Description(),
		Unit:            metric.Unit(),
		Type:            metricType,
		Flags:           flags,
		HistogramBounds: histogramBounds,
	}
}

func (m *SortedMetrics) SortValues() {
	iter, err := m.byMetric.SeekFirst()
	if err != nil {
		return
	}
	for {
		_, v, err := iter.Next()
		if err == io.EOF {
			break
		}
		v.SortValues()
	}
}

func (m *SortedMetrics) Iter(f func(metric *types.Metric, byMetric *ByMetric) error) error {
	iter, err := m.byMetric.SeekFirst()
	if err != nil {
		return nil
	}
	for {
		k, v, err := iter.Next()
		if err == io.EOF {
			break
		}
		if err := f(k, v); err != nil {
			return err
		}
	}
	return nil
}

func (m *ByMetric) ByResource(resource otlpcommon.Resource, schemaUrl string) *ByResource {
	res := resourceToResource(resource, schemaUrl, m.encoder)
	elem, exists := m.byResource.Get(res)
	if !exists {
		elem = &ByResource{encoder: m.encoder, byScope: b.TreeNew[*types.Scope, *ByScope](types.CmpScope)}
		m.byResource.Set(res, elem)
	}
	return elem
}

func resourceToResource(resource otlpcommon.Resource, schemaUrl string, encoder *anyvalue.Encoder) *types.Resource {
	//if resource == nil {
	//	return &types.Resource{SchemaURL: schemaUrl}
	//}

	var attrs types.AttrList
	MapToSortedAttrs(resource.Attributes(), encoder, &attrs)
	return &types.Resource{
		Attrs:     attrs,
		SchemaURL: schemaUrl,
	}
}

func (m *ByMetric) SortValues() {
	iter, err := m.byResource.SeekFirst()
	if err != nil {
		return
	}
	for {
		_, v, err := iter.Next()
		if err == io.EOF {
			break
		}
		v.SortValues()
	}
}

func (m *ByMetric) Iter(f func(resource *types.Resource, byResource *ByResource) error) error {
	iter, err := m.byResource.SeekFirst()
	if err != nil {
		return nil
	}
	for {
		k, v, err := iter.Next()
		if err == io.EOF {
			break
		}
		if err := f(k, v); err != nil {
			return err
		}
	}
	return nil
}

func (m *ByResource) ByScope(scope otlpcommon.InstrumentationScope, schemaUrl string) *ByScope {
	sc := scope2scope(scope, schemaUrl, m.encoder)
	elem, exists := m.byScope.Get(sc)
	if !exists {
		elem = &ByScope{byAttrs: b.TreeNew[types.AttrList, *types.TimedValues](cmpAttrs)}
		m.byScope.Set(sc, elem)
	}
	return elem
}

func (m *ByResource) Iter(f func(scope *types.Scope, byScope *ByScope) error) error {
	iter, err := m.byScope.SeekFirst()
	if err != nil {
		return nil
	}
	for {
		k, v, err := iter.Next()
		if err == io.EOF {
			break
		}
		if err := f(k, v); err != nil {
			return err
		}
	}
	return nil
}

func scope2scope(scope otlpcommon.InstrumentationScope, schemaUrl string, encoder *anyvalue.Encoder) *types.Scope {
	//if scope. == nil {
	//	return &types.Scope{SchemaURL: schemaUrl}
	//}
	var attrs types.AttrList
	MapToSortedAttrs(scope.Attributes(), encoder, &attrs)
	return &types.Scope{
		Name:      scope.Name(),
		Version:   scope.Version(),
		SchemaURL: schemaUrl,
		Attrs:     attrs,
	}
}

func (m *ByResource) SortValues() {
	iter, err := m.byScope.SeekFirst()
	if err != nil {
		return
	}
	for {
		_, v, err := iter.Next()
		if err == io.EOF {
			break
		}
		v.SortValues()
	}
}

func cmpAttrs(left, right types.AttrList) int {
	return left.Compare(right)
}

func (m *ByScope) ByAttrs(attrs types.AttrList) *types.TimedValues {
	elem, exists := m.byAttrs.Get(attrs)
	if !exists {
		elem = &types.TimedValues{}
		m.byAttrs.Set(attrs.Clone(), elem)
	}
	return elem
}

func (m *ByScope) SortValues() {
	iter, err := m.byAttrs.SeekFirst()
	if err != nil {
		return
	}
	for {
		_, v, err := iter.Next()
		if err == io.EOF {
			break
		}
		v.SortValues()
	}
}

func (m *ByScope) Iter(f func(attrs types.AttrList, values *types.TimedValues) error) error {
	iter, err := m.byAttrs.SeekFirst()
	if err != nil {
		return nil
	}
	for {
		k, v, err := iter.Next()
		if err == io.EOF {
			break
		}
		if err := f(k, v); err != nil {
			return err
		}
	}
	return nil
}

func EnsureLen[T byte | int64 | float64 | any](data []T, dataLen int) []T {
	if cap(data) < int(dataLen) {
		return append(data[0:cap(data)], make([]T, int(dataLen)-cap(data))...)
	}
	return data[0:int(dataLen)]
}

func MapToSortedAttrs(m otlpcommon.Map, encoder *anyvalue.Encoder, out *types.AttrList) {
	*out = EnsureLen(*out, m.Len())
	i := 0
	m.Range(
		func(k string, v otlpcommon.Value) bool {
			(*out)[i].Key = k
			anyValueToSTEF(v, encoder)
			(*out)[i].Value = types.AttrValue(encoder.ConsumeBytes())
			i++
			return true
		},
	)
	sort.Sort(*out)
}

func anyValueToSTEF(anyVal otlpcommon.Value, into *anyvalue.Encoder) {
	if anyVal.Type() == otlpcommon.ValueTypeEmpty {
		into.AppendNull()
		return
	}

	switch anyVal.Type() {
	case otlpcommon.ValueTypeStr:
		into.AppendString(anyVal.Str())

	case otlpcommon.ValueTypeBool:
		into.AppendBool(anyVal.Bool())

	case otlpcommon.ValueTypeDouble:
		into.AppendFloat64(anyVal.Double())

	case otlpcommon.ValueTypeInt:
		into.AppendInt64(anyVal.Int())

	case otlpcommon.ValueTypeBytes:
		into.AppendBytes(anyVal.Bytes().AsRaw())

	case otlpcommon.ValueTypeSlice:
		arrStart := into.StartArray()
		for i := 0; i < anyVal.Slice().Len(); i++ {
			anyValueToSTEF(anyVal.Slice().At(i), into)
		}
		into.FinishArray(arrStart)

	case otlpcommon.ValueTypeMap:
		kvStart := into.StartKVList()
		anyVal.Map().Range(
			func(k string, v otlpcommon.Value) bool {
				into.AppendString(k)
				anyValueToSTEF(v, into)
				return true
			},
		)
		into.FinishKVList(kvStart)

	default:
		panic("unknown anyValue type")
	}
}
