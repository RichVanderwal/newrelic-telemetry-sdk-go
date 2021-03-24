// Copyright 2019 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"testing"
	"time"

	"github.com/newrelic/newrelic-telemetry-sdk-go/internal"
)

func TestMetrics(t *testing.T) {
	metrics := &MetricBatch{}
	start := time.Date(2014, time.November, 28, 1, 1, 0, 0, time.UTC)
	metrics.Metrics = []Metric{
		Summary{
			Name: "mySummary",
			Attributes: map[string]interface{}{
				"attribute": "string",
			},
			Count:     3,
			Sum:       15,
			Min:       4,
			Max:       6,
			Timestamp: start,
			Interval:  5 * time.Second,
		},
		Gauge{
			Name: "myGauge",
			Attributes: map[string]interface{}{
				"attribute": true,
			},
			Value:     12.3,
			Timestamp: start,
		},
		Count{
			Name: "myCount",
			Attributes: map[string]interface{}{
				"attribute": 123,
			},
			Value:     100,
			Timestamp: start,
			Interval:  5 * time.Second,
		},
	}
	commonAttributes := &commonAttributes{RawJSON: json.RawMessage(`{"zip":"zap"}`)}
	commonBlock := &metricCommonBlock{Attributes: commonAttributes}

	expect := compactJSONString(`[{
		"common":{
			"attributes":{"zip":"zap"}
		},
		"metrics":[
			{
				"name":"mySummary",
				"type":"summary",
				"value":{"sum":15,"count":3,"min":4,"max":6},
				"timestamp":1417136460000,
				"interval.ms":5000,
				"attributes":{"attribute":"string"}
			},
			{
				"name":"myGauge",
				"type":"gauge",
				"value":12.3,
				"timestamp":1417136460000,
				"attributes":{"attribute":true}
			},
			{
				"name":"myCount",
				"type":"count",
				"value":100,
				"timestamp":1417136460000,
				"interval.ms":5000,
				"attributes":{"attribute":123}
			}
		]
	}]`)

	factory, _ := NewMetricRequestFactory(WithNoDefaultKey())
	reqs, err := newRequests([]Batch{{commonBlock, metrics}}, factory)
	if err != nil {
		t.Error("error creating request", err)
	}
	if len(reqs) != 1 {
		t.Fatal(reqs)
	}
	req := reqs[0]
	bodyReader, _ := req.GetBody()
	compressedBytes, _ := ioutil.ReadAll(bodyReader)
	data, _ := internal.Uncompress(compressedBytes)
	if string(data) != expect {
		t.Error("metrics JSON mismatch", string(data), expect)
	}
	body, err := ioutil.ReadAll(req.Body)
	req.Body.Close()
	if err != nil {
		t.Fatal("unable to read body", err)
	}
	if len(body) != int(req.ContentLength) {
		t.Error("compressed body length mismatch", len(body), req.ContentLength)
	}
	uncompressed, err := internal.Uncompress(body)
	if err != nil {
		t.Fatal("unable to uncompress body", err)
	}
	if string(uncompressed) != expect {
		t.Error("metrics JSON mismatch", string(uncompressed), expect)
	}
}

func testBatchJSON(t testing.TB, batches []Batch, expect string) {
	if th, ok := t.(interface{ Helper() }); ok {
		th.Helper()
	}
	factory, _ := NewMetricRequestFactory(WithNoDefaultKey())
	reqs, err := newRequests(batches, factory)
	if nil != err {
		t.Fatal(err)
	}
	if len(reqs) != 1 {
		t.Fatal(reqs)
	}
	bodyReader, _ := reqs[0].GetBody()
	compressedBytes, _ := ioutil.ReadAll(bodyReader)
	js, _ := internal.Uncompress(compressedBytes)
	actual := string(js)
	compactExpect := compactJSONString(expect)
	if actual != compactExpect {
		t.Errorf("\nexpect=%s\nactual=%s\n", compactExpect, actual)
	}
}

