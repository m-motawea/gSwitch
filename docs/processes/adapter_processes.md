## Adapter Proceeses
* An adapter process is run at the end of each layer to prepare the payload for the next layer (if the message is passed to upper layers) as well as for its current layer (if the message is coming from upper layer)
* Ingress Adapter function should decide whether the message needs to be passed to upper layer or not (finished).
* Ingress adapter function sets the `LayerPayload` field to `[]byte` containing the upper layer data in the packet.
* Egress adapter function sets the `LayerPayload` filed to the payload type used in its layer (`ip.IPv4` in layer3 adapter for example).
* To be able to recreate the current layer payload when receiving messages from upper layers, the Ingress adapter function typically preserves the current message by assigning the entire `pipeline.PipelineMessage` back into the `PreMessage` property of the `ControlMessage`. You retrieve it later recursively on Egress.
* Because communication is transported via `[]byte` pointer IDs mapped to central sync Registries (such as `StoreMessage`), your Egress adapter should pull operations utilizing `controlplane.FetchMessage(msg.Content)` and ultimately write modified payloads directly with `msg.Content = controlplane.StoreMessage(msgContent)`.

(Checkout `l2/layer2_adapter.go` for a complete example)
