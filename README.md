# OpenCHAMI Magellan

The `magellan` CLI tool is a Redfish-based, board management controller (BMC) discovery tool designed to scan networks and is written in Go. The tool collects information from BMC nodes using the provided Redfish RESTful API with [`gofish`](https://github.com/stmcginnis/gofish) and loads the queried data into an [SMD](https://github.com/OpenCHAMI/smd/) instance. The tool strives to be more flexible by implementing multiple methods of discovery to work for a wider range of systems (WIP) and is capable of being used independently of other tools or services.

> [!NOTE]
> The v0.1.0 version of `magellan` is incompatible with `smd` v2.15.3 and earlier due to `smd` lacking the inventory parsing code used with `magellan`'s output.**

<!-- TOC start (generated with https://github.com/derlin/bitdowntoc) -->

- [OpenCHAMI Magellan](#openchami-magellan)
  - [Main Features](#main-features)
  - [Getting Started](#getting-started)
  - [Building the Executable](#building-the-executable)
    - [Building on Debian 12 (Bookworm)](#building-on-debian-12-bookworm)
    - [Docker](#docker)
    - [Arch Linux (AUR)](#arch-linux-aur)
  - [Usage](#usage)
    - [Checking for Redfish](#checking-for-redfish)
    - [BMC ID Mapping](#bmc-id-mapping)
    - [Running the Tool](#running-the-tool)
    - [Managing Secrets](#managing-secrets)
    - [Starting the Emulator](#starting-the-emulator)
    - [Updating Firmware](#updating-firmware)
    - [Getting an Access Token (WIP)](#getting-an-access-token-wip)
    - [Running with Docker](#running-with-docker)
  - [How It Works](#how-it-works)
  - [TODO](#todo)
  - [Copyright](#copyright)

<!-- TOC end -->

<!-- TOC --><a name="openchami-magellan"></a>

## Main Features

The `magellan` tool comes packed with a handleful of features for doing discovery, such as:

- Simple network scanning
- Redfish-based inventory collection
- Redfish-based firmware updating
- Integration with OpenCHAMI SMD
- Write inventory data to JSON
- Store and manage BMC secrets

See the [TODO](#todo) section for a list of soon-ish goals planned.

## Getting Started

[Build](#building) and [run on bare metal](#running-the-tool) or run and test with Docker using the [latest prebuilt image](#running-with-docker). For quick testing, the repository integrates a Redfish emulator that can be run by executing the `emulator/setup.sh` script or running `make emulator`.

## Building the Executable

The `magellan` tool can be built to run on bare metal. Install the required Go tools, clone the repo, and then build the binary in the root directory with the following:

```bash
git clone https://github.com/OpenCHAMI/magellan
cd magellan
go mod tidy && go build
```

And that's it. The last line should find and download all of the required dependencies to build the project. Although other versions of Go may work, the project has been tested to work with versions v1.20 and later on MacOS and Linux.

### Building on Debian 12 (Bookworm)

Getting the `magellan` tool to work with Go 1.21 on Debian 12 may require installing the `golang-1.21` meta-package from `bookworm-backports` through `apt` along with GCC for comping the `go-sqlite3` driver.

```bash
apt install gcc golang-1.21/bookworm-backport
```

The binary executable for the `golang-1.21` executable can then be found using `dpkg`.v2.0.1

```bash
dpkg -L golang-1.21-go
```

Using the correct binary, set the `CGO_ENABLED` environment variable and build the executable with `cgo` enabled:

```bash
export GOBIN=/usr/bin/golang-1.21/bin/go
go env -w CGO_ENABLED=1
go mod tidy && go build
```

This might take some time to complete initially because of the `go-sqlite3` driver, but should be much faster for subsequent builds.

### Docker

The tool can also run using Docker. To build the Docker container, run `docker build -t magellan:testing .` in the project's directory. This is useful if you to run `magellan` on a different system through Docker desktop without having to install and build with Go (or if you can't do so for some reason). [Prebuilt images](https://github.com/OpenCHAMI/magellan/pkgs/container/magellan) are available as well on `ghcr`. Images can be pulled directly from the repository:

```bash
docker pull ghcr.io/openchami/magellan:latest
```

See the ["Running with Docker"](#running-with-docker) section below about running with the Docker container.


### Arch Linux (AUR)

The `magellan` tool is in the AUR as a binary package and can be installed via your favorite AUR helper.

```bash
yay -S magellan-bin
```
> [!NOTE]
> The AUR package may not always be in sync with the latest release. It is recommended to install `magellan` from source for the latest version.

## Usage

The sections below assume that the BMC nodes have an IP address available to query Redfish. Currently, `magellan` does not support discovery with MAC addresses although that may change in the future.

### Checking for Redfish

Before using the tool, confirm that the identified node has Redfish with `curl`. Assuming the IP address for the BMC node is `172.16.0.10`, we can send a request to see if it we get a response. You might need to pass the `-k` flag if the node uses TLS or point to the appropriate certificate.

```bash
curl -k https://172.16.0.10/redfish/v1 --cacert cacert.pem | jq
```

This should return a JSON response with general information. The output below has been truncated:

```json
{
  "@odata.context": "/redfish/v1/$metadata#ServiceRoot.ServiceRoot",
  "@odata.etag": "W/\"1715279084\"",
  "@odata.id": "/redfish/v1/",
  "@odata.type": "#ServiceRoot.v1_5_2.ServiceRoot",
  "AccountService": {
    "@odata.id": "/redfish/v1/AccountService"
  },
  "CertificateService": {
    "@odata.id": "/redfish/v1/CertificateService"
  },
  "Chassis": {
    "@odata.id": "/redfish/v1/Chassis"
  },
  ...
}
```

### BMC ID Mapping

While the `magellan collect` command collects data from RedFish servers and can produce node data suitable for use with SMD, the RedFish data has no defined way to provide BMC IDs that have the are "meaningful" with respect to location semantics that some consumers of SMD data require. SMD consumes BMC IDs in the form of XNAMEs, which are names that provide both unique identification within a cluster and the topographical location information needed to phyically identify a BMC. Since RedFish does not provide a way to communicate BMC XNAMEs or other meaningful BMC IDs, `magellan` provides a mechanism to generate meaningful BMC IDs based on an external mapping. The `--bmc-id-map` (`-m`) option to `magellan` provides this mapping either in the form of a command line JSON string or in the form of a JSON or YAML file (by prepending the path to the file with an `@` sign). The YAML form of the mapping data is as follows:

```yaml
map_key: bmc-ip-addr
id_map:
    172.21.0.1: x0c0s1b0
    172.21.0.2: x0c0s2b0
    ...
```

Where the `map_key` is the name of the attribute known to `magellan` that identifies the BMC (currently the only supported `map_key` is `bmc-ip-addr` which is the IPv4 address of the BMC) and the `id_map` section is a map between that attribute an the ID string to be passed to the consumer of the data. In the case of SMD that is an XNAME in the form:

```
x<cabinet>c<chassis>s<shelf>b<blade>
```


where `<cabinet>` is a cabinet number in the cluster, `<chassis>` is a chassis within the cabinet, `<shelf>` is the shelf within the chassis and `<blade>` is the blade within a shelf where the BMC is located. The above mapping file (minus the elipsis) will work with the example described in the [Starting the Emulator](#starting-the-emulator) section.

If you are using `magellan` within a system deployed using RIE in the [Quickstart Deployment Recipe](https://github.com/OpenCHAMI/deployment-recipes/blob/main/quickstart/README.md) you can generate a BMC ID Map with XNAMEs that match the RIE configured XNAMEs from the RIE instances running under `docker-compose`. You can do this outside of the docker containers by running this script:

```bash
#! /bin/sh
bmc_id_map() {
    echo "map_key: bmc-ip-addr"
    echo "id_map:"
    # IP Address to XNAME mappings for RIE containers
    docker ps --format json | jq -r '.Names | select(test("^rf-x"))' | while read container; do
        xname="$(docker inspect "${container}" \
           | jq -r '.[] | .NetworkSettings.Networks.quickstart_internal.Aliases[] | select(test("^x"))')"
        address="$(docker inspect "${container}" \
           | jq -r '.[] | .NetworkSettings.Networks.quickstart_internal.IPAddress')"
        echo "    ${address}: ${xname}"
    done
}
bmc_id_map
```

directing the output into a ID mapping file, then copying the ID mapping file into whatever container you are using to run `magellan` in your OpenCHAMI system an using it as follows on the magellan command line:

```bash
magellan collect --bmc-id-map @my_bmc_id_map.yaml -o nodes.yaml
```

If you have real BMCs present in your system in addition to those presented by RIE, and you want their XNAMEs to be meaningful, you will need to add each IP Address to XNAME mapping to the BMC ID Map. How you do that for any given configuration is beyond the scope of this README.

If you do not care about the meaning of XNAMEs produced by `collect` you can omit the `--bmc-id-map` (`-m`) option entirely and `collect` will generate XNAMEs algorithmically based on the IPv4 address of each BMC.

If you are using `magellan` in an application that is not OpenCHAMI and have a need for a different BMC ID mapping, you can construct a different kind of mapping file by replacing the XNAMEs with whatever your IDs should be.

> [!NOTE]
> If you do provide a BMC ID Map but some of the BMC map keys (IPv4 addresses) don't match anything in the map, those BMCs will be suppressed in the resulting data. This reflects the fact that `collect` does not know how to map those BMCs to meaningful IDs.

### Running the Tool

There are three main commands to use with the tool: `scan`, `list`, and `collect`. To see all of the available commands, run `magellan` with the `help` subcommand which will print this output:

```
Redfish-based BMC discovery tool

Usage:
  magellan [flags]
  magellan [command]

Available Commands:
  collect     Collect system information by interrogating BMC node
  completion  Generate the autocompletion script for the specified shell
  crawl       Crawl a single BMC for inventory information
  help        Help about any command
  list        List information stored in cache from a scan
  login       Log in with identity provider for access token
  scan        Scan to discover BMC nodes on a network
  update      Update BMC node firmware
  version     Print version info and exit

Flags:
      --access-token string   Set the access token
      --cache string          Set the scanning result cache path (default "/tmp/allend/magellan/assets.db")
      --concurrency int       Set the number of concurrent processes (default -1)
  -c, --config string         Set the config file path
  -d, --debug                 Set to enable/disable debug messages
  -h, --help                  help for magellan
      --timeout int           Set the timeout for requests (default 5)
  -v, --verbose               Set to enable/disable verbose output

Use "magellan [command] --help" for more information about a command.
```

To start a network scan for BMC nodes, use the `scan` command. If the port is not specified, `magellan` will probe the common Redfish port 443 by default:

```bash
./magellan scan \
    --subnet 172.16.0.0 \
    --subnet-mask 255.255.255.0 \
    --format json \
    --cache data/assets.db
```

This will scan the `172.16.0.0` subnet returning the host and port that return a response and store the results in a local cache with at the `data/assets.db` path. Additional flags can be set such as `--host` to add more hosts to scan that are not included on the subnet, `--timeout` to set how long to wait for a response from the BMC node, or `--concurrency` to set the number of requests to make concurrently with goroutines. Try using `./magellan help scan` for a complete set of options this subcommand. Alternatively, the same scan can be started using CIDR notation and with additional hosts:

```bash
./magellan scan https://10.0.0.100:5000 --subnet 172.16.0.0/24
```

Once the scan is complete, inspect the cache to see a list of found hosts with the `list` command. Make sure to point to the same database used before if you set the `--cache` flag.

```bash
./magellan list --cache data/assets.db
```

This will print a list of host information needed for the `collect` step. Set the `ACCESS_TOKEN` if necessary and invoke `magellan` again with the `collect` subcommand to query the node BMCs stored in cache.

We can then save the output and make a request with the `send` subcommand or pipe the output directly to the specified URL. The `-u/--username` and `-p/--password` flags must be set if the BMC requires basic authentication if the `--secrets-file` flag and `MASTER_KEY` environment variable is not set.

```bash
./magellan collect \
    --cache data/assets.db \
    --timeout 5 \
    --username $USERNAME \
    --password $PASSWORD \
    --format yaml \
    --output-file nodes.yaml \
    --cacert cacert.pem
```

This will initiate a crawler to fetch inventory data from the specified BMC host. The data can be saved, viewed, or modified from standard output by setting the `-v/--verbose` flag. Similarly, this output can also be saved by using the `-o/--output-file` flag and providing a path argument.

To make a request with the `collect` output, we specify the `-d/--data` flag for `send`. For files, use the `@` symbol before the file path. Make sure that you set the correct input format with `-F/--format`. Finally, specify the host as a positional argument.

```bash
magellan send -F yaml -d @nodes.yaml https://example.openchami.cluster:8443
```

This allows for modification of the data before making the request. However, be cautious as there is no data validation done before the request is made.

Alternatively, we can pass the output of `collect` into `send` using pipes. The `--verbose` flag is currently required to do this.

```bash
# collect and send data in YAML format
magellan collect -u $USERNAME -p $PASSWORD -v -F yaml | magellan send -F yaml https://example.openchami.cluster:8443

# collect and send data using default JSON format and secret store (see below)
export MASTER_KEY=mysecret
magellan secrets store default $USERNAME:$PASSWORD
magellan collect -v | magellan send https://example.openchami.cluster:8443
```

This maintains the original behavior of passing the `--host` flag to `collect` with the added flexibility of having the intermediate step.

> [!TIP]
> If the `cache` flag is not set, `magellan` will use `/tmp/$USER/magellan.db` by default.


> [!TIP]
> The output of `collect` can be saved in separate directories using the `-O/--output-dir` flag. The output will be organized similar to below for the following command in YAML format:
>
> ```bash
> ./magellan collect -F yaml -v -O nodes
> nodes
> ├── x1000c1s7b0
> │   └── 1747550498.yaml
> └── x1000c1s7b1
>     └── 1747550498.yaml
> ```

### PDU Inventory Collection

In addition to collecting Redfish inventory from BMCs, `magellan` can also collect inventory from Power Distribution Units (PDUs) that expose a JAWS-style API. The `collect` command has a `pdu` subcommand for this purpose.

The command connects to the specified PDU host(s), gathers all outlet information, and transforms it into the nested JSON format required by SMD in OpenCHAMI.

The most common workflow is to collect from the PDU and pipe the JSON output directly to the `magellan send` command, which then POSTs the data to a running SMD instance.

```bash
# Collect from a PDU and pipe the output directly to a local SMD instance
./magellan collect pdu pdu.example.com --username admin --password "pdu-password" | ./magellan send http://localhost:27779

### Managing Secrets

When connecting to an array of BMC nodes, some nodes may have different secret credentials than the rest. These secrets can be stored and used automatically by `magellan` when performing a `collect` or a `crawl`. All secrets are encrypted and are only accessible using the same `MASTER_KEY` as when stored originally.

To store secrets using `magellan`:

1. Set the `MASTER_KEY` environment variable. This can be generated using `magellan secrets generatekey`.

```bash
export MASTER_KEY=$(magellan secrets generatekey)
```

2. Store secret credentials for hosts shown by `magellan list`. There should be no output unless an error occurred.

```bash
export bmc_host=https://172.16.0.105:443
magellan secrets store $bmc_host $bmc_username:$bmc_password
```

3. Print the list of hosts to confirm secrets are stored.

```bash
magellan secrets list
```

If you see your `bmc_host` listed in the output, that means that your secrets were stored successfully.

Additionally, if you want to see the actually contents, make sure the `MASTER_KEY` environment variable is correctly set and do the following:

```bash
magellan secrets retrieve $bmc_host
```

4. Run either a `crawl` or `collect` and `magellan` should be a do find the credentials for each host.

```bash
magellan crawl -i $bmc_host
magellan collect \
  --username $default_bmc_username \
  --password $default_bmc_password
```

If you pass arguments with the `--username/--password` flags, the arguments will override all credentials set in the secret store for each flag. However, it is possible only override a single flag (e.g. `magellan collect --username`).

> [!WARNING]
> Make sure that the `secretID` is EXACTLY as show with `magellan list`. Otherwise, `magellan` will not be able to do the lookup from the secret store correctly.

> [!TIP]
> You can set default fallback credentials by storing a secret with the `secretID` of "default". This is used if no `secretID` is found in the local store for the specified host. This is useful when you want to set a username and password that is the same for all BMCs with the exception of the ones specified.
> 
> ```bash
> magellan secrets default $username:$password
> ```

### Starting the Emulator

This repository includes a quick and dirty way to test `magellan` using a Redfish emulator with little to no effort to get running.

1. Make sure you have `docker` with Docker compose and optionally `make`.

2. Run the `emulator/setup.sh` script or alternatively `make emulator`.

This will start a flask server that you can make requests to using `curl`.

```bash
export emulator_host=https://172.21.0.2:5000
export emulator_username=root           # set in the `rf_emulator.yml` file
export emulator_password=root_password  # set in the `rf_emulator.yml` file
curl -k $emulator_host/redfish/v1/ -u $emulator_username:$emulator_password
```

...or with `magellan` using the secret store...

```bash
export MASTER_KEY=$(magellan secrets generatekey)
magellan scan --port 5000 --subnet 172.21.0.0/24
magellan secrets store default $emulator_username:$emulator_password
magellan collect -o node_info.json
```

This example should work just like running on real hardware, and produce a `node-info.json` output file that contains the collected data.

### Updating Firmware

The `magellan` tool is capable of updating firmware with using the `update` subcommand via the Redfish API. This may sometimes necessary if some of the `collect` output is missing or is not including what is expected. The subcommand expects to find a running HTTP/HTTPS server that has an accessible URL path to the firmware download. Specify the URL with the `--firmware-path` flag and the firmware type with the `--component` flag (optional) with all the other usual arguments like in the example below:

```bash
./magellan update https://172.16.0.110:443 \
  --username $bmc_username \
  --password $bmc_password \
  --firmware-path http://172.16.0.255:8005/firmware/bios/image.RBU \
```

Then, the update status can be viewed by including the `--status` flag along with the other usual arguments or with the `watch` command:

```bash
./magellan update https://172.16.0.110 \
  --status \
  --username $bmc_username \
  --password $bmc_password | jq '.'
# ...or...
watch -n 1 "./magellan update https://172.16.0.110 --status --username $bmc_username --password $bmc_password | jq '.'"
```

### Managing Power

The `magellan power` tool facilitates control of node power states by identifying and communicating with the specified node(s)' controlling BMC.
As such, it requires a `collect` to be performed before it can translate a node name into a particular ComputerSystem within the correct BMC.
(For now, `collect` output should be saved to a file, and passed to `power` via the `-f/--inventory-file` flag. Support for retrieving inventory from SMD will be added soon.)

Power control is accomplished via the Redfish [Reset action](https://pkg.go.dev/github.com/stmcginnis/gofish/redfish#ComputerSystem.Reset), which supports various types of resets.
The supported reset types depend on BMC firmware implementation, and can be queried with the `-l/--list-reset-types` flag.
Once the desired reset type is identified, it can be applied via the `-r/--reset-type` flag.

```bash
# get power state
./magellan power x1000c0s0b3n0
# perform a particular type of reset
./magellan power x1000c0s0b3n0 -r On
./magellan power x1000c0s0b3n0 -r PowerCycle
# list supported reset types
./magellan power x1000c0s0b3n0 -l
```

All `power` commands demonstrated here can accept additional options and multiple target nodes, for example `magellan power -u USER -p PASS -f collect.json x1000c0s0b3n0 x1000c0s0b3n1 x1000c0s0b3n2`.
These options are omitted from the examples above for clarity.

### Getting an Access Token (WIP)

The `magellan` tool has a `login` subcommand that works with the [`opaal`](https://github.com/OpenCHAMI/opaal) service to obtain a token needed to access the SMD service. If the SMD instance requires authentication, set the `ACCESS_TOKEN` environment variable to have `magellan` include it in the header for HTTP requests to SMD.

```bash
# must have a running OPAAL instance
./magellan login --url https://opaal:4444/login

# ...complete login flow to get token
export ACCESS_TOKEN=eyJhbGciOiJIUzI1NiIs...
```

Alternatively, if you are running the OpenCHAMI quickstart in the [deployment recipes](https://github.com/OpenCHAMI/deployment-recipes), you can run the provided script to generate a token and set the environment variable that way.

```bash
quickstart_dir=path/to/deployment/recipes/quickstart
source $quickstart_dir/bash_functions.sh
export ACCESS_TOKEN=$(gen_access_token)
```

### Running with Docker

The `magellan` tool can be run in a Docker container after pulling the latest image:

```bash
docker pull ghcr.io/openchami/magellan:latest
```

Then, run either with the helper script found in `bin/magellan.sh` or the binary in the container:

```bash
docker run ghcr.io/openchami/magellan:latest /magellan.sh --scan "--subnet 172.16.0.0 --port 443 --timeout 3" --collect "--user admin --pass password --host http://vm01 --port 27779"
# ... or ..
docker ghcr.io/openhami/magellan:latest /magellan scan --subnet 172.16.0.0 --subnet-mask 255.255.255.0
```

## How It Works

At its core, `magellan` is designed to do three basic things:

1. Scan for BMC nodes in cluster available on a network
2. Query information about each BMC node through Redfish API
3. Store queried information into a system management database

First, the tool performs a scan to find running services on a network. This is done by sending a raw TCP packet to all specified hosts (either IP or host name) and taking note which services respond. At this point, `magellan` has no way of knowing whether this is a Redfish service or not, so another HTTP request is made to verify. Once the BMC responds with an OK status code, `magellan` will store the necessary information in a local cache database to allow collecting more information about the node later. This allows for users to only have to scan their cluster once to find systems that are currently available and scannable.

Next, the tool queries information about the BMC node using `gofish` API functions, but requires access to BMC node found in the scanning step mentioned above to work. If the node requires basic authentication, a user name and password is required to be supplied as well. Once the BMC information is retrieved from each node, the info is aggregated and place either in a file or on standard output which can be read by the `magellan send` command which makes an HTTP request to an SMD instance to store the data. This can be done in a command line pipeline in the shell or Optionally, the information can be written to disk for inspection and debugging purposes.

In summary, `magellan` needs at minimum the following configured to work on each node:

1. Available Redfish service with its known host and port
2. A running instance of SMD service with its known host and port
3. Docker to pull and run containers or Go to build binaries

## TODO

See the [issue list](https://github.com/OpenCHAMI/magellan/issues) for plans for `magellan`. Here is a list of other features left to add, fix, or do (and some ideas!):

* [X] Confirm loading different components into SMD
* [X] Add ability to set subnet mask for scanning
* [ ] Add ability to scan with other protocols like LLDP and SSDP
* [X] Add more debugging messages with the `-v/--verbose` flag
* [X] Separate `collect` subcommand with making request to endpoint
* [X] Support logging in with `opaal` to get access token
* [X] Support using CA certificates with HTTP requests to SMD
* [X] Add tests for the regressions and compatibility
* [X] Clean up, remove unused, and tidy code (first round)
* [X] Add `secrets` command to manage secret credentials
* [ ] Add server component to make `magellan` a micro-service

## Copyright

Copyright

© 2023 Triad National Security, LLC. All rights reserved. This program was produced under U.S. Government contract 89233218CNA000001 for Los Alamos National Laboratory (LANL), which is operated by Triad National Security, LLC for the U.S. Department of Energy/National Nuclear Security Administration. All rights in the program are reserved by Triad National Security, LLC, and the U.S. Department of Energy/National Nuclear Security Administration. The Government is granted for itself and others acting on its behalf a nonexclusive, paid-up, irrevocable worldwide license in this material to reproduce, prepare derivative works, distribute copies to the public, perform publicly and display publicly, and to permit others to do so.
