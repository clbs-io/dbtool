# DB Tool

## Installation

### Binary

```shell
# with `go install`
go install github.com/clbs-io/dbtool/cmd/dbtool@latest
```

### Docker

```shell
docker run -v migrations:/migrations registry.clbs.io/clbs-io/dbtool --app-id=your-app --migrations-dir=/migrations --connection-string=postgres://user:pass@example.com:5432/db
```

## Usage

### Command line tool from your machine

```shell
# or go run ./cmd/dbtool
dbtool \
    --migration-dir ./some/path/to/migrations \
    --connection-string postgres://user:password@localhost:5432/dbname \
    --steps 1 \
    --skip-file-validation
```

### Usage with Docker

```dockerfile
FROM registry.clbs.io/clbs-io/dbtool:v1.0.0 AS dbtool

FROM alpine:3.20 AS runtime

COPY --from=dbtool /usr/local/bin/dbtool /usr/local/bin/dbtool

COPY . .

CMD [ "ash" ]
```

### Usage with Kubernetes


1. Build your own custom image containing the database migrations (SQL files)

    ```dockerfile
    FROM registry.clbs.io/clbs-io/dbtool AS dbtool

    FROM alpine:3.20 AS runtime

    COPY --from=dbtool /usr/local/bin/dbtool /usr/local/bin/dbtool

    COPY . .

    CMD [ "ash" ]
    ```

2. Create a Kubernetes manifest

    ```yaml
    # migrations.yaml
    apiVersion: batch/v1
    kind: Job
    metadata:
      name: migrations
    spec:
      template:
        spec:
          containers:
          - name: dbtool
            image: your/migrations-image
            command: ["dbtool", "--app-id=your-app", "--migrations-dir=./your/migrations/dir", "--connection-string=postgres://user:pass@host:5432/db"]
          restartPolicy: Never
    ```

3. Deploy to your cluster

    ```shell
    kubectl apply -f migrations.yaml
    ```
