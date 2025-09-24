MAGELLAN(1) "OpenCHAMI" "Manual Page for magellan"

# NAME

magellan - Redfish-based BMC discovery tool

# SYNOPSIS

magellan [OPTIONS] COMMAND

# DESCRIPTION

The `magellan` CLI tool provides a frontend for interface with board management 
controllers (BMCs) through the Redfish protocol. 

List of available commands:

[[ *Comand*
:< *Description*
|  *scan*
:  
|  *collect*
:
|  *send*
:
|  *list*
:
|  *secrets*
:
|  *update*

# GLOBAL FLAGS

The *magellan* command accepts

--access-token string   Set the access token
      --cache string          Set the scanning result cache path (default "/tmp/allend/magellan/assets.db")
  -j, --concurrency int       Set the number of concurrent processes (default -1)
  -c, --config string         Set the config file path
      --log-file string       Set the path to store a log file
  -l, --log-level LogLevel    Set the logger log-level (debug|info|warn|error|trace|disabled) (default info)
  -t, --timeout int           Set the timeout for requests in seconds (default 5)


## GETTING STARTED

## CONFIGURATION

# AUTHOR

# SEE ALSO

*magellan-scan*(1), *magellan-collect*(1), *magellan-crawl*(1),
*magellan-list*(1), *magellan-secrets*(1), *magellan-update*(1)
