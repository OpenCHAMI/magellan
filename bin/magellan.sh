
function build(){
    go mod tidy && go build -C bin/magellan
}

function scan() {
    ./magellan scan --subnet 172.16.0.0 --db.path data/assets.db --port 443,623,5000
}

function list(){
    ./magellan list --db.path data/assets.db
}

function collect() {
    ./magellan collect --db.path data/assets.db --driver redfish --timeout 30 --user admin --pass password
}

