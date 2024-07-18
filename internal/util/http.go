// SPDX-License-Identifier: MPL-2.0
/*
 * Copyright (C) 2024 Damian Peckett <damian@pecke.tt>.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/.
 */

package util

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"time"
)

// ServeWithContext starts an HTTP server and listens for incoming requests. It
// will shut down the server when the provided context is canceled.
func ServeWithContext(ctx context.Context, srv *http.Server, lis net.Listener) error {
	srv.BaseContext = func(_ net.Listener) context.Context {
		return ctx
	}

	go func() {
		<-ctx.Done()

		shutdownCtx, shutdownCtxCancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer shutdownCtxCancel()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			slog.Error("HTTP server shutdown", slog.Any("error", err))
		}
	}()

	if srv.TLSConfig != nil {
		slog.Info("Starting HTTPS server", slog.Any("addr", lis.Addr()))

		if err := srv.ServeTLS(lis, "", ""); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	} else {
		slog.Info("Starting HTTP server", slog.Any("addr", lis.Addr()))

		if err := srv.Serve(lis); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	}

	return nil
}

// LoggingMiddleware is an HTTP middleware that logs information about the
// incoming request.
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		slog.Info("HTTP request",
			slog.String("method", r.Method),
			slog.String("url", r.URL.String()),
			slog.String("remote_addr", r.RemoteAddr),
			slog.String("user_agent", r.UserAgent()),
			slog.String("duration", time.Since(start).String()),
		)
	})
}
