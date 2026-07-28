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
	// RESPONSE_System_Node0 is a single ComputerSystem detail document for the
	// Node0 member of RESPONSE_Systems. Unlike RESPONSE_EthernetInterface it
	// advertises a concrete set of allowable reset types, so tests that exercise
	// GetResetTypes/GetPowerState/Reset have a deterministic system to assert on.
	RESPONSE_System_Node0 = `{
    "@odata.id": "/redfish/v1/Systems/Node0",
    "@odata.type": "#ComputerSystem.v1_5_0.ComputerSystem",
    "Id": "Node0",
    "Name": "Node0",
    "PowerState": "On",
    "Actions": {
        "#ComputerSystem.Reset": {
            "ResetType@Redfish.AllowableValues": [
                "On",
                "ForceOff",
                "GracefulShutdown",
                "ForceRestart"
            ],
            "target": "/redfish/v1/Systems/Node0/Actions/ComputerSystem.Reset"
        }
    }
}`
	// RESPONSE_ServiceRoot_HPE is a minimal ServiceRoot that identifies its
	// manufacturer via the Redfish "Vendor" property. It lets tests drive the
	// vendor-detection/dispatch path (e.g. asserting the HPE plugin is selected)
	// without a full emulator.
	RESPONSE_ServiceRoot_HPE = `{
    "@odata.id": "/redfish/v1/",
    "@odata.type": "#ServiceRoot.v1_2_0.ServiceRoot",
    "Id": "RootService",
    "Name": "Root Service",
    "RedfishVersion": "1.2.0",
    "Vendor": "HPE",
    "Managers": {
        "@odata.id": "/redfish/v1/Managers"
    },
    "Systems": {
        "@odata.id": "/redfish/v1/Systems"
    }
}`
)
