#/bin/sh

script_dir=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )

# clone the CSM redfish emulator if needed
if [ ! -d ${script_dir}/rf-emulator ]; then
	git clone https://github.com/Cray-HPE/csm-redfish-interface-emulator ${script_dir}/rf-emulator
fi

# build docker image and run with docker compose
docker build -t openchami-rie:latest -f ${script_dir}/Dockerfile ${script_dir}/rf-emulator
docker compose -f ${script_dir}/rf-emulator.yml up
