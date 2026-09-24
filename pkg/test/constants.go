package test

const (
	RESPONSE_ServiceRoot = `{
		"@odata.etag": "W/\"1646860561\"",
		"@odata.id": "/redfish/v1/",
		"@odata.type": "#ServiceRoot.v1_2_0.ServiceRoot",
		"AccountService": {
			"@odata.id": "/redfish/v1/AccountService"
		},
		"CertificateService": {
			"@odata.id": "/redfish/v1/CertificateService"
		},
		"Chassis": {
			"@odata.id": "/redfish/v1/Chassis"
		},
		"Description": "The service root for all Redfish requests on this host",
		"EventService": {
			"@odata.id": "/redfish/v1/EventService"
		},
		"Id": "RootService",
		"JsonSchemas": {
			"@odata.id": "/redfish/v1/JsonSchemas"
		},
		"Links": {
			"Sessions": {
				"@odata.id": "/redfish/v1/SessionService/Sessions"
			}
		},
		"Managers": {
			"@odata.id": "/redfish/v1/Managers"
		},
		"Name": "Root Service",
		"RedfishVersion": "1.2.0",
		"Registries": {
			"@odata.id": "/redfish/v1/Registries"
		},
		"SessionService": {
			"@odata.id": "/redfish/v1/SessionService"
		},
		"Systems": {
			"@odata.id": "/redfish/v1/Systems"
		},
		"Tasks": {
			"@odata.id": "/redfish/v1/TaskService"
		},
		"UpdateService": {
			"@odata.id": "/redfish/v1/UpdateService"
		}
	}`
	RESPONSE_EthernetInterface = `{
    "@odata.etag": "W/\"1646792654\"",
    "@odata.id": "/redfish/v1/Systems/Node0",
    "@odata.type": "#ComputerSystem.v1_5_0.ComputerSystem",
    "Actions": {
        "#ComputerSystem.Reset": {
            "@Redfish.ActionInfo": "/redfish/v1/Systems/Node0/ResetActionInfo",
            "target": "/redfish/v1/Systems/Node0/Actions/ComputerSystem.Reset"
        },
        "#ComputerSystem.SetDefaultBootOrder": {
            "@Redfish.ActionInfo": "/redfish/v1/Systems/Node0/SetDefaultBootOrderActionInfo",
            "target": "/redfish/v1/Systems/Node0/Actions/ComputerSystem.SetDefaultBootOrder"
        }
    },
    "Bios": {
        "@odata.id": "/redfish/v1/Systems/Node0/Bios"
    },
    "BiosVersion": "ex235a.bios-1.3.6",
    "Boot": {
        "BootOptions": {
            "@odata.id": "/redfish/v1/Systems/Node0/BootOptions"
        },
        "BootOrder": [
            "ME0-PXE-IP4",
            "ME0-PXE-IP6",
            "HSN0-PXE-IP4",
            "HSN0-PXE-IP6",
            "HSN1-PXE-IP4",
            "HSN1-PXE-IP6",
            "HSN2-PXE-IP4",
            "HSN3-PXE-IP4",
            "HSN2-PXE-IP6",
            "HSN3-PXE-IP6",
            "Boot000B"
        ]
    },
    "Description": "BardPeakNC",
    "EthernetInterfaces": {
        "@odata.id": "/redfish/v1/Systems/Node0/EthernetInterfaces"
    },
    "Id": "Node0",
    "Manufacturer": "HPE",
    "Memory": {
        "@odata.id": "/redfish/v1/Systems/Node0/Memory"
    },
    "MemorySummary": {
        "TotalSystemMemoryGiB": 512
    },
    "Model": "HPE CRAY EX235a",
    "Name": "Node0",
    "PartNumber": "P37085-001.A",
    "PowerState": "On",
    "ProcessorSummary": {
        "Count": 9,
        "Model": "AMD INSTINCT MI200 (MCM) OAM LC"
    },
    "Processors": {
        "@odata.id": "/redfish/v1/Systems/Node0/Processors"
    },
    "SerialNumber": "GHU4464825942",
    "Status": {
        "Health": "OK",
        "State": "Enabled"
    },
    "SystemType": "Physical"
}`
	RESPONSE_Systems = `{
    "@odata.etag": "W/\"1646792654\"",
    "@odata.id": "/redfish/v1/Systems",
    "@odata.type": "#ComputerSystemCollection.ComputerSystemCollection",
    "Description": "Collection of Computer Systems",
    "Members": [
        {
            "@odata.id": "/redfish/v1/Systems/Node0"
        }
    ],
    "Members@odata.count": 1,
    "Name": "Systems Collection"
}`
	RESPONSE_Managers = `{
    "@odata.id": "/redfish/v1/Managers",
    "@odata.type": "#ManagerCollection.ManagerCollection",
    "Members": [
        {
            "@odata.id": "/redfish/v1/Managers/bmc"
        }
    ],
    "Members@odata.count": 1,
    "Name": "Manager Collection"
}`
	RESPONSE_Manager = `{
    "@odata.id": "/redfish/v1/Managers/bmc",
    "@odata.type": "#Manager.v1_11_0.Manager",
    "Id": "bmc",
    "Name": "Manager",
    "FirmwareVersion": "1.0.0",
    "ManagerType": "BMC",
    "Model": "Mock BMC",
    "SerialNumber": "MOCK0001",
    "EthernetInterfaces": {
        "@odata.id": "/redfish/v1/Managers/bmc/EthernetInterfaces"
    },
    "NetworkProtocol": {
        "@odata.id": "/redfish/v1/Managers/bmc/NetworkProtocol"
    },
    "Status": {
        "Health": "OK",
        "State": "Enabled"
    }
}`
	RESPONSE_ManagerNetworkProtocol = `{
    "@odata.id": "/redfish/v1/Managers/bmc/NetworkProtocol",
    "@odata.type": "#ManagerNetworkProtocol.v1_8_0.ManagerNetworkProtocol",
    "Id": "NetworkProtocol",
    "Name": "ManagerNetworkProtocol",
    "FQDN": "bmc.example.com",
    "HostName": "bmc",
    "HTTPS": {
        "Port": 443,
        "ProtocolEnabled": true
    },
    "IPMI": {
        "Port": 623,
        "ProtocolEnabled": true
    },
    "NTP": {
        "NTPServers": [
            "10.0.0.254"
        ],
        "Port": 123,
        "ProtocolEnabled": true
    },
    "SSH": {
        "Port": 22,
        "ProtocolEnabled": true
    },
    "Status": {
        "Health": "OK",
        "State": "Enabled"
    }
}`
	RESPONSE_EthernetInterfaceCollection = `{
    "@odata.id": "/redfish/v1/Managers/bmc/EthernetInterfaces",
    "@odata.type": "#EthernetInterfaceCollection.EthernetInterfaceCollection",
    "Members": [
        {
            "@odata.id": "/redfish/v1/Managers/bmc/EthernetInterfaces/1"
        }
    ],
    "Members@odata.count": 1,
    "Name": "Ethernet Interface Collection"
}`
	RESPONSE_ManagerEthernetInterface = `{
    "@odata.id": "/redfish/v1/Managers/bmc/EthernetInterfaces/1",
    "@odata.type": "#EthernetInterface.v1_5_0.EthernetInterface",
    "Id": "1",
    "Name": "Manager Ethernet Interface",
    "MACAddress": "02:00:00:00:00:01",
    "IPv4Addresses": [
        {
            "Address": "192.0.2.10",
            "Gateway": "192.0.2.1",
            "SubnetMask": "255.255.255.0"
        }
    ],
    "DHCPv4": {
        "DHCPEnabled": false
    },
    "Status": {
        "Health": "OK",
        "State": "Enabled"
    }
}`
	RESPONSE_AccountService = `{
    "@odata.id": "/redfish/v1/AccountService",
    "@odata.type": "#AccountService.v1_7_0.AccountService",
    "Id": "AccountService",
    "Name": "Account Service",
    "Accounts": {
        "@odata.id": "/redfish/v1/AccountService/Accounts"
    },
    "ServiceEnabled": true
}`
	RESPONSE_AccountCollection = `{
    "@odata.id": "/redfish/v1/AccountService/Accounts",
    "@odata.type": "#ManagerAccountCollection.ManagerAccountCollection",
    "Members": [
        {
            "@odata.id": "/redfish/v1/AccountService/Accounts/1"
        }
    ],
    "Members@odata.count": 1,
    "Name": "Accounts Collection"
}`
	RESPONSE_ManagerAccount = `{
    "@odata.id": "/redfish/v1/AccountService/Accounts/1",
    "@odata.type": "#ManagerAccount.v1_10_0.ManagerAccount",
    "Id": "1",
    "Name": "User Account",
    "UserName": "admin",
    "RoleId": "Administrator",
    "Enabled": true,
    "AccountTypes": [
        "Redfish"
    ],
    "Password": null
}`
)
