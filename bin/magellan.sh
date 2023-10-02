#!/bin/bash

EXE=./magellan
SUBNETS=""
PORTS=""
USER=""
PASS=""
ARGS=""


function build(){
	go mod tidy && go build -C bin/magellan
}

function scan() {
	# ./magellan scan --subnet 172.16.0.0 --port 443
	${EXE} scan --subnet ${SUBNETS} --port ${PORTS}
}

function list(){
	# ./magellan list
	${EXE} list 
}

function collect() {
	# ./magellan collect --user admin --pass password
	${EXE} collect --user ${USER} --pass ${PASS}
}


# parse incoming arguments to set variables
while [[ $# -gt 0 ]]; do
  case $1 in
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

echo "subnets 	= ${SUBNETS}"
echo "ports	 	= ${PORTS}"
echo "user 		= ${USER}"
echo "pass 		= ..."

if [[ -n $1 ]]; then
	echo "Last line of file specified as non-opt/last argument:"
	tail -1 "$1"
fi

scan
collect