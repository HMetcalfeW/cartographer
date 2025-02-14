![cart-gopher logo](assets/CartographerMain.png)

# Cartographer

Cartographer is a lightweight CLI tool written in Go that analyzes and visualizes relationships between Kubernetes resources in a DOT file. It ingests Kubernetes manifests—either from YAML files or via the Helm SDK—and produces dependency graphs to help you understand and document your application's architecture. The DOT file output can be run through a visualizer like GraphViz.

## Features

- **Kubernetes Manifest Ingestion**  
  - Parse multi-document YAML files and convert them into structured Kubernetes objects for analysis.
  - Support for ephemeral containers, environment variable references, volume references (Secrets, ConfigMaps, PVCs), and more.

- **Helm Chart Support**  
  - Render and analyze Kubernetes manifests from Helm charts via the Helm SDK.
  - Specify the chart path similarly to Helm CLI usage (e.g., `--chart`, `--release`, `--values`).

- **Dependency Analysis with Labeled Edges**  
  - Detect references such as:
    - **Owner References** (e.g., Deployment owned by a HelmRelease).
    - **Pod Spec References** (Secrets, ConfigMaps, PVCs, ServiceAccounts, imagePullSecrets).
    - **Label Selectors** (Service → Pod, NetworkPolicy → Pod, PodDisruptionBudget → Pod).
    - **Ingress** routes (Ingress → Service → TLS Secret).
    - **HPA** scale targets (HPA → Deployment).
  - Each edge is annotated with a **reason** (e.g., `ownerRef`, `secretRef`, `selector`) to clarify how resources are connected.

- **Graph Generation**  
  - Output graphs in [DOT format](https://graphviz.org/) for visualization with Graphviz or other tools.
  - Edges are automatically labeled with the reference reason, making the graph easy to interpret.

- **Cobra & Viper CLI**  
  - Built with [Cobra](https://github.com/spf13/cobra) for intuitive subcommands and flags.
  - Uses [Viper](https://github.com/spf13/viper) for flexible configuration (e.g., reading from config files or environment variables).

- **Containerized Deployment**  
  - Dockerfile and docker-compose configuration for building and running Cartographer in a containerized environment.
  - Make targets for multi-platform builds (e.g., Linux, Mac ARM).

## Installation

### Prerequisites

- **Go 1.23+** (modules enabled)
- **Helm** (if using Helm chart ingestion)
- **Docker** (for containerized builds)
- **Docker Compose** (optional, for multi-container setups)
- **Graphviz** (to visualize DOT output, installed via `brew install graphviz` on macOS or your preferred package manager)

### Clone the Repository

```bash
git clone https://github.com/HMetcalfeW/cartographer.git
cd cartographer
```

### Install Dependencies
Cartographer uses Go modules. From the repository root:

```bash
make deps
```
This fetches all necessary dependencies, including:

Cobra & Viper for CLI and config
Helm SDK for chart rendering
Kubernetes API packages for unstructured manifest parsing

### Update Dependencies
Cartographer uses Go modules. From the repository root:

```bash
make update-deps
```
This fetches updates all dependencies to the latest version

### Usage
Cartographer offers a flexible CLI with an analyze subcommand using the Helm SDK to render the chart and then process the resulting YAML for dependencies. Here are a few examples:

#### Key Flags

- `--input`: Path to a Kubernetes YAML file.
- `--chart`: Local path or remote chart name (bitnami/postgresql).
- `--values`: Optional path to a Helm values file.
- `--release`: Name for the Helm release (defaults to `cartographer-release`).
- `--output-format=dot`: Generate DOT output to stdout or to a file with `--output-file`.
- `--output-file`: Location to store the output DOT file
- `--config`: (Optional) Path to a configuration file for advanced settings.

#### 1. Analyze Kubernetes Manifests from YAML

```bash
cartographer analyze --input /path/to/manifest.yaml --output-format dot --output-file test.dot
```
Cartographer reads the YAML, parses each document into Kubernetes unstructured objects. 

#### 2. Analyze a Locally Downloaded Helm Chart

```bash
cartographer analyze --chart /path/to/chart --release my-release --values values.yaml --output-format dot --output-file test.dot
```
#### 3. Analyze a Helm Chart from a Local Helm Registry
Note: the registry will need to be added to your local Helm index.

```bash
helm repo add bitnami https://charts.bitnami.com/bitnami
cartographer analyze --chart bitnami/postgresql --release my-release --values values.yaml --version 16.4.8 --output-format dot --output-file test.dot
```

#### 4. Analyze a Remote Helm Chart from an OCI Registry

```bash
cartographer analyze --chart oci://registry-1.docker.io/bitnamicharts/postgresql --release my-db --version 16.4.8 --output-format dot --output-file test.dot
```

### Run Cartographer and Visualize the DOT File

1. Ensure you have GraphViz installed
```bash
brew install graphviz
```

2. Run cartographer
```bash
cartographer analyze --chart oci://registry-1.docker.io/bitnamicharts/postgresql --release my-db --version 16.4.8 --output-format dot --output-file bitnami-postgresql.dot
```

3. Run GraphViz using the 
```bash
dot -Tpng bitnami-postgresql.dot -o bitnami-postgresql.png
```

## Configuration
The default location of cartographer's configuration file if the `--config` flag is undefined is `$HOME/.cartographer.yaml`

Right now the configuration supports changing the log level of the application. For example:

```yaml
log:
  level: "debug"
```

## Repo Maintenance

### Lint

```bash
make lint
```
This runs golangci-lint with your configuration, ensuring consistent code style.

### Unit Testing

```bash
make test
```
A coverage report is generated upon completion, with coverage typically above 80% due to thorough unit tests.

### Building
To build Cartographer as a CLI executable:
```bash
make build
```
The binary is placed in the build/ directory.

### Docker & Docker Compose
Cartographer can be containerized for easy deployment or CI/CD usage.

#### Docker
```bash
make docker
```
Builds a Docker image (e.g., cartographer:latest). You can run it like so:
```bash
docker run --rm cartographer:latest analyze --input /manifests/test.yaml
```

#### Docker Compose
```bash
docker compose up
```
Runs Cartographer in a container, optionally alongside other services you define. An example docker-compose.yaml has been provided.

## Versioning
Cartographer uses a `VERSION` file in the root directory. The Makefile reads this to inject version metadata at build time (e.g., `-ldflags -X main.Version=$(VERSION)`).

## Contributing
Contributions are welcome! If you find a bug or have an improvement, feel free to:

1. Open an issue describing your idea or problem.
2. Submit a pull request with your changes and relevant tests.

## License
This project is licensed under the Apache 2.0 License. See the LICENSE file for full details.