# Skywalking Receiver
Support tenant

| Status                   |               |
| ------------------------ |---------------|
| Stability                | [beta]        |
| Supported pipeline types | traces        |
| Distributions            | [contrib]     |

Receives trace data in [Skywalking](https://skywalking.apache.org/) format.

## Getting Started

By default, the Skywalking receiver will not serve any protocol. A protocol must be
named under the `protocols` object for the Skywalking receiver to start. The
below protocols are supported, each supports an optional `endpoint`
object configuration parameter.

- `grpc` (default `endpoint` = 0.0.0.0:11800)
- `http` (default `endpoint` = 0.0.0.0:12800)

### holoinsight_server
[holoinsight server](https://github.com/traas-stack/holoinsight)
- `http` holoinsight server http endpoint

Examples:

```yaml
receivers:
  holoinsight_skywalking:
    holoinsight_server:
      http:
        endpoint: 127.0.0.1:8080
    protocols:
      grpc:
        endpoint: 0.0.0.0:11800
      http:
        endpoint: 0.0.0.0:12800

service:
  pipelines:
    traces:
      receivers: [holoinsight_skywalking]
```

[beta]: https://github.com/open-telemetry/opentelemetry-collector#beta
[contrib]: https://github.com/open-telemetry/opentelemetry-collector-releases/tree/main/distributions/otelcol-contrib
