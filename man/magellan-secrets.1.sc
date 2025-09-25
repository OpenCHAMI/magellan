MAGELLAN-SECRETS "OpenCHAMI" "Manual Page for magellan-secrets"

# NAME

magellan-secrets - Manage BMC credentials

# SYNOPSIS

magellan secrets generatekey
magellan secrets list
magellan secrets remove _secret_id_...
magellan secrets retrieve _secret_id_
magellan secrets store

# EXAMPLES

  // generate new key and set environment variable
  export MASTER_KEY=$(magellan secrets generatekey)

  // store specific BMC node creds for collect and crawl in default secrets store (--file/-f flag not set)
  magellan secrets store $bmc_host $bmc_creds

  // retrieve creds from secrets store
  magellan secrets retrieve $bmc_host -f nodes.json

  // list creds from specific secrets
  magellan secrets list -f nodes.json

# FLAGS

See *magellan*(1) for information about global flags used for all commands.

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

*store* [-f _path_] _secret_id_ _data_
    *-F, --format* _format_       
        Set the input data format to store secrets in the secrets file.

    *-i, --input-file* _string_
        Set the file to read as input.


Stores the given string value under secretID.

# AUTHOR

Written by David J. Allen and maintained by the OpenCHAMI developers.

# SEE ALSO

*magellan*(1), *magellan-collect*(1), *magellan-crawl*(1)

; Vim modeline settings
; vim: set tw=80 noet sts=4 ts=4 sw=4 syntax=scdoc: