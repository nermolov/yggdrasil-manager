# Yggdrasil Manager

## Integration Tests

- Build the main entrypoint `go build ./cmd/yggdrasil/`
- Configure 4 fully connected network namespaces `sudo ./scripts/configure-four-connected-node-namespaces.sh`
- Run namespaced node tests `sudo go test -count=1 -v ./...`

## Extending yggdrasil-go

At the moment this package inlines pieces of https://pkg.go.dev/github.com/yggdrasil-network/yggdrasil-go when necessary. If the amount of private code that needs to be updated grows, I might switch to using a submodule to make syncing changes from upstream easier.
