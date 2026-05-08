package handler

import (
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
	io_prometheus_client "github.com/prometheus/client_model/go"
	"github.com/valyala/fasthttp"
)

var (
	requestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "Duration of HTTP requests.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"path"},
	)
)

func init() {
	prometheus.MustRegister(requestDuration)
}

// MetricsHandler handles prometheus metrics requests for fasthttp
func MetricsHandler(ctx *fasthttp.RequestCtx) {
	ctx.SetContentType("text/plain; version=0.0.4; charset=utf-8")

	mfs, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		ctx.SetStatusCode(fasthttp.StatusInternalServerError)
		ctx.SetBodyString("Failed to gather metrics: " + err.Error())
		return
	}

	for _, mf := range mfs {
		writeMetricFamily(ctx, mf)
	}

	ctx.SetStatusCode(fasthttp.StatusOK)
}

func writeMetricFamily(ctx *fasthttp.RequestCtx, mf *io_prometheus_client.MetricFamily) {
	name := mf.GetName()
	help := mf.GetHelp()
	mtype := mf.GetType()

	ctx.WriteString(fmt.Sprintf("# HELP %s %s\n", name, help))
	ctx.WriteString(fmt.Sprintf("# TYPE %s %s\n", name, mtype.String()))

	for _, m := range mf.GetMetric() {
		labels := ""
		for i, lp := range m.GetLabel() {
			if i > 0 {
				labels += ","
			}
			labels += fmt.Sprintf("%s=\"%s\"", lp.GetName(), lp.GetValue())
		}

		switch mtype {
		case io_prometheus_client.MetricType_COUNTER:
			if labels != "" {
				ctx.WriteString(fmt.Sprintf("%s{%s} %v\n", name, labels, m.GetCounter().GetValue()))
			} else {
				ctx.WriteString(fmt.Sprintf("%s %v\n", name, m.GetCounter().GetValue()))
			}
		case io_prometheus_client.MetricType_GAUGE:
			if labels != "" {
				ctx.WriteString(fmt.Sprintf("%s{%s} %v\n", name, labels, m.GetGauge().GetValue()))
			} else {
				ctx.WriteString(fmt.Sprintf("%s %v\n", name, m.GetGauge().GetValue()))
			}
		case io_prometheus_client.MetricType_HISTOGRAM:
			h := m.GetHistogram()
			if labels != "" {
				ctx.WriteString(fmt.Sprintf("%s_sum{%s} %v\n", name, labels, h.GetSampleSum()))
				ctx.WriteString(fmt.Sprintf("%s_count{%s} %v\n", name, labels, h.GetSampleCount()))
			} else {
				ctx.WriteString(fmt.Sprintf("%s_sum %v\n", name, h.GetSampleSum()))
				ctx.WriteString(fmt.Sprintf("%s_count %v\n", name, h.GetSampleCount()))
			}
		case io_prometheus_client.MetricType_SUMMARY:
			s := m.GetSummary()
			if labels != "" {
				ctx.WriteString(fmt.Sprintf("%s_sum{%s} %v\n", name, labels, s.GetSampleSum()))
				ctx.WriteString(fmt.Sprintf("%s_count{%s} %v\n", name, labels, s.GetSampleCount()))
			} else {
				ctx.WriteString(fmt.Sprintf("%s_sum %v\n", name, s.GetSampleSum()))
				ctx.WriteString(fmt.Sprintf("%s_count %v\n", name, s.GetSampleCount()))
			}
		}
	}
}
