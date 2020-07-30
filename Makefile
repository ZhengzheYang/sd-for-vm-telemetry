.PHONY: format

BINARY=vm-discovery
FINDFILES=find . \( -path ./common-protos -o -path ./.git -o -path ./out -o -path ./.github -o -path ./licenses -o -path ./vendor \) -prune -o -type f
XARGS = xargs -0 -r

clean:
	rm $(BINARY)

build:
	GOOS=linux GOARCH=amd64 go build -o out/$(BINARY)

test:
	go test `go list ./...`

format: fmt ## Auto formats all code. This should be run before sending a PR.
fmt: format-go tidy-go

tidy-go:
	@go mod tidy

format-go: tidy-go
	@${FINDFILES} -name '*.go' \( ! \( -name '*.gen.go' -o -name '*.pb.go' \) \) -print0 | ${XARGS} goimports -w -local "istio.io"