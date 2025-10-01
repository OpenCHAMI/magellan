MAGELLAN-SCAN(1) "OpenCHAMI" "Manual Page for magellan-scan"

# NAME

magellan-scan - Scan network for BMCs or PDUs and save data to cache

# SYNOPSIS

magellan scan [OPTIONS] _host_...

# EXAMPLES

// assumes host https://10.0.0.101:443++
magellan scan 10.0.0.101

// assumes subnet using HTTPS and port 443 except for specified host++
magellan scan http://10.0.0.101:80 https://$user:$password@10.0.0.102:443 http://172.16.0.105:8080 --subnet 172.16.0.0/24

// assumes hosts http://10.0.0.101:8080 and http://10.0.0.102:8080++
magellan scan 10.0.0.101 10.0.0.102 https://172.16.0.10:443 --port 8080 --protocol tcp

// assumes subnet using default unspecified subnet-masks++
magellan scan --subnet 10.0.0.0

// assumes subnet using HTTPS and port 443 with specified CIDR++
magellan scan --subnet 10.0.0.0/16

// assumes subnet using HTTP and port 5000 similar to 192.168.0.0/16++
magellan scan --subnet 192.168.0.0 --protocol tcp --scheme https --port 5000 --subnet-mask 255.255.0.0

// assumes subnet without CIDR has a subnet-mask of 255.255.0.0++
magellan scan --subnet 10.0.0.0 --subnet 172.16.0.0 --subnet-mask 255.255.0.0 --cache ./assets.db

# FLAGS

*--disable-cache*
	Disable saving found remote assets that respond to a Redfish request to a
	cache database specified with *--cache* flag. By default, the cache is saved
	at */tmp/$USER/magellan/assets.db* as a SQLite3 file with a table named
	*magellan_scanned_assets*. It is set to _false_ by default.

	See the *--cache* flag in *magellan*(1) for more details.

*--disable-probing*
	Disable making the probing request after finding assets on the specified
	networks. The purpose of this probing request is to determine which remote
	assets having an accessible Redfish service on the BMC node(s).

*-F, --format* _format_
	Sets the output format to print the found assets in either JSON or YAML.
	By default, the value of _format_ is empty and therefore no output is printed
	from the scan.

	Possible format values:

	- _json_
	- _yaml_

*--include* _type_...
	Set which asset types to include in the scan. BMC nodes are detected using
	Redfish where as PDU nodes are found using JAWS. Multiple values can be set
	for a single scan (e.g. *--include=bmcs,pdus*).

	Possible _type_ values:

	- _bmcs_ (default)
	- _pdus_

	For more information related to the JAWS API, see the following:

	- https://cdn10.servertech.com/assets/documents/documents/968/original/JSON_API_Web_Service_%28JAWS%29_V1.06.pdf?1641867726

*--insecure*
	Skip TLS certificate verification with the BMC when performing probing
	requests. After finding remote assets on a network, a subsequent request is
	made to determine which assets run a Redfish service. The default value for
	this flag is *false*.

	When *--insecure* is set to *false*, the BMC expects a CA certificate to be
	supplied in the request. Currently, *magellan* does not support including
	certificates in requests to BMCs, but may support this in a future version.

	It is recommended that the *--insecure* flag be set to *true* in when the BMC
	does not require TLS verification for HTTPS requests.
	

*-o, --output* _path_
	Output file path (for json/yaml formats)

	See *--format* for possible output formats.

*--port* _value_,...
	Add additional ports to scan per host with unspecified ports.

	For example, if we specify a host with a port, a host without a port, and
	a subnet, then the host without a port and all hosts included in the subnet
	will have ports set to _value_ scanned.

	```
	magellan scan http://10.0.0.105:5000 http://10.0.0.105 --subnet 172.10.0.0/24 --ports 5050
	```

	In the example above, the *http://10.0.0.105* host as well as the hosts from
	the *172.10.0.0/24* subnet will have port 5050 scanned for BMC nodes.

*--protocol* _type_
	Set the protocol to use in scan.

	Protocol types available are:

	- _tcp_ (default)
	- _udp_

*--scheme* _scheme_
	Set the default scheme to use if not specified in host URI. The default
	value of _scheme_ is *https*.

*--subnet* _cidr_...
	Specify subnets to include in scan. By default, *magellan* will only scan
	port 443 unless the *--port* flag is specified. When the flag is set, it will
	only use the ports provided without the default 443 port.

	Subnets can be specified either as an IP address or in CIDR notation. When
	using only an IP address the *--subnet-mask* flag must be supplied in con-
	junction with the *--subnet* flag.

	```
	magellan scan --subnet 172.16.0.0/24
	magellan scan --subnet 172.16.0.0 --subnet-mask 255.255.255.0
	```

	Both examples above are equivalent, but using CIDR notation only sets the
	subnet mask for a single subnet, whereas the other example sets it for all
	other included subnets with only an IP address.

	See the *--subnet-mask* flag documentation for more details.

*--subnet-mask* _ip_mask_
	Set the default subnet mask to use for with all subnets specified with the
	*--subnet* flag that does not use CIDR notation. If no subnet mask is specified,
	the default is *255.255.255.0*.

See *magellan*(1) for information about global flags used for all commands.

# AUTHOR

Written by David J. Allen and maintained by the OpenCHAMI developers.

# SEE ALSO

*magellan*(1)

; Vim modeline settings
; vim: set tw=80 noet sts=4 ts=4 sw=4 syntax=scdoc:
