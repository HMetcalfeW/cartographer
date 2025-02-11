![cobra logo](assets/CartographerMain.png)

# Cartographer
Cartographer is a lightweight CLI tool written in Go that maps and visualizes relationships between Kubernetes resources. It ingests Kubernetes manifests—either directly from YAML files or via Helm charts using the Helm SDK—and generates dependency graphs to help you better understand your application's architecture.

## Features

- **Kubernetes Manifest Ingestion**  
  Parse multi-document YAML files and convert them into structured Kubernetes objects for analysis.
- **Helm Chart Support**  
  Render and analyze Kubernetes manifests from Helm charts by specifying the chart path and repository via command-line flags, similar to the Helm CLI.
- **Dependency Analysis**  
  Analyze resource relationships (e.g., linking Services to Deployments, referencing ConfigMaps/Secrets) to build a dependency graph.
- **Graph Generation**  
  Output graphs (e.g., DOT files) for visualization with Graphviz.
- **Modern CLI with Cobra & Viper**  
  Built using Cobra for command management and Viper for configuration, ensuring a flexible, user-friendly interface.
- **Containerized Deployment**  
  Includes a Dockerfile and docker-compose configuration for building and running the application in a containerized environment.

## Installation

### Prerequisites

- Go 1.23+ (ensure modules are enabled)
- Helm (if using Helm chart ingestion)
- Docker (for containerized builds)
- Docker Compose (optional, for multi-container setups)

### Clone the Repository

```bash
git clone https://github.com/HMetcalfeW/cartographer.git
cd cartographer
```

### Install Dependencies
Cartographer uses Go modules. From the repository root, run:

```bash
go mod tidy
```

This command downloads all necessary dependencies, including:
- Cobra
- Viper
- Helm SDK (for chart rendering)
- Kubernetes API packages for manifest conversion


## Usage
Cartographer provides a flexible CLI with several subcommands. Examples include:

### Analyze Kubernetes Manifests from YAML
```bash
cartographer analyze --input /path/to/manifest.yaml
```

### Analyze a Helm Chart
Render a Helm chart by specifying its path and (optionally) the repository URL:

```bash
cartographer analyze --chart /path/to/chart --repo https://charts.example.com
```

Common Flags:
- `--input`: Path to a Kubernetes YAML file.
- `--chart`: Path to a Helm chart directory.
- `--repo`: (Optional) Helm chart repository URL.
- `--config`: Custom configuration file (default is $HOME/.cartographer.yaml).

When a Helm chart is specified, Cartographer uses the Helm SDK to render the chart into Kubernetes manifests, then processes them as usual.

## Building
To build the Cartographer executable, run:

```bash
go build -o cartographer .
```

Make sure you have a proper main.go in the project root (in package main) that calls your CLI command execution logic (for example, by calling cmd/cartographer.Execute()).

## Docker & Docker Compose
Cartographer can be containerized for easy deployment.

### Build the Docker Image

```bash
docker build -t cartographer:<your_tag> .
```

### Run with Docker Compose

```bash
docker-compose up
```

## Testing

Unit tests are provided for all functions. Run tests with:

```bash
go test ./...
```

## Versioning

Cartographer uses a dedicated VERSION file to manage its version. The version from this file is injected into the binary at build time via build arguments in the Dockerfile. Update the VERSION file to reflect new releases.

## Contributing

Contributions are welcome! Please open an issue or submit a pull request with your improvements or bug fixes.

## License

This project is licensed under the Apache 2.0 License. See the LICENSE file for details.