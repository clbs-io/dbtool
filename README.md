# dbtool

## Usage

```shell
# or go run ./cmd/dbtool
dbtool \
    --migration-dir ./some/path/to/migrations \
    --database-url postgres://user:password@localhost:5432/dbname \
    --steps 1 \
    --skip-file-validation
```
