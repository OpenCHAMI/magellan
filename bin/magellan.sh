#!/bin/bash

EXE=./magellan
SUBNETS=""
PORTS=""
USER=""
PASS=""
SMD_HOST=""
SMD_PORT=""
THREADS="1"
TIMEOUT="30"
ARGS=""
FORCE_UPDATE=false
SCAN_PARAMS=""
COLLECT_PARAMS=""


function scan() {
	# ./magellan scan --subnet 172.16.0.0 --port 443
	${EXE} scan ${SCAN_PARAMS}
		# --subnet ${SUBNETS} \
		# --port ${PORTS} \
		# --timeout ${TIMEOUT} \
		# --threads ${THREADS}
}

function list(){
	# ./magellan list
	${EXE} list
}

function collect() {
	# ./magellan collect --user admin --pass password
	${EXE} collect ${COLLECT_PARAMS}
		# --user ${USER} \
		# --pass ${PASS} \
		# --timeout ${TIMEOUT} \
		# --threads ${THREADS} \
		# --host ${SMD_HOST} \
		# --port ${SMD_PORT} \
		# --force-update ${FORCE_UPDATE}
}


# parse incoming arguments to set variables
while [[ $# -gt 0 ]]; do
	case $1 in
	--scan)
		SCAN_PARAMS="$2"
		shift
		shift
		;;
	--collect)
		COLLECT_PARAMS="$2"
		shift
		shift
		;;
	--subnet)
		SUBNETS="$2"
		shift # past argument
		shift # past value
		;;
	-p|--port)
		PORTS="$2"
		shift # past argument
		shift # past value
		;;
	--user)
		USER="$2"
		shift # past argument
		shift # past value
		;;
	--pass|--password)
		PASS="$2"
		shift
		shift
		;;
	--smd-host)
		SMD_HOST="$2"
		shift
		shift
		;;
	--smd-port)
		SMD_PORT="$2"
		shift
		shift
		;;
	--timeout)
		TIMEOUT="$2"
		shift
		shift
		;;
	--threads)
		THREADS="$2"
		shift
		shift
		;;
	-*|--*)
		echo "Unknown option $1"
		exit 1
		;;
	*)
		ARGS+=("$1") # save positional arg
		shift # past argument
		;;
	esac
done

set -- "${POSITIONAL_ARGS[@]}" # restore positional parameters

if [[ -n $1 ]]; then
	echo "Last line of file specified as non-opt/last argument:"
	tail -1 "$1"
fi

scan
collect

# run with docker
# docker run magellan:latest magellan.sh \
# 	--scan "--subnet 127.16.0.0 --port 443" \
#	--collect "--user admin --pass password --timeout 300 --threads 1 --smd-host host --smd-port port"