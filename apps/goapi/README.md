# goapi

This is a simple golang api designed to be run in a kuberenetes cluster.

## Dependencies

[pkg/api](/apps/pkg/api) provides `/readiness` and `/liveness` endpoints, as well as graceful shutdown

## Endpoints

### /hello

Calls the userapi's /user endpoint and returns a greeting which include the returned username

#### Request

> HTTP POST

#### Response

``` json
{
    "data": "string"
}
```