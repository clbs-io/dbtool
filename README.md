# dbtool

A PostgreSQL database migration tool for managing and applying database schema changes.

## Features

- Apply database migrations from SQL files
- Support for step-by-step migration execution
- Docker and Kubernetes ready
- Optional file validation
- Built with Go for performance and reliability

## Installation

### Go Install

```shell
go install github.com/clbs-io/dbtool/cmd/dbtool@latest
```

### Docker Image

```shell
docker pull registry.clbs.io/clbs-io/dbtool/dbtool:latest
```

## Usage

### CLI

Run migrations from your local machine:

```shell
dbtool \
    --app-id your-app \
    --migrations-dir ./path/to/migrations \
    --connection-string postgres://user:password@localhost:5432/dbname
```

#### CLI Options

**Required:**

- `--app-id`: Application identifier
- `--migrations-dir`: Path to directory containing migration SQL files
- `--connection-string`: PostgreSQL connection string (or use `--connection-string-file`)

**Optional:**

- `--connection-string-file`: Path to file containing database connection string (alternative to `--connection-string`)
- `--connection-string-format`: Connection string format: `default` or `ado` (default: `default`)
- `--steps`: Number of migration steps to apply (default: `-1` for all migrations)
- `--skip-file-validation`: Skip validation of migration files (default: `false`)
- `--connection-timeout`: Connection timeout in seconds (default: `45`)

**Environment Variables:**

All options can be configured via environment variables:

- `APP_ID`
- `MIGRATIONS_DIR`
- `CONNECTION_STRING`
- `CONNECTION_STRING_FILE`
- `CONNECTION_STRING_FORMAT`
- `STEPS`
- `SKIP_FILE_VALIDATION`
- `CONNECTION_TIMEOUT`

#### Development

Run directly with Go:

```shell
go run ./cmd/dbtool --app-id your-app --migrations-dir ./migrations --connection-string postgres://...
```

### Docker

Run migrations using Docker:

```shell
docker run -v $(pwd)/migrations:/migrations \
    registry.clbs.io/clbs-io/dbtool/dbtool:latest \
    --app-id your-app \
    --migrations-dir /migrations \
    --connection-string postgres://user:pass@host:5432/db
```

### Docker Multi-stage Build

Use dbtool in a multi-stage Docker build to bundle migrations with your application:

```dockerfile
# Import dbtool from registry
FROM registry.clbs.io/clbs-io/dbtool/dbtool AS dbtool

# Your application build stage
FROM golang:1.25-alpine AS builder
WORKDIR /build
COPY . .
RUN go build -o myapp ./cmd/myapp

# Copy migrations and set permissions
COPY ./db/migrations /build/migrations
RUN chmod -R a+r /build/migrations

# Final runtime image
FROM scratch

WORKDIR /app

# Copy dbtool binary
COPY --from=dbtool /usr/local/bin/dbtool /usr/local/bin/dbtool

# Copy your application
COPY --from=builder /build/myapp .
COPY --from=builder /build/migrations /app/migrations

# Run as non-root user
USER 1001

CMD ["/app/myapp"]
```

### Kubernetes

Deploy migrations as a Kubernetes Job:

1. **Build a custom image** containing your migration files:

```dockerfile
FROM registry.clbs.io/clbs-io/dbtool/dbtool AS dbtool

FROM scratch

WORKDIR /app

COPY --from=dbtool /usr/local/bin/dbtool /usr/local/bin/dbtool
COPY ./db/migrations /app/migrations

USER 1001

ENTRYPOINT ["/usr/local/bin/dbtool"]
```

2. **Create the database credentials secret**:

```shell
# Create secret containing the connection string
kubectl create secret generic postgres-credentials \
  --from-literal=db_tool_connection_string='postgres://user:pass@host:5432/db?sslmode=require'
```

Or using a YAML manifest:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: postgres-credentials
type: Opaque
stringData:
  db_tool_connection_string: |
    postgres://user:pass@host:5432/db?sslmode=require
```

3. **Create a Job manifest** (migrations.yaml):

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: database-migrations
spec:
  ttlSecondsAfterFinished: 86400 # Auto-cleanup after 1 day
  template:
    spec:
      automountServiceAccountToken: false
      containers:
        - name: dbtool
          image: your-registry/your-app-migrations:latest
          command: ["/usr/local/bin/dbtool"]
          env:
            - name: APP_ID
              value: "your-app"
            - name: MIGRATIONS_DIR
              value: "/app/migrations"
            - name: CONNECTION_STRING_FILE
              value: "/etc/dbtool/db_tool_connection_string"
            - name: CONNECTION_STRING_FORMAT
              value: "default"
          volumeMounts:
            - name: db-credentials
              mountPath: /etc/dbtool
              readOnly: true
      volumes:
        - name: db-credentials
          secret:
            secretName: postgres-credentials
      restartPolicy: Never
  backoffLimit: 3
```

4. **Deploy to your cluster**:

```shell
kubectl apply -f migrations.yaml
```

5. **Check migration status**:

```shell
kubectl logs job/database-migrations
kubectl get job database-migrations
```

## Configuration

### Connection String Format

PostgreSQL connection string format:

```
postgres://username:password@hostname:port/database?sslmode=require
```

### Migration Files

Migration files should be SQL files stored in a directory structure. The tool will process them in order.

## About

This project is part of the [clbs.io](https://clbs.io) initiative - a public-source-code brand by [cybros labs](https://www.cybroslabs.com).

## License

This project is licensed under the Mozilla Public License 2.0 - see the [LICENSE](LICENSE) file for details.
