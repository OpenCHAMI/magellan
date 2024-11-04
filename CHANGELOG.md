# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.5]

### Added

 * Added Init() to Client interface
 * Added temporary solution for creating new clients

### Changed

 * Changed interface func from GetClient() to GetInternalClient()

### Fixed

 * Fixed field tag in crawler
 * Fixed panic when setting --cacert from invalid client

### Updated

 * Updated warning message and changed SMD client to use pointer receivers

### Miscellaneous

 * Merge pull request #55 from OpenCHAMI/cacert-hotfix

## [0.1.4]

### Added

 * Added response body into error messages
 * Added schema version to output

### Changed

 * Changed collect messages to using JSON format

### Miscellaneous

 * Merge branch 'main' into minor-changes
 * Merge pull request #50 from OpenCHAMI/container-build
 * Merge pull request #51 from OpenCHAMI/minor-changes
 * Merge pull request #52 from OpenCHAMI/minor-changes
 * Merge pull request #53 from OpenCHAMI/minor-changes
 * Merge pull request #54 from OpenCHAMI/update-readme
 * Rearranged collect error to only show when not force updating
 * Updated README.md and fixed outdated info
 * magellan.sh: remove unused build helper function
 * release: prefix all version tags with "v"

## [0.1.3]

### Fixed

 * Fixed automatic builds with docker container
 * Fixed deprecation warning in goreleaser
 * Fixed permissions in workflow
 * Fixed typo in workflow

## [0.1.2]

### Fixed

 * Fixed automatic builds with docker container
 * Fixed typo in workflow

## [0.1.1]

### Added

 * Added container building working
 * Added more information to crawler output

### Removed

 * Removed copying script in container

### Miscellaneous

 * Merge pull request #49 from OpenCHAMI/add-types

## [0.1.0]

### Added

 * Added TODO comments to tests and other minor change
 * Added URL sanitization for SMD host and moved auth from util
 * Added check for output directory for collect
 * Added disclaimer about incompatibility with SMD
 * Added flag to show cache info with list command and other minor changes

### Changed

 * Changed 'docker' rule to 'container'
 * Changed build rule and added release rule to Makefile
 * Changed firmware.* back to firmware-*
 * Changed host to hostname being stored in cache
 * Changed how arguments are passed to update command
 * Changed how based URL is derived in update functions
 * Changed order of adding default ports to add host correctly
 * Changed saving host to include scheme for collect
 * Changed short help message for root command
 * Changed showing target host to use debug instead of verbose flag
 * Changed transfer-protocol flag to scheme to match other commands
 * Changed the username/password flag names

### Fixed

 * Fixed '--subnet' flag not adding hosts to scan
 * Fixed crawl command help string
 * Fixed error message format for list command
 * Fixed getting ethernet interfaces in CollectEthernetInterfaces()
 * Fixed imports and removed unused query params
 * Fixed issue with collect requests and other minor changes
 * Fixed issue with host string and added internal url package
 * Fixed lint errors
 * Fixed passing the correct argument in Sanitize()
 * Fixed port not being added to probing request
 * Fixed root persistent flags not binding correctly
 * Fixed scan not probing the host correctly
 * Fixed small issue with command string
 * Fixed typo errors in changelog and readme
 * Fixed viper flag binding in collect cmd

### Removed

 * Removed 'dora' API
 * Removed commented out code
 * Removed extra print statement
 * Removed files from util
 * Removed magellan's internal logger for zerolog
 * Removed storage file
 * Removed unused code, rename vars, and changed output to use hive partitioning strategy
 * Removed unused functions in collect.go
 * Removed unused port and clarified default in README.md
 * Removed unused query params
 * Removed unused updating code and bmclib dependency and other minor changes
 * Removed unused variables in client package

### Updated

 * Updated 'cmd' package
 * Updated .gitignore
 * Updated Makefile to include GOPATH in some targets
 * Updated README.md with features section
 * Updated example config
 * Updated go dependencies
 * Updated tests to reflect new API changes

### Renamed

 * Renamed smd package to client
 * Renamed struct
 * Renamed vars and switched to use zerolog

### Miscellaneous

 * Minor changes and improvements
 * Minor changes to fix lint errors
 * Minor changes to tests
 * More minor changes
 * Moved SMD-related API to pkg
 * Refactored how clients work to reduce hard-coded dependencies
 * Refactored/reorganized utils
 * Reformatted scan help message
 * Separated auth from util and fixed help strings

## [0.0.20]

 * Updated workflows to publish container

## [0.0.19]

### Added

 * Added 'docs' rule to Makefile
 * Added initial round of comments for API documentation
 * Added initial tests for API and compatibiilty coverage
 * Added more API documentation
 * Added more documentation and changed param names

### Changed

 * Changed Dockerfile to use binary instead of script

### Fixed

 * Fixed issue with required param
 * Fixed small typo
 * Fixed syntax error with command description

### Removed

 * Removed unused code that used bmclib

### Updated

 * Updated README to include information about building on Debian
 * Updated go dependencies removing bmclib
 * Updated dependencies

### Miscellaneous

Minor changes to README.md
Tidied up CLI flag names

## [0.0.18]

### Fixed

 * Fixed formatting error in workflow

## [0.0.17]

 * Addressed x/net dependabot issue

## [0.0.16]

 * Updated attestation path

## [0.0.15]

### Removed

 * Removed unnecessary attestation support script

## [0.0.14]

 * Updated to goreleaser v2

## [0.0.13]

 * Updated to goreleaser v2

## [0.0.12]

 * Removed attestation of non-existent container

## [0.0.11]

### Removed

 * Removed docker container from goreleaser to address build errors

## [0.0.10]

 * Updated .goreleaser.yaml

## [0.0.9]

 * Included Checkout in workflow

## [0.0.8]
## [0.0.7]

## [0.0.6]

### Added

 * Adding dev container to standardize Linux build
 * Merge pull request #1 from OpenCHAMI/rehome

## [0.0.5] - 2023-11-02

### Added

 * Ability to update firmware
 * Refactored connection handling for faster scanning
 * Updated to reflect home at github.com/davidallendj
 * Updated to reflect ghcr.io as container home

 ## [Unreleased]

## [0.0.1] - 2023-09-14

### Added

* Ability to scan subnets for devices
* Ability to store results in a database
* Ability to generate an inventory from walking Redfish commands
* Ability to send inventory information to SMD
