module github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/search

go 1.24.0

require (
	github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared v0.0.0-00010101000000-000000000000
	github.com/elastic/go-elasticsearch/v8 v8.15.0
	github.com/gorilla/mux v1.8.1
	github.com/joho/godotenv v1.5.1
	github.com/kelseyhightower/envconfig v1.4.0
	github.com/segmentio/kafka-go v0.4.47
)

require (
	github.com/elastic/elastic-transport-go/v8 v8.6.0 // indirect
	github.com/fatih/color v1.16.0 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/klauspost/compress v1.15.11 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/pierrec/lz4/v4 v4.1.16 // indirect
	go.opentelemetry.io/otel v1.28.0 // indirect
	go.opentelemetry.io/otel/metric v1.28.0 // indirect
	go.opentelemetry.io/otel/trace v1.28.0 // indirect
	golang.org/x/sys v0.38.0 // indirect
)

replace github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared => ../../shared
