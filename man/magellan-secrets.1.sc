MAGELLAN-SECRETS "OpenCHAMI" "Manual Page for magellan-secrets"

# NAME

magellan-secrets - Manage BMC credentials in a flat file

# SYNOPSIS

magellan secrets generatekey++
magellan secrets list [OPTIONS]++
magellan secrets remove [OPTIONS] _secret_id_...++
magellan secrets retrieve [OPTIONS] _secret_id_++
magellan secrets store [OPTIONS] _secret_id_ _data_

# EXAMPLES

// store specific BMC node creds for collect and crawl in default secrets store (--file/-f flag not set)
magellan secrets store $bmc_host $bmc_creds

// retrieve creds from secrets store
magellan secrets retrieve $bmc_host -f nodes.json

// list creds from specific secrets
magellan secrets list -f nodes.json

# FLAGS

*-f, --file* _path_
	Set path to a secrets file to manage secrets.

	Requires the MASTER_KEY environment variable to be set. This can be set by
	generating a new key with the *magellan secrets generatekey* command.

	Credentials from the secrets file can only be accessed using the same key
	initially used to store the credential.

See *magellan*(1) for information about global flags used for all commands.

# COMMANDS

Manage, list, retrieve, remove, and store BMC credentials. All

## generatekey

Generates a new 32-byte master key (in hex).

*list* [-f _path_]

Lists all the secret IDs and their values for secrets file specified with _path_.

*remove* [-f _path_] _secret_id_

Remove secrets by IDs from secrets file specified with _path_.

*retrieve* [-f _path_] _secret_id_

Retrieve secret by ID from secrets file specified with _path_.

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