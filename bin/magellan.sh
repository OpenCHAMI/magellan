
function build(){
	go mod tidy && go build -C bin/magellan
}

function scan() {
	./magellan scan --subnet 172.16.0.0 --port 443
}

function list() {
	./magellan list
}

function update() {
	./magellan update --user admin --pass password --host 172.16.0.109 --component BMC --protocol HTTP --firmware-path ""
}

function collect() {
	./magellan collect --user admin --pass password
}