func TestSplit(t *testing.T) {
	// test len 0
	batch := &MetricBatch{}
	split := batch.split()
	if split != nil {
		t.Error(split)
	}

	// test len 1
	batch = &MetricBatch{Metrics: []Metric{Count{}}}
	split = batch.split()
	if split != nil {
		t.Error(split)
	}

	// test len 2
	batch = &MetricBatch{Metrics: []Metric{Count{Name: "c1"}, Count{Name: "c2"}}}
	split = batch.split()
	if len(split) != 2 {
		t.Error("split into incorrect number of slices", len(split))
	}
	testBatchJSON(t, []Batch{{split[0]}}, `[{"metrics":[{"name":"c1","type":"count","value":0}]}]`)
	testBatchJSON(t, []Batch{{split[1]}}, `[{"metrics":[{"name":"c2","type":"count","value":0}]}]`)

	// test len 3
	batch = &MetricBatch{Metrics: []Metric{Count{Name: "c1"}, Count{Name: "c2"}, Count{Name: "c3"}}}
	split = batch.split()
	if len(split) != 2 {
		t.Error("split into incorrect number of slices", len(split))
	}
	testBatchJSON(t, []Batch{{split[0]}}, `[{"metrics":[{"name":"c1","type":"count","value":0}]}]`)
	testBatchJSON(t, []Batch{{split[1]}}, `[{"metrics":[{"name":"c2","type":"count","value":0},{"name":"c3","type":"count","value":0}]}]`)
}

func BenchmarkMetricsJSON(b *testing.B) {
	// This benchmark tests the overhead of turning metrics into JSON.
	commonAttributes := commonAttributes{RawJSON: json.RawMessage(`{"zip": "zap"}`)}
	commonBlock := &metricCommonBlock{Attributes: &commonAttributes}
	batch := &MetricBatch{}
	numMetrics := 10 * 1000
	start := time.Date(2014, time.November, 28, 1, 1, 0, 0, time.UTC)

	for i := 0; i < numMetrics/3; i++ {
		batch.Metrics = append(batch.Metrics, Summary{
			Name:       "mySummary",
			Attributes: map[string]interface{}{"attribute": "string"},
			Count:      3,
			Sum:        15,
			Min:        4,
			Max:        6,
			Timestamp:  start,
			Interval:   5 * time.Second,
		})
		batch.Metrics = append(batch.Metrics, Gauge{
			Name:       "myGauge",
			Attributes: map[string]interface{}{"attribute": true},
			Value:      12.3,
			Timestamp:  start,
		})
		batch.Metrics = append(batch.Metrics, Count{
			Name:       "myCount",
			Attributes: map[string]interface{}{"attribute": 123},
			Value:      100,
			Timestamp:  start,
			Interval:   5 * time.Second,
		})
	}

	b.ResetTimer()
	b.ReportAllocs()

	entries := []MapEntry{commonBlock, batch}
	estimate := len(batch.Metrics) * 256
	for i := 0; i < b.N; i++ {
		buf := bytes.NewBuffer(make([]byte, 0, estimate))

		buf.Write([]byte{'[', '{'})
		w := internal.JSONFieldsWriter{Buf: buf}
		for _, entry := range entries {
			w.RawField(entry.Type(), entry.Bytes())
		}
		buf.Write([]byte{'}', ']'})

		bts := buf.Bytes()
		if len(bts) == 0 {
			b.Fatal(string(bts))
		}
	}
}

func TestMetricAttributesJSON(t *testing.T) {
	tests := []struct {
		key    string
		val    interface{}
		expect string
	}{
		{"string", "string", `[{"metrics":[{"name":"","type":"count","value":0,"attributes":{"string":"string"}}]}]`},
		{"true", true, `[{"metrics":[{"name":"","type":"count","value":0,"attributes":{"true":true}}]}]`},
		{"false", false, `[{"metrics":[{"name":"","type":"count","value":0,"attributes":{"false":false}}]}]`},
		{"uint8", uint8(1), `[{"metrics":[{"name":"","type":"count","value":0,"attributes":{"uint8":1}}]}]`},
		{"uint16", uint16(1), `[{"metrics":[{"name":"","type":"count","value":0,"attributes":{"uint16":1}}]}]`},
		{"uint32", uint32(1), `[{"metrics":[{"name":"","type":"count","value":0,"attributes":{"uint32":1}}]}]`},
		{"uint64", uint64(1), `[{"metrics":[{"name":"","type":"count","value":0,"attributes":{"uint64":1}}]}]`},
		{"uint", uint(1), `[{"metrics":[{"name":"","type":"count","value":0,"attributes":{"uint":1}}]}]`},
		{"uintptr", uintptr(1), `[{"metrics":[{"name":"","type":"count","value":0,"attributes":{"uintptr":1}}]}]`},
		{"int8", int8(1), `[{"metrics":[{"name":"","type":"count","value":0,"attributes":{"int8":1}}]}]`},
		{"int16", int16(1), `[{"metrics":[{"name":"","type":"count","value":0,"attributes":{"int16":1}}]}]`},
		{"int32", int32(1), `[{"metrics":[{"name":"","type":"count","value":0,"attributes":{"int32":1}}]}]`},
		{"int64", int64(1), `[{"metrics":[{"name":"","type":"count","value":0,"attributes":{"int64":1}}]}]`},
		{"int", int(1), `[{"metrics":[{"name":"","type":"count","value":0,"attributes":{"int":1}}]}]`},
		{"float32", float32(1), `[{"metrics":[{"name":"","type":"count","value":0,"attributes":{"float32":1}}]}]`},
		{"float64", float64(1), `[{"metrics":[{"name":"","type":"count","value":0,"attributes":{"float64":1}}]}]`},
		{"default", func() {}, `[{"metrics":[{"name":"","type":"count","value":0,"attributes":{"default":"func()"}}]}]`},
	}

	for _, test := range tests {
		batch := &MetricBatch{}
		batch.Metrics = append(batch.Metrics, Count{
			Attributes: map[string]interface{}{
				test.key: test.val,
			},
		})
		testBatchJSON(t, []Batch{{batch}}, test.expect)
	}
}

