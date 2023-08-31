
function build(){
    go mod tidy && go build -C bin/magellan
}

function scan() {
    ./magellan scan --subnet 172.16.0.0 --dbpath data/assets.db --driver ipmi --port 623
}

function list(){
    ./magellan list --dbpath data/assets.db
}

function collect() {
    ./magellan collect --dbpath data/assets.db --driver ipmi --timeout 5 --user admin --pass password
}

