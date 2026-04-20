module github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/notification

go 1.24.0

require (
	github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared v0.0.0-00010101000000-000000000000
	github.com/google/uuid v1.6.0
	github.com/gorilla/mux v1.8.1
	github.com/joho/godotenv v1.5.1
	github.com/kelseyhightower/envconfig v1.4.0
	github.com/lib/pq v1.10.9
	github.com/redis/go-redis/v9 v9.18.0
	github.com/segmentio/kafka-go v0.4.47
)

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/fatih/color v1.16.0 // indirect
	github.com/golang-migrate/migrate/v4 v4.19.1 // indirect
	github.com/klauspost/compress v1.15.11 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/pierrec/lz4/v4 v4.1.16 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	golang.org/x/sys v0.38.0 // indirect
)

replace github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared => ../../shared
