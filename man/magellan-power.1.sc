MAGELLAN-POWER(1) "OpenCHAMI" "Manual Page for magellan-power"

# NAME

magellan-power - Power a node using Redfish actions

# SYNOPSIS

magellan power <node-{serial-number|ipaddr|uuid|mac}> [OPTIONS]

# EXAMPLES

// show all actions given the node's serial number
magellan power N0D3S3R14L --list-reset-types -i --inventory-file nodes.json

// turn off node using its IP address (no inventory file required)
magellan power 172.16.0.105 -o off -i

// turn on/off a node with it's UUID (will error if attempting to turn on/off if already in state)
magellan power 6dd96f87-48e1-4b8e-9f00-f1f0015f3ef5 -o on --inventory-file nodes.json -i
magellan power 6dd96f87-48e1-4b8e-9f00-f1f0015f3ef5 -o off --inventory-file nodes.json -i

# FLAGS

*--cacert* _path_
	Set the path to a certificate file. This certificate is NOT included in requests
	made to BMC nodes. When this flag is not provided, the default system
	certificates are used instead.

    *-i, --insecure*                   
	Skip TLS certificate verification for requests for power reset actions.

*-f, --inventory-file* _path_
    YAML file containing node inventory.

*-L, --list-reset-types*
    List supported Redfish reset types.

*-F, --output-format _format_
    Set the output format (json|yaml). (default json)

*-p, --password _value_
    Set the password to _value_ used for basic authentication to the BMC node.
	When this flag is set, the value overrides all of the values loaded from the
	secrets file.

*-r, --reset-type _type_
    Set the Redfish reset type to perform. The supported values can be found via
    the '--list-reset-types'. 

*--secrets-file _path_
    Set path to a secrets file.

	Requires the *MASTER_KEY* environment variable to be set. This can be set by
	generating a new key with the *magellan secrets generatekey* command.

	Credentials from the secrets file can only be accessed using the same key
	initially used to store the credential.

	See *magellan-secrets*(1) for more details.

*-u, --username _value_
    Set the username to _value_ used for basic authentication to the BMC node.
	When this flag is set, the value overrides all of the values loaded from the
	secrets file.


See *magellan*(1) for information about global flags and environment variables
used for all commands.

# AUTHOR

Written by David J. Allen and maintained by the OpenCHAMI developers.

# SEE ALSO

*magellan*(1), *magellan-crawl*(1), *magellan-list*(1), *magellan-secrets*(1)

; Vim modeline settings
; vim: set tw=80 noet sts=4 ts=4 sw=4 syntax=scdoc: