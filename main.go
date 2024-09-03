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

package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	stdtime "time"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/armor"
	"github.com/ProtonMail/go-crypto/openpgp/packet"
	"github.com/adrg/xdg"
	"github.com/dpeckett/aptify/internal/config"
	"github.com/dpeckett/aptify/internal/config/v1alpha1"
	"github.com/dpeckett/aptify/internal/constants"
	"github.com/dpeckett/aptify/internal/deb"
	"github.com/dpeckett/aptify/internal/sha256sum"
	"github.com/dpeckett/aptify/internal/util"
	"github.com/dpeckett/aptify/internal/util/appcontext"
	"github.com/dpeckett/deb822"
	"github.com/dpeckett/deb822/types"
	"github.com/dpeckett/deb822/types/arch"
	"github.com/dpeckett/deb822/types/list"
	"github.com/dpeckett/deb822/types/time"
	"github.com/dpeckett/telemetry"
	telemetryv1alpha1 "github.com/dpeckett/telemetry/v1alpha1"
	"github.com/dpeckett/uncompr"
	cp "github.com/otiai10/copy"
	"github.com/urfave/cli/v2"
	"golang.org/x/crypto/acme"
	"golang.org/x/crypto/acme/autocert"
	"golang.org/x/sync/errgroup"
)

