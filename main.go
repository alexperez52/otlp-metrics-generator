package main

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/sdk/metric/metrictest"
)

func main() {

	cfg := newConfig(opts...)

	c := controller.New(
		processor.NewFactory(
			selector.NewWithHistogramDistribution(),
			cfg.temporalitySelector,
		),
		controller.WithCollectPeriod(0),
	)
	exp := &Exporter{
		controller:          c,
		temporalitySelector: cfg.temporalitySelector,
	}

	return c, exp

	ctx := context.Background()
	mp, exp := metrictest.NewTestMeterProvider()
	meter := mp.Meter("test")
	fcnt, err := meter.SyncInt64().Counter("iCount")
	if err != nil {
		fmt.Println(err)
	}
	fcnt.Add(ctx, 2)
	fcnt.Add(ctx, 4)

	err = exp.Collect(context.Background())
	if err != nil {
		fmt.Println(err)
	}
	out, err := exp.GetByName("iCount")
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(out.Sum.AsInt64())

}
