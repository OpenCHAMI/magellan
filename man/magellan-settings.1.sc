MAGELLAN-SETTINGS(1) "OpenCHAMI" "Manual Page for magellan-settings"

# NAME

magellan-settings - Configure BMC settings through Redfish

# SYNOPSIS

magellan settings [OPTIONS]++
magellan settings list <node> [<category> [<item> [<property>...]]] [OPTIONS]++
magellan settings get <node> <category> [item] [property...] [OPTIONS]++
magellan settings set <node> <category> <property> <value> [OPTIONS]++
magellan settings reset <node> [OPTIONS]

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

*list* <node> [<category> [<item> [<property>...]]]
:	Query the BMC and list the setting categories, items, and properties
	that are actually present. When called with just a node, lists the
	setting categories present on that BMC. When called with a category,
	lists the items under that category. When called with a category and
	item, lists the properties of that item. Additional property arguments
	walk deeper into nested structures with no upper bound on depth.

*get* <node> <category> [item] [property...]
:	Get a BMC setting value by category and property path. The path is
	walked into nested properties as deeply as the BMC schema allows. For
	*NetworkProtocol*, the first item is the protocol name (e.g. SSH, HTTPS,
	IPMI). For *EthernetInterface*, the first item is the interface index
	(0, 1, ...). For *ComputerSystem* and *Manager*, the first item is a
	property name on the first resource exposed by the BMC. For *Accounts*,
	the first item is the account ID. Any additional items walk deeper into
	nested properties.

*set* <node> <category> <property> <value>
:	Set a BMC setting value. The *value* should be a JSON string for
	complex types or a simple string for scalar values.

*reset* <node>
:	Factory reset the BMC manager. By default, resets all settings.
	Use *--preserve-config* to preserve specific settings during the
	reset. If the BMC does not support resetting to defaults via the
	Manager.ResetToDefaults action, or does not support the requested
	preserve type, an error is reported before any reset is attempted.

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

List the setting categories present on a BMC:
```
magellan settings list 172.16.0.105
```

List the network protocols present on a BMC:
```
magellan settings list 172.16.0.105 NetworkProtocol
```

List the properties of the SSH protocol:
```
magellan settings list 172.16.0.105 NetworkProtocol SSH
```

Walk deeper into a nested structure:
```
magellan settings list 172.16.0.105 ComputerSystem Node0 Boot
```

Get SSH protocol settings from a BMC:
```
magellan settings get 172.16.0.105 NetworkProtocol SSH
```

Get a nested computer system property:
```
magellan settings get 172.16.0.105 ComputerSystem Boot BootOrder
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
