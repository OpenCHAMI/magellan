MAGELLAN(1) "OpenCHAMI" "Manual Page for magellan"

# NAME

magellan - Redfish-based BMC discovery tool

# SYNOPSIS

magellan [OPTIONS] COMMAND

# DESCRIPTION

The `magellan` CLI tool provides a frontend for interface with board management
controllers (BMCs) through the Redfish protocol.

List of available commands:

[[ *Command*
:< *Description*
|  *scan*
:  Perform scan to discover BMC or PDU nodes
|  *collect*
:  Retrieve node data using Redfish or JAWS API concurrently
|  *crawl*
:  Crawl a single BMC for hardware inventory
|  *send*
:  Send retrieve node data to specified host
|  *list*
:  Show nodes found from scan
|  *secrets*
:  Manage BMC credentials
|  *settings*
:  Configure BMC properties through Redfish
|  *update*
:  Update firmware through Redfish API
|  *serve*
:  Run magellan as a long-lived BMC service (REST API)
|  *version*
:  Print version info and exit

# GLOBAL FLAGS

The *magellan* command accepts

*--access-token* _encoded_jwt_
	Set the access token as an encoded JSON Web Token. Access tokens will be
	included in request headers with the *send* command. The access token can
	also be set using the ACCESS_TOKEN environment variable as well.

*--cache* _path_
	Set the path to cache data from a scan. The default path to the cache is
	'/tmp/allend/magellan/assets.db'. 

	When *--cache* is passed explicitly (or set through the 'CACHE' environment
	variable or config file), it takes precedence over standard input for
	commands that read scan results, such as *magellan-collect*(1): the cache is
	read even when stdin is connected to a pipe, so scripted and CI usage
	behaves the same with and without a TTY. When no other input source
	provides data, the cache path is consulted as a fallback.

	The *magellan* CLI can run full workflows without writing to disk. See the
	*Simple Workflow* on how to do so in the "Getting Started" section of this
	document.

*-j, --concurrency* _process_count_
	Set the number of concurrent processes. This will affect the number of
	processes used for commands such as *collect* (e.g. number of simultaneous
	requests made to BMCs). 
	
	By default, the _process_count_ is set to -1 which
	automatically calculates the number of processes optimally needed. For
	example, if *collect* is ran to gather hardware inventory from 50 BMCs, then
	50 proccesses (goroutines) will spawn to make a requests per BMC.

*-c, --config* _path_
	Set the path to a config file. When _path_ is not set, *magellan* will attempt
	to find a file in `$HOME/config/magellan.yaml`.

	See the *CONFIGURATION* section for more details on creating a config file.

*--log-file* _path_
	Set the path to write logs. If no _path_ is set, then no log file is created
	or used (default). Messages written to the log file are always recorded as
	raw JSON events with timestamps, regardless of the *--log-format* setting.

*-l, --log-level* _level_
	Set the global logger log level. Log levels with higher internal values than
	the log level that is set will be printed as well. For example, if _level_ is
	set to *debug* then all other messages other than *trace* level messages will
	be printed. Likewise, if _level_ is set to *error*, then only error messages
	will be printed.

	Available log levels:

	- _trace_
	- _debug_
	- _info_ (default)
	- _warn_
	- _error_
	- _disabled_

*--log-format* _format_
	Set the format of log messages written to standard error. This affects log
	messages only; the data written by commands (see the *--output-format* and
	*--input-format* options) is not affected.

	Available log formats:

	- _json_ (default): raw JSON log events
	- _rfc3339_: human-readable console format with RFC3339 timestamps
	- _basic_: compact console format without timestamps

*--log-color* _mode_
	Control ANSI colorization of log messages written to standard error in the
	*rfc3339* and *basic* log formats. This option has no effect when the log
	format is *json*.

	Available color modes:

	- _auto_ (default): colorize when standard error is a terminal and the
	  *NO_COLOR* environment variable is not set
	- _on_: always colorize, unless the *NO_COLOR* environment variable is set
	- _off_: never colorize

	The *NO_COLOR* environment variable (see https://no-color.org) overrides
	every mode, including _on_, so that no ANSI color codes are emitted.

*--secrets-backend* _backend_
	Set the secret store backend used to look up BMC credentials. This is a
	global option honored by every command that reads credentials (*collect*,
	*crawl*, *power*, *secrets*, *serve*, *settings*, and *update*).

	Available backends:

	- _local_ (default): encrypted JSON file on disk; requires the *MASTER_KEY*
	  environment variable and a file path (*--secrets-file* or *secrets --file*)
	- _bao_: OpenBao KV v2 secrets engine; authenticated with *BAO_TOKEN* or
	  *BAO_ROLE_ID*+*BAO_SECRET_ID*, server address from *BAO_ADDR*
	- _vault_: HashiCorp Vault KV v2 secrets engine; configured with the
	  equivalent *VAULT_* environment variables

	For the _bao_ and _vault_ backends, credentials are stored under the
	*magellan* KV v2 mount by default (override with *BAO_MOUNT*/*VAULT_MOUNT*,
	optionally below a path prefix set with *BAO_PREFIX*/*VAULT_PREFIX*). BMC
	URIs are stored as base64url-encoded path segments, and the default
	credentials use the literal path *default*. Use *--insecure=secrets* to skip
	TLS verification for the secret manager connection.

*-t, --timeout* _time_in_secs_
	Set the timeout for requests in seconds. This includes requests used in *scan*,
	*crawl*, *collect*, and *send*. By default, the value of _time_in_secs_ is 5.

# ENVIRONMENT VARIABLES

The *magellan* CLI tool allows configuring its flags using environment variables. These are automatically parsed based on the flag names, with dots and hyphens converted to underscores, and fully capitalized.

*Global Variables*
- *CONCURRENCY*: Sets the number of concurrent processes
- *TIMEOUT*: Sets the timeout for requests in seconds
- *LOG_LEVEL*: Sets the logger log-level
- *LOG_FORMAT*: Sets the log message format (json, rfc3339, basic)
- *LOG_COLOR*: Sets log color output (auto, on, off)
- *ACCESS_TOKEN*: Sets the access token
- *CACHE*: Sets the scanning result cache path
- *CONFIG*: Sets the config file path
- *SECRETS_BACKEND*: Sets the secret store backend (local, bao, vault)

*Scan Variables*
- *SCAN_PORTS*: Adds additional ports to scan
- *SCAN_SCHEME*: Sets the default scheme to use
- *SCAN_PROTOCOL*: Sets the default protocol to use
- *SCAN_SUBNETS*: Adds subnets to scan
- *SCAN_SUBNET_MASKS*: Sets the default subnet mask
- *SCAN_DISABLE_PROBING*: Disables probing found assets for Redfish services
- *SCAN_DISABLE_CACHE*: Disables saving found assets to cache
- *SCAN_INSECURE*: Skips TLS certificate verification during probe
- *SCAN_OUTPUT_FORMAT*: Sets the output format (json, yaml)
- *SCAN_OUTPUT*: Sets the output file path (for json/yaml formats)

*Collect Variables*
- *COLLECT_PROTOCOL*: Sets the protocol used to query
- *COLLECT_OUTPUT_FILE*: Sets the path to store collection data
- *COLLECT_OUTPUT_DIR*: Sets the path to store collection data using HIVE
- *COLLECT_USERNAME*: Sets the master BMC username
- *COLLECT_PASSWORD*: Sets the master BMC password
- *COLLECT_SECRETS_FILE*: Sets path to the node secrets file
- *COLLECT_INSECURE*: Skips TLS certificate verification for Redfish requests
- *COLLECT_SHOW_OUTPUT*: Shows the output of a collect run
- *COLLECT_INPUT_FORMAT*: Sets the default input data format
- *COLLECT_OUTPUT_FORMAT*: Sets the default output data format
- *COLLECT_BMC_ID_MAP*: Sets the BMC ID mapping

*Crawl Variables*
- *CRAWL_INSECURE*: Skip TLS certificate verification for Redfish request
- *CRAWL_SHOW_OUTPUT*: Show the hardware inventory found from BMC

*Power Variables*
- *POWER_CACERT* (or *CACERT*): Sets the path to CA cert file
- *POWER_FORMAT* (or *FORMAT*): Sets the output format
- *LIST_RESET_TYPES*: Lists supported Redfish reset types
- *RESET_TYPE*: Redfish reset type to perform
- *INVENTORY_FILE*: YAML file containing node inventory

*Settings Variables*
- *SETTINGS_USERNAME*: Sets the master BMC username
- *SETTINGS_PASSWORD*: Sets the master BMC password
- *SETTINGS_SECRETS_FILE*: Sets the secrets file with BMC credentials
- *SETTINGS_INSECURE*: Skips TLS certificate verification during probe
- *SETTINGS_CACERT*: Sets the path to CA cert file
- *SETTINGS_INVENTORY_FILE*: File containing node inventory
- *SETTINGS_INPUT_FORMAT*: Sets the inventory input format (json or yaml)
- *SETTINGS_PRESERVE_CONFIG*: Preserve settings during reset

*Update Variables*
- *UPDATE_SCHEME*: Sets the transfer protocol
- *UPDATE_FIRMWARE_URI*: Sets the URI to retrieve the firmware
- *UPDATE_STATUS*: Gets the status of the update
- *UPDATE_INSECURE*: Allows insecure connections to the server

*Secrets Variables*
- *SECRETS_FILE*: Sets the secrets file with BMC credentials
- *FORMAT*: Sets the input format for the secrets file
- *INPUT_FILE*: Sets the file to read as input
- *MASTER_KEY*: Set the generated key for the secrets file

*Secret Store Variables* (bao/vault backends)
- *BAO_ADDR* (or *VAULT_ADDR*): URL of the OpenBao/Vault server
- *BAO_TOKEN* (or *VAULT_TOKEN*): Token used to authenticate
- *BAO_ROLE_ID* and *BAO_SECRET_ID* (or *VAULT_ROLE_ID* and
  *VAULT_SECRET_ID*): AppRole credentials, used when no token is set
- *BAO_CACERT* (or *VAULT_CACERT*): Path to the CA bundle for the secret
  manager TLS connection
- *BAO_MOUNT* (or *VAULT_MOUNT*): KV v2 mount path (default: magellan)
- *BAO_PREFIX* (or *VAULT_PREFIX*): Path prefix below the mount
- *BAO_LOGIN_PATH* (or *VAULT_LOGIN_PATH*): AppRole login path (default:
  auth/approle/login)

Note: Environment variables that take multiple arguments (like *SCAN_SUBNETS* or
*SCAN_PORTS*) should have their values delimited by a comma *,* (e.g., *SCAN_PORTS=5000,5001*).

Note: The *--insecure* option accepted by *scan*, *crawl*, *collect*, *power*,
*update*, *serve*, and the *settings* subcommands takes a scope value:
*i*/*--insecure* with no value (or *true*) skips TLS verification for BMC
connections only, matching earlier releases, while *--insecure=none*,
*--insecure=bmc*, *--insecure=secrets* (the secret manager connection only),
and *--insecure=both* select the scope explicitly. The *INSECURE*,
*SCAN_INSECURE*, *COLLECT_INSECURE*, *CRAWL_INSECURE*, *SETTINGS_INSECURE*,
and *UPDATE_INSECURE* variables accept the same values.

# GETTING STARTED

The *magellan* CLI is a frontend tool for dynamically scanning and collecting 
inventory data from board management controllers (BMCs) through a running Redfish 
service. Here are some of the command ways to use the tool:

1. Simple Workflow:    scan  -> collect -> send++
2. Complex Workflow:   scan  -> list    -> secrets -> collect -> send++
3. Recursive Workflow: scan  -> collect -> send    -> collect -> send++
4. Debug Workflow:     crawl -> send    -> crawl   -> send++
5. Static Workflow:    send

## Simple Workflow

This is the simplest and most minimalistic way for using *magellan*. This method
does not use the secrets store and no changes are made after an inventory
collection. This example also stores the cache in a local file as opposed to the
default location.

```
magellan scan --subnet 172.16.0.0/24 --cache assets.db
magellan collect -u $username -p $password --cache assets.db | 
magellan send https://demo.openchami.cluster:8443/hsm/v2
```

We can also use JSON or YAML files in between the *scan* and *collect* commands.

```
magellan scan --subnet 172.18.0.0 --subnet-mask 255.255.255.0 --disable-cache -F json -o assets.json
magellan collect -i -d@assets.json --secrets-file secrets.json -o nodes.json
```

Note that it is possible to run entire workflow without touching the filesystem
using pipes. This example also allows for modification between each step using YAML.

```
magellan scan --subnet 172.18.0.0/24 --port 5000 -F yaml |
magellan collect -i --secrets-file secrets.json -f yaml -F yaml | 
magellan send -f yaml https://demo.openchami.cluster:8443/hsm/v2
```


## Complex Workflow

This workflow is more complex and will include using a local cache database, 
using the secret store, and allowing for editting the inventory collection before 
sending it to our state manager.

```
// scan for assets on multiple networks
magellan scan 10.0.0.101 \
	--subnet 172.16.0.0/24 \
	--subnet 172.21.0.0 \
	--subnet-mask 255.255.255.0
	--port 5000
	--cache ./assets.db

// show which hosts we found
magellan list --cache ./assets.db

// store secrets for host (saves to 'secrets.json' by default)
magellan secrets store default $default_username:$default_password
magellan secrets store $bmc_host1 $bmc_username1:$bmc_password1
magellan secrets store $bmc_host2 $bmc_username2:$bmc_password2

// perform collect using secrets store and save to YAML
magellan collect --secrets-file secrets.json -o nodes.yaml -F yaml

// make edits to inventory data
vim nodes.yaml

// read editted inventory and send data to host
magellan send -d @nodes.yaml -f yaml https://demo.openchami.cluster:8443/hsm/v2
```

## Recursive Workflow

If we already have cache data, we can completely bypass doing a scan. Instead,
we can repeatedly do a collect with little effort if we already have the secrets
store set up and send the data to our remote host all in one step like in the
"Simple Workflow".

```
// assume we already have a cache database and secrets store somewhere...
magellan collect --secrets-file secrets.json | magellan send https://demo.openchami.cluster:8443/hsm/v2

// alternatively, just update the file we send our output
magellan collect --secrets-file secrets.json -o nodes.json
```

We can continuously do this whenever we want to update the state of our inventory.
Alternatively, we can also pipe the contents from 'send' directly to 'collect'.

```
magellan scan --subnet 172.16.0.0/24 | magellan collect
```

This allows getting the inventory data dynamically while avoiding writing to disk.
We can also do this in two steps if for some reason, we want to edit the scanned
asset data before a 'collect'.

```
// write output of scan to file
magellan scan --subnet 172.16.0.0/24 -F yaml -o assets.yaml

// read the output of scan back as input to collect
magellan collect -d @assets.yaml -f yaml
```

## Debug Workflow

This workflow is similar to the "Collect" workflow except we crawl a single BMC
that we specify instead of multiple BMCs found from the scan. Like with the previous
workflow, this can be done repeatedly to update the state of the inventory.

```
magellan crawl -i -u $u -p $p https://bmc.example.com -o node.json
magellan send -d @node.json https://demo.openchami.cluster:8443/hsm/v2
```

Note that this only uses a single go routine instead of multiple like with *collect*.

## Static Workflow

Instead of using the tool for dynamic hardware inventory collection, we can instead
just create a file similar to the 'collect' output and use the 'send' command to
make a request to the specified host.

```
magellan send -d@inventory.yaml -f yaml https://demo.openchami.cluster:8443/hsm/v2
```

# REFERENCES

For more information about Redfish, visit https://https://redfish.dmtf.org/.
For Redfish specifications and other documents, visit https://www.dmtf.org/standards/redfish.

# CONFIGURATION

The *magellan* CLI configuration is handled by passing the *--config* in a single 
YAML file. An example file can be found at in the root repository named
*example.config.yaml*.

# AUTHOR

Written by David J. Allen and maintained by the OpenCHAMI developers.

# SEE ALSO

*magellan-collect*(1), *magellan-crawl*(1), *magellan-list*(1),
*magellan-scan*(1), *magellan-secrets*(1), *magellan-send*(1),
*magellan-settings*(1), *magellan-update*(1)
