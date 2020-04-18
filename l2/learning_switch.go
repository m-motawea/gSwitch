package l2

import "github.com/m-motawea/gSwitch/dataplane"

type MACTable map[string]*dataplane.SwitchPort // mac address string to *port
type SwitchTable map[int]MACTable              // vlan id to mac table

func init() {
}
