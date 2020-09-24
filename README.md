# Vulcan
![img](https://img.shields.io/docker/cloud/build/decentr/vulcan.svg)

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
    --log.level=debug \
    --postgres="host=localhost port=5432 user=postgres password=root sslmode=disable" \
    --posttres.migrations="scripts/migrations/postgres" \
    --mandrill.api_key="MANDRILL_SUCCESS" \
    --mandrill.email_subject="Email confirmation" \
    --mandrill.email_template_name="confirmation" \
    --mandrill.from_name="decentr noreply" \
    --mandrill.from_email="noreply@decentr.xyz" \
    --blockchain.node="zeus.testnet.decentr.xyz:26656" \
    --blockchain.from="zeus" \
    --blockchain.tx_memo="you're beautiful" \
    --blockchain.initial_stake=1
```

## Parameters
| CLI param         | Environment var          | Default | Description
|---------------|------------------|---------------|---------------------------------
| http.host         | HTTP_HOST         | 0.0.0.0  | host to bind server
| http.port    | HTTP_PORT    | 8080  | port to listen
| postgres    | POSTGRES    | host=localhost port=5432 user=postgres password=root sslmode=disable  | postgres dsn
| postgres.max_open_connections    | POSTGRES_MAX_OPEN_CONNECTIONS    | 0  | postgres maximal open connections count, 0 means unlimited
| postgres.max_idle_connections    | POSTGRES_MAX_IDLE_CONNECTIONS    | 5  | postgres maximal idle connections count
| postgres.migrations    | POSTGRES_MIGRATIONS    | scripts/migrations/postgres | postgres migrations directory
| mandrill.api_key    | MANDRILL_API_KEY   |   |  mandrillapp.com api key
| mandrill.email_subject    | MANDRILL_EMAIL_SUBJECT    | decentr.xyz - Verification  | subject for emails
| mandrill.email_template_name    | MANDRILL_EMAIL_TEMPLATE_NAME    |   | mandrill's template to be sent
| mandrill.from_name    | MANDRILL_FROM_NAME    | decentr.xyz  | name for emails sender
| mandrill.from_email    | MANDRILL_FROM_NAME    | noreply@decentrdev.com  | email for emails sender
| blockchain.node   | BLOCKCHAIN_NODE    | zeus.testnet.decentr.xyz:26656  | decentr node address
| blockchain.from   | BLOCKCHAIN_FROM    |  | decentr account name to send stakes
| blockchain.tx_memo   | BLOCKCHAIN_TX_MEMO    |  | decentr tx's memo
| blockchain.chain_id   | BLOCKCHAIN_CHAIN_ID    | testnet | decentr chain id
| blockchain.client_home   | BLOCKCHAIN_CLIENT_HOME    | ~/.decentrcli | decentrcli home directory
| blockchain.keyring_backend   | BLOCKCHAIN_KEYRING_BACKEND    | test | decentrcli keyring backend
| blockchain.keyring_prompt_input   | BLOCKCHAIN_KEYRING_PROMPT_INPUT    | | decentrcli keyring prompt input
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
