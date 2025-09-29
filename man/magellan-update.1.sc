MAGELLAN-UPDATE(1) "OpenCHAMI" "Manual Page for magellan-update"

# NAME

magellan-update - Update firmware using the Redfish API

# SYNOPSIS

magellan update [OPTIONS] _host_...++

# EXAMPLES

// perform an firmware update
magellan update 172.16.0.108:443 -i -u $bmc_username -p $bmc_password \
    --firmware-uri http://172.16.0.200:8005/firmware/bios/image.RBU

// check update status
magellan update 172.16.0.108:443 -i -u $bmc_username -p $bmc_password --status

# FLAGS

*--firmware-uri* _uri_
    Set the URI to retrieve the firmware binary or executable. A download request
    is made using the protocol set with the *--scheme* flag.

*-i, --insecure*
    Skip TLS verification when making HTTP requests. This allows making requests
    to HTTPS hosts without needing to supply a CA certificate.

*-p, --password* _value_
    Set the password for basic authentication for requests to the BMC node.

*--scheme* _scheme_
    Specify the transfer protocol scheme to use for the request to the remote
    host. Values are case-insensitive and converted to use upper-case letters.
    Additionally, the default value for _scheme_ is HTTPS.

*--status*
    Return the status of active update jobs.

*-u, --username* _value_
    Set the username for basic authentication for requests to the BMC node.

See *magellan*(1) for information about global flags used for all commands.

# AUTHOR

Written by David J. Allen and maintained by the OpenCHAMI developers.

# SEE ALSO

*magellan*(1)

; Vim modeline settings
; vim: set tw=80 noet sts=4 ts=4 sw=4 syntax=scdoc: