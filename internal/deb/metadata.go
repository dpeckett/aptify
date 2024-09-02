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

package deb

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"

	"github.com/dpeckett/archivefs/arfs"
	"github.com/dpeckett/archivefs/tarfs"
	"github.com/dpeckett/deb822"
	"github.com/dpeckett/deb822/types"
	"github.com/dpeckett/uncompr"
)

func GetMetadata(path string) (*types.Package, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open package file: %w", err)
	}
	defer f.Close()

	debFS, err := arfs.Open(f)
	if err != nil {
		return nil, fmt.Errorf("failed to open archive: %w", err)
	}

	if err := ensureIsDebianPackage(debFS); err != nil {
		return nil, err
	}

	// Look for control archive in the debian package.
	entries, err := debFS.ReadDir(".")
	if err != nil {
		return nil, fmt.Errorf("failed to read debian package: %w", err)
	}

	var controlArchiveFilename string
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "control.tar") {
			controlArchiveFilename = entry.Name()
			break
		}
	}
	if controlArchiveFilename == "" {
		return nil, fmt.Errorf("failed to find control archive in debian package")
	}

	controlArchiveFile, err := debFS.Open(controlArchiveFilename)
	if err != nil {
		return nil, fmt.Errorf("failed to open control archive: %w", err)
	}

	controlArchiveReader, err := uncompr.NewReader(controlArchiveFile)
	if err != nil {
		return nil, fmt.Errorf("failed to decompress control archive: %w", err)
	}

	// Read control archive entirely into memory (as we need a seekable reader for
	// the tarfs implementation).
	controlArchiveData, err := io.ReadAll(controlArchiveReader)
	if err != nil {
		return nil, fmt.Errorf("failed to read control archive: %w", err)
	}

	controlArchiveFS, err := tarfs.Open(bytes.NewReader(controlArchiveData))
	if err != nil {
		return nil, fmt.Errorf("failed to open control archive: %w", err)
	}

	// Look for control file in the control archive.
	controlFile, err := controlArchiveFS.Open("control")
	if err != nil {
		return nil, fmt.Errorf("failed to open control file: %w", err)
	}

	dec, err := deb822.NewDecoder(controlFile, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create control file decoder: %w", err)
	}

	var pkg types.Package
	if err := dec.Decode(&pkg); err != nil {
		return nil, fmt.Errorf("failed to decode control file: %w", err)
	}

	return &pkg, nil
}

// Check that the package is a debian 2.0 format package.
func ensureIsDebianPackage(debFS fs.FS) error {
	debianBinaryFile, err := debFS.Open("debian-binary")
	if err != nil {
		return fmt.Errorf("failed to open debian-binary file: %w", err)
	}

	debianBinary, err := io.ReadAll(debianBinaryFile)
	if err != nil {
		return fmt.Errorf("failed to read debian-binary file: %w", err)
	}

	if string(debianBinary) != "2.0\n" {
		return fmt.Errorf("unsupported debian package version: %s", debianBinary)
	}

	return nil
}
