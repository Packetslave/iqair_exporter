// Binary iqair_exporter implements a Prometheus collector for iqAir AirVisual air quality monitors.
package main

// Tested with the iqAir Air Visual Pro
// https://www.iqair.com/us/air-quality-monitors/airvisual-series

// Based on code from the HAproxy exporter (https://github.com/prometheus/haproxy_exporter)

import (
	"encoding/json"
	"io"
	"net/http"
	_ "net/http/pprof"
	"os"
	"sync"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/promlog"
	"github.com/prometheus/common/promlog/flag"
	"github.com/prometheus/common/version"
	"github.com/prometheus/exporter-toolkit/web"
	webflag "github.com/prometheus/exporter-toolkit/web/kingpinflag"
	"gopkg.in/alecthomas/kingpin.v2"
)

const (
	namespace = "iqair" // For Prometheus metrics.
)

var (
	iqAirUp       = prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "up"), "Was the last scrape of iqAir successful.", nil, nil)
	iqAirCO2      = prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "co2"), "CO2 reading.", nil, nil)
	iqAirP25      = prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "p25"), "p2.5 particulate reading.", nil, nil)
	iqAirP10      = prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "p10"), "p10 particulate reading.", nil, nil)
	iqAirTemp     = prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "temperature"), "Temperature reading in Celsius.", nil, nil)
	iqAirHumidity = prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "humidity"), "Humidity reading.", nil, nil)
)

// Exporter collects iqAir stats from the given URI and exports them using
// the prometheus metrics package.
type Exporter struct {
	URI   string
	mutex sync.RWMutex

	totalScrapes, jsonParseFailures prometheus.Counter
	logger                          log.Logger
}

// NewExporter returns an initialized Exporter.
func NewExporter(uri string, timeout time.Duration, logger log.Logger) (*Exporter, error) {
	return &Exporter{
		URI: uri,
		totalScrapes: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "exporter_scrapes_total",
			Help:      "Current total iqAir scrapes.",
		}),
		jsonParseFailures: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "exporter_json_parse_failures_total",
			Help:      "Number of errors while parsing JSON.",
		}),
		logger: logger,
	}, nil
}

// Describe describes all the metrics ever exported by the iqAir exporter. It
// implements prometheus.Collector.
func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- iqAirUp
	ch <- e.totalScrapes.Desc()
	ch <- e.jsonParseFailures.Desc()
}

// Collect fetches the stats from configured iqAir location and delivers them
// as Prometheus metrics. It implements prometheus.Collector.
func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	e.mutex.Lock() // To protect metrics from concurrent collects.
	defer e.mutex.Unlock()

	up, result := e.scrape(ch)

	ch <- e.totalScrapes
	ch <- e.jsonParseFailures
	ch <- prometheus.MustNewConstMetric(iqAirUp, prometheus.GaugeValue, up)
	ch <- prometheus.MustNewConstMetric(iqAirCO2, prometheus.GaugeValue, float64(result.CO2))
	ch <- prometheus.MustNewConstMetric(iqAirP25, prometheus.GaugeValue, float64(result.P25))
	ch <- prometheus.MustNewConstMetric(iqAirP10, prometheus.GaugeValue, float64(result.P10))
	ch <- prometheus.MustNewConstMetric(iqAirTemp, prometheus.GaugeValue, float64(result.Temperature))
	ch <- prometheus.MustNewConstMetric(iqAirHumidity, prometheus.GaugeValue, float64(result.Humidity))
}

type APIData struct {
	CO2         int     `json:"co"`
	P25         int     `json:"p2"`
	P10         int     `json:"p1"`
	Temperature float64 `json:"tp"`
	Humidity    int     `json:"hm"`
}

type APIResponse struct {
	Current APIData `json:"current"`
}

func (e *Exporter) scrape(ch chan<- prometheus.Metric) (up float64, result *APIData) {
	e.totalScrapes.Inc()

	resp, err := http.Get(e.URI)
	if err != nil {
		e.logger.Log("failed to scrape: %v", err)
		return 0, nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		e.logger.Log("failed to read body: %v", err)
		return 0, nil
	}

	var parsed APIResponse
	e.logger.Log("parsed: %v", parsed)

	if err := json.Unmarshal(body, &parsed); err != nil {
		e.logger.Log("failed to parse body: %v", err)
		return 0, nil
	}

	return 1, &parsed.Current
}

func main() {
	var (
		webConfig      = webflag.AddFlags(kingpin.CommandLine)
		listenAddress  = kingpin.Flag("web.listen-address", "Address to listen on for web interface and telemetry.").Default(":9861").String()
		metricsPath    = kingpin.Flag("web.telemetry-path", "Path under which to expose metrics.").Default("/metrics").String()
		iqairScrapeURI = kingpin.Flag("iqair.scrape-uri", "URI on which to scrape iqAir.").String()
	)

	promlogConfig := &promlog.Config{}
	flag.AddFlags(kingpin.CommandLine, promlogConfig)

	kingpin.Version(version.Print("iqair_exporter"))
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()

	logger := promlog.New(promlogConfig)

	level.Info(logger).Log("msg", "Starting iqair", "version", version.Info())
	level.Info(logger).Log("msg", "Build context", "context", version.BuildContext())

	exporter, err := NewExporter(*iqairScrapeURI, 10*time.Second, logger)
	if err != nil {
		level.Error(logger).Log("msg", "Error creating an exporter", "err", err)
		os.Exit(1)
	}

	prometheus.MustRegister(exporter)
	prometheus.MustRegister(version.NewCollector("iqair_exporter"))

	http.Handle(*metricsPath, promhttp.Handler())
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
             <head><title>iqAir Exporter</title></head>
             <body>
             <h1>iqAir Exporter</h1>
             <p><a href='` + *metricsPath + `'>Metrics</a></p>
             </body>
             </html>`))
	})

	level.Info(logger).Log("msg", "Listening on address", "address", *listenAddress)
	srv := &http.Server{Addr: *listenAddress}

	if err := web.ListenAndServe(srv, *webConfig, logger); err != nil {
		level.Error(logger).Log("msg", "Error starting HTTP server", "err", err)
		os.Exit(1)
	}
}
