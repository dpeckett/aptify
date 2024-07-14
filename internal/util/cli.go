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
	"github.com/urfave/cli/v2"
)

// BeforeAll runs multiple BeforeFuncs in order
func BeforeAll(fns ...cli.BeforeFunc) cli.BeforeFunc {
	return func(c *cli.Context) error {
		for _, fn := range fns {
			if err := fn(c); err != nil {
				return err
			}
		}

		return nil
	}
}
