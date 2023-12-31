# Magellan

Magellan is a board management controller discovery tool designed to scan a network
and collect information about a BMC node and load that data into an
[`hms-smd`](https://github.com/bikeshack/smd/tree/master) instance.

## How It Works

Magellan is designed to do three things:

1. Scan for BMC nodes in cluster available on a network
2. Query information about each BMC node
3. Store queried information into a database

Magellan first tries to probe for specified hosts using the [`dora`](https://github.com/bmc-toolbox/dora)
API. If that fails, it then tries to use its own built-in, simpler scanner as a fallback.
This is done by sending a raw TCP request to a number of potential hosts over a
network, and noting which requests are successful. At this point, `magellan` sees
no difference between a services.

Next, it tries to query information about the BMC node using `bmclib` functions,
but requires access to a redfish interface on the node to work. Once the BMC 
information is received, it is then stored into `hms-smd` using its API.

In summary, `magellan` needs at minimum the following configured to work on each node:

1. Available redfish interface with its host and port
2. A running instance of `hms-smd` with its host and port
3. Additional dependencies for `bmclib` such as `ipmitool`

## Building

Install Go, clone the repo, and then run the following in the project root:

```bash
git clone https://github.com/bikeshack/magellan
cd magellan
go mod tidy && go build
```

This should find and download all of the required dependencies. Although other
versions of Go may work, the project has only been tested with v1.20.

To build the Docker container, run `docker build -t magellan:latest .` in the
project's directory.

## Usage

There are three main commands to use with the tool: `scan`, `list`, and `collect`.
To scan a network for BMC nodes, use the `scan` command. If the port is not specified,
`magellan` will probe ports 623, 442 (redfish and IPMI) by default:

```bash
./magellan scan --subnet 192.168.0.0 --db.path data/assets.db --port 623
```

This will scan the `192.168.0.0` subnet returning the host and port that return a response
and store the results in database with path `data/assets.db`. Additional flags can
be set such as `host` to add more hosts to scan not included on the subnet, `timeout` to set how long
to wait for a response from the BMC node, or `threads` to set the number of requests
to make concurrently. Try using `./magellan help scan` for a complete set of options.

To see the available BMC nodes found from the scan, use the `list` command. Make
sure to point to the same database used before:

```bash
./magellan list --db.path data/assets.db
```

This will print a list of IP address and ports found and stored from the scan.
Finally, run the `collect` command to store BMC info into `hms-smd`:

```bash
./magellan collect --db.path data/assets.db --driver ipmi --timeout 5 --user admin --pass password
```

This uses the info store in the database above to request information about each
BMC node if possible. It uses the driver specified by the `driver` flag which is
passed to and set in `bmclib`. Like with the scan, the time to wait for a response
can be set with the `timeout` flag as well. This command also requires the `user`
and `pass/password` flag to be set to use `ipmitool` (which must installed as well).
Additionally, it may be necessary to set the `host` and `port` flags for `magellan`
to find the `hms-smd` API.

Note: If the `db.path` flag is not set, `magellan` will use /tmp/magellan.db by default.

Both the `scan` and `collect` commands can be ran via Docker after pulling the image:

```bash
docker pull bikeshack/magellan:latest
docker run bikeshack/magellan:latest /magellan.sh --scan "--subnet 172.16.0.0 --port 443 --timeout 3" --collect "--user admin --pass password --host http://vm01 --port 27779"
```

## TODO

List of things left to fix, do, or ideas...

* [ ] Switch to internal scanner if `dora` fails
* [ ] Set default port automatically depending on the driver used to scan
* [X] Test using different `bmclib` supported drivers (mainly 'redfish')
* [X] Confirm loading different components into `hms-smd`
* [X] Add ability to set subnet mask for scanning
* [ ] Add unit tests for `scan`, `list`, and `collect` commands
* [X] Clean up, remove unused, and tidy code

## Copyright

Copyright

© 2023 Triad National Security, LLC. All rights reserved. This program was produced under U.S. Government contract 89233218CNA000001 for Los Alamos National Laboratory (LANL), which is operated by Triad National Security, LLC for the U.S. Department of Energy/National Nuclear Security Administration. All rights in the program are reserved by Triad National Security, LLC, and the U.S. Department of Energy/National Nuclear Security Administration. The Government is granted for itself and others acting on its behalf a nonexclusive, paid-up, irrevocable worldwide license in this material to reproduce, prepare derivative works, distribute copies to the public, perform publicly and display publicly, and to permit others to do so.
