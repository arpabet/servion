/*
 * Copyright (c) 2026 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package serviongrpc

import "context"

// tokenAuth implements credentials.PerRPCCredentials, attaching a bearer token
// to every outgoing call as the "authorization" metadata header. It is paired
// with the server-side AuthInterceptor.
type tokenAuth struct {
	token  string
	secure bool
}

func (t tokenAuth) GetRequestMetadata(ctx context.Context, in ...string) (map[string]string, error) {
	return map[string]string{
		"authorization": "Bearer " + t.token,
	}, nil
}

func (t tokenAuth) RequireTransportSecurity() bool {
	return t.secure
}
