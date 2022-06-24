package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/metric/instrument"
	controller "go.opentelemetry.io/otel/sdk/metric/controller/basic"
	"go.opentelemetry.io/otel/sdk/metric/export/aggregation"
	processor "go.opentelemetry.io/otel/sdk/metric/processor/basic"
	selector "go.opentelemetry.io/otel/sdk/metric/selector/simple"
)

/*
   This application currently creates an Async counter and updates it to be collected every 3 seconds
   This application is for experimental purposes only and will break once the OpenTelemetry SDK
   receives updates or has major changes.
   Eventually this will be evolved into a functional Sample application that emits Metrics, Logs
   and traces.
   Unstable components will be identified and documented.
   This sample app will eventually follow a specific `Sample app requirements spec`
*/

var (
	meter = global.MeterProvider().Meter("OTLP_METRIC_SAMPLE_APP")
)

func main() {
	// Start a context background and configure a new otel instance to being sending metrics to localhost:4318
	ctx := context.Background()
	shutdown := startClient(ctx)

	defer shutdown()
	// Async create counter and counterobserver
	go counterObserver(ctx)
	fmt.Println("reporting measurements to localhost:4318... (press Ctrl+C to stop)")

	ch := make(chan os.Signal, 3)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGTERM)
	<-ch

}

// Function that creates and returns a New client with certain options
// In this case we are sending insecure options (http instead of https)
func otlpmetricClient(endpoint string) otlpmetric.Client {
	options := []otlpmetrichttp.Option{
		otlpmetrichttp.WithInsecure(),
		otlpmetrichttp.WithEndpoint(endpoint),
	}

	return otlpmetrichttp.NewClient(options...)
}

// This function starts the client
func startClient(ctx context.Context) func() {

	// Check if environment variable is set
	endpoint := os.Getenv("OTLP_EXPORTER_OTLP_ENDPOINT")
	if endpoint == "" {
		// Default otlp http endpoint (gRPC is 4317, http is 4318)
		endpoint = "0.0.0.0:4318"
	}

	// The default endpoint is being used (localhost:4318)

	// Cumulative Temporality Selector indicates new values SHOULD never be less than the previous value
	cumulativeSelector := aggregation.CumulativeTemporalitySelector()

	// Delta Temporality Selector indicates that values SHOULD be non negative. Could be less than the previous value
	// **Commented out for now**
	// deltaSelector := aggregation.DeltaTemporalitySelector()

	metricExp, err := otlpmetric.New(ctx, otlpmetricClient(endpoint), otlpmetric.WithMetricAggregationTemporalitySelector(cumulativeSelector))
	if err != nil {
		fmt.Printf("otlpmetric.New failed: %s", err)
	}

	ctrl := controller.New(
		processor.NewFactory(
			selector.NewWithHistogramDistribution(),
			metricExp,
		),
		controller.WithExporter(metricExp),
		controller.WithCollectPeriod(3*time.Second), // same as default
	)

	// Client is started here through the controller created above
	if err := ctrl.Start(ctx); err != nil {
		fmt.Printf("ctrl.Start failed: %s", err)
	}
	global.SetMeterProvider(ctrl)

	// Pass function to shutdown the controller in a defer statement
	return func() {
		cxt, cancel := context.WithTimeout(ctx, time.Second)
		defer cancel()

		// pushes any last exports to the receiver
		if err := ctrl.Stop(cxt); err != nil {
			otel.Handle(err)
		}
	}
}

// Counter that increments by 1 every call
func counterObserver(ctx context.Context) {
	counter, _ := meter.AsyncInt64().Counter(
		"MyCounter_observer",
		instrument.WithUnit("1"),
		instrument.WithDescription("This is a sample counter observer"),
	)

	// This could be any other incremental data point to be observed and sent as a Counter metric
	var number int64
	if err := meter.RegisterCallback(
		[]instrument.Asynchronous{
			counter,
		},
		// SDK periodically calls this function to collect data.
		func(ctx context.Context) {
			number++
			counter.Observe(ctx, number)
		},
	); err != nil {
		panic(err)
	}
}
