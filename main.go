package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/metric/instrument"
	controller "go.opentelemetry.io/otel/sdk/metric/controller/basic"
	"go.opentelemetry.io/otel/sdk/metric/export/aggregation"
	processor "go.opentelemetry.io/otel/sdk/metric/processor/basic"
	selector "go.opentelemetry.io/otel/sdk/metric/selector/simple"
)

var (
	meter        = global.MeterProvider().Meter("app_or_package_name")
	atomicClient atomic.Value
	//fallback client currently now used because otel will default to localhost:4318 on HTTP
	fallbackClient = newClient(&DSN{
		Original: "http://localhost:8000",
		Host:     "localhost",
		Port:     "8000",
	})
)

// Structure to store custom DSN settings
type DSN struct {
	Original string
	Port     string
	Host     string
}

// Client struct that works compatible with otel sdk metric controller
type client struct {
	dsn  *DSN
	ctrl *controller.Controller
}

// Dummy struct (probably not needed)
type own struct {
}

func main() {
	// Start a context background and configure a new otel instance to being sending metrics to localhost:4318
	ctx := context.Background()
	own := newOwn()
	own.configureOtel()

	defer Shutdown(ctx)
	// Async create counter and counterobserver
	go counter(ctx)
	// go counterObserver(ctx)
	fmt.Println("reporting measurements to localhost:4318... (press Ctrl+C to stop)")

	ch := make(chan os.Signal, 3)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGTERM)
	<-ch
}

// Function that sets a custom DSN and starts a new client with those DSN settings
// Also starts the otel client to begin to send metrics generated
func (own *own) configureOtel(opts ...instrument.Option) {
	ctx := context.TODO()

	dsn := &DSN{
		Original: "http://localhost:8000",
		Host:     "localhost",
		Port:     "8000",
	}
	client := newClient(dsn)
	configureMetrics(ctx, client)
	atomicClient.Store(client)

}

// Function to join a host and a port
func (dsn *DSN) AppAddr() string {
	return "http://" + net.JoinHostPort(dsn.Host, dsn.Port)
}

// Function that creates and returns a New client with certain options
// In this case we are sending insecure options (http instead of https)
func otlpmetricClient(dsn *DSN) otlpmetric.Client {
	options := []otlpmetrichttp.Option{
		otlpmetrichttp.WithInsecure(),
	}

	return otlpmetrichttp.NewClient(options...)
}

// This function starts the client
func configureMetrics(ctx context.Context, client *client) {
	exportKindSelector := aggregation.StatelessTemporalitySelector()

	// client.dsn is currently not being used. the default endpoint is being used (localhost:4318)
	exp, err := otlpmetric.New(ctx, otlpmetricClient(client.dsn),
		otlpmetric.WithMetricAggregationTemporalitySelector(exportKindSelector))
	if err != nil {
		fmt.Printf("otlpmetric.New failed: %s", err)
		return
	}

	ctrl := controller.New(
		processor.NewFactory(
			selector.NewWithHistogramDistribution(),
			exportKindSelector,
		),
		controller.WithExporter(exp),
		controller.WithCollectPeriod(3*time.Second), // same as default
	)

	// Client is started here through the controller created above
	if err := ctrl.Start(ctx); err != nil {
		fmt.Printf("ctrl.Start failed: %s", err)
		return
	}
	global.SetMeterProvider(ctrl)
	client.ctrl = ctrl

	// Commenting this out to not get dummy metrics from runtime data
	// if err := runtimemetrics.Start(); err != nil {
	// 	fmt.Printf("runtimemetrics.Start failed: %s", err)
	// }
}

// Wrapper function to shutdown the active client
func Shutdown(ctx context.Context) error {
	return activeClient().Shutdown(ctx)
}

// Function to shutdown a specific client
func (c *client) Shutdown(ctx context.Context) (lastErr error) {
	if c.ctrl != nil {
		if err := c.ctrl.Stop(ctx); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

// Function that returns the active client stored in atomic client
func activeClient() *client {
	v := atomicClient.Load()
	if v == nil {
		return fallbackClient
	}
	return v.(*client)
}

// Creates a new client given a dsn
func newClient(dsn *DSN) *client {
	return &client{
		dsn: dsn,
	}
}

// Returns a new dummy struct
func newOwn() *own {
	return &own{}
}

// Created a counter and adds 1 every second
func counter(ctx context.Context) {
	counter, _ := meter.SyncInt64().Counter(
		"MyCounter_1",
		instrument.WithUnit("1"),
		instrument.WithDescription("This is a sample counter that increments by 1 every second."),
	)

	for {
		counter.Add(ctx, 1)
		time.Sleep(time.Second)
	}
}

// func counterObserver(ctx context.Context) {
// 	counter, _ := meter.AsyncInt64().Counter(
// 		"some.prefix.counter_observer",
// 		instrument.WithUnit("1"),
// 		instrument.WithDescription("TODO"),
// 	)

// 	var number int64
// 	if err := meter.RegisterCallback(
// 		[]instrument.Asynchronous{
// 			counter,
// 		},
// 		// SDK periodically calls this function to collect data.
// 		func(ctx context.Context) {
// 			number++
// 			counter.Observe(ctx, number)
// 		},
// 	); err != nil {
// 		panic(err)
// 	}
// }
