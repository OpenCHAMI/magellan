MAGELLAN-COLLECT(1) "OpenCHAMI" "Manual Page for magellan-collect"

# NAME

magellan-collect - Retrieve Redfish data from scanned BMC nodes

# SYNOPSIS

magellan collect [OPTIONS]++
magellan collect pdu [OPTIONS]++

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
magellan collect pdu x3000m0 --username admin --password initial0

// Collect from multiple PDUs and send to SMD
magellan collect pdu x3000m0 x3000m1 -u admin -p initial0 | magellan send https://smd.example.com

# FLAGS

*-m, --bmc-id-map* (_data_ | @_path_)
    Set the BMC ID mapping from raw JSON data or use @<path> to specify a file
    path. The specified file can either be JSON or YAML and is determined by the
    file extension.

    An example of a valid BMC ID map would look like the following specified in
    YAML format:

    ```
    # map.yaml
    map_key: bmc-ip-addr
    id_map:
        172.21.0.1: x0c0s1b0
        172.21.0.2: x0c0s2b0
        ...
    ```

*--cacert* _path_
    Set the path to a certificate file. This certificate is NOT included in requests
    made to BMC nodes. When this flag is not provided, the default system
    certificates are used instead.

    **DEPRECATED**

*--force-update*
    Set this flag to force updating the *RedfishEndpoint*s, *Component*s, and
    *ComponentEndpoint*s in SMD. This is done by making seperate requests to
    remove and then re-create the objects whenever a 409 is received from the
    initial response.

*-F, --format* _format_
    Set the data format used for the collection output. This value is overridden
    whenever a file is specified with *--output-file* with a file extension

    Supported values _format_ are:

        - _json (default)
        - _yaml_

*-O, --output-dir* _path_
    Set the path to store collection data using the HIVE partitioning strategy.

    For example, after running *collect* and specifying the *--output-dir* flag
    with a value of *nodes* for _path_, we get output with the following structure:

    ```
    nodes
    ├── x1000c1s7b0
    │   └── 1747550498.yaml
    └── x1000c1s7b1
        └── 1747550498.yaml
    ```

    Notice that the output is separated by host and then timestamp. If the command
    is ran again in the same environment, we should get the same structure but
    with more files per node (assuming no nodes are added or removed).

    See https://duckdb.org/docs/stable/data/partitioning/hive_partitioning.html#hive-partitioning
    for more information on the HIVE partitioning strategy.

*-o, --output-file* _path_
    Set the path to store collection data in a single file. This will take the
    output that is normally printed to standard output and write it to a file as
    one of the specified formats with *--format*.

    Additionally, this file can be specified as input for the *send* command. See
    *magellan-send*(1) for details.

*-p, --password* string
    Set the master BMC password

*--protocol* _type_       
    Set the protocol used to make requests. The default value for _type_ is "tcp".

*--secrets-file* _path_
    Set path to a secrets file. 
    
    Requires the MASTER_KEY environment variable to be set. This can be set by
    generating a new key with the *magellan secrets generatekey* command.
    
    Credentials from the 
    secrets file can only be accessed using the same key initially used to store 
    the credential.

    See *magellan-secrets*(1) for more details.

*--show*
    Show the output of a collect run

*-u, --username* _value_       
    Set the master BMC username


See *magellan*(1) for information about global flags used for all commands.

# COMMANDS

These are the subcommands for *collect*:

*pdus* _host_,... [-u _username_] [-p _password]
    Retrieve PDU-related data using JAWS instead of Redfish service. The _host_
    argument expects a list of valid URI strings. A request is made to each of
    the _host_ provided similar to the base *collect* command and returns a list
    of dictionaries.

    *-p, --password* _password_
        Set the password to _password_ used for basic authentication to the PDU node.

    *-u, --username* _username_
        Set the username to _username_ used for basic authentication to the PDU node.

# AUTHOR

Written by David J. Allen and maintained by the OpenCHAMI developers.

# SEE ALSO

*magellan*(1)

; Vim modeline settings
; vim: set tw=80 noet sts=4 ts=4 sw=4 syntax=scdoc:
