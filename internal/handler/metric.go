package handler

import (
	"bytes"
	"html/template"
	"math"
	"net/http"
	"strconv"

	"github.com/astre-ash/omnigo/internal/domain"
	"github.com/go-chi/chi/v5"
)

type MetricHandler struct {
	storage domain.MetricStorage
	tmpl    *template.Template
}

func NewMericHandler(storage domain.MetricStorage) *MetricHandler {
	parsedTemplate := template.Must(template.New("metrics").Parse(htmlTemplate))
	return &MetricHandler{
		storage: storage,
		tmpl:    parsedTemplate,
	}
}

func (h *MetricHandler) Update(w http.ResponseWriter, r *http.Request) {
	metricType := chi.URLParam(r, "type")
	metricName := chi.URLParam(r, "name")
	metricValueStr := chi.URLParam(r, "value")

	switch metricType {

	case domain.TypeGauge:
		val, err := strconv.ParseFloat(metricValueStr, 64)
		if err != nil || math.IsNaN(val) || math.IsInf(val, 0) {
			http.Error(w, "invalid gauge value", http.StatusBadRequest)
			return
		}

		h.storage.UpdateGauge(metricName, val)

	case domain.TypeCounter:
		val, err := strconv.ParseInt(metricValueStr, 10, 64)
		if err != nil {
			http.Error(w, "invalid counter value", http.StatusBadRequest)
			return
		}
		h.storage.UpdateCounter(metricName, val)

	default:
		http.Error(w, "unknown metric type", http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)

}

func (h *MetricHandler) GetValue(w http.ResponseWriter, r *http.Request) {
	metricType := chi.URLParam(r, "type")
	metricName := chi.URLParam(r, "name")

	var respVal string

	switch metricType {
	case domain.TypeGauge:
		val, ok := h.storage.GetGauge(metricName)
		if !ok {
			http.Error(w, "metric not found", http.StatusNotFound)
			return
		}

		respVal = strconv.FormatFloat(val, 'f', -1, 64)

	case domain.TypeCounter:
		val, ok := h.storage.GetCounter(metricName)
		if !ok {
			http.Error(w, "metric not found", http.StatusNotFound)
			return
		}
		respVal = strconv.FormatInt(val, 10)

	default:

		http.Error(w, "unknown metric type", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(respVal))

}

func (h *MetricHandler) GetAll(w http.ResponseWriter, r *http.Request) {
	data := struct {
		Gauges   map[string]float64
		Counters map[string]int64
	}{
		Gauges:   h.storage.GetAllGauges(),
		Counters: h.storage.GetAllCounters(),
	}

	var buf bytes.Buffer

	if err := h.tmpl.Execute(&buf, data); err != nil {
		http.Error(w, "internal server error", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = buf.WriteTo(w)
}
