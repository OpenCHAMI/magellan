MAGELLAN-SECRETS(1) "OpenCHAMI" "Manual Page for magellan-secrets"

# NAME

magellan-secrets - Manage BMC credentials in a local file or secret manager

# SYNOPSIS

magellan secrets generatekey++
magellan secrets list [OPTIONS]++
magellan secrets remove [OPTIONS] _secret_id_...++
magellan secrets retrieve [OPTIONS] _secret_id_++
magellan secrets store [OPTIONS] _secret_id_ _data_

# EXAMPLES

// perform scan and store in local cache++
magellan scan --subnet 172.16.0.0/24 --cache ./assets.db

// view the results of the scan in local cache++
magellan list --cache ./assets.db

// export secret key to variable++
export MASTER_KEY=$(magellan secrets generatekey)

// store specific BMC node creds for *collect* and *crawl* in default secrets store++
// which uses *secrets.json* by default (e.g. when --file flag not set)++
magellan secrets store $bmc_host $bmc_creds

// retrieve creds from secrets store++
magellan secrets retrieve $bmc_host --file secrets.json

// list creds from specific secrets++
magellan secrets list --file secrets.json

// perform a collect using the secret store specified with --secrets-file flag++
magellan collect --secrets-file secrets.json -o nodes.json

// manage credentials in an OpenBao KV v2 store instead of a local file++
export BAO_ADDR=https://bao.example.com:8200++
export BAO_TOKEN=$bao_token++
magellan secrets --secrets-backend bao store $bmc_host $bmc_creds++
magellan secrets --secrets-backend bao list

# FLAGS

*--file* _path_
	Set path to a secrets file to manage secrets. Only applies to the _local_
	backend; ignored by the _bao_ and _vault_ backends.

	Requires the *MASTER_KEY* environment variable to be set. This can be set by
	generating a new key with the *magellan secrets generatekey* command.

	Credentials from the secrets file can only be accessed using the same key
	initially used to store the credential.

See *magellan*(1) for information about global flags used for all commands.

# BACKENDS

The *--secrets-backend* global option selects where credentials are stored.
Every command that reads or writes credentials honors it, not just *secrets*.

## local (default)

Credentials are kept in an encrypted JSON file (see *--file* and
*--secrets-file*). The *MASTER_KEY* environment variable, generated with
*magellan secrets generatekey*, encrypts and decrypts every value.

## bao

Credentials are kept in a KV v2 secrets engine on an OpenBao server. The
client is configured through the environment, with TLS flag fallbacks:

- *BAO_ADDR*: URL of the OpenBao server (e.g. _https://bao.example.com:8200_)
- *BAO_TOKEN*: Token with create, read, update, and delete on the KV v2
  engine. Alternatively, set *BAO_ROLE_ID* and *BAO_SECRET_ID* to log in via
  AppRole (path *BAO_LOGIN_PATH*, default _auth/approle/login_)
- *BAO_CACERT*: Path to a CA bundle when the server certificate is not signed
  by a system CA; *--cacert* is used only if no environment CA file, bytes,
  or directory is configured
- *BAO_MOUNT*: KV v2 mount path (default _magellan_)
- *BAO_PREFIX*: Optional path prefix below the mount

Use *--insecure=secrets* to skip TLS verification for the connection to the
secret manager; this does not affect BMC connections (see *--insecure* in
*magellan*(1)).

The *--cacert* and long-form *--insecure* flags are inherited by *store*,
*retrieve*, *list*, and *remove*. Bare *--insecure* selects _bmc_ and does not
disable secret-manager verification. The *store -i* shorthand remains reserved
for *--input-file*.

Magellan renews AppRole-issued tokens and reauthenticates when renewal ends.
The deployed RoleID and SecretID must remain valid for reauthentication;
Magellan does not rotate them. Explicit *BAO_TOKEN*/*VAULT_TOKEN* values are
caller-managed and are not renewed by Magellan.

Secret IDs are BMC URIs such as _https://172.16.0.10_, which are stored as
base64url-encoded path segments below the mount and prefix so any URI is a
valid KV path. The default credentials are stored at the literal path
*default*. The credential value is the JSON document
_{"username": ..., "password": ...}_ under the *value* field.

*magellan secrets list* lists metadata only; it does not fetch credential
values. It requires metadata _list_ permission, not data _read_ permission.

## vault

Identical to _bao_, using HashiCorp Vault and the *VAULT_ADDR*, *VAULT_TOKEN*,
*VAULT_ROLE_ID*, *VAULT_SECRET_ID*, *VAULT_CACERT*, *VAULT_MOUNT*,
*VAULT_PREFIX*, and *VAULT_LOGIN_PATH* environment variables. When both the
*BAO_* and *VAULT_* forms of a variable are set, the *BAO_* form wins.

# COMMANDS

Manage, list, retrieve, remove, and store BMC credentials.

## generatekey

Generates a new 32-byte master key (in hex).

## list

Lists sorted secret IDs only for the configured store (see *BACKENDS*; _--file_ applies to the local backend only). Default output is one ID per line. JSON and YAML output is an array of IDs, replacing the previous ID-to-value map. Use *retrieve* to explicitly reveal a credential. Listing skips subdirectories rather than recursing.

The format of this command is:

*list* [--file _path_]
	*-F, --output-format* _format_
		Set the output format to list secrets.

## remove

Remove secrets by IDs from the configured secret store (see *BACKENDS*; _--file_ applies to the local backend only).

The format of this command is:

*remove* [--file _path_] _secret_id_

## retrieve

Retrieve a secret by ID from the configured secret store (see *BACKENDS*; _--file_ applies to the local backend only).

The format of this command is:

*retrieve* [--file _path_] _secret_id_

## store

Stores the given string value under secretID.

The format of this command is:

*store* [--file _path_] _secret_id_ _data_
	*-f, --input-format* _format_
		Set the input data format for the credentials stored in the secret store.

	*-i, --input-file* _string_
		Set the file to read as input.

See *magellan*(1) for information about global flags and environment variables used for all commands.

# DOCKER WALKTHROUGH

Development only: this disposable server uses HTTP and a known root token.
Use fake credentials, never production secrets. Docker must be running.
From the source repository, build and create a test input file:

```
TEST_DIR=$(mktemp -d)
go build -o "$TEST_DIR/magellan" .
MAGELLAN="$TEST_DIR/magellan"
printf '%s\n' '{"username":"review-user","password":"fake-review-password"}' > "$TEST_DIR/credentials.json"

docker run --rm -d --name magellan-review-bao \
  -p 127.0.0.1:18200:8200 quay.io/openbao/openbao:latest \
  server -dev -dev-listen-address=0.0.0.0:8200 \
  -dev-root-token-id=magellan-review-only

(
  for name in $(env | sed -n 's/^\(\(BAO\|VAULT\)_[A-Za-z0-9_]*\)=.*/\1/p'); do
    unset "$name"
  done
  export BAO_ADDR=http://127.0.0.1:18200
  export BAO_TOKEN=magellan-review-only
  export BAO_MOUNT=secret
  export SECRETS_BACKEND=bao
  curl --fail --silent --show-error --retry 10 --retry-connrefused \
    --retry-delay 1 "$BAO_ADDR/v1/sys/health"
  "$MAGELLAN" secrets store review-check \
    --input-format=json -i "$TEST_DIR/credentials.json"
  "$MAGELLAN" secrets retrieve review-check
  "$MAGELLAN" secrets list
  "$MAGELLAN" secrets list --output-format=json
  "$MAGELLAN" secrets remove review-check
)

docker stop magellan-review-bao
```

Retrieval prints the fake credentials; listing prints only _review-check_
(JSON: _["review-check"]_). Removal deletes all versions. Stopping the
container discards its data. Podman may replace Docker. This tests remote
CRUD, not BMC access, TLS, AppRole renewal, or restricted policies.

The dev server mounts KV v2 at _secret_, hence *BAO_MOUNT=secret* rather
than Magellan's default _magellan_. The *-i* file is input, not a destination.
Remote stores ignore *--file* and *MASTER_KEY*. Local storage requires
*MASTER_KEY* and defaults to _./secrets.json_ unless *--file* is supplied.

# POLICY PERMISSIONS

Have an authorized administrator provision policies and service identities;
do not give root tokens to production services. With mount _magellan_ and no
prefix, a read-only BMC consumer uses:

```
path "magellan/data/*" {
  capabilities = ["read"]
}
```

Listing is separate and optional for BMC consumers. An IDs-only listing
identity needs only:

```
path "magellan/metadata/" {
  capabilities = ["list"]
}
```

An identity authorized to manage secrets can additionally use:

```
path "magellan/data/*" {
  capabilities = ["create", "update", "read"]
}
path "magellan/metadata/*" {
  capabilities = ["delete"]
}
```

Narrow wildcard permissions to the required scope. Use _secret_ instead of
_magellan_ for the walkthrough. With *BAO_PREFIX=team*, listing requires
_magellan/metadata/team_; values live at
_magellan/data/team/<encoded-ID>_. IDs use base64url without padding, except
the literal _default_. *remove* deletes metadata and all secret versions.

# TROUBLESHOOTING

- *403 / permission denied*: inspect the request URL. AppRole login failures
  differ from KV data or metadata denials. Use *bao token lookup* and
  *bao token capabilities* _path_ (omit _/v1/_). Read-only identities cannot
  store/remove; list-only identities cannot retrieve values.
- *AppRole not used*: nonempty *BAO_TOKEN* or *VAULT_TOKEN* wins. Unset both
  to use AppRole. Supplied tokens are caller-managed; AppRole tokens are
  renewed/reacquired, requiring a valid deployed RoleID/SecretID.
- *Wrong mount or missing secret*: check KV v2, mount, and prefix. Depending
  on policy, an incorrect path may return 403 or 404.
- *TLS errors*: configure the environment CA file, bytes, or directory;
  *--cacert* is only a fallback. Development-only *--insecure=secrets* skips
  secret-manager verification; bare *--insecure* selects BMC scope instead.
- *Local path/key errors*: verify the backend. *-i* is not the destination;
  local storage needs *MASTER_KEY* and optionally *--file*. Remote storage
  needs neither.

Share only sanitized errors and paths, never tokens, SecretIDs, input
credential files, or *retrieve* output.

# AUTHOR

Written by David J. Allen and maintained by the OpenCHAMI developers.

# SEE ALSO

*magellan*(1), *magellan-collect*(1), *magellan-crawl*(1)

; Vim modeline settings
; vim: set tw=80 noet sts=4 ts=4 sw=4 syntax=scdoc:
