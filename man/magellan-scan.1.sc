MAGELLAN-SCAN(1) "OpenCHAMI" "Manual Page for magellan-scan"

# NAME

magellan-scan - Scan network for Redfish devices

# SYNOPSIS

magellan scan [OPTIONS] _host_...++

# FLAGS

*--disable-cache*
    Disable saving found assets to a cache database specified with 'cache' flag

*--disable-probing*
    Disable probing found assets for Redfish service(s) running on BMC nodes

*-F, --format* _DataFormat_
    Output format (json, yaml)

*-h, --help*                 
    help for scan

*--include* _type_...
    Asset types to scan for (bmcs, pdus) (default [bmcs])

*--insecure*
    Skip TLS certificate verification during probe (default true)

*-o, --output* _string_
    Output file path (for json/yaml formats)

*--port* _ints_            
    Adds additional ports to scan for each host with unspecified ports.

*--protocol* _string_
    Set the default protocol to use in scan. (default "tcp")

*--scheme* _string_
    Set the default scheme to use if not specified in host URI. (default "https")

*--subnet* _strings_...
    Add additional hosts from specified subnets to scan.

*--subnet-mask* _ipMask_
    Set the default subnet mask to use for with all subnets not using CIDR notation. (default ffffff00)

See *magellan*(1) for information about global flags used for all commands.

# AUTHOR

Written by David J. Allen and maintained by the OpenCHAMI developers.

# SEE ALSO

*magellan*(1)

; Vim modeline settings
; vim: set tw=80 noet sts=4 ts=4 sw=4 syntax=scdoc: