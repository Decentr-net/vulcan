module github.com/Decentr-net/vulcan

go 1.15

replace github.com/docker/docker => github.com/docker/engine v0.0.0-20190717161051-705d9623b7c1 // fix logrus for testcontainers

require (
	github.com/Decentr-net/decentr v1.2.2
	github.com/Decentr-net/go-api v0.0.2
	github.com/Decentr-net/go-broadcaster v0.0.1
	github.com/Decentr-net/logrus v0.7.2-0.20210316223658-7a9b48625189
	github.com/cosmos/cosmos-sdk v0.39.2
	github.com/getsentry/sentry-go v0.10.0 // indirect
	github.com/go-chi/chi v4.1.2+incompatible
	github.com/go-chi/cors v1.1.1
	github.com/go-openapi/strfmt v0.19.5
	github.com/golang-migrate/migrate/v4 v4.12.2
	github.com/golang/mock v1.4.4
	github.com/jessevdk/go-flags v1.4.0
	github.com/jmoiron/sqlx v1.2.0
	github.com/keighl/mandrill v0.0.0-20170605120353-1775dd4b3b41
	github.com/lib/pq v1.3.0
	github.com/sirupsen/logrus v1.8.1
	github.com/stretchr/testify v1.7.0
	github.com/testcontainers/testcontainers-go v0.8.0
	golang.org/x/sync v0.0.0-20200625203802-6e8e738ad208
)
