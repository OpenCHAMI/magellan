# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.5.0]

### Changed

- Add --bmc-id-map (-m) option and ID Mapping logic to 'collect'
- Remove XNAME generation logic from 'collect'
- Add README content describing use of BMC ID Mapping
- Clean up left-over --host option remnants from 'collect'

## 0.4.1

### Fixed

- Fixed `secrets` cmd not showing error message for commands that do not exist

## 0.4.0

### Added

- Added PDU scanning and `--include` flag

## 0.3.1

### Fixed

- Fixed panic from invalid chassis in crawler

## 0.3.0

### Added

- Added PDU collect command to gather PDU inventory
- Added output need for power control

### Changed

- Removed 'https' from RedfishEndpoints POST

### Fixed

- Fixed host IP without prefix causing issue with 'FindMACAddressWithIP'

## [0.2.2]

### Changed

- Split the `collect` command into `collect` and `send` commands
- Allowed piping `collect` output into `send` to allow intermediate data modification

## [0.2.1]

### Added

- Added defaults to secret store
- Added secret store support to `update` command
- Added `pkg/bmc` package to handle credentials internal

### Changed

- Changed behavior of `--username` and `--password` flags to partial override credentials
- Changed CLI to have more consistent flags

## [0.2.0]

### Added

- Added `secrets` command for managing secrets with `SecretStore`
- Added `--username` and `--password` flags to `collect` command
- Added short option flags for `--username` and `--password` flags
- Added `--secrets-file` flag to `crawl` command
- Added static secrets store as fallback
- Added function to remove secrets from secrets store
- Added secrets lookup to `collect` command

### Changed

- Changed short options for secret store
- Changed details to error messages

### Fixed

- Fixed `golangci-lint` install command
- Fixed issues from running `golangci-lint`

### Updated

- Updated `golangci-lint` version
- Updated logging to be use consistentn JSON formatting

## [0.1.10]

### Fixed

- Fixed README documentation

## [0.1.9]

### Added

- Added collection of data return from `CollectInventory()` output
- Added initial `SecretStore` interface with `StaticStore` and `LocalStore` implementations for credentials management
- Added `--insecure` flag to allow skipping TLS verification for firmware updates

### Fixed

- Fixed dependabot security issues related to `crypto` package
- Fixed URL param not being set for `UpdateFirmwareRemote()`
- Fixed links in README documentation

### Changes

- Improved firmware updating functionality and added BMC identification support
- Improved Redfish service connection handling and update status retrieval
- Moved internal implementations to `pkg` and updated references
- Updated `update` command to use `gofish` package internally

## [0.1.8]

- Updated build workflow and added container build script
- Exported `cobra` commands for external use
- Fixed AMD64 microcode version in attestation

## [0.1.7]

- Refactor how versioning information is indicated in the build and source
- Updated Go version

## [0.1.6]

### Added

- Added functionality to fetch BMC manager data and include in `crawler`'s output
- Added IP to manager's ethernet interfaces
- Added check to exclude ethernet interface without IPs
- Added MACAddr to manager's output
- Added function to wait for emulator to start in tests
- Added API tests
- Added revision to `go install` commands
- Added PKGBUILD to install `magellan` as binary on Arch Linux
- Added `version` command and corresponding implementation

### Fixed

- Fixed issue writing output to file with `--output` flag
- Fixed hook to output correct filename with `goreleaser`
- Fixed typo in Makefile
- Fixed releaser .PHONY in Makefile
- Fixed issue with tests not working

### Updated

- Updated `crawl` to fetch and include BMC `Manager` data in output
- Updated and refactored `util` package
- Updated README.md documentation
- Updated `goreleaser` to v2 (v2.3.2)
- Updated go dependencies
- Updated tests to fix some issues
- Updated .gitignore file
- Updated Makefile to include `magellan.1` rule
- Updated Makefile to build with ldflags

### Changed

- Changed `crawler`'s internal function names
- Changed `test` rule in Makefile to use specific tests

### Removed

- Removed extra unused `gofish` imports
- Removed internal version implementation

## [0.1.5]

### Added

- Added Init() to Client interface
- Added temporary solution for creating new clients

### Changed

- Changed interface func from GetClient() to GetInternalClient()

### Fixed

- Fixed field tag in crawler
- Fixed panic when setting --cacert from invalid client

### Updated

- Updated warning message and changed SMD client to use pointer receivers

### Miscellaneous

- Merge pull request #55 from OpenCHAMI/cacert-hotfix

## [0.1.4]

### Added

- Added response body into error messages
- Added schema version to output

### Changed

- Changed collect messages to using JSON format

### Miscellaneous

- Merge branch 'main' into minor-changes
- Merge pull request #50 from OpenCHAMI/container-build
- Merge pull request #51 from OpenCHAMI/minor-changes
- Merge pull request #52 from OpenCHAMI/minor-changes
- Merge pull request #53 from OpenCHAMI/minor-changes
- Merge pull request #54 from OpenCHAMI/update-readme
- Rearranged collect error to only show when not force updating
- Updated README.md and fixed outdated info
- magellan.sh: remove unused build helper function
- release: prefix all version tags with "v"