func TestCountAttributesJSON(t *testing.T) {
	batch := &MetricBatch{}
	batch.Metrics = append(batch.Metrics, Count{
		Attributes: map[string]interface{}{
			"zip": "zap",
		},
		AttributesJSON: json.RawMessage(`{"zing":"zang"}`),
	})
	testBatchJSON(t, []Batch{{batch}}, `[{"metrics":[{"name":"","type":"count","value":0,"attributes":{"zip":"zap"}}]}]`)

	batch = &MetricBatch{}
	batch.Metrics = append(batch.Metrics, Count{
		AttributesJSON: json.RawMessage(`{"zing":"zang"}`),
	})
	testBatchJSON(t, []Batch{{batch}}, `[{"metrics":[{"name":"","type":"count","value":0,"attributes":{"zing":"zang"}}]}]`)
}

func TestGaugeAttributesJSON(t *testing.T) {
	start := time.Date(2014, time.November, 28, 1, 1, 0, 0, time.UTC)

	batch := &MetricBatch{}
	batch.Metrics = append(batch.Metrics, Gauge{
		Attributes: map[string]interface{}{
			"zip": "zap",
		},
		AttributesJSON: json.RawMessage(`{"zing":"zang"}`),
		Timestamp:      start,
	})
	testBatchJSON(t, []Batch{{batch}}, `[{"metrics":[{"name":"","type":"gauge","value":0,"timestamp":1417136460000,"attributes":{"zip":"zap"}}]}]`)

	batch = &MetricBatch{}
	batch.Metrics = append(batch.Metrics, Gauge{
		AttributesJSON: json.RawMessage(`{"zing":"zang"}`),
		Timestamp:      start,
	})
	testBatchJSON(t, []Batch{{batch}}, `[{"metrics":[{"name":"","type":"gauge","value":0,"timestamp":1417136460000,"attributes":{"zing":"zang"}}]}]`)
}

func TestSummaryAttributesJSON(t *testing.T) {
	batch := &MetricBatch{}
	batch.Metrics = append(batch.Metrics, Summary{
		Attributes: map[string]interface{}{
			"zip": "zap",
		},
		AttributesJSON: json.RawMessage(`{"zing":"zang"}`),
	})
	testBatchJSON(t, []Batch{{batch}}, `[{"metrics":[{"name":"","type":"summary","value":{"sum":0,"count":0,"min":0,"max":0},"attributes":{"zip":"zap"}}]}]`)

	batch = &MetricBatch{}
	batch.Metrics = append(batch.Metrics, Summary{
		AttributesJSON: json.RawMessage(`{"zing":"zang"}`),
	})
	testBatchJSON(t, []Batch{{batch}}, `[{"metrics":[{"name":"","type":"summary","value":{"sum":0,"count":0,"min":0,"max":0},"attributes":{"zing":"zang"}}]}]`)
}

func TestBatchAttributesJSON(t *testing.T) {
	commonAttributes := &commonAttributes{RawJSON: json.RawMessage(`{"zing":"zang"}`)}
	commonBlock := &metricCommonBlock{Attributes: commonAttributes}
	batch := &MetricBatch{}
	testBatchJSON(t, []Batch{{commonBlock, batch}}, `[{"common":{"attributes":{"zing":"zang"}},"metrics":[]}]`)
}

