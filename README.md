# Yggdrasil Manager

## Integration Tests

- Build the main entrypoint `go build ./cmd/yggdrasil/`
- Start 4 fully connected nodes `sudo ./misc/run-four-fully-connected-nodes.sh` (and wait for startup to complete)
- Run namespaced node tests `sudo go test ./..
