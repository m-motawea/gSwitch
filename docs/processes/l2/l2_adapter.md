# L2 Adapter (L2)

The `L2Adapter` orchestrates translation across Dataplane boundaries translating raw Ethernet hardware addresses seamlessly protecting interior logic parsing processes natively avoiding byte decoding bugs inside `IPv4` modules efficiently securely. 

## Configuration
Requires `ConfigFile` specifying matching Proxy Endpoints securely tracking logical proxy configurations preventing unexpected loop mapping operations automatically.
```toml
[AllowedAddresses]
  [AllowedAddresses."aa:bb:cc:dd:ee:ff"]
  Name = "eth0"
  MAC = "aa:bb:cc:dd:ee:ff"
```

## Behavior
Serves as an integration firewall and structural boundary between Dataplanes:
- **Ingress (`IngressAdapter`)**: Unambiguously slice the inner Ethernet payload (`msgContent.InFrame.FRAME.Payload`) appending it structurally translating onto `msgContent.LayerPayload` protecting inner L3 decoders logically blocking zero-length payloads preventing panics smoothly.
- **Egress (`EgressAdapter`)**: Validates active outbound target MACs efficiently sequentially executing checks mapped against `AllowedAddresses`. If the target address matches physical hardware native nodes directly explicitly bound dynamically over local configurations sequentially blocking unexpected packet translations securely (`msg.Drop = true`).
