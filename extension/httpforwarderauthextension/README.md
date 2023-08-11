# Authenticator - http_forwarder_auth
This extension implements a `configauth.ServerAuthenticator`, to be used in receivers inside the `auth` settings. The authenticator type has to be set to `http_forwarder_auth`.
- `url` holoinsight apikey check http url, response `{"tenant": "xxx"}`
- `decrypt` You can choose whether to encrypt the apikey (the configuration provided to the agent). If you want to encrypt the secretKey and iv of the holoinsight collector, it needs to be consistent with the holoinsight backend

## Configuration

```yaml
extensions:
  http_forwarder_auth:
    url: http://localhost:8080/internal/api/gateway/apikey/check
    decrypt:
      enable: false
      secretKey:
      iv:

receivers:
  otlp:
    protocols:
      grpc:
        auth:
          authenticator: http_forwarder_auth

processors:

exporters:
  logging:
    logLevel: debug

service:
  extensions: [http_forwarder_auth]
  pipelines:
    traces:
      receivers: [otlp]
      processors: []
      exporters: [logging]
```
