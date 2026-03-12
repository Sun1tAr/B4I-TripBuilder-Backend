module github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/services/auth

go 1.22

require (
	github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared v0.0.0-00010101000000-000000000000
	github.com/golang-jwt/jwt/v5 v5.2.1
	github.com/google/uuid v1.6.0
	github.com/gorilla/mux v1.8.1
	github.com/joho/godotenv v1.5.1
	github.com/kelseyhightower/envconfig v1.4.0
	github.com/lib/pq v1.10.9
	golang.org/x/crypto v0.21.0
)

require (
	github.com/fatih/color v1.16.0 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	golang.org/x/sys v0.18.0 // indirect
)

replace github.com/Sun1tAr/B4I-TripBuilder-Backend/trip-builder/shared => ../../shared
