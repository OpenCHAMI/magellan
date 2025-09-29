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
:  Retrieve node data using Redfish or JAWS API
|  *send*
:  Send retrieve node data to specified host
|  *list*
:  Show nodes found from scan
|  *secrets*
:  Manage BMC credentials
|  *update*
:  Update firmware through Redfish API

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


## GETTING STARTED

Nothing here yet.

## CONFIGURATION

Nothing here yet.

# AUTHOR

Written by David J. Allen and maintained by the OpenCHAMI developers.

# SEE ALSO

*magellan-scan*(1), *magellan-collect*(1), *magellan-crawl*(1),
*magellan-list*(1), *magellan-secrets*(1), *magellan-update*(1)
*magellan-send*(1)


