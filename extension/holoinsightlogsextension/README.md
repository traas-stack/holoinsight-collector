#  holoinsight_logs Customized log collection and storage
- `alibabacloud_logservice` sls
- `decrypt` You can choose whether to encrypt the logstore. If you want to encrypt the secretKey and iv of the holoinsight collector, it needs to be consistent with the holoinsight backend

## Configuration

```yaml
extensions:
  holoinsight_logs:
    http:
      endpoint: 0.0.0.0:5551
    alibabacloud_logservice:
      endpoint: "xxxx"
      project: "demo-project"
      access_key_id: "access-key-id"
      access_key_secret: "access-key-secret"
    decrypt:
      enable: false
      secretKey:
      iv:
```
