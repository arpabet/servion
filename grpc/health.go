/*
 * Copyright (c) 2026 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package serviongrpc

import (
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

// registerHealth installs the standard grpc.health.v1.Health service and marks
// the overall server ("") and every already-registered service as SERVING. It is
// enabled by the "health" flag in "<server>.options" and is well suited to
// Kubernetes gRPC liveness/readiness probes.
func registerHealth(srv *grpc.Server) {
	hsrv := health.NewServer()
	hsrv.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	for name := range srv.GetServiceInfo() {
		hsrv.SetServingStatus(name, healthpb.HealthCheckResponse_SERVING)
	}
	healthpb.RegisterHealthServer(srv, hsrv)
}
