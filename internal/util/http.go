// SPDX-License-Identifier: AGPL-3.0-or-later
/*
 * Copyright (C) 2024 Damian Peckett <damian@pecke.tt>.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program. If not, see <https://www.gnu.org/licenses/>.
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
