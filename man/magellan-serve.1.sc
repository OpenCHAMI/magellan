magellan-serve(1) "magellan" "OpenCHAMI"

# NAME

magellan serve - Run magellan as a long-lived BMC service (REST API)

# SYNOPSIS

*magellan serve* [OPTIONS]

# DESCRIPTION

The magellan serve command runs magellan as a persistent daemon exposing a REST
API for BMC inventory and power operations. The server runs until it receives
SIGINT or SIGTERM, then drains in-flight requests. The API is backed by the same
shared core as the CLI, so behavior is consistent across front-ends.

# OPTIONS

*--host* <host>
: Host/IP to bind (default: all interfaces)

*--port* <port>
: Port to listen on (default: 8500)

*--tls-cert* <file>
: Path to TLS certificate (enables HTTPS when set with --tls-key)

*--tls-key* <file>
: Path to TLS private key

*--auth-token* <token>
: Require this bearer token on /v1 routes (auth disabled when empty)

*--secrets-file* <file>
: Path to the node secrets file (used to resolve BMC credentials)

*-i, --insecure*
: Ignore BMC TLS verification errors

# ENVIRONMENT VARIABLES

- *SERVER_HOST* / *HOST*: Host/IP to bind
- *SERVER_PORT* / *PORT*: Port to listen on
- *SERVER_TLS_CERT* / *TLS_CERT*: TLS certificate path
- *SERVER_TLS_KEY* / *TLS_KEY*: TLS private key path
- *SERVER_AUTH_TOKEN* / *AUTH_TOKEN*: Bearer token required for /v1 routes
- *SECRETS_FILE*: Path to node secrets file
- *INSECURE*: Ignore TLS verification errors

# AUTHENTICATION

When *--auth-token* is set, all requests to /v1/\* must include an
Authorization: Bearer <token> header. Liveness/readiness endpoints (/healthz,
/readyz) are unauthenticated. For simple testing, generate a token with
tokensmith (https://github.com/OpenCHAMI/tokensmith):

```
token=$(tokensmith --duration 1h)
curl -H "Authorization: Bearer $token" https://localhost:8500/v1/...
```

# API ENDPOINTS

## GET /healthz

Liveness probe. Returns 200 OK when the server is running.

Example:

```
curl -s http://localhost:8500/healthz
```

Response:

```
{"status":"ok"}
```

## GET /readyz

Readiness probe. Returns 200 OK when the server is ready.

Example:

```
curl -s http://localhost:8500/readyz
```

Response:

```
{"status":"ready"}
```

## POST /v1/inventory

Crawl a single BMC for its systems and managers.

Request headers:
- *Content-Type*: application/json
- *Authorization*: Bearer <token> (required if auth enabled)

Body:
```
{
  "bmc": "https://bmc.example.com"
}
```

Body fields (unknown fields are rejected with 400):
- *bmc* (required): BMC base URL (e.g. https://172.16.0.10)

Example:

```
curl -s -X POST https://localhost:8500/v1/inventory \
  -H "Authorization: Bearer $token" \
  -H "Content-Type: application/json" \
  -d '{"bmc":"https://bmc.example.com"}'
```

Response headers: *Content-Type*: application/json

Response (200):

```
{
  "bmc": "https://bmc.example.com",
  "systems": [...],
  "managers": [...]
}
```

Errors: 400 (missing/invalid body), 401 (bad token), 502 (BMC failure).

## GET /v1/power

Get current power state of a ComputerSystem.

Query parameters:
- *bmc* (required): BMC base URL
- *system* (required): ComputerSystem Redfish ID

Headers:
- *Authorization*: Bearer <token> (required if auth enabled)

Example:

```
curl -s "https://localhost:8500/v1/power?bmc=https://bmc.example.com&system=Node0" \
  -H "Authorization: Bearer $token"
```

Response (200):

```
{
  "bmc": "https://bmc.example.com",
  "system": "Node0",
  "powerState": "On"
}
```

## GET /v1/power/reset-types

Get supported reset types for a ComputerSystem.

Query parameters:
- *bmc* (required): BMC base URL
- *system* (required): ComputerSystem Redfish ID

Headers:
- *Authorization*: Bearer <token> (required if auth enabled)

Example:

```
curl -s "https://localhost:8500/v1/power/reset-types?bmc=https://bmc.example.com&system=Node0" \
  -H "Authorization: Bearer $token"
```

Response (200):

```
{
  "bmc": "https://bmc.example.com",
  "system": "Node0",
  "resetTypes": ["On", "ForceOff", "GracefulShutdown", "ForceRestart", "Nmi"]
}
```

## POST /v1/power

Issue a power operation or raw reset, optionally confirming the resulting state.

Headers:
- *Content-Type*: application/json
- *Authorization*: Bearer <token> (required if auth enabled)

Body (vendor-neutral operation, optional wait):

```
{
  "bmc": "https://bmc.example.com",
  "system": "Node0",
  "operation": "off",
  "wait": true,
  "timeoutSeconds": 120
}
```

Body (raw reset type):

```
{
  "bmc": "https://bmc.example.com",
  "system": "Node0",
  "resetType": "ForceRestart"
}
```

Fields:
- *bmc* (required): BMC base URL
- *system* (required): ComputerSystem Redfish ID
- *operation* or *resetType* (exactly one required): vendor-neutral operation name or raw Redfish ResetType
- *wait* (optional): if true with operation, confirms transition to expected state
- *timeoutSeconds* (optional): timeout for wait confirmation

Vendor-neutral operations: on, off, soft-off, force-off, soft-restart, hard-restart, init.

Responses:
- 202 Accepted: operation issued (may not be confirmed)
- 200 OK: operation issued and confirmed
- 400 Bad Request: invalid/missing fields, unknown operation, or *wait* with *resetType*
- 401 Unauthorized: missing/invalid bearer token (if auth enabled)
- 422 Unprocessable Entity: operation not supported by the target
- 502 Bad Gateway: BMC operation failed

Response headers: *Content-Type*: application/json

Response bodies:

```
{"issued": true, "operation": "off"}
{"issued": true, "resetType": "ForceRestart"}
{"operation": "off", "status": "...", "finalState": "Off", "escalated": false, "escalatedTo": ""}
```

The last form is returned when *wait* is true.

Example (graceful off with confirmation):

```
curl -s -X POST https://localhost:8500/v1/power \
  -H "Authorization: Bearer $token" \
  -H "Content-Type: application/json" \
  -d '{"bmc":"https://bmc.example.com","system":"Node0","operation":"off","wait":true}'
```

# ERROR RESPONSES

Errors return JSON:

```
{"error": "message"}
```

Common cases: missing query params (400), unknown operation (400), missing/invalid token (401), unsupported operation (422), BMC connectivity/Redfish errors (502).

# EXAMPLES

Start with HTTPS and token:

```
magellan serve --port 8500 --tls-cert cert.pem --tls-key key.pem --auth-token "$TOKEN"
```

Start on localhost, insecure (dev only):

```
magellan serve --host 127.0.0.1 --port 8080 --insecure
```

Start with the default port (8500) and a secrets file:

```
magellan serve --secrets-file secrets.json
```

Configure with environment variables:

```
SERVER_PORT=9000 SERVER_AUTH_TOKEN="$TOKEN" magellan serve
```

# SEE ALSO

magellan(1), magellan-collect(1), magellan-power(1), magellan-settings(1)

