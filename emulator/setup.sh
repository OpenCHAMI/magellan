#!/bin/sh

# SPDX-FileCopyrightText: © 2024-2025 Triad National Security, LLC. All rights reserved.
# SPDX-FileCopyrightText: © 2025-2026 OpenCHAMI a Series of LF Projects, LLC
#
# SPDX-License-Identifier: MIT

set -eu

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)

# clone the CSM redfish emulator if needed
if [ ! -d "${script_dir}/rf-emulator" ]; then
	git clone https://github.com/Cray-HPE/csm-redfish-interface-emulator "${script_dir}/rf-emulator"
fi

# Run the prebuilt emulator with the requested Docker Compose options.
docker compose -f "${script_dir}/rf-emulator.yml" up "$@"
