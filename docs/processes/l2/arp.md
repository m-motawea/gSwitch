# ARP Process (L2)

The `ARP` process is responsible for Address Resolution Protocol caching, detection, and broadcasting across the distributed pipeline.

## Configuration
Currently there is no specific `.toml` configuration required. The ARP process dynamically observes interfaces and logs MAC targets passively tracking over memory mappings natively.

## Implementation Behavior
When interacting with the pipeline graph asynchronously via `localqueue`:

- **Ingress (`ARPInFunc`)**: Inspects incoming `EtherTypeARP` payloads seamlessly preventing logic panics. If it encounters a valid ARP message, it parses the MAC response successfully updating the `controlplane` central Switch ARP/MAC registration table sequentially preventing downstream data-locking dynamically.
- **Egress (`ARPOutFunc`)**: Serves as a dynamic resolver. Intercepts `IPv4` destination endpoints leaving `eth0` natively and evaluates the active ARP table map. If a target IP endpoint is missing from the table natively, it gracefully suspends egressing the IPv4 structure mapping automatically emitting a synthetic `EtherTypeARP` broadcast resolution payload directly triggering an outbound ARP Request!

## Messaging Handlers
Retrieve the context using Central Store bypass logic:
```go
msgContent, _ := controlplane.FetchMessage(msg.Content)
...
msg.Content = controlplane.StoreMessage(msgContent)
```
