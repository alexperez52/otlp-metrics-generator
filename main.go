package main

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric/metrictest"
)

func main() {

	// Utilizing metrictest
	// Exporter being used here is not thread safe; Collect() has to be manually called
	// TODO: Use OTLP stock exporter instead of the custom exporter found in metrictest

	otlpMetricWithCustomExporter()

}

func otlpMetricWithCustomExporter() {
	ctx := context.Background()
	mp, exp := metrictest.NewTestMeterProvider()
	meter := mp.Meter("OTLP-Metrics")

	attrs := []attribute.KeyValue{attribute.Bool("test", true)}

	//count
	fcnt, err := meter.SyncInt64().Counter("iCount")

	if err != nil {
		fmt.Println(err)
	}
	fcnt.Add(ctx, 2, attrs...)
	fcnt.Add(ctx, 4, attrs...)
	err = exp.Collect(context.Background())

	if err != nil {
		fmt.Println(err)
	}
	counterOut, err := exp.GetByName("iCount")
	if err != nil {
		fmt.Println(err)
	}

	//UpDownCounter, Gauge
	iudcnt, err := meter.SyncInt64().UpDownCounter("iUDCount")
	if err != nil {
		fmt.Println(err)
	}
	iudcnt.Add(ctx, 23, attrs...)

	err = exp.Collect(context.Background())
	if err != nil {
		fmt.Println(err)
	}
	gaugeOut, err := exp.GetByName("iUDCount")

	if err != nil {
		fmt.Println(err)
	}

	// Histogram
	ihis, err := meter.SyncInt64().Histogram("iHist")
	if err != nil {
		fmt.Println(err)
	}

	ihis.Record(ctx, 24)
	ihis.Record(ctx, 25)

	err = exp.Collect(context.Background())
	if err != nil {
		fmt.Println(err)
	}
	hisOut, err := exp.GetByName("iHist")
	if err != nil {
		fmt.Println(err)
	}

	ihis2, err := meter.SyncInt64().Histogram("iHist2")
	if err != nil {
		fmt.Println(err)
	}

	ihis2.Record(ctx, 24)
	ihis2.Record(ctx, 25)
	ihis2.Record(ctx, 26)

	err = exp.Collect(context.Background())
	if err != nil {
		fmt.Println(err)
	}
	hisOut2, err := exp.GetByName("iHist2")
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(hisOut2)
	// Printing out the metrics
	fmt.Println(counterOut)
	fmt.Println(gaugeOut)
	fmt.Println(hisOut)
}
