module github.com/jakewan/finch/mcp

go 1.24.0

toolchain go1.24.1

replace github.com/jakewan/finch/daemon => ../daemon

replace github.com/jakewan/finch/core => ../core

require (
	github.com/jakewan/finch/daemon v0.0.0-00010101000000-000000000000
	github.com/modelcontextprotocol/go-sdk v1.4.0
	google.golang.org/grpc v1.79.2
)

require (
	github.com/google/jsonschema-go v0.4.2 // indirect
	github.com/segmentio/asm v1.1.3 // indirect
	github.com/segmentio/encoding v0.5.3 // indirect
	github.com/yosida95/uritemplate/v3 v3.0.2 // indirect
	golang.org/x/net v0.48.0 // indirect
	golang.org/x/oauth2 v0.34.0 // indirect
	golang.org/x/sys v0.40.0 // indirect
	golang.org/x/text v0.32.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20251202230838-ff82c1b0f217 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
)
