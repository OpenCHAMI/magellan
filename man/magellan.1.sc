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
|  *update*
:  Update firmware through Redfish API
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

*-j, --concurrency* _process_count_
	Set the number of concurrent processes (default -1)

*-c, --config* _path_
	Set the path to a config file. When _path_ is not set, *magellan* will attempt
	to find a file in `$HOME/config/magellan.yaml`.

	See the *CONFIGURATION* section for more details on creating a config file.

*--log-file* _path_
	Set the path to write logs. If no _path_ is set, then no log file is created
	or used (default).

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

*-t, --timeout* _time_in_secs_
	Set the timeout for requests in seconds. This includes requests used in *scan*,
	*crawl*, *collect*, and *send*. By default, the value of _time_in_secs_ is 5.

# ENVIRONMENT VARIABLES

The *magellan* CLI tool allows configuring its flags using environment variables. These are automatically parsed based on the flag names, with dots (".") and hyphens ("-") converted to underscores ("_"), and fully capitalized.

*Global Variables*
- *CONCURRENCY*: Sets the number of concurrent processes
- *TIMEOUT*: Sets the timeout for requests in seconds
- *LOG_LEVEL*: Sets the logger log-level
- *ACCESS_TOKEN*: Sets the access token
- *CACHE*: Sets the scanning result cache path
- *CONFIG*: Sets the config file path

*Scan Variables*
- *SCAN_PORTS*: Adds additional ports to scan
- *SCAN_SCHEME*: Sets the default scheme to use
- *SCAN_PROTOCOL*: Sets the default protocol to use
- *SCAN_SUBNETS*: Adds subnets to scan
- *SCAN_SUBNET_MASKS*: Sets the default subnet mask
- *SCAN_DISABLE_PROBING*: Disables probing found assets for Redfish services
- *SCAN_DISABLE_CACHE*: Disables saving found assets to cache
- *SCAN_INSECURE*: Skips TLS certificate verification during probe

*Collect Variables*
- *COLLECT_PROTOCOL* (or *PROTOCOL*): Sets the protocol used to query
- *COLLECT_OUTPUT_FILE* (or *OUTPUT_FILE*): Sets the path to store collection data
- *COLLECT_OUTPUT_DIR* (or *OUTPUT_DIR*): Sets the path to store collection data using HIVE
- *COLLECT_USERNAME* (or *USERNAME*): Sets the master BMC username
- *COLLECT_PASSWORD* (or *PASSWORD*): Sets the master BMC password
- *COLLECT_SECRETS_FILE*: Sets path to the node secrets file
- *COLLECT_INSECURE*: Skips TLS certificate verification for Redfish requests
- *COLLECT_SHOW_OUTPUT*: Shows the output of a collect run
- *COLLECT_OUTPUT_FORMAT*: Sets the default output data format
- *COLLECT_BMC_ID_MAP*: Sets the BMC ID mapping

*Crawl Variables*
- *CRAWL_INSECURE*: Skip TLS certificate verification for Redfish request

*Power Variables*
- *POWER_CACERT* (or *CACERT*): Sets the path to CA cert file
- *POWER_FORMAT* (or *FORMAT*): Sets the output format
- *LIST_RESET_TYPES*: Lists supported Redfish reset types
- *RESET_TYPE*: Redfish reset type to perform
- *INVENTORY_FILE*: YAML file containing node inventory

*Update Variables*
- *UPDATE_SCHEME*: Sets the transfer protocol
- *UPDATE_FIRMWARE_URI*: Sets the URI to retrieve the firmware
- *UPDATE_STATUS*: Gets the status of the update
- *UPDATE_INSECURE*: Allows insecure connections to the server

*Secrets Variables*
- *FILE*: Sets the secrets file with BMC credentials
- *FORMAT*: Sets the input format for the secrets file
- *INPUT_FILE*: Sets the file to read as input
- *MASTER_KEY*: Set the generated key for the secrets file

Note: Environment variables that take multiple arguments (like `SCAN_SUBNETS` or `SCAN_PORTS`) should have their values delimited by a comma `,` (e.g., `SCAN_PORTS=5000,5001`).

# GETTING STARTED

The *magellan* CLI is a frontend tool for dynamically scanning and collecting 
inventory data from board management controllers (BMCs) through a running Redfish 
service. Here are some of the command ways to use the tool:

1. Simple Workflow: 	scan -> collect -> send -> *++
2. Complex Workflow: 	scan -> list -> secrets -> collect -> send -> *++
3. Recursive Workflow:	scan -> collect -> send -> collect -> send -> *++
4. Debug Workflow:		crawl -> send -> crawl -> send -> *++
5. Static Workflow:		send -> *

## Simple Workflow

This is the simplest and most minimalistic way for using *magellan*. This method
does not use the secrets store and no changes are made after an inventory
collection.

```
magellan scan --subnet 172.16.0.0/24
magellan collect -u $u -p $p | magellan send https://smd.example.com
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
magellan send -d @node.json https://smd.example.com
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

*magellan-scan*(1), *magellan-collect*(1), *magellan-crawl*(1),
*magellan-list*(1), *magellan-secrets*(1), *magellan-update*(1)
*magellan-send*(1)


