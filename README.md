# Vulcan
![img](https://img.shields.io/docker/cloud/build/decentr/vulcan.svg)

Vulcan provides Decentr off-chain functionality.

## Run
### Docker
#### Local image
```
make image
docker run -it --rm -e "HTTP_HOST=0.0.0.0" -e "HTTP_PORT=7070" -e "LOG_LEVEL=debug" -p "7080:7070" vulcan-local
```
#### Docker Compose
```
make image
docker-compose -f scripts/docker-compose.yml up -d
```
### From source
`will be fixed later`
```
go run cmd/vulcan/main.go \
    --http.host=0.0.0.0 \
    --http.port=8080 \
    --log.level=debug
```

## Parameters
| CLI param         | Environment var          | Default | Description
|---------------|------------------|---------------|---------------------------------
| http.host         | HTTP_HOST         | 0.0.0.0  | host to bind server
| http.port    | HTTP_PORT    | 8080  | port to listen
| postgres    | POSTGRES    | host=localhost port=5432 user=postgres password=root sslmode=disable  | postgres dsn
| mandrill.api_key    | MANDRILL_API_KEY   |   |  mandrillapp.com api key
| mandrill.email_subject    | MANDRILL_EMAIL_SUBJECT    | decentr.xyz - Verification  | subject for emails
| mandrill.email_template_name    | MANDRILL_EMAIL_TEMPLATE_NAME    |   | mandrill's template to be sent
| mandrill.from_name    | MANDRILL_FROM_NAME    | decentr.xyz  | name for emails sender
| mandrill.from_email    | MANDRILL_FROM_NAME    | noreply@decentrdev.com  | email for emails sender
| log.level   | LOG_LEVEL   | info  | level of logger (debug,info,warn,error)
| blockchain.initial_stake | BLOCKCHAIN_INITIAL_STAKE | 1 | stakes count to be sent


## Development
### Makefile
#### Update vendors
Use `make vendor`
#### Install required for development tools
You can check all tools existence with `make check-all` or force installing them with `make install-all` 
##### golangci-lint 1.29.0
Use `make install-linter`
##### swagger v0.25.0
Use `make install-swagger`
##### gomock v1.4.3
Use `make install-mockgen`
#### Build docker image
Use `make image` to build local docker image named `vulcan-local`
#### Build binary
Use `make build` to build for your OS or use `make linux` to build for linux(used in `make image`) 
#### Run tests
Use `make test` to run tests. Also you can run tests with `integration` tag with `make fulltest`
