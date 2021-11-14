module github.com/Decentr-net/vulcan

go 1.16

require (
	github.com/Decentr-net/decentr v1.5.0
	github.com/Decentr-net/go-api v0.1.0
	github.com/Decentr-net/go-broadcaster v0.1.0
	github.com/Decentr-net/logrus v0.7.2-0.20210316223658-7a9b48625189
	github.com/avast/retry-go v3.0.0+incompatible
	github.com/cosmos/cosmos-sdk v0.44.3
	github.com/ethereum/go-ethereum v1.10.8
	github.com/getsentry/sentry-go v0.10.0 // indirect
	github.com/go-chi/chi v4.1.2+incompatible
	github.com/go-chi/cors v1.1.1
	github.com/go-openapi/strfmt v0.19.5
	github.com/golang-migrate/migrate/v4 v4.12.2
	github.com/golang/mock v1.6.0
	github.com/jessevdk/go-flags v1.4.0
	github.com/jmoiron/sqlx v1.2.0
	github.com/keighl/mandrill v0.0.0-20170605120353-1775dd4b3b41
	github.com/lib/pq v1.10.4
	github.com/sirupsen/logrus v1.8.1
	github.com/stretchr/testify v1.7.0
	github.com/tendermint/tendermint v0.34.14 // indirect
	github.com/testcontainers/testcontainers-go v0.11.0
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c
	google.golang.org/grpc v1.40.0
)

replace (
	github.com/99designs/keyring => github.com/cosmos/keyring v1.1.7-0.20210622111912-ef00f8ac3d76
	github.com/gogo/protobuf => github.com/regen-network/protobuf v1.3.3-alpha.regen.1
	google.golang.org/grpc => google.golang.org/grpc v1.33.2
)
