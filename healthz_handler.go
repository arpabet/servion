/*
 * Copyright (c) 2026 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package servion

import (
	"encoding/json"
	"net/http"
)

type implHealthHandler struct {
	Runtime    Runtime     `inject:""`
	Components []Component `inject:"optional,level=1"`

	HealthPattern string `value:"health.pattern,default=/healthz"`
	Detailed      bool   `value:"health.detailed,default=false"`
}

// HealthHandler creates a health check HttpHandler bean for Kubernetes
// liveness/readiness probes. Include it in the glue context to register
// the endpoint automatically.
//
// Configuration properties:
//
//	health.pattern  – URL pattern (default "/healthz")
//	health.detailed – include per-component stats (default false)
func HealthHandler() HttpHandler {
	return &implHealthHandler{}
}

func (t *implHealthHandler) Pattern() string {
	return t.HealthPattern
}

type healthResponse struct {
	Status     string                       `json:"status"`
	Components map[string]map[string]string `json:"components,omitempty"`
}

func (t *implHealthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	status := "UP"
	code := http.StatusOK

	if !t.Runtime.Active() {
		status = "DOWN"
		code = http.StatusServiceUnavailable
	}

	resp := healthResponse{Status: status}

	if t.Detailed && len(t.Components) > 0 {
		resp.Components = make(map[string]map[string]string, len(t.Components))
		for _, comp := range t.Components {
			stats := make(map[string]string)
			_ = comp.GetStats(func(name, value string) bool {
				stats[name] = value
				return true
			})
			resp.Components[comp.BeanName()] = stats
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(resp)
}
