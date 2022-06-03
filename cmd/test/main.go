package main

import (
	"github.com/containernetworking/plugins/plugins/ipam/host-local/backend/allocator"
	"log"
)

func main() {

	ipamConf, _, err := allocator.LoadIPAMConfig(args.StdinData, args.Args)
	if err != nil {
		return err
	}
}
