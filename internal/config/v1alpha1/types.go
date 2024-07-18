// SPDX-License-Identifier: MPL-2.0
/*
 * Copyright (C) 2024 Damian Peckett <damian@pecke.tt>.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/.
 */

package v1alpha1

import (
	"fmt"

	"github.com/dpeckett/aptify/internal/config/types"
)

const APIVersion = "aptify/v1alpha1"

type Repository struct {
	types.TypeMeta `yaml:",inline"`
	// Releases is the list of releases to generate.
	Releases []ReleaseConfig
}

// ReleaseConfig is the configuration for a release.
type ReleaseConfig struct {
	// Name is the name of the release.
	Name string
	// Version is the version of the release.
	Version string
	// Origin is the origin of the release.
	// This specifies the source or the entity responsible for creating and distributing the release.
	Origin string
	// Label is the label of the release.
	// This provides a human-readable identifier or tag for the release.
	Label string
	// Suite is the suite of the release.
	// This categorizes the release into a broader collection or group of releases.
	Suite string
	// Description is a description of the release.
	Description string
	// Components is the list of components (and their packages) within the release.
	Components []ComponentConfig
}

// ComponentConfig is the configuration for a component.
type ComponentConfig struct {
	// Name is the name of the component.
	Name string
	// Packages is the list of file system paths/glob patterns to deb files that
	// will be included within the component.
	Packages []string
}

func (r *Repository) GetAPIVersion() string {
	return APIVersion
}

func (r *Repository) GetKind() string {
	return "Repository"
}

func (r *Repository) PopulateTypeMeta() {
	r.TypeMeta = types.TypeMeta{
		APIVersion: APIVersion,
		Kind:       "Repository",
	}
}

func GetConfigByKind(kind string) (types.Config, error) {
	switch kind {
	case "Repository":
		return &Repository{}, nil
	default:
		return nil, fmt.Errorf("unsupported kind: %s", kind)
	}
}
