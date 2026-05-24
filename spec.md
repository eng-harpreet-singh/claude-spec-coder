# Feature Spec: Health Check Endpoint

## Goal
Add a health check endpoint to a Go HTTP service so that load balancers
and monitoring tools can verify the service is running.

## Requirements
- Route: `GET /health`
- Returns HTTP 200 with a JSON body
- The JSON body contains:
  - `status`: `"ok"`
  - `timestamp`: current time, RFC3339
  - `uptime_seconds`: process uptime as an integer
- Standard library only.

## Output
Provide the handler function and a short example of how to register it
with `http.ServeMux`.