## [0.1.3]

### Fixed

- Fixed automatic builds with docker container
- Fixed deprecation warning in goreleaser
- Fixed permissions in workflow
- Fixed typo in workflow

## [0.1.2]

### Fixed

- Fixed automatic builds with docker container
- Fixed typo in workflow

## [0.1.1]

### Added

- Added container building working
- Added more information to crawler output

### Removed

- Removed copying script in container

### Miscellaneous

- Merge pull request #49 from OpenCHAMI/add-types

## [0.1.0]

### Added

- Added TODO comments to tests and other minor change
- Added URL sanitization for SMD host and moved auth from util
- Added check for output directory for collect
- Added disclaimer about incompatibility with SMD
- Added flag to show cache info with list command and other minor changes

### Changed

- Changed 'docker' rule to 'container'
- Changed build rule and added release rule to Makefile
- Changed firmware._back to firmware-_
- Changed host to hostname being stored in cache
- Changed how arguments are passed to update command
- Changed how based URL is derived in update functions
- Changed order of adding default ports to add host correctly
- Changed saving host to include scheme for collect
- Changed short help message for root command
- Changed showing target host to use debug instead of verbose flag
- Changed transfer-protocol flag to scheme to match other commands
- Changed the username/password flag names

### Fixed

- Fixed '--subnet' flag not adding hosts to scan
- Fixed crawl command help string
- Fixed error message format for list command
- Fixed getting ethernet interfaces in CollectEthernetInterfaces()
- Fixed imports and removed unused query params
- Fixed issue with collect requests and other minor changes
- Fixed issue with host string and added internal url package
- Fixed lint errors
- Fixed passing the correct argument in Sanitize()
- Fixed port not being added to probing request
- Fixed root persistent flags not binding correctly
- Fixed scan not probing the host correctly
- Fixed small issue with command string
- Fixed typo errors in changelog and readme
- Fixed viper flag binding in collect cmd

### Removed

- Removed 'dora' API
- Removed commented out code
- Removed extra print statement
- Removed files from util
- Removed magellan's internal logger for zerolog
- Removed storage file
- Removed unused code, rename vars, and changed output to use hive partitioning strategy
- Removed unused functions in collect.go
- Removed unused port and clarified default in README.md
- Removed unused query params
- Removed unused updating code and bmclib dependency and other minor changes
- Removed unused variables in client package

### Updated

- Updated 'cmd' package
- Updated .gitignore
- Updated Makefile to include GOPATH in some targets
- Updated README.md with features section
- Updated example config
- Updated go dependencies
- Updated tests to reflect new API changes

### Renamed

- Renamed smd package to client
- Renamed struct
- Renamed vars and switched to use zerolog

### Miscellaneous

- Minor changes and improvements
- Minor changes to fix lint errors
- Minor changes to tests
- More minor changes
- Moved SMD-related API to pkg
- Refactored how clients work to reduce hard-coded dependencies
- Refactored/reorganized utils
- Reformatted scan help message
- Separated auth from util and fixed help strings

## [0.0.20]

- Updated workflows to publish container

## [0.0.19]

### Added

- Added 'docs' rule to Makefile
- Added initial round of comments for API documentation
- Added initial tests for API and compatibiilty coverage
- Added more API documentation
- Added more documentation and changed param names

### Changed

- Changed Dockerfile to use binary instead of script

### Fixed

- Fixed issue with required param
- Fixed small typo
- Fixed syntax error with command description

### Removed

- Removed unused code that used bmclib

### Updated

- Updated README to include information about building on Debian
- Updated go dependencies removing bmclib
- Updated dependencies

### Miscellaneous

Minor changes to README.md
Tidied up CLI flag names

## [0.0.18]

### Fixed

- Fixed formatting error in workflow

## [0.0.17]

- Addressed x/net dependabot issue

## [0.0.16]

- Updated attestation path

## [0.0.15]

### Removed

- Removed unnecessary attestation support script

## [0.0.14]

- Updated to goreleaser v2

## [0.0.13]

- Updated to goreleaser v2

## [0.0.12]

- Removed attestation of non-existent container

## [0.0.11]

### Removed

- Removed docker container from goreleaser to address build errors

## [0.0.10]

- Updated .goreleaser.yaml

## [0.0.9]

- Included Checkout in workflow

## [0.0.8]

## [0.0.7]

## [0.0.6]

### Added

- Adding dev container to standardize Linux build
- Merge pull request #1 from OpenCHAMI/rehome

## [0.0.5] - 2023-11-02

### Added

- Ability to update firmware
- Refactored connection handling for faster scanning
- Updated to reflect home at github.com/OpenCHAMI
- Updated to reflect ghcr.io as container home

## [Unreleased]

## [0.0.1] - 2023-09-14

### Added

- Ability to scan subnets for devices
- Ability to store results in a database
- Ability to generate an inventory from walking Redfish commands
- Ability to send inventory information to SMD
