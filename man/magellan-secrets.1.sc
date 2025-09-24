MAGELLAN-SECRETS "OpenCHAMI" "Manual Page for magellan-secrets"

# NAME

magellan-secrets - Manage BMC credentials

# SYNOPSIS

magellan secrets generatekey
magellan secrets list
magellan secrets remove _secret_id_...
magellan secrets retrieve _secret_id_
magellan secrets store

# COMMANDS

Manage, list, retrieve, remove, and store BMC credentials. All 

## generatekey

Generates a new 32-byte master key (in hex).

*list*

Lists all the secret IDs and their values.

*remove* _secret_id_

Remove secrets by IDs from secret store.

*retrieve* _secret_id_

Retrieve secret by ID from secret store.

*store* [-f _format_] _secret_id_ _data_
    *-F, --format* _string_       
        Format the input to store secrets in the secrets file.

        Supported values are:

        - _basic_ (default)
        - _json_
        - _base64_

    *-i, --input-file* _string_
        Set the file to read as input.


Stores the given string value under secretID.

# AUTHOR

Written by David J. Allen and maintained by the OpenCHAMI developers.

# SEE ALSO

*magellan*(1)

; Vim modeline settings
; vim: set tw=80 noet sts=4 ts=4 sw=4 syntax=scdoc: