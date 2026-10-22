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
client is configured entirely through the environment:

- *BAO_ADDR*: URL of the OpenBao server (e.g. _https://bao.example.com:8200_)
- *BAO_TOKEN*: Token with create, read, update, and delete on the KV v2
  engine. Alternatively, set *BAO_ROLE_ID* and *BAO_SECRET_ID* to log in via
  AppRole (path *BAO_LOGIN_PATH*, default _auth/approle/login_)
- *BAO_CACERT*: Path to a CA bundle when the server certificate is not signed
  by a system CA; the global *--cacert* flag is used as a fallback
- *BAO_MOUNT*: KV v2 mount path (default _magellan_)
- *BAO_PREFIX*: Optional path prefix below the mount

Use *--insecure=secrets* to skip TLS verification for the connection to the
secret manager; this does not affect BMC connections (see *--insecure* in
*magellan*(1)).

Secret IDs are BMC URIs such as _https://172.16.0.10_, which are stored as
base64url-encoded path segments below the mount and prefix so any URI is a
valid KV path. The default credentials are stored at the literal path
*default*. The credential value is the JSON document
_{"username": ..., "password": ...}_ under the *value* field.

*magellan secrets list* reads the value of every secret, so its policy must
grant read (not only list) on the engine.

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

Lists all the secret IDs and their values for the configured secret store (see *BACKENDS*; _--file_ applies to the local backend only).

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

# AUTHOR

Written by David J. Allen and maintained by the OpenCHAMI developers.

# SEE ALSO

*magellan*(1), *magellan-collect*(1), *magellan-crawl*(1)

; Vim modeline settings
; vim: set tw=80 noet sts=4 ts=4 sw=4 syntax=scdoc:
