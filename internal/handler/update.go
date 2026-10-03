package handler

import (
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/astre-ash/omnigo/internal/domain"
)

type UpdateHandler struct {
	storage domain.MetricStorage
}

func NewUpdateHandler(storage domain.MetricStorage) *UpdateHandler {
	return &UpdateHandler{
		storage: storage,
	}
}

func (h *UpdateHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "only POST method is allowed", http.StatusMethodNotAllowed)
		return
	}

	path := strings.Trim(r.URL.Path, "/")
	parts := strings.Split(path, "/")

	if len(parts) != 4 || parts[0] != "update" {
		http.Error(w, "invalid path format", http.StatusNotFound)
		return
	}
	metricType := parts[1]
	metricName := parts[2]
	metricValueStr := parts[3]

	if metricName == "" {
		http.Error(w, "metric name is empty", http.StatusNotFound)
		return
	}

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
