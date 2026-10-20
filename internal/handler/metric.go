package handler

import (
	"bytes"
	"errors"
	"html/template"
	"net/http"

	"github.com/astre-ash/omnigo/internal/domain"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

type MetricService interface {
	Update(metricType, name, valueStr string) error
	Get(metricType, name string) (string, error)
	GetAll() (map[string]float64, map[string]int64)
}

type MetricHandler struct {
	service MetricService
	tmpl    *template.Template
	logger  *zap.Logger
}

func NewMetricHandler(s MetricService, l *zap.Logger) *MetricHandler {
	parsedTemplate := template.Must(template.New("metrics").Parse(htmlTemplate))
	return &MetricHandler{
		service: s,
		tmpl:    parsedTemplate,
		logger:  l,
	}
}

func (h *MetricHandler) Update(w http.ResponseWriter, r *http.Request) {
	metricType := chi.URLParam(r, "type")
	metricName := chi.URLParam(r, "name")
	metricValueStr := chi.URLParam(r, "value")

	err := h.service.Update(metricType, metricName, metricValueStr)
	if err != nil {
		h.handleError(w, err)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *MetricHandler) GetValue(w http.ResponseWriter, r *http.Request) {
	metricType := chi.URLParam(r, "type")
	metricName := chi.URLParam(r, "name")

	respVal, err := h.service.Get(metricType, metricName)
	if err != nil {
		h.handleError(w, err)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(respVal))
}

func (h *MetricHandler) GetAll(w http.ResponseWriter, r *http.Request) {
	gauges, counters := h.service.GetAll()

	data := struct {
		Gauges   map[string]float64
		Counters map[string]int64
	}{
		Gauges:   gauges,
		Counters: counters,
	}

	var buf bytes.Buffer
	if err := h.tmpl.Execute(&buf, data); err != nil {
		h.logger.Error("failed to execute template", zap.Error(err))
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = buf.WriteTo(w)
}

func (h *MetricHandler) handleError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrMetricNotFound):
		http.Error(w, domain.ErrMetricNotFound.Error(), http.StatusNotFound)
	case errors.Is(err, domain.ErrInvalidValue), errors.Is(err, domain.ErrUnknownMetricType):
		h.logger.Warn("bad request", zap.Error(err))
		http.Error(w, err.Error(), http.StatusBadRequest)
	default:
		h.logger.Error("internal server error", zap.Error(err))
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}
