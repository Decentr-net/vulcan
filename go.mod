module github.com/Decentr-net/vulcan

go 1.14

replace github.com/docker/docker => github.com/docker/engine v0.0.0-20190717161051-705d9623b7c1 // fix logrus for testcontainers

require (
	github.com/asaskevich/govalidator v0.0.0-20200428143746-21a406dcc535 // indirect
	github.com/davecgh/go-spew v1.1.1
	github.com/go-chi/chi v4.1.2+incompatible
	github.com/go-openapi/strfmt v0.19.5
	github.com/golang-migrate/migrate/v4 v4.12.2
	github.com/golang/mock v1.4.4
	github.com/jessevdk/go-flags v1.4.0
	github.com/lib/pq v1.3.0
	github.com/sirupsen/logrus v1.6.0
	github.com/stretchr/testify v1.6.1
	github.com/testcontainers/testcontainers-go v0.8.0
	github.com/tomasen/realip v0.0.0-20180522021738-f0c99a92ddce
	golang.org/x/oauth2 v0.0.0-20200107190931-bf48bf16ab8d
	golang.org/x/sync v0.0.0-20200625203802-6e8e738ad208
	gopkg.in/check.v1 v1.0.0-20190902080502-41f04d3bba15 // indirect
)
