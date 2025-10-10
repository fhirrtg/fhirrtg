# FHIR REST to GraphQL Service

       ________  __________     ____  ____________
       / ____/ / / /  _/ __ \   / __ \/_  __/ ____/
      / /_  / /_/ // // /_/ /  / /_/ / / / / / __
     / __/ / __  // // _, _/  / _, _/ / / / /_/ /
    /_/   /_/ /_/___/_/ |_|  /_/ |_| /_/  \____/

The FHIR REST to GraphQL (fhirrtg) service provides a bridge between FHIR REST APIs and GraphQL interfaces. This service allows applications that expect FHIR REST endpoints to communicate with backends that expose FHIR data through GraphQL.

## Features

- Translates FHIR REST API calls to equivalent GraphQL queries
- Maintains FHIR compliance across both interfaces
- Supports standard FHIR search parameters
- Preserves resource integrity during translation

## Installation

```bash
# Clone the repository
git clone https://github.com/username/fhirrtg.git

# Navigate to the project directory
cd fhirrtg

# Install dependencies
go mod download
```

## Usage

```bash
# Build the application
go build -o fhirrtg

# Run the server
./fhirrtg https://your-fhir-graphql-server/graphql
```

By default, the server runs on port 8888 and expects a FHIR GraphQL endpoint to be configured.

## Configuration

Configuration is done via environment variables:

| Variable | Description | Default Value |
|----------|-------------|---------------|
| `RTG_PORT` | Port the server will listen on | `8888` |
| `RTG_LOG_LEVEL` | Logging verbosity (debug, info, warn, error) | `info` |
| `RTG_SKIP_TLS_VERIFY` | Skip upstream certificate verification | `false` |
| `RTG_GRAPHQL_TIMEOUT` | Timeout for GraphQL requests (in seconds) | `30` |
| `RTG_GQL_ACCEPT_HEADER` | HTTP Accept header for upstream server | `application/graphql-response+json;charset=utf-8, application/json;charset=utf-8` |

Example:

```bash

  # Run with custom configuration
  LOG_LEVEL=debug PORT=9000 ./fhirrtg https://your-graphql-endpoint/graphql
```

## API Documentation

The service exposes standard FHIR REST endpoints:

- `GET /[resource]`: Search for resources
- `GET /[resource]/[id]`: Read a specific resource
- `POST /[resource]`: Create a resource
- `PUT /[resource]/[id]`: Update a resource
- `DELETE /[resource]/[id]`: Delete a resource

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

This project is licensed under the MIT License - see the LICENSE file for details.
