MAGELLAN-SEND(1) "OpenCHAMI" "Manual Page for magellan-send"

# NAME

magellan-send - Send node data from collect output to remote host

# SYNOPSIS

magellan send [OPTIONS] _host_

# EXAMPLES

// minimal working example++
magellan send -d @inventory.json https://smd.openchami.cluster

// send data from multiple files (must specify -f/--format if not JSON)++
magellan send -d @cluster-1.json -d @cluster-2.json https://smd.openchami.cluster++
magellan send -d '{...}' -d @cluster-1.json https://proxy.example.com

// send data to remote host by piping output of collect directly++
magellan collect -v -F yaml | magellan send -d @inventory.yaml -F yaml https://smd.openchami.cluster

# FLAGS

*--cacert* string
	Set the path to CA cert file (defaults to system CAs when blank)

*-d, --data* -F _format_ (_node_object_,... | @_path_)
	Specify node data objects to send to specified host. Objects can be loaded
	from files using the '@' symbol followed by the path to the file. The input
	format for the objects can be specified to be either JSON or YAML by setting
	the *--format* flag.

	An example of a node data object would look like the following using the
	JSON format:

	```
	{
		"FQDN": "172.16.0.104",
		"ID": "x1000c1s7b3",
		"MACAddr": "...",
		"MACRequired": true,
		"Managers": [
			{
			"uri": "https://172.16.0.104:443/redfish/v1/Managers/1",
			"uuid": "...",
			"name": "Manager",
			"model": "iLO 5",
			"type": "BMC",
			"firmware_version": "iLO 5 v3.02",
			"ethernet_interfaces": [
				{
				"uri": "https://172.16.0.104:443/redfish/v1/Managers/1/EthernetInterfaces/1/",
				"mac": "...",
				"ip": "172.16.0.104",
				"name": "Manager Dedicated Network Interface",
				"description": "Configuration of this Manager Network Interface",
				"enabled": true
				},
				{
				"uri": "https://172.16.0.104:443/redfish/v1/Managers/1/EthernetInterfaces/2/",
				"mac": "...",
				"ip": "0.0.0.0",
				"name": "Manager Shared Network Interface",
				"description": "Configuration of this Manager Network Interface"
				}
			]
			}
		],
		"Name": "",
		"RediscoverOnUpdate": false,
		"SchemaVersion": 1,
		"Systems": [
			{
			"uri": "https://172.16.0.104:443/redfish/v1/Systems/1",
			"uuid": "...",
			"manufacturer": "HPE",
			"system_type": "Physical",
			"name": "Computer System",
			"model": "ProLiant DL325 Gen10 Plus v2",
			"serial": "...",
			"bios_version": "A43 v2.90 (10/27/2023)",
			"ethernet_interfaces": [
				{
				"uri": "https://172.16.0.104:443/redfish/v1/Systems/1/EthernetInterfaces/DE07A001/",
				"mac": "..."
				},
				{
				"uri": "https://172.16.0.104:443/redfish/v1/Systems/1/EthernetInterfaces/DE07A000/",
				"mac": "..."
				}
			],
			"network_interfaces": [
				{
				"uri": "https://172.16.0.104:443/redfish/v1/Systems/1/NetworkInterfaces/DE07A000",
				"name": "NetworkInterface",
				"adapter": {
					"uri": "https://172.16.0.104:443/redfish/v1/Chassis/1/NetworkAdapters/DE07A000",
					"name": "Marvell FastLinQ 41000 Series - 2P 25GbE SFP28 QL41232HQCU-HC OCP3 Adapter",
					"serial": "..."
				}
				}
			],
			"actions": [
				"On",
				"ForceOff",
				"GracefulShutdown",
				"ForceRestart",
				"Nmi",
				"PushPowerButton",
				"GracefulRestart"
			],
			"power": {
				"state": "On",
				"restore_policy": ""
			},
			"processor_count": 1,
			"processor_type": "AMD EPYC 7713P 64-Core Processor               ",
			"memory_total": 256,
			"trusted_modules": [
				"TPM2_0 73.64"
			],
			"chassis_sku": "R9K56A",
			"chassis_serial": "...",
			"chassis_manufacturer": "HPE",
			"chassis_model": "ProLiant DL325 Gen10 Plus v2",
			"links": {
				"chassis": [
				"/redfish/v1/Chassis/1/"
				],
				"managers": [
				"/redfish/v1/Managers/1/"
				]
			}
			},
			{
			"uri": "https://172.16.0.104:443/redfish/v1/Systems/1",
			"uuid": "...",
			"manufacturer": "HPE",
			"system_type": "Physical",
			"name": "Computer System",
			"model": "ProLiant DL325 Gen10 Plus v2",
			"serial": "...",
			"bios_version": "A43 v2.90 (10/27/2023)",
			"ethernet_interfaces": [
				{
				"uri": "https://172.16.0.104:443/redfish/v1/Systems/1/EthernetInterfaces/DE07A001/",
				"mac": "..."
				},
				{
				"uri": "https://172.16.0.104:443/redfish/v1/Systems/1/EthernetInterfaces/DE07A000/",
				"mac": "..."
				}
			],
			"network_interfaces": [
				{
				"uri": "https://172.16.0.104:443/redfish/v1/Systems/1/NetworkInterfaces/DE07A000",
				"name": "NetworkInterface",
				"adapter": {
					"uri": "https://172.16.0.104:443/redfish/v1/Chassis/1/NetworkAdapters/DE07A000",
					"name": "Marvell FastLinQ 41000 Series - 2P 25GbE SFP28 QL41232HQCU-HC OCP3 Adapter",
					"serial": "..."
				}
				}
			],
			"actions": [
				"On",
				"ForceOff",
				"GracefulShutdown",
				"ForceRestart",
				"Nmi",
				"PushPowerButton",
				"GracefulRestart"
			],
			"power": {
				"state": "On",
				"restore_policy": ""
			},
			"processor_count": 1,
			"processor_type": "AMD EPYC 7713P 64-Core Processor               ",
			"memory_total": 256,
			"trusted_modules": [
				"TPM2_0 73.64"
			],
			"links": {
				"managers": [
				"/redfish/v1/Managers/1/"
				]
			}
			}
		],
		"Type": "",
		"User": "Administrator"
	}
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
