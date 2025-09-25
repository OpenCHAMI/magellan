MAGELLAN-SEND(1) "OpenCHAMI" "Manual Page for magellan-send"

# NAME

magellan-send - Send node data from collect output to remote host

# SYNOPSIS

magellan send [OPTIONS] _host_

# EXAMPLES

  // minimal working example
  magellan send -d @inventory.json https://smd.openchami.cluster

  // send data from multiple files (must specify -f/--format if not JSON)
  magellan send -d @cluster-1.json -d @cluster-2.json https://smd.openchami.cluster
  magellan send -d '{...}' -d @cluster-1.json https://proxy.example.com

  // send data to remote host by piping output of collect directly
  magellan collect -v -F yaml | magellan send -d @inventory.yaml -F yaml https://smd.openchami.cluster

# FLAGS

*--cacert* string
    Set the path to CA cert file (defaults to system CAs when blank)

*-d, --data* -F _format_ (_node_object_,... | @_path_)
    Specify node data objects to send to specified host. Objects can be loaded
    from files using the '@' symbol followed by the path to the file. The input
    format for (json, yaml)he objects can be specified to be either JSON or YAML by setting
    the *--format* flag.

    An example of a node data object would look like the following using the
    JSON format:

    ```
    
    ```

*-f, --force-update*
    Set flag to force update data sent to SMD

*-F, --format* _format_
    Set the default data input format (json|yaml) can be overridden by file extension (default json)

See *magellan*(1) for information about global flags used for all commands.

# AUTHOR

Written by David J. Allen and maintained by the OpenCHAMI developers.

# SEE ALSO

*magellan*(1)

; Vim modeline settings
; vim: set tw=80 noet sts=4 ts=4 sw=4 syntax=scdoc:
