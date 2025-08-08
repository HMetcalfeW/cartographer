![cart-gopher logo](assets/CartographerMain.png)

# Cartographer

Cartographer is a lightweight CLI tool written in Go that analyzes and visualizes relationships between Kubernetes resources in a DOT file. It ingests Kubernetes manifests—either from YAML files or via the Helm SDK—and produces dependency graphs to help you understand and document your application's architecture. The DOT file output can be run through a visualizer like GraphViz.

## Features

- **Kubernetes Manifest Ingestion**  
  - Parse multi-document YAML files and convert them into structured Kubernetes objects for analysis.
  - Support for ephemeral containers, environment variable references, volume references (Secrets, ConfigMaps, PVCs), and more.

- **Helm Chart Support**  
  - Render and analyze Kubernetes manifests from Helm charts via the Helm SDK.
  - Specify the chart path similarly to Helm CLI usage (e.g., `--chart`, `--release`, `--values`, `--version`).

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
    Example DOT snippet: `A -> B [label="uses"];`
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

### Getting Started

To quickly get Cartographer up and running and visualize your first Kubernetes manifest:

1.  **Build the executable:**
    ```bash
    make build
    ```
    This will create the `cartographer` binary in the `build/` directory.

2.  **Analyze a sample manifest:**
    Let's assume you have a simple `deployment.yaml` file:
    ```yaml
    apiVersion: apps/v1
    kind: Deployment
    metadata:
      name: my-nginx-deployment
      labels:
        app: nginx
    spec:
      replicas: 1
      selector:
        matchLabels:
          app: nginx
      template:
        metadata:
          labels:
            app: nginx
        spec:
          containers:
          - name: nginx
            image: nginx:latest
    ```
    Run Cartographer to analyze it and generate a DOT file:
    ```bash
    ./build/darwin_arm64/cartographer analyze --input deployment.yaml --output-format dot --output-file deployment.dot
    ```
    (Adjust the path to the binary based on your OS, e.g., `./build/linux/cartographer` for Linux.)

3.  **Visualize the graph:**
    Use Graphviz to convert the DOT file into a PNG image:
    ```bash
    dot -Tpng deployment.dot -o deployment.png
    ```
    This will create `deployment.png` showing the dependency graph.

This basic workflow demonstrates how to use Cartographer to gain insights into your Kubernetes configurations.

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
- `--chart`: Local path or remote chart name (e.g., `myrepo/mychart`).
- `--values`: Optional path to a Helm values file.
- `--release`: Name for the Helm release (defaults to `cartographer-release`).
- `--version`: The Helm Chart version you wish to use.
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
helm repo add myrepo https://charts.example.com/myrepo
cartographer analyze --chart myrepo/mychart --release my-release --values values.yaml --version 1.0.0 --output-format dot --output-file test.dot
```

#### 4. Analyze a Remote Helm Chart from an OCI Registry

```bash
cartographer analyze --chart oci://registry-1.docker.io/mycharts/mychart --release my-app --version 1.0.0 --output-format dot --output-file test.dot
```

#### 5. Generate Mermaid Diagram

```bash
cartographer analyze --input deployment.yaml --output-format mermaid --output-file deployment.mmd
```

This will create `deployment.mmd` containing the Mermaid syntax for your dependency graph, which can be rendered in many Markdown viewers (e.g., GitHub, GitLab).

#### 6. Generate JSON Output

```bash
cartographer analyze --input deployment.yaml --output-format json --output-file deployment.json
```

This will create `deployment.json` containing a structured JSON representation of your dependency graph, useful for programmatic consumption.

#### 7. Analyze a Live Kubernetes Cluster (Requires `kubectl` and Kubeconfig)

Cartographer can also analyze resources directly from a live Kubernetes cluster. This requires `kubectl` to be configured to access your cluster.

```bash
# Example: Analyze all Deployments in the 'default' namespace
kubectl get deployments -n default -o yaml | cartographer analyze --input - --output-format dot --output-file live-deployments.dot
```

This command pipes the YAML output of `kubectl get deployments` directly into Cartographer's `--input -` (standard input) flag. This is a powerful way to visualize the current state of your cluster.

#### 8. Analyze a Specific Resource Type from a Live Cluster

```bash
# Example: Analyze a specific Deployment and its related resources
kubectl get deployment my-app -o yaml | cartographer analyze --input - --output-format dot --output-file my-app-deployment.dot
```

### Important Note on Bitnami Catalog Changes

As of August 28, 2025, Bitnami is making significant changes to its public container catalog. This may impact how Cartographer interacts with Bitnami Helm charts and images.

-   Most existing images will be moved to a `bitnamilegacy` repository and will no longer receive updates.
-   A limited set of hardened images will remain free for development under the "latest" tag.
-   For production use, Bitnami Secure Images will be a paid offering.

What this means for Cartographer users:

-   If you are using Bitnami charts, you may need to update your chart references to point to the new `bitnamilegacy` repository or consider using the paid Bitnami Secure Images for continued support and updates.
-   Alternatively, explore non-Bitnami chart sources or maintain your own chart repositories.

Please refer to the official Bitnami announcement for detailed information:
https://github.com/bitnami/containers/issues/83267



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

### Example: Metallb Output

```bash
cartographer analyze --chart oci://registry-1.docker.io/bitnamicharts/metallb --output-file bitnami-metallb.dot 
```

```bash
dot -Tpng bitnami-metallb.dot -o bitnami-metallb.png
```

#### DOT File Visualized with GraphViz
![cart-gopher logo](assets/bitnami-metallb.png)

## Configuration
The default location of cartographer's configuration file if the `--config` flag is undefined is `$HOME/.cartographer.yaml`

Right now the configuration supports changing the log level of the application. For example:

```yaml
log:
  level: "debug"
```

## Troubleshooting

This section addresses common issues you might encounter while using Cartographer.

-   **Error: File or Chart Not Found**
    If Cartographer reports that a file or Helm chart cannot be found, double-check the provided path or chart reference. Ensure the file exists at the specified location or that the Helm repository is correctly added and updated.

-   **Error: `golangci-lint` version mismatch or internal error**
    This project uses `golangci-lint` for code quality checks. If you encounter errors related to version mismatches or internal panics during linting, it might be due to environmental factors or specific Go toolchain versions. While the core functionality of Cartographer remains unaffected, you might need to:
    -   Ensure your Go environment is correctly set up.
    -   Try installing a specific version of `golangci-lint` that is known to be compatible with your Go version.
    -   Consult the `golangci-lint` documentation for advanced troubleshooting.

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

### Further Testing Needs

While core functionalities are tested, more comprehensive unit tests are needed for:

-   **Refactored Dependency Analyzers:** Comprehensive unit tests are needed for each `Analyzer` implementation (e.g., `OwnerRefAnalyzer`, `LabelSelectorAnalyzer`, `IngressAnalyzer`, `HPAAnalyzer`, `PodSpecAnalyzer`) to ensure their isolated logic is thoroughly validated.
-   **Enhanced Error Handling:** Specific test cases to trigger and verify the new, more descriptive error messages for conflicting flags and malformed YAML inputs.

These tests will improve the project's robustness and prevent regressions in these critical areas.

## License
This project is licensed under the Apache 2.0 License. See the LICENSE file for full details.