func TestBatchStartEndTimesJSON(t *testing.T) {
	start := time.Date(2014, time.November, 28, 1, 1, 0, 0, time.UTC)

	commonBlock := &metricCommonBlock{}
	emptyBatch := &MetricBatch{}

	testBatchJSON(t, []Batch{{commonBlock, emptyBatch}}, `[{"common":{},"metrics":[]}]`)

	commonBlock = &metricCommonBlock{
		Timestamp: start,
	}
	testBatchJSON(t, []Batch{{commonBlock, emptyBatch}}, `[{"common":{"timestamp":1417136460000},"metrics":[]}]`)

	commonBlock = &metricCommonBlock{
		Interval: 5 * time.Second,
	}
	testBatchJSON(t, []Batch{{commonBlock, emptyBatch}}, `[{"common":{"interval.ms":5000},"metrics":[]}]`)

	commonBlock = &metricCommonBlock{
		Timestamp: start,
		Interval:  5 * time.Second,
	}
	testBatchJSON(t, []Batch{{commonBlock, emptyBatch}}, `[{"common":{"timestamp":1417136460000,"interval.ms":5000},"metrics":[]}]`)
}

func TestCommonAttributes(t *testing.T) {
	// Tests when the "common" key is included in the metrics payload
	type testStruct struct {
		start          time.Time
		interval       time.Duration
		attributesJSON json.RawMessage
		expect         string
	}
	sometime := time.Date(2014, time.November, 28, 1, 1, 0, 0, time.UTC)
	testcases := []testStruct{
		{expect: `[{"common":{},"metrics":[]}]`},
		{start: sometime, expect: `[{"common":{"timestamp":1417136460000},"metrics":[]}]`},
		{interval: 5 * time.Second, expect: `[{"common":{"interval.ms":5000},"metrics":[]}]`},
		{start: sometime, interval: 5 * time.Second,
			expect: `[{"common":{"timestamp":1417136460000,"interval.ms":5000},"metrics":[]}]`},
		{attributesJSON: json.RawMessage(`{"zip":"zap"}`),
			expect: `[{"common":{"attributes":{"zip":"zap"}},"metrics":[]}]`},
	}

	emptyBatch := &MetricBatch{}
	for _, test := range testcases {
		commonBlock := &metricCommonBlock{
			Timestamp:  test.start,
			Interval:   test.interval,
			Attributes: &commonAttributes{RawJSON: test.attributesJSON},
		}
		testBatchJSON(t, []Batch{{commonBlock, emptyBatch}}, test.expect)
	}
}

func TestMetricsJSONWithCommonAttributesJSON(t *testing.T) {
	commonBlock := &metricCommonBlock{
		Timestamp:  time.Date(2014, time.November, 28, 1, 1, 0, 0, time.UTC),
		Interval:   5 * time.Second,
		Attributes: &commonAttributes{RawJSON: json.RawMessage(`{"zup":"wup"}`)},
	}

	batch1 := &MetricBatch{
		Metrics: []Metric{
			&Summary{
				Name:       "foo",
				Attributes: map[string]interface{}{"zip": "zap"},
			},
		},
	}
	batch2 := &MetricBatch{
		Metrics: []Metric{
			&Summary{
				Name: "bar",
			},
		},
	}
	testBatchJSON(t, []Batch{{commonBlock, batch1}, {batch2}}, `[
		{
			"common": {
				"timestamp":1417136460000,
				"interval.ms":5000,
				"attributes": {"zup":"wup"}
			},
			"metrics":[
				{
					"name":"foo",
					"type":"summary",
					"value":{"sum":0,"count":0,"min":0,"max":0},
					"attributes":{"zip":"zap"}
				}
			]
		},
		{
			"metrics":[
				{
					"name":"bar",
					"type":"summary",
					"value":{"sum":0,"count":0,"min":0,"max":0}
				}
			]
		}
	]`)
}
func TestMetricBatchSplittable(t *testing.T) {
	batch := &MetricBatch{
		Metrics: []Metric{
			&Summary{
				Name:       "foo",
				Attributes: map[string]interface{}{"zip": "zap"},
			},
		},
	}
	_ = splittablePayloadEntry(batch)
}
