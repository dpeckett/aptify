// SPDX-License-Identifier: MPL-2.0
/*
 * Copyright (C) 2024 Damian Peckett <damian@pecke.tt>.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/.
 *
 * Portions of this file are based on code originally from: github.com/moby/buildkit
 *
 * Copyright 2024 The BuildKit Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package appcontext

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"sync"
)

var appContextCache context.Context
var appContextOnce sync.Once

// Context returns a static context that reacts to termination signals of the
// running process. Useful in CLI tools.
func Context() context.Context {
	appContextOnce.Do(func() {
		signals := make(chan os.Signal, 2048)
		signal.Notify(signals, terminationSignals...)

		const exitLimit = 3
		retries := 0

		ctx, cancel := context.WithCancelCause(context.Background())
		appContextCache = ctx

		go func() {
			for {
				<-signals
				retries++
				err := fmt.Errorf("got %d SIGTERM/SIGINTs, forcing shutdown", retries)
				cancel(err)
				if retries >= exitLimit {
					slog.Error("Failed to shutdown gracefully, forcing exit", slog.Int("retries", retries))
					os.Exit(1)
				}
			}
		}()
	})
	return appContextCache
}
