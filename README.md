# Vulcan
![img](https://img.shields.io/docker/cloud/build/decentr/vulcan.svg) ![img](https://img.shields.io/github/go-mod/go-version/Decentr-net/vulcan) ![img](https://img.shields.io/github/v/tag/Decentr-net/vulcan?label=version)

Vulcan provides Decentr off-chain functionality. The Vulcan uses decentrcli home for sending messages to blockchain.  
You should provide decentrcli home directory to this service. The Vulcan will use it for transactions signing. 


## Run
### Docker
#### Local image
```
make image
docker run -it --rm -e "HTTP_HOST=0.0.0.0" -e "HTTP_PORT=7070" -e "LOG_LEVEL=debug" -p "7080:7070" vulcan-local
```
### From source
```
go run cmd/vulcan/main.go \
    --http.host=0.0.0.0 \
    --http.port=8080 \
    --http.request-timeout 10s \
    --log.level=debug \
    --postgres="host=localhost port=5432 user=postgres password=root sslmode=disable" \
    --postgres.migrations="scripts/migrations/postgres" \
    --mandrill.api_key="MANDRILL_SUCCESS" \
    --mandrill.email_verification_subject="Email confirmation" \
    --mandrill.email_verification_template_name="confirmation" \
    --mandrill.email_welcome_subject="Welcome" \
    --mandrill.email_welcome_template_name="welcome" \
    --mandrill.from_name="decentr noreply" \
    --mandrill.from_email="noreply@decentrdev.com" \
    --blockchain.node="zeus.testnet.decentr.xyz:26656" \
    --blockchain.from="zeus" \
    --blockchain.tx_memo="you're beautiful" \
    --blockchain.initial_stake=1000000
```

## Parameters
| CLI param         | Environment var          | Default | Required | Description
|---------------|------------------|---------------|-------|---------------------------------
| http.host         | HTTP_HOST         | 0.0.0.0  | true | host to bind server
| http.port    | HTTP_PORT    | 8080  | true | port to listen
| http.request-timeout | HTTP_REQUEST_TIMEOUT | 45s | false | request processing timeout
| postgres    | POSTGRES    | host=localhost port=5432 user=postgres password=root sslmode=disable  | true | postgres dsn
| postgres.max_open_connections    | POSTGRES_MAX_OPEN_CONNECTIONS    | 0 | true | postgres maximal open connections count, 0 means unlimited
| postgres.max_idle_connections    | POSTGRES_MAX_IDLE_CONNECTIONS    | 5 | true | postgres maximal idle connections count
| postgres.migrations    | POSTGRES_MIGRATIONS    | /migrations/postgres | true | postgres migrations directory
| mandrill.api_key    | MANDRILL_API_KEY   |  | true |  mandrillapp.com api key
| mandrill.verification_email_subject    | MANDRILL_VERIFICATION_EMAIL_SUBJECT    | decentr.xyz - Verification | false | subject for verification emails
| mandrill.verification_email_template_name    | MANDRILL_VERIFICATION_EMAIL_TEMPLATE_NAME    |  | true | mandrill's verification template to be sent
| mandrill.welcome_email_subject    | MANDRILL_WELCOME_EMAIL_SUBJECT    | decentr.xyz - Verification | false | subject for welcome emails
| mandrill.welcome_email_template_name    | MANDRILL_WELCOME_EMAIL_TEMPLATE_NAME    |  | true | mandrill's welcome template to be sent
| mandrill.from_name    | MANDRILL_FROM_NAME    | decentr.xyz | false | name for emails sender
| mandrill.from_email    | MANDRILL_FROM_NAME    | noreply@decentrdev.com | true | email for emails sender
| blockchain.node   | BLOCKCHAIN_NODE    | http://zeus.testnet.decentr.xyz:26657 | true | decentr node address
| blockchain.from   | BLOCKCHAIN_FROM    | | true | decentr account name to send stakes
| blockchain.tx_memo   | BLOCKCHAIN_TX_MEMO    | | false | decentr tx's memo
| blockchain.chain_id   | BLOCKCHAIN_CHAIN_ID    | testnet | true| decentr chain id
| blockchain.client_home   | BLOCKCHAIN_CLIENT_HOME    | ~/.decentrcli | true | decentrcli home directory
| blockchain.keyring_backend   | BLOCKCHAIN_KEYRING_BACKEND    | test | true | decentrcli keyring backend
| blockchain.keyring_prompt_input   | BLOCKCHAIN_KEYRING_PROMPT_INPUT    | | false | decentrcli keyring prompt input
| log.level   | LOG_LEVEL   | info | false | level of logger (debug,info,warn,error)
| sentry.dsn    | SENTRY_DSN    |  | sentry dsn
| blockchain.initial_stake | BLOCKCHAIN_INITIAL_STAKE | 1000000 | true | stakes count to be sent, 1DEC = 1000000 uDEC

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
