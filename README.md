# OpenCHAMI Magellan

The `magellan` CLI tool is a Redfish-based, board management controller (BMC) discovery tool designed to scan networks and is written in Go. The tool collects information from BMC nodes using the provided Redfish RESTful API with [`gofish`](https://github.com/stmcginnis/gofish) and loads the queried data into an [SMD](https://github.com/OpenCHAMI/smd/) instance. The tool strives to be more flexible by implementing multiple methods of discovery to work for a wider range of systems (WIP) and is capable of using independently of other tools or services.

**Note: `magellan` v0.1.0 is incompatible with SMD v2.15.3 and earlier.**

## Main Features

The `magellan` tool comes packed with a handleful of features for doing discovery, such as:

- Simple network scanning
- Redfish-based inventory collection
- Redfish-based firmware updating
- Integration with OpenCHAMI SMD
- Write inventory data to JSON

See the [TODO](#todo) section for a list of soon-ish goals planned.

## Getting Started

[Build](#building) and [run on bare metal](#running-the-tool) or run and test with Docker using the [latest prebuilt image](#running-with-docker). For quick testing, the repository integrates a Redfish emulator that can be ran by executing the `emulator/setup.sh` script or running `make emulator`.

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

The binary executable for the `golang-1.21` executable can then be found using `dpkg`.

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

### Running the Tool

There are three main commands to use with the tool: `scan`, `list`, and `collect`. To see all of the available commands, run `magellan` with the `help` subcommand which will print this output:

```bash
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
    --cache data/assets.db \
```

This will scan the `172.16.0.0` subnet returning the host and port that return a response and store the results in a local cache with at the `data/assets.db` path. Additional flags can be set such as `--host` to add more hosts to scan that are not included on the subnet, `--timeout` to set how long to wait for a response from the BMC node, or `--concurrency` to set the number of requests to make concurrently with goroutines. Try using `./magellan help scan` for a complete set of options this subcommand. Alternatively, the same scan can be started using CIDR notation and with additional hosts:

```bash
./magellan scan https://10.0.0.100:5000 --subnet 172.16.0.0/24
```

Once the scan is complete, inspect the cache to see a list of found hosts with the `list` command. Make sure to point to the same database used before if you set the `--cache` flag.

```bash
./magellan list --cache data/assets.db
```

This will print a list of host information needed for the `collect` step. Set the `ACCESS_TOKEN` if necessary and invoke `magellan` again with the `collect` subcommand to query the node BMCs stored in cache. If the `--host` flag is set, then an additional request will be made to send the output to the specified URL. The `--userame` and `--password` flags must be set if the BMC requires basic authentication.

```bash
./magellan collect \
    --cache data/assets.db \
    --timeout 5 \
    --username $USERNAME \
    --password $PASSWORD \
    --host https://example.openchami.cluster:8443 \
    --output logs/
    --cacert cacert.pem
```

This will initiate a crawler that will find as much inventory data as possible. The data can be viewed from standard output by setting the `--verbose` flag. This output can also be saved by using the `--output` flag and providing a path argument.

Note: If the `cache` flag is not set, `magellan` will use `/tmp/$USER/magellan.db` by default.

### Updating Firmware

The `magellan` tool is capable of updating firmware with using the `update` subcommand via the Redfish API. This may sometimes necessary if some of the `collect` output is missing or is not including what is expected. The subcommand expects there to be a running HTTP/HTTPS server running that has an accessible URL path to the firmware download. Specify the URL with the `--firmware-path` flag and the firmware type with the `--component` flag (optional) with all the other usual arguments like in the example below:

```bash
./magellan update 172.16.0.108:443 \
  --username $USERNAME \ 
  --password $PASSWORD \
  --firmware-path http://172.16.0.255:8005/firmware/bios/image.RBU \
  --component BIOS
```

Then, the update status can be viewed by including the `--status` flag along with the other usual arguments or with the `watch` command:

```bash
./magellan update 172.16.0.110 --status --username $USERNAME --pass $PASSWORD | jq '.'
# ...or...
watch -n 1 "./magellan update 172.16.0.110 --status --username $USERNAME --password $PASSWORD | jq '.'"
```

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

The `magellan` tool can be ran in a Docker container after pulling the latest image:

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

Next, the tool queries information about the BMC node using `gofish` API functions, but requires access to BMC node found in the scanning step mentioned above to work. If the node requires basic authentication, a user name and password is required to be supplied as well. Once the BMC information is retrieved from each node, the info is aggregated and a HTTP request is made to a SMD instance to be stored. Optionally, the information can be written to disk for inspection and debugging purposes.

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
* [ ] Separate `collect` subcommand with making request to endpoint
* [X] Support logging in with `opaal` to get access token
* [X] Support using CA certificates with HTTP requests to SMD
* [ ] Add tests for the regressions and compatibility
* [X] Clean up, remove unused, and tidy code (first round)

## Copyright

Copyright

© 2023 Triad National Security, LLC. All rights reserved. This program was produced under U.S. Government contract 89233218CNA000001 for Los Alamos National Laboratory (LANL), which is operated by Triad National Security, LLC for the U.S. Department of Energy/National Nuclear Security Administration. All rights in the program are reserved by Triad National Security, LLC, and the U.S. Department of Energy/National Nuclear Security Administration. The Government is granted for itself and others acting on its behalf a nonexclusive, paid-up, irrevocable worldwide license in this material to reproduce, prepare derivative works, distribute copies to the public, perform publicly and display publicly, and to permit others to do so.