func main() {
	defaultConfDir, _ := xdg.ConfigFile("aptify")

	persistentFlags := []cli.Flag{
		&cli.GenericFlag{
			Name:  "log-level",
			Usage: "Set the log verbosity level",
			Value: util.FromSlogLevel(slog.LevelInfo),
		},
		&cli.StringFlag{
			Name:  "config-dir",
			Usage: "Directory to store configuration",
			Value: defaultConfDir,
		},
	}

	initLogger := func(c *cli.Context) error {
		slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level: (*slog.Level)(c.Generic("log-level").(*util.LevelFlag)),
		})))

		return nil
	}

	initConfDir := func(c *cli.Context) error {
		confDir := c.String("config-dir")
		if confDir == "" {
			return fmt.Errorf("no configuration directory specified")
		}

		if err := os.MkdirAll(confDir, 0o700); err != nil {
			return fmt.Errorf("failed to create configuration directory: %w", err)
		}

		return nil
	}

	// Collect anonymized usage statistics.
	var telemetryReporter *telemetry.Reporter

	initTelemetry := func(c *cli.Context) error {
		telemetryReporter = telemetry.NewReporter(c.Context, slog.Default(), telemetry.Configuration{
			BaseURL: constants.TelemetryURL,
			Tags:    []string{"aptify"},
		})

		// Some basic system information.
		info := map[string]string{
			"os":      runtime.GOOS,
			"arch":    runtime.GOARCH,
			"num_cpu": fmt.Sprintf("%d", runtime.NumCPU()),
			"version": constants.Version,
		}

		// Are we running in a container?
		if os.Getenv("container") != "" {
			info["container"] = os.Getenv("container")
		}

		telemetryReporter.ReportEvent(&telemetryv1alpha1.TelemetryEvent{
			Kind:   telemetryv1alpha1.TelemetryEventKindInfo,
			Name:   "ApplicationStart",
			Values: info,
		})

		return nil
	}

	shutdownTelemetry := func(c *cli.Context) error {
		if telemetryReporter == nil {
			return nil
		}

		telemetryReporter.ReportEvent(&telemetryv1alpha1.TelemetryEvent{
			Kind: telemetryv1alpha1.TelemetryEventKindInfo,
			Name: "ApplicationStop",
		})

		// Don't want to block the shutdown of the application for too long.
		ctx, cancel := context.WithTimeout(context.Background(), 3*stdtime.Second)
		defer cancel()

		if err := telemetryReporter.Shutdown(ctx); err != nil {
			slog.Error("Failed to close telemetry reporter", slog.Any("error", err))
		}

		return nil
	}

	app := &cli.App{
		Name:    "aptify",
		Usage:   "Create apt repositories from Debian packages",
		Version: constants.Version,
		Commands: []*cli.Command{
			{
				Name:  "init-keys",
				Usage: "Generate a new GPG key pair for signing releases",
				Flags: append([]cli.Flag{
					&cli.StringFlag{
						Name:  "name",
						Usage: "Name of the key owner",
					},
					&cli.StringFlag{
						Name:  "comment",
						Usage: "Comment to add to the key",
					},
					&cli.StringFlag{
						Name:  "email",
						Usage: "Email address of the key owner",
					},
				}, persistentFlags...),
				Before: util.BeforeAll(initLogger, initConfDir, initTelemetry),
				After:  shutdownTelemetry,
				Action: func(c *cli.Context) error {
					entityConfig := &packet.Config{
						RSABits: 4096,
						Time:    stdtime.Now,
					}

					slog.Info("Generating RSA key")

					// Create a new entity.
					entity, err := openpgp.NewEntity(c.String("name"), c.String("comment"), c.String("email"), entityConfig)
					if err != nil {
						return fmt.Errorf("failed to create entity: %w", err)
					}

					slog.Info("Saving key pair", slog.String("dir", c.String("config-dir")))

					// Serialize the private key.
					var privateKey bytes.Buffer
					privateKeyWriter, err := armor.Encode(&privateKey, openpgp.PrivateKeyType, nil)
					if err != nil {
						return fmt.Errorf("failed to encode private key: %w", err)
					}
					if err := entity.SerializePrivate(privateKeyWriter, nil); err != nil {
						return fmt.Errorf("failed to serialize private key: %w", err)
					}
					if err := privateKeyWriter.Close(); err != nil {
						return fmt.Errorf("failed to close private key writer: %w", err)
					}

					confDir := c.String("config-dir")

					// Write private key to file.
					if err := os.WriteFile(filepath.Join(confDir, "aptify_private.asc"), privateKey.Bytes(), 0o600); err != nil {
						return fmt.Errorf("failed to write private key: %w", err)
					}

					return nil
				},
			},
			{
				Name:  "build",
				Usage: "Build a Debian repository from a configuration file",
				Flags: append([]cli.Flag{
					&cli.StringFlag{
						Name:     "config",
						Aliases:  []string{"c"},
						Usage:    "Configuration file",
						Required: true,
					},
					&cli.StringFlag{
						Name:    "repository-dir",
						Aliases: []string{"d"},
						Usage:   "Directory to store the repository",
						Value:   "repository",
					},
				}, persistentFlags...),
				Before: util.BeforeAll(initLogger, initConfDir, initTelemetry),
				After:  shutdownTelemetry,
				Action: func(c *cli.Context) error {
					repoDir := c.String("repository-dir")

					slog.Info("Building repository", slog.String("dir", repoDir))

					privateKeyPath := filepath.Join(c.String("config-dir"), "aptify_private.asc")

					return buildRepository(
						repoDir,
						c.String("config"),
						privateKeyPath,
					)
				},
			},
			{
				Name:  "serve",
				Usage: "Serve a Debian repository over HTTP/s",
				Flags: append([]cli.Flag{
					&cli.StringFlag{
						Name:     "repository-dir",
						Aliases:  []string{"d"},
						Usage:    "Directory containing the repository files",
						Required: true,
					},
					&cli.StringFlag{
						Name:    "listen",
						Aliases: []string{"l"},
						Usage:   "Address to listen on",
						Value:   "localhost",
					},
					&cli.IntFlag{
						Name:  "http-port",
						Usage: "Port to listen on for HTTP",
						Value: 8080,
					},
					&cli.IntFlag{
						Name:  "https-port",
						Usage: "Port to listen on for HTTPS (if enabled)",
						Value: 8443,
					},
					&cli.BoolFlag{
						Name:  "tls",
						Usage: "Enable serving the repository over HTTPS using Let's Encrypt",
					},
					&cli.StringFlag{
						Name:  "domain",
						Usage: "Public TLS domain for Let's Encrypt",
					},
					&cli.StringFlag{
						Name:  "email",
						Usage: "Email address for Let's Encrypt",
					},
				}, persistentFlags...),
				Before: util.BeforeAll(initLogger, initConfDir, initTelemetry),
				After:  shutdownTelemetry,
				Action: func(c *cli.Context) error {
					repoDir := c.String("repository-dir")

					slog.Info("Serving repository", slog.String("dir", repoDir))

					mux := http.NewServeMux()
					mux.Handle("/", http.FileServer(http.Dir(repoDir)))

					var httpHandler http.Handler = mux
					var tlsConfig *tls.Config

					if c.Bool("tls") {
						if c.String("domain") == "" {
							return errors.New("`domain` is required when using TLS")
						}

						if c.String("email") == "" {
							return errors.New("`email` is required when using TLS")
						}

						autoTLSManager := autocert.Manager{
							Prompt:     autocert.AcceptTOS,
							Cache:      autocert.DirCache(filepath.Join(c.String("config-dir"), "autocert")),
							HostPolicy: autocert.HostWhitelist(c.String("domain")),
							Email:      c.String("email"),
						}

						tlsConfig = &tls.Config{
							ServerName:     c.String("domain"),
							GetCertificate: autoTLSManager.GetCertificate,
							NextProtos:     []string{acme.ALPNProto},
						}

						httpHandler = autoTLSManager.HTTPHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
							// If the request is for a signing key (eg. the asc file extension), redirect to HTTPS.
							if strings.HasSuffix(r.URL.Path, ".asc") {
								http.Redirect(w, r, "https://"+r.Host+r.RequestURI, http.StatusMovedPermanently)
								return
							}

							// Otherwise, serve the request over HTTP.
							mux.ServeHTTP(w, r)
						}))
					}

					g, ctx := errgroup.WithContext(appcontext.Context())

					httpListener, err := net.Listen("tcp", net.JoinHostPort(c.String("listen"), strconv.Itoa(c.Int("http-port"))))
					if err != nil {
						return fmt.Errorf("failed to listen on http port: %w", err)
					}

					httpSrv := &http.Server{
						Handler:     util.LoggingMiddleware(httpHandler),
						BaseContext: func(_ net.Listener) context.Context { return ctx },
					}

					g.Go(func() error {
						return util.ServeWithContext(ctx, httpSrv, httpListener)
					})

					if c.Bool("tls") {
						httpsListener, err := net.Listen("tcp", net.JoinHostPort(c.String("listen"), strconv.Itoa(c.Int("https-port"))))
						if err != nil {
							return fmt.Errorf("failed to listen on https port: %w", err)
						}

						httpsSrv := &http.Server{
							Handler:     util.LoggingMiddleware(mux),
							BaseContext: func(_ net.Listener) context.Context { return ctx },
							TLSConfig:   tlsConfig,
						}

						g.Go(func() error {
							return util.ServeWithContext(ctx, httpsSrv, httpsListener)
						})
					}

					return g.Wait()
				},
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		slog.Error("Error", slog.Any("error", err))
		os.Exit(1)
	}
}

func buildRepository(repoDir, confPath, privateKeyPath string) error {
	if _, err := os.Stat(privateKeyPath); os.IsNotExist(err) {
		return fmt.Errorf("private key not found; run 'aptify init-keys' to generate one")
	}

	privateKey, err := loadPrivateKey(privateKeyPath)
	if err != nil {
		return fmt.Errorf("failed to read private key: %w", err)
	}

	confFile, err := os.Open(confPath)
	if err != nil {
		return fmt.Errorf("failed to open config file: %w", err)
	}
	defer confFile.Close()

	conf, err := config.FromYAML(confFile)
	if err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}

	packagesForReleaseComponent := make(map[string][]types.Package)
	archsForReleaseComponent := make(map[string]map[string]bool)
	pkgPoolPaths := make(map[string]string)

	// Copy packages to the pool directory.
	for _, releaseConf := range conf.Releases {
		for _, componentConf := range releaseConf.Components {
			releaseComponent := fmt.Sprintf("%s/%s", releaseConf.Name, componentConf.Name)

			for _, pattern := range componentConf.Packages {
				matches, err := filepath.Glob(pattern)
				if err != nil {
					return fmt.Errorf("failed to find deb files for %s: %w", pattern, err)
				}

				for _, pkgPath := range matches {
					pkg, err := deb.GetMetadata(pkgPath)
					if err != nil {
						return fmt.Errorf("failed to get package metadata: %w", err)
					}

					pkg.SHA256, err = sha256sum.File(pkgPath)
					if err != nil {
						return fmt.Errorf("failed to hash package: %w", err)
					}

					if _, ok := archsForReleaseComponent[releaseComponent]; !ok {
						archsForReleaseComponent[releaseComponent] = make(map[string]bool)
					}
					archsForReleaseComponent[releaseComponent][pkg.Architecture.String()] = true

					// Only copy each deb file once.
					// Use the component name from the first release that includes the package.
					if existingPoolPath, ok := pkgPoolPaths[pkgPath]; !ok {
						pkg.Filename = poolPathForPackage(componentConf.Name, pkg)

						if err := os.MkdirAll(filepath.Dir(filepath.Join(repoDir, pkg.Filename)), 0o755); err != nil {
							return fmt.Errorf("failed to create pool subdirectory: %w", err)
						}

						if err := cp.Copy(pkgPath, filepath.Join(repoDir, pkg.Filename)); err != nil {
							return fmt.Errorf("failed to copy package: %w", err)
						}

						pkgPoolPaths[pkgPath] = pkg.Filename
					} else {
						pkg.Filename = existingPoolPath
					}

					// Get the size of the package file.
					fi, err := os.Stat(filepath.Join(repoDir, pkg.Filename))
					if err != nil {
						return fmt.Errorf("failed to get package size: %w", err)
					}
					pkg.Size = int(fi.Size())

					packagesForReleaseComponent[releaseComponent] = append(packagesForReleaseComponent[releaseComponent], *pkg)
				}
			}
		}
	}

	// Create release files.
	for _, releaseConf := range conf.Releases {
		var architectures []arch.Arch

		for _, componentConf := range releaseConf.Components {
			releaseComponent := fmt.Sprintf("%s/%s", releaseConf.Name, componentConf.Name)

			for architecture := range archsForReleaseComponent[releaseComponent] {
				componentDir := filepath.Join(repoDir, "dists", releaseConf.Name, componentConf.Name)
				archDir := filepath.Join(componentDir, "binary-"+architecture)

				if err := os.MkdirAll(archDir, 0o755); err != nil {
					return fmt.Errorf("failed to create dists subdirectory: %w", err)
				}

				packages := packagesForReleaseComponent[releaseComponent]

				// Filter out packages that don't match the architecture.
				filteredPackages := make([]types.Package, 0, len(packages))
				for _, pkg := range packages {
					if pkg.Architecture.String() == architecture {
						filteredPackages = append(filteredPackages, pkg)
					}
				}
				packages = filteredPackages

				sort.Slice(packages, func(i, j int) bool {
					return packages[i].Compare(packages[j]) < 0
				})

				if err := writePackagesIndice(archDir, packages); err != nil {
					return fmt.Errorf("failed to write package lists: %w", err)
				}

				if err := writeContentsIndice(repoDir, componentDir, packages, architecture); err != nil {
					return fmt.Errorf("failed to write contents file: %w", err)
				}

				architectures = append(architectures, arch.MustParse(architecture))
			}
		}

		releaseDir := filepath.Join(repoDir, "dists", releaseConf.Name)
		if err := os.MkdirAll(releaseDir, 0o755); err != nil {
			return fmt.Errorf("failed to create release directory: %w", err)
		}

		if err := writeReleaseFile(releaseDir, releaseConf, architectures, privateKey); err != nil {
			return fmt.Errorf("failed to write release: %w", err)
		}
	}

	// Save a copy of the signing key.
	signingKeyFile, err := os.Create(filepath.Join(repoDir, "signing_key.asc"))
	if err != nil {
		return fmt.Errorf("failed to create signing key file: %w", err)
	}
	defer signingKeyFile.Close()

	publicKeyWriter, err := armor.Encode(signingKeyFile, openpgp.PublicKeyType, nil)
	if err != nil {
		return fmt.Errorf("failed to encode public key: %w", err)
	}

	if err := privateKey.Serialize(publicKeyWriter); err != nil {
		return fmt.Errorf("failed to serialize public key: %w", err)
	}

	if err := publicKeyWriter.Close(); err != nil {
		return fmt.Errorf("failed to close public key writer: %w", err)
	}

	return nil
}

func writePackagesIndice(archDir string, packages []types.Package) error {
	slog.Info("Writing Packages indice",
		slog.String("dir", archDir), slog.Int("count", len(packages)))

	var packageList bytes.Buffer
	if err := deb822.Marshal(&packageList, packages); err != nil {
		return fmt.Errorf("failed to marshal packages: %w", err)
	}

	for _, name := range []string{"Packages", "Packages.xz"} {
		f, err := os.Create(filepath.Join(archDir, name))
		if err != nil {
			return fmt.Errorf("failed to create Packages file: %w", err)
		}
		defer f.Close()

		w, err := uncompr.NewWriter(f, f.Name())
		if err != nil {
			return fmt.Errorf("failed to create compression writer: %w", err)
		}
		defer w.Close()

		if _, err := w.Write(packageList.Bytes()); err != nil {
			return fmt.Errorf("failed to write Packages file: %w", err)
		}
	}

	return nil
}

func writeContentsIndice(repoDir, componentDir string, packages []types.Package, arch string) error {
	f, err := os.Create(filepath.Join(componentDir, fmt.Sprintf("Contents-%s.gz", arch)))
	if err != nil {
		return fmt.Errorf("failed to create Contents file: %w", err)
	}
	defer f.Close()

	w, err := uncompr.NewWriter(f, f.Name())
	if err != nil {
		return fmt.Errorf("failed to create compression writer: %w", err)
	}
	defer w.Close()

	slog.Info("Collecting package contents", slog.String("dir", componentDir))

	contents := make(map[string][]string)
	for _, pkg := range packages {
		pkgContents, err := deb.GetPackageContents(filepath.Join(repoDir, pkg.Filename))
		if err != nil {
			return fmt.Errorf("failed to get package contents: %w", err)
		}

		qualifiedPackageName := pkg.Name
		if pkg.Section != "" {
			qualifiedPackageName = fmt.Sprintf("%s/%s", pkg.Section, pkg.Name)
		}

		for _, path := range pkgContents {
			contents[path] = append(contents[path], qualifiedPackageName)
		}
	}

	paths := make([]string, 0, len(contents))
	for k := range contents {
		paths = append(paths, k)
	}

	sort.Strings(paths)

	slog.Info("Writing Contents indice",
		slog.String("dir", componentDir), slog.Int("count", len(paths)))

	for _, path := range paths {
		if _, err := fmt.Fprintf(w, "%s %s\n", path, strings.Join(contents[path], ",")); err != nil {
			return fmt.Errorf("failed to write contents: %w", err)
		}
	}

	return nil
}

func writeReleaseFile(releaseDir string, releaseConf v1alpha1.ReleaseConfig, architectures []arch.Arch, privateKey *openpgp.Entity) error {
	slog.Info("Writing Release file", slog.String("dir", releaseDir))

	var components []string
	for _, component := range releaseConf.Components {
		components = append(components, component.Name)
	}

	r := types.Release{
		Origin:        releaseConf.Origin,
		Label:         releaseConf.Label,
		Suite:         releaseConf.Suite,
		Version:       releaseConf.Version,
		Codename:      releaseConf.Name,
		Changelogs:    "no",
		Date:          time.Time(stdtime.Now().UTC()),
		Architectures: list.SpaceDelimited[arch.Arch](architectures),
		Components:    list.SpaceDelimited[string](components),
		Description:   releaseConf.Description,
	}

	var err error
	r.SHA256, err = sha256sum.Directory(releaseDir)
	if err != nil {
		return fmt.Errorf("failed to hash release: %w", err)
	}

	releaseFile, err := os.Create(filepath.Join(releaseDir, "InRelease"))
	if err != nil {
		return fmt.Errorf("failed to create Release file: %w", err)
	}
	defer releaseFile.Close()

	encoder, err := deb822.NewEncoder(releaseFile, privateKey)
	if err != nil {
		return fmt.Errorf("failed to create encoder: %w", err)
	}
	defer encoder.Close()

	if err := encoder.Encode(r); err != nil {
		return fmt.Errorf("failed to encode release: %w", err)
	}

	return nil
}

func poolPathForPackage(componentName string, pkg *types.Package) string {
	source := strings.TrimSpace(pkg.Source)
	if pkg.Source == "" {
		source = strings.TrimSpace(pkg.Name)
	}

	// If the source has a version, lop it off.
	if strings.Contains(source, "(") {
		source = source[:strings.Index(source, "(")]
	}

	prefix := source[:1]
	if strings.HasPrefix(source, "lib") {
		prefix = source[:4]
	}

	return filepath.Join("pool", componentName, prefix, source,
		fmt.Sprintf("%s_%s_%s.deb", pkg.Name, pkg.Version, pkg.Architecture))
}

func loadPrivateKey(path string) (*openpgp.Entity, error) {
	keyFile, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open private key: %w", err)
	}
	defer keyFile.Close()

	keyRing, err := openpgp.ReadArmoredKeyRing(keyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read armored key ring: %w", err)
	}

	return keyRing[0], nil
}
