MAGELLAN-UPDATE(1) "OpenCHAMI" "Manual Page for magellan-update"

# NAME

magellan-update - Update firmware using the Redfish API

# SYNOPSIS

magellan update [OPTIONS] _host_...++

# EXAMPLES

// perform an firmware update
magellan update 172.16.0.108:443 -i -u $bmc_username -p $bmc_password \
    --firmware-url http://172.16.0.200:8005/firmware/bios/image.RBU \
    --component BIOS

// check update status
magellan update 172.16.0.108:443 -i -u $bmc_username -p $bmc_password --status


# FLAGS

--firmware-uri _string_   
    Set the URI to retrieve the firmware

-i, --insecure
    Allow insecure connections to the server

-p, --password string
    Set the BMC password

--scheme string
    Set the transfer protocol (default "https")

--status
    Get the status of the update

-u, --username string
    Set the BMC user

# AUTHOR

Written by David J. Allen and maintained by the OpenCHAMI developers.

# SEE ALSO

*magellan*(1)

; Vim modeline settings
; vim: set tw=80 noet sts=4 ts=4 sw=4 syntax=scdoc: