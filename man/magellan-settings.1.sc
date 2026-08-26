MAGELLAN-SETTINGS(1) "OpenCHAMI" "Manual Page for magellan-settings"

# NAME

magellan-settings - Configure BMC settings through Redfish

# SYNOPSIS

magellan settings [OPTIONS]++
magellan settings list [category] [OPTIONS]++
magellan settings get <node> <category> [property] [OPTIONS]++
magellan settings set <node> <category> <property> <value> [OPTIONS]++
magellan settings reset <node> [OPTIONS]++

# DESCRIPTION

Configure BMC properties through Redfish. The *settings* command provides read
and write access to BMC configuration across five categories:

- *NetworkProtocol*: Network service settings (SSH, HTTPS, IPMI, NTP, SNMP, etc.)
- *EthernetInterface*: Network interface settings (IP, MAC, DHCP, etc.)
- *ComputerSystem*: System-level settings (boot order, asset tag, etc.)
- *Manager*: Manager properties (firmware version, model, etc.)
- *Accounts*: BMC user accounts (username, role, etc.)

The *node* argument can be a direct BMC IP address/hostname, or a node
identifier (e.g. xname) from a previous *magellan-collect*(1) inventory when
used with the *--inventory-file* flag.

See *magellan*(1) for information about global flags and environment variables
used for all commands.

# COMMANDS

*list* [category]
:	When called without arguments, lists all available setting categories.
	When called with a category name, lists the properties available in
	that category.

*get* <node> <category> [property]
:	Get a BMC setting value. For *NetworkProtocol*, specify the protocol
	name (e.g. SSH, HTTPS, IPMI). For *EthernetInterface*, specify the
	interface index (0, 1, ...). For *ComputerSystem*, *Manager*, or
	*Accounts*, specify the property name or account ID. ComputerSystem and
	Manager operations use the first resource exposed by the BMC.

*set* <node> <category> <property> <value>
:	Set a BMC setting value. The *value* should be a JSON string for
	complex types or a simple string for scalar values.

*reset* <node>
:	Factory reset the BMC manager. By default, resets all settings.
	Use *--preserve-config* to preserve specific settings during the reset.

# FLAGS

See *magellan*(1) for information about global flags and environment variables
used for all commands.

*-F, --output-format* _format_
:	Set the output format for the *get* command (json, yaml).

*--input-format* _format_
:	Set the inventory input format (json, yaml). The default is yaml.

*--preserve-config* _level_
:	(Reset only) Preserve settings during reset. Valid values are
	*PreserveNetwork* and *PreserveNetworkAndUsers*. If not specified,
	all settings are reset.

*-f, --inventory-file* _path_
:	File containing node inventory from a previous *collect*.

*-u, --username* _username_
:	Set the master BMC username.

*-p, --password* _password_
:	Set the master BMC password.

*--secrets-file* _path_
:	Set the secrets file with BMC credentials.

*-i, --insecure*
:	Skip TLS certificate verification during probe.

*--cacert* _path_
:	Set the path to CA cert file (defaults to system CAs when blank).

# EXAMPLES

List all available setting categories:
```
magellan settings list
```

List properties in the NetworkProtocol category:
```
magellan settings list NetworkProtocol
```

Get SSH protocol settings from a BMC:
```
magellan settings get 172.16.0.105 NetworkProtocol SSH
```

Set SSH protocol settings:
```
magellan settings set 172.16.0.105 NetworkProtocol SSH \
    '{"ProtocolEnabled":true,"Port":22}'
```

Get the first ethernet interface:
```
magellan settings get 172.16.0.105 EthernetInterface 0
```

Update the asset tag on the first computer system:
```
magellan settings set 172.16.0.105 ComputerSystem AssetTag rack-12-node-4
```

Get the firmware version from the first manager:
```
magellan settings get 172.16.0.105 Manager FirmwareVersion
```

Get all BMC user accounts:
```
magellan settings get 172.16.0.105 Accounts
```

Factory reset the BMC manager:
```
magellan settings reset 172.16.0.105
```

Factory reset preserving network settings:
```
magellan settings reset 172.16.0.105 --preserve-config PreserveNetwork
```

Use inventory lookup for a node identifier:
```
magellan settings get x1000c0s0b0n0 NetworkProtocol SSH -f nodes.yaml
```

# AUTHOR

Written by David J. Allen and maintained by the OpenCHAMI developers.

# SEE ALSO

*magellan*(1), *magellan-collect*(1), *magellan-power*(1), *magellan-secrets*(1)

; Vim modeline settings
; vim: set tw=80 noet sts=4 ts=4 sw=4 syntax=scdoc:
