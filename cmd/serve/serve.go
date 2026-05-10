/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

// Package serve implements the "serve" Cobra subcommand, which starts an
// embedded Go MCP server directly from an Arazzo document.
package serve

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/wso2/arazzo-mcp-generator/internal/docker"
	"github.com/wso2/arazzo-mcp-generator/internal/mcpserver"
	"github.com/wso2/arazzo-mcp-generator/internal/models"
	"github.com/wso2/arazzo-mcp-generator/internal/telemetry"
)

var (
	filePath      string
	port          int
	bearerToken   string
	apiKey        string
	apiKeyHeader  string
	traceEndpoint string
	otlpEndpoint  string
	disableTLS    bool
	dockerMode    bool
	outputDir     string
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start an embedded MCP server for an Arazzo workflow document",
	Long: `Start an embedded Go MCP server that exposes each Arazzo workflow as an
MCP tool accessible over streamable HTTP.

The server listens on the specified port and exposes:
  /mcp               — MCP protocol endpoint (streamable HTTP)
  /run/{workflowId}  — Direct workflow execution (POST)
  /lastResult/{id}   — Cached last-run result (GET)

Use --docker to cross-compile and package the server into a Docker image
instead of starting it locally (requires a working Go toolchain and Docker).`,
	RunE: runServe,
}

func init() {
	serveCmd.Flags().StringVarP(&filePath, "file", "f", "", "Path to the Arazzo YAML file or folder (required)")
	serveCmd.Flags().IntVarP(&port, "port", "p", 8080, "Port to listen on")
	serveCmd.Flags().StringVar(&bearerToken, "bearer-token", "", "Bearer token for API authentication")
	serveCmd.Flags().StringVar(&apiKey, "api-key", "", "API key for authentication")
	serveCmd.Flags().StringVar(&apiKeyHeader, "api-key-header", "X-API-Key", "Header name for API key")
	serveCmd.Flags().StringVar(&traceEndpoint, "trace-endpoint", "", "URL of the local tracer server to receive span events (e.g. http://127.0.0.1:59600/span-events)")
	serveCmd.Flags().StringVar(&otlpEndpoint, "otlp-endpoint", "", "Base URL of an OTLP/HTTP trace backend (e.g. http://localhost:4318 for Jaeger/Honeycomb)")
	serveCmd.Flags().BoolVar(&disableTLS, "disable-tls", false, "Disable TLS certificate verification for outbound HTTP requests (development only)")
	serveCmd.Flags().BoolVar(&dockerMode, "docker", false, "Package the Arazzo server into a Docker image instead of starting it locally")
	serveCmd.Flags().StringVarP(&outputDir, "output-dir", "o", "", "Output folder for Docker build artifacts; only valid with --docker")

	_ = serveCmd.MarkFlagRequired("file")
}

func runServe(cmd *cobra.Command, args []string) error {
	// Resolve -f: accept both a direct file path and a folder.
	resolvedPath, err := resolveArazzoFilePath(filePath)
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	filePath = resolvedPath

	// -o / --output-dir is only meaningful with --docker.
	if outputDir != "" && !dockerMode {
		return fmt.Errorf("-o / --output-dir can only be used with --docker")
	}

	// --docker mode: package the server into a Docker image and exit.
	if dockerMode {
		if err := docker.BuildImage(docker.BuildConfig{
			ArazzoFilePath: filePath,
			Port:           port,
			OutputDir:      outputDir,
		}); err != nil {
			return fmt.Errorf("Docker packaging failed: %w", err)
		}
		return nil
	}

	// Build runtime params
	if disableTLS {
		log.Println("WARNING: TLS certificate verification is disabled")
	}
	runtimeParams := &models.RuntimeParams{
		BearerToken:            bearerToken,
		APIKey:                 apiKey,
		APIKeyHeader:           apiKeyHeader,
		AuthHeaders:            make(map[string]string),
		DisableTLSVerification: disableTLS,
	}

	// Create trace sink — combine whichever endpoints are configured.
	// VS Code plugin always passes --trace-endpoint (local custom JSON sink).
	// Standalone users can pass --otlp-endpoint to reach Jaeger, Honeycomb, etc.
	// Both flags may be provided simultaneously.
	var sink telemetry.SpanEventSink
	var sinks []telemetry.SpanEventSink
	if traceEndpoint != "" {
		log.Printf("Local tracing enabled → %s", traceEndpoint)
		sinks = append(sinks, telemetry.NewHTTPSink(traceEndpoint))
	}
	if otlpEndpoint != "" {
		log.Printf("OTLP tracing enabled → %s/v1/traces", otlpEndpoint)
		sinks = append(sinks, telemetry.NewOTLPSink(otlpEndpoint))
	}
	switch len(sinks) {
	case 0:
		sink = &telemetry.NoopSink{}
	case 1:
		sink = sinks[0]
	default:
		sink = telemetry.NewMultiSink(sinks...)
	}
	defer sink.Shutdown()

	// Create and start MCP server
	srv, err := mcpserver.NewMCPServer(filePath, port, runtimeParams, sink)
	if err != nil {
		return fmt.Errorf("failed to create MCP server: %w", err)
	}

	log.Printf("Starting Arazzo MCP server for: %s", filePath)
	if err := srv.Start(); err != nil {
		return fmt.Errorf("server error: %w", err)
	}
	return nil
}

// resolveArazzoFilePath accepts either a path to an Arazzo file or a folder.
// When given a folder it scans for exactly one .yaml/.yml file containing the
// top-level "arazzo" key and returns its path. Returns an error if the path
// does not exist, is a folder with zero or multiple Arazzo files, or if a
// direct file path does not exist.
func resolveArazzoFilePath(input string) (string, error) {
	info, err := os.Stat(input)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("path does not exist: %s", input)
		}
		return "", fmt.Errorf("failed to access path: %w", err)
	}

	if !info.IsDir() {
		// Direct file path — use as-is.
		return input, nil
	}

	// Folder — scan for an Arazzo file.
	entries, err := os.ReadDir(input)
	if err != nil {
		return "", fmt.Errorf("failed to read folder %q: %w", input, err)
	}

	var matches []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := strings.ToLower(e.Name())
		if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
			continue
		}
		candidate := filepath.Join(input, e.Name())
		if isArazzoFile(candidate) {
			matches = append(matches, candidate)
		}
	}

	switch len(matches) {
	case 0:
		return "", fmt.Errorf("no Arazzo file found in folder %q\n\nAn Arazzo file must be a .yaml or .yml file containing the top-level 'arazzo' key", input)
	case 1:
		fmt.Printf("Auto-detected Arazzo file: %s\n", matches[0])
		return matches[0], nil
	default:
		return "", fmt.Errorf("multiple Arazzo files found in folder %q:\n  %s\n\nPlease specify the exact file using -f <file>", input, strings.Join(matches, "\n  "))
	}
}

// isArazzoFile returns true if the YAML file contains the top-level "arazzo" key.
func isArazzoFile(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return false
	}
	_, ok := raw["arazzo"]
	return ok
}

// Register adds the serve command to the given parent command.
func Register(root *cobra.Command) {
	root.AddCommand(serveCmd)
}
