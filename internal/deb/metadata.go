// SPDX-License-Identifier: MPL-2.0
/*
 * Copyright (C) 2024 Damian Peckett <damian@pecke.tt>.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/.
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
	"github.com/dpeckett/compressmagic"
	"github.com/dpeckett/deb822"
	"github.com/dpeckett/deb822/types"
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

	controlArchiveReader, err := compressmagic.NewReader(controlArchiveFile)
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
