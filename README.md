# azctl

`azctl` is a CLI tool that turns an [Arazzo specification](https://spec.openapis.org/arazzo/latest.html) and its referenced OpenAPI files into a high-performance Go-based MCP (Model Context Protocol) server. Each Arazzo workflow becomes an MCP tool that any AI agent can call.

---

## Table of Contents

1. [What It Does](#what-it-does)
2. [Prerequisites](#prerequisites)
3. [Installation](#installation)
4. [Commands](#commands)
   - [validate](#validate)
   - [inspect](#inspect)
   - [visualize](#visualize)
   - [serve](#serve)
5. [User Scenario: End-to-End Walkthrough](#user-scenario-end-to-end-walkthrough)
6. [Dockerization](#dockerization)
7. [Sample Arazzo File](#sample-arazzo-file)
8. [License](#license)

---

## What It Does

Given a folder containing:
- one Arazzo `.yaml` file (describes multi-step API workflows)
- referenced OpenAPI `.yaml` files (describe individual API operations)

…the CLI will:

| Step | What happens |
|------|-------------|
| Validate | Checks the Arazzo file for correctness (requires Spectral or uses built-in checks) |
| Inspect | Shows a human-readable summary of workflows and steps |
| Visualize | Renders a Mermaid flowchart of the workflow logic |
| Serve | Hosts the spec as an MCP server with a high-performance Go execution engine |
| direct-run | Execute workflows directly via HTTP REST endpoints (`/run`, `/lastResult`) |

---

## Prerequisites

| Tool | Why | Install |
|------|-----|---------|
| **Go 1.21+** | Required only if building from source | [go.dev](https://go.dev) |
| **Docker** | Required only if running in a containerized environment | [docs.docker.com/get-docker](https://docs.docker.com/get-docker/) |
| **Node.js + npx** *(optional)* | Enables the Spectral validator for in-depth Arazzo checks | [nodejs.org](https://nodejs.org) |

---

## Installation

Download the latest version for your operating system from the [Releases](https://github.com/wso2/arazzo-mcp-generator/releases) page, or use the quick install commands below.

### macOS / Linux

```bash
# For macOS (Apple Silicon)
curl -L https://github.com/wso2/arazzo-mcp-generator/releases/latest/download/azctl_Darwin_arm64.tar.gz -o azctl.tar.gz
tar -xzf azctl.tar.gz
sudo mv azctl-darwin-arm64 /usr/local/bin/azctl
rm azctl.tar.gz

# For macOS (Intel)
curl -L https://github.com/wso2/arazzo-mcp-generator/releases/latest/download/azctl_Darwin_x86_64.tar.gz -o azctl.tar.gz
tar -xzf azctl.tar.gz
sudo mv azctl-darwin-amd64 /usr/local/bin/azctl
rm azctl.tar.gz

# For Linux (x86_64)
curl -L https://github.com/wso2/arazzo-mcp-generator/releases/latest/download/azctl_Linux_x86_64.tar.gz -o azctl.tar.gz
tar -xzf azctl.tar.gz
sudo mv azctl-linux-amd64 /usr/local/bin/azctl
rm azctl.tar.gz

# For Linux (ARM64)
curl -L https://github.com/wso2/arazzo-mcp-generator/releases/latest/download/azctl_Linux_arm64.tar.gz -o azctl.tar.gz
tar -xzf azctl.tar.gz
sudo mv azctl-linux-arm64 /usr/local/bin/azctl
rm azctl.tar.gz
```

### Windows

```powershell
# Download and extract (PowerShell)
Invoke-WebRequest -Uri https://github.com/wso2/arazzo-mcp-generator/releases/latest/download/azctl_Windows_x86_64.zip -OutFile azctl.zip
Expand-Archive -Path azctl.zip -DestinationPath .
# Move azctl.exe to a directory in your PATH, or run it directly
```

Verify the installation:

```bash
azctl --version
```

---

<!-- ## Quick Start

If you don't have an Arazzo spec yet, let the CLI create a sample one:

```bash
azctl sample my-project
cd my-project
```

Then validate, inspect, and serve in three commands:

```bash
azctl validate -f .
azctl inspect  -f .
azctl serve -f . -p 8080
```

The server is then live at `http://localhost:8080/mcp`.

--- -->

## Commands

### `validate`

Validates an Arazzo specification for correctness and completeness.

Uses **Spectral** (via `npx @stoplight/spectral-cli`) with the official `spectral:arazzo` ruleset as the primary validator when available. Falls back to the built-in Go validator when Node.js is not installed, showing install instructions.

```bash
azctl validate -f <file-or-folder>
```

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--file` | `-f` | Path to an Arazzo file or folder (auto-detects Arazzo file if a folder is given) | — |
| `--check-remote` | | Also probe remote source URLs for accessibility | `false` |
| `--strict` | | Treat warnings as errors (exits with code 1 on warnings) | `false` |

**Examples**

```bash
# Validate a folder (auto-detects the Arazzo file)
azctl validate -f ./my-arazzo-folder

# Validate a single file
azctl validate -f ./workflow.yaml

# Validate and also check that remote OpenAPI URLs are reachable
azctl validate -f ./my-arazzo-folder --check-remote

# Strict mode: fail if there are any warnings
azctl validate -f ./my-arazzo-folder --strict
```

**What it checks (Spectral ruleset)**
- Full JSON Schema validation against the Arazzo 1.0.x spec
- Unique `workflowId` and `stepId` values
- Step targets (`operationId`, `operationPath`, `workflowId`) are present and valid
- Parameter `name`, `in`, and `value` fields
- Success criteria condition syntax
- Unique `onSuccess` / `onFailure` action names
- Output expression syntax
- `dependsOn` cross-references

**Additional built-in checks (always run)**
- Local source file existence
- Remote URL accessibility (only with `--check-remote`)
- Multiple `$statusCode` criteria that are AND-ed together (a common mistake)

**Exit codes**

| Code | Meaning |
|------|---------|
| `0` | Passed (no errors) |
| `1` | Errors found, or warnings in `--strict` mode |

---

### `inspect`

Parses and prints a detailed, colour-coded overview of an Arazzo spec — without generating anything. Use this to understand a spec or debug step-flow routing before generating an MCP server.

```bash
azctl inspect -f <file-or-folder>
```

| Flag | Short | Description |
|------|-------|-------------|
| `--file` | `-f` | Path to an Arazzo file or folder (auto-detects Arazzo file if a folder is given) |

**Examples**

```bash
# Inspect a folder (auto-detects the Arazzo file)
azctl inspect -f ./my-arazzo-folder

# Inspect a specific file
azctl inspect -f ./workflow.yaml
```

**Output includes**
- Spec metadata: title, version, Arazzo version
- All source descriptions with types and URLs
- For each workflow:
  - Input schema with types
  - Each step: operation target, parameter bindings, success criteria
  - `onSuccess` / `onFailure` routing with conditions (GOTO, END, RETRY)
  - Step outputs and their expressions
  - Workflow-level outputs

---

### `visualize`

Generates a Mermaid flowchart diagram of the Arazzo spec's workflow logic. By default opens the rendered diagram in your browser (no extra tools needed). Can also save to a file.

```bash
azctl visualize -f <file-or-folder> [-o <output-file>]
```

Alias: `viz`

| Flag | Short | Description |
|------|-------|-------------|
| `--file` | `-f` | Path to an Arazzo file or folder (auto-detects Arazzo file if a folder is given) |
| `--output` | `-o` | Output file path. `.md` → Mermaid in fenced code block; `.mmd` → raw Mermaid syntax |

**Examples**

```bash
# Open diagram in browser (default)
azctl visualize -f ./my-arazzo-folder

# Save to GitHub-renderable Markdown
azctl visualize -f ./workflow.yaml -o diagram.md

# Save raw Mermaid source
azctl visualize -f ./my-arazzo-folder -o flow.mmd

# Short alias
azctl viz -f ./my-arazzo-folder
```

**Diagram shows**
- Start and end nodes for each workflow
- Steps with operation targets
- `onSuccess` / `onFailure` branches labelled with conditions
- Implicit sequential flow and fallthrough paths (dashed arrows)
- Cross-workflow `goto` references

> Paste any `.mmd` file into [mermaid.live](https://mermaid.live) for a shareable interactive link.

---

### `serve`

Start the MCP server to host your Arazzo workflows.

```bash
azctl serve -f <file-or-folder> [flags]
```

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--file` | `-f` | Path to an Arazzo file or folder | — |
| `--port` | `-p` | Port the MCP server listens on | `8080` |
| `--docker` | | Package the server into a Docker image instead of starting it locally | `false` |
| `--output-dir` | `-o` | Keep Docker build artifacts in this folder (only with `--docker`) | — |

**Examples**

```bash
# Host from a folder
azctl serve -f ./my-arazzo-folder

# Host from a specific file with custom port
azctl serve -f ./workflow.yaml -p 9000
```

**Endpoints**
- `http://localhost:8080/mcp`: The MCP protocol endpoint (Stateful/Streamable HTTP)
- `http://localhost:8080/run/{workflowId}`: Direct REST endpoint to execute a workflow
- `http://localhost:8080/lastResult/{workflowId}`: Retrieve the last execution result for a workflow

---

## User Scenario: End-to-End Walkthrough

> **Scenario:** You have an OpenAPI spec for a pet store API and want to expose a "check if a pet exists, then create or update it" workflow as an MCP tool for an AI agent.

### Step 1 — Prepare your project folder

Create a folder containing your Arazzo specification and its referenced OpenAPI files:

1. Create a folder named `pet-project`.
2. Save your Arazzo file (e.g., `petstore_workflow.yaml`) inside it.
3. Ensure all OpenAPI `.yaml` files referenced in the Arazzo spec are also in this folder.

```text
pet-project/
├── petstore_workflow.yaml   ← Your Arazzo spec
└── petstore_openapi.yaml    ← Your OpenAPI spec
```

### Step 2 — Validate the spec

```bash
azctl validate -f .
```

**Expected output (Spectral available):**
```text
Validating: /path/to/pet-project/petstore_workflow.yaml
────────────────────────────────────────────────────────────
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Validation Result: PASSED
  ✓ All arazzo rules passed
  ─ Validated using Spectral (spectral:arazzo ruleset)
  💡 Tip: Run 'azctl serve -f .' to host this Arazzo spec as an MCP server.
```

Fix any errors reported before continuing. Warnings are informational; use `--strict` to treat them as errors in CI.

### Step 3 — Inspect the spec

```bash
azctl inspect -f .
```

Review the printed summary to confirm:
- The correct source descriptions (your OpenAPI file/URL)
- Every step has an `operationId` that matches your OpenAPI spec
- Input schema, success criteria, and routing look correct

### Step 4 — Visualize the flow

```bash
azctl visualize -f .
```

Your browser opens with an interactive Mermaid flowchart. Check the branching logic visually.

To save it:

```bash
# As a Markdown file (renders on GitHub)
azctl visualize -f . -o flow.md
```

### Step 5 — Start the MCP server

```bash
azctl serve -f . -p 8080
```

**Expected output:**
```text
MCP server listening on http://localhost:8080/mcp
Run endpoint available at http://localhost:8080/run/{workflowId}
```

### Step 6 — Connect an MCP client

The server is now live at `http://localhost:8080/mcp`. To connect it to an MCP client like **Claude Desktop**, you can use `supergateway` to bridge the HTTP endpoint:

```json
{
  "mcpServers": {
    "my-mcp-server": {
      "command": "npx",
      "args": [
        "-y",
        "supergateway",
        "--streamableHttp",
        "http://localhost:8080/mcp"
      ]
    }
  }
}
```

---

## Dockerization

Use the `--docker` flag to cross-compile the server and package it into a self-contained Docker image.

> **Note:** `--docker` requires a working Go toolchain and the `go.mod` file to be reachable from the current directory.
> It is designed for development workflows, not for downloaded release binaries.

```bash
# Build a Docker image from an Arazzo file
azctl serve --docker -f workflow.yaml

# Optionally keep the build artifacts for inspection
azctl serve --docker -f workflow.yaml -o ./docker-output
```

Once the image is built, the CLI prints the exact `docker run` command to use:

```bash
docker run --rm -p 8080:8080 <image-name>
```

---

## Sample Arazzo File

To get started, create a folder for your project and save the following as `petstore_workflow.yaml` inside it. This is a ready-to-use Arazzo spec targeting the public [Petstore v3 API](https://petstore3.swagger.io) — it checks whether a pet exists by ID, updates its name if found, or creates it if not.

```yaml
arazzo: "1.0.0"
info:
  title: Pet Upsert Workflow (V3)
  summary: A sample workflow that conditionally creates or updates a pet using Petstore V3
  description: Workflow targeting Petstore V3 API. Takes an id and name - renames the pet if it exists, creates it if not.
  version: 1.0.0

sourceDescriptions:
  - name: petstoreApiV3
    url: https://petstore3.swagger.io/api/v3/openapi.json
    type: openapi

workflows:
  - workflowId: ensurePetExistsV3
    summary: Check if a pet exists by ID; update its name if found, create it if not.
    description: This workflow demonstrates conditional logic based on API responses. It first checks if a pet with the given ID exists. If it does, it updates the pet's name. If it doesn't, it creates a new pet with the provided ID and name.
    inputs:
      type: object
      properties:
        petId: { type: integer }
        newName: { type: string }

    steps:
      - stepId: checkStep
        description: Check if the pet exists and route accordingly.
        operationId: getPetById
        parameters:
          - name: petId
            in: path
            value: $inputs.petId

        successCriteria:
          - condition: $statusCode == 200

        # Branch based on which status code was returned
        onSuccess:
          - name: petFoundRouteToUpdate
            criteria:
              - condition: $statusCode == 200
            type: goto
            stepId: updateStep

        # Retry on true server errors
        onFailure:
          - name: retryOnServerError
            criteria:
              - condition: $statusCode >= 500
            type: retry
            retryAfter: 5

      - stepId: createStep
        description: Pet not found - create it with the given id and name.
        operationId: addPet
        requestBody:
          contentType: application/json
          payload:
            id: $inputs.petId
            name: $inputs.newName
            category:
              id: 1
              name: Dogs
            photoUrls:
              - "https://example.com/pet.jpg"
            tags:
              - id: 0
                name: string
            status: "available"
        onSuccess:
          - name: endAfterCreation
            type: end

      - stepId: updateStep
        description: Pet found - rename it using a full PUT update.
        operationId: updatePet
        requestBody:
          contentType: application/json
          payload:
            id: $inputs.petId
            name: $inputs.newName
            category:
              id: 1
              name: Dogs
            photoUrls:
              - "https://example.com/pet.jpg"
            tags:
              - id: 0
                name: string
            status: "available"
        onSuccess:
          - name: endAfterUpdate
            type: end
```

Once you have this file saved, follow the [End-to-End Walkthrough](#user-scenario-end-to-end-walkthrough) to validate, inspect, and generate an MCP server from it.

---

## License

Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
