MAGELLAN-CRAWL(1) "OpenCHAMI" "Manual Page for magellan-crawl"

# NAME

magellan-crawl - Retrieve Redfish data from a single BMC node

# SYNOPSIS

magellan crawl [OPTIONS] _host_

# EXAMPLES

magellan crawl https://bmc.example.com
magellan crawl https://bmc.example.com -i -u username -p password

# FLAGS

*-F, --format* _format_
	Set the output data format.
	
	Possible output formats:
	
	- json (default)
	- yaml 

*-i, --insecure*              
	Skip TLS verification when making HTTP requests. This allows making requests
	to HTTPS hosts without needing to supply a CA certificate.

*-p, --password* _value_
	Set the password for basic authentication for requests to the BMC node.

*-f, --secrets-file* _path_
	Set the path to a secrets file. The MASTER_KEY environment variable must be
	set first. The default _path_ value is "secrets.json".

	See *magellan-secrets*(1) for more information about using the secrets file.

*--show*
	Show the output of a successful crawl. 

*-u, --username* _value_
	Set the username for basic authentication for requests to the BMC node.

See *magellan*(1) for information about global flags used for all commands.

# AUTHOR

Written by David J. Allen and maintained by the OpenCHAMI developers.

# SEE ALSO

*magellan*(1)

; Vim modeline settings
; vim: set tw=80 noet sts=4 ts=4 sw=4 syntax=scdoc: