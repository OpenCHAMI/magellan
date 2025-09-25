MAGELLAN-CRAWL(1) "OpenCHAMI" "Manual Page for magellan-crawl"

# NAME

magellan-crawl - Retrieve Redfish data from a single BMC node

# SYNOPSIS

magellan crawl [OPTIONS] _host_++

# EXAMPLES

magellan crawl https://bmc.example.com
  magellan crawl https://bmc.example.com -i -u username -p password

# FLAGS

*-F, --format* _DataFormat_
    Set the output format (json|yaml) (default json)

*-i, --insecure*              
    Ignore SSL errors

*-p, --password* _string_
    Set the password for the BMC

*-f, --secrets-file* _string_
    Set path to the node secrets file (default "secrets.json")

*--show*
    Show the output of a collect run

*-u, --username* _string_
    Set the username for the BMC

See *magellan*(1) for information about global flags used for all commands.

# AUTHOR

Written by David J. Allen and maintained by the OpenCHAMI developers.

# SEE ALSO

*magellan*(1)

; Vim modeline settings
; vim: set tw=80 noet sts=4 ts=4 sw=4 syntax=scdoc: