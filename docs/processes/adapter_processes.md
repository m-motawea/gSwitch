## Adapter Proceeses
* An adapter process is run at the end of each layer to prepare the payload for the next layer (if the message is passed to upper layers) as well as for its current layer (if the message is coming from upper layer)
* Ingress Adapter function should decide whether the message needs to be passed to upper layer or not (finished).
* Ingress adapter function sets the `LayerPayload` field to `[]byte` containing the upper layer data in the packet.
* Egress adapter function sets the `LayerPayload` filed to the payload type used in its layer (`ip.IPv4` in layer3 adapter for example).
* To be able to recreate the current layer payload when recieving messages from upper layers, in Ingress adapter function you might need to store the current message in the new message that is sent to the upper layer in `PreMessage` of the message content

(Checkout `l3/l2_adapter.go` for a complete example)
