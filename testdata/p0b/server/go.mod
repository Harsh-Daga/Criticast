module github.com/criticast/criticast/testdata/p0b/server

go 1.22

require (
	github.com/criticast/criticast v0.0.0
	go.opentelemetry.io/otel v1.28.0
	go.opentelemetry.io/otel/exporters/stdout/stdouttrace v1.28.0
	go.opentelemetry.io/otel/sdk v1.28.0
	go.opentelemetry.io/otel/trace v1.28.0
)

require (
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/google/uuid v1.6.0 // indirect
	go.opentelemetry.io/otel/metric v1.28.0 // indirect
	golang.org/x/sys v0.28.0 // indirect
)

replace github.com/criticast/criticast => ../../..
