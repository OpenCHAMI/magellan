
function build(){
    go mod tidy && go build -C bin/magellan
}

function scan() {
    ./magellan scan --subnet 172.16.0.0 --port 443
}

function list(){
    ./magellan list
}

function collect() {
    ./magellan collect --user admin --pass password
}

