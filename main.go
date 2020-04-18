package main

import (
	"log"
	"os"
	"sync"

	"github.com/m-motawea/gSwitch/config"
	"github.com/m-motawea/gSwitch/controlplane"
)

func main() {
	RegisterProcs()
	var wg sync.WaitGroup
	configPath := "config.toml"
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}
	CONFIG, err := config.ReadConfig(configPath)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Config: %v", CONFIG)
	sw := controlplane.NewSwitch("main switch", CONFIG, &wg)
	sw.Start()
	for name, portCfg := range CONFIG.SwitchPorts {
		sw.AddSwitchPort(name, portCfg)
		if portCfg.Up {
			sw.UpPort(name)
		}
	}
	wg.Wait()
}
