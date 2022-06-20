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

	runtimemetrics "go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/metric/instrument"
	controller "go.opentelemetry.io/otel/sdk/metric/controller/basic"
	"go.opentelemetry.io/otel/sdk/metric/export/aggregation"
	processor "go.opentelemetry.io/otel/sdk/metric/processor/basic"
	selector "go.opentelemetry.io/otel/sdk/metric/selector/simple"
)

var meter = global.MeterProvider().Meter("app_or_package_name")
var atomicClient atomic.Value
var fallbackClient = newClient(&DSN{
	Original: "http://localhost:8000",
	Host:     "localhost",
	Port:     "8000",
})

type DSN struct {
	Original string
	Port     string
	Host     string
}

type client struct {
	dsn  *DSN
	ctrl *controller.Controller
}

type own struct {
}

func (dsn *DSN) AppAddr() string {
	return "http://" + net.JoinHostPort(dsn.Host, dsn.Port)
}
func (dsn *DSN) OTLPHost() string {
	if dsn.Host == "uptrace.dev" {
		return "otlp.uptrace.dev:4317"
	}
	return dsn.Host
}

func otlpmetricClient(dsn *DSN) otlpmetric.Client {
	options := []otlpmetrichttp.Option{
		// otlpmetrichttp.WithEndpoint(dsn.OTLPHost()),
		otlpmetrichttp.WithInsecure(),
		otlpmetrichttp.WithCompression(otlpmetrichttp.NoCompression),
	}

	return otlpmetrichttp.NewClient(options...)
}

func configureMetrics(ctx context.Context, client *client) {
	exportKindSelector := aggregation.StatelessTemporalitySelector()

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

	if err := ctrl.Start(ctx); err != nil {
		fmt.Printf("ctrl.Start failed: %s", err)
		return
	}
	global.SetMeterProvider(ctrl)
	client.ctrl = ctrl

	if err := runtimemetrics.Start(); err != nil {
		fmt.Printf("runtimemetrics.Start failed: %s", err)
	}
}

func main() {

	// Utilizing metrictest
	// Exporter being used here is not thread safe; Collect() has to be manually called
	// TODO: Use OTLP stock exporter instead of the custom exporter found in metrictest

	// otlpMetricWithCustomExporter()

	ctx := context.Background()
	own := newOwn()

	own.configureOtel()

	defer Shutdown(ctx)
	// Synchronous instruments.
	go counter(ctx)
	go counterObserver(ctx)
	fmt.Println("reporting measurements to localhost... (press Ctrl+C to stop)")

	ch := make(chan os.Signal, 3)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGTERM)
	<-ch
}
func Shutdown(ctx context.Context) error {
	return activeClient().Shutdown(ctx)
}
func (c *client) Shutdown(ctx context.Context) (lastErr error) {
	if c.ctrl != nil {
		if err := c.ctrl.Stop(ctx); err != nil {
			lastErr = err
		}
	}
	return lastErr
}
func activeClient() *client {
	v := atomicClient.Load()
	if v == nil {
		return fallbackClient
	}
	return v.(*client)
}

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
func newClient(dsn *DSN) *client {
	return &client{
		dsn: dsn,
	}
}

func newOwn() *own {
	return &own{}
}

func counter(ctx context.Context) {
	counter, _ := meter.SyncInt64().Counter(
		"some.prefix.counter",
		instrument.WithUnit("1"),
		instrument.WithDescription("TODO"),
	)

	for {
		counter.Add(ctx, 1)
		time.Sleep(time.Millisecond)
	}
}
func counterObserver(ctx context.Context) {
	counter, _ := meter.AsyncInt64().Counter(
		"some.prefix.counter_observer",
		instrument.WithUnit("1"),
		instrument.WithDescription("TODO"),
	)

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
