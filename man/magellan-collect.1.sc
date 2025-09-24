MAGELLAN-COLLECT(1) "OpenCHAMI" "Manual Page for magellan-collect"

# NAME

magellan-collect - Retrieve Redfish data from scanned BMC nodes

# SYNOPSIS

magellan collect [OPTIONS]++
magellan collect pdu [OPTIONS]++

# FLAGS

See *magellan*(1) for information about global flags used for all commands.

# EXAMPLES

// basic collect after scan without making a follow-up request
magellan collect --cache ./assets.db --cacert ochami.pem -o nodes.yaml -t 30

// set username and password for all nodes and produce the collected
// data in a file called 'nodes.yaml'
magellan collect -u $bmc_username -p $bmc_password -o nodes.yaml

// run a collect using secrets from the secrets manager
export MASTER_KEY=$(magellan secrets generatekey)
magellan secrets store $node_creds_json -f nodes.json
magellan collect -o nodes.yaml

// Collect inventory from a single PDU using credentials
magellan collect pdu x3000m0 --username admin --password inital0

// Collect from multiple PDUs and send to SMD
magellan collect pdu x3000m0 x3000m1 -u admin -p initial0 | ./magellan send <smd-endpoint>

# COMMANDS

## pdus

Retrieve PDU-related data through JAWS API instead of Redfish.

# AUTHOR

Written by David J. Allen and maintained by the OpenCHAMI developers.

# SEE ALSO

*magellan*(1)

; Vim modeline settings
; vim: set tw=80 noet sts=4 ts=4 sw=4 syntax=scdoc: