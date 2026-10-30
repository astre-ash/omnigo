package handler

import (
	"bytes"
	"encoding/json"
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

	GetMetric(m domain.Metrics) (domain.Metrics, error)
	UpdateMetric(m domain.Metrics) (domain.Metrics, error)
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

func (h *MetricHandler) GetValueJSON(w http.ResponseWriter, r *http.Request) {
	var req domain.Metrics
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Warn("cannot decode request JSON", zap.Error(err))
		http.Error(w, "invalid JSON payload", http.StatusBadRequest)
		return
	}

	if req.ID == "" {
		http.Error(w, "metric name is empty", http.StatusNotFound)
		return
	}
	res, err := h.service.GetMetric(req)
	if err != nil {
		h.handleError(w, err)
		return
	}

	h.sendJSON(w, http.StatusOK, res)

}

func (h *MetricHandler) UpdateJSON(w http.ResponseWriter, r *http.Request) {
	var req domain.Metrics
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Warn("cannot decode request JSON", zap.Error(err))
		http.Error(w, "invalid JSON payload", http.StatusBadRequest)
		return
	}

	if req.ID == "" {
		http.Error(w, "metric ID cannot be empty", http.StatusNotFound)
		return
	}

	res, err := h.service.UpdateMetric(req)
	if err != nil {

		h.handleError(w, err)
		return
	}

	h.sendJSON(w, http.StatusOK, res)
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

func (h *MetricHandler) sendJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.logger.Error("failed to encode response JSON", zap.Error(err))
	}
}
