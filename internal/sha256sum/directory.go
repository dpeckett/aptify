// SPDX-License-Identifier: MPL-2.0
/*
 * Copyright (C) 2024 Damian Peckett <damian@pecke.tt>.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/.
 */

package sha256sum

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/dpeckett/deb822/types/filehash"
)

// Directory returns the sha256sum of all files in a directory.
func Directory(dir string) ([]filehash.FileHash, error) {
	var hashes []filehash.FileHash
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		sum, err := File(path)
		if err != nil {
			return err
		}

		relativePath, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}

		fi, err := d.Info()
		if err != nil {
			return err
		}

		hashes = append(hashes, filehash.FileHash{
			Filename: relativePath,
			Hash:     sum,
			Size:     fi.Size(),
		})

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to walk directory: %w", err)
	}

	return hashes, nil
}
