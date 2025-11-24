package main

import (
	"context"
	"html/template"
	"log"
	"net/http"
	"time"

	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/disk"
	"github.com/shirou/gopsutil/mem"

	"go.opentelemetry.io/otel"
	otelmetric "go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

type Metrics struct {
	CPUUsage    float64
	MemoryUsed  uint64
	MemoryTotal uint64
	DiskUsed    uint64
	DiskTotal   uint64
}

var meter otelmetric.Meter

func initMeter() {
	provider := sdkmetric.NewMeterProvider()
	otel.SetMeterProvider(provider)
	meter = provider.Meter("system-metrics-dashboard")
}

func getMetrics() Metrics {
	cpuPercent, _ := cpu.Percent(time.Second, false)
	vmStat, _ := mem.VirtualMemory()
	diskStat, _ := disk.Usage("/")

	return Metrics{
		CPUUsage:    cpuPercent[0],
		MemoryUsed:  vmStat.Used / (1024 * 1024),
		MemoryTotal: vmStat.Total / (1024 * 1024),
		DiskUsed:    diskStat.Used / (1024 * 1024 * 1024),
		DiskTotal:   diskStat.Total / (1024 * 1024 * 1024),
	}
}

func recordMetrics(ctx context.Context) {
	cpuGauge, _ := meter.Float64ObservableGauge("cpu.usage")
	memGauge, _ := meter.Float64ObservableGauge("memory.used")
	diskGauge, _ := meter.Float64ObservableGauge("disk.used")

	_, err := meter.RegisterCallback(func(ctx context.Context, o otelmetric.Observer) error {
		m := getMetrics()
		o.ObserveFloat64(cpuGauge, m.CPUUsage)
		o.ObserveFloat64(memGauge, float64(m.MemoryUsed))
		o.ObserveFloat64(diskGauge, float64(m.DiskUsed))
		return nil
	}, cpuGauge, memGauge, diskGauge)

	if err != nil {
		log.Fatalf("Failed to register callback: %v", err)
	}
}

func handler(w http.ResponseWriter, r *http.Request) {
	tmpl := template.Must(template.ParseFiles("templates/index.html"))
	metrics := getMetrics()
	tmpl.Execute(w, metrics)
}

func main() {
	ctx := context.Background()
	initMeter()
	recordMetrics(ctx)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[%s] %d  %s", r.Method, http.StatusOK, r.URL.String())
		handler(w, r)
	})
	log.Println("Server started at :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
