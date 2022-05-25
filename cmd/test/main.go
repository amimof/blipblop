package main 

import (
	"log"
	"github.com/containernetworking/plugins/plugins/ipam/host-local/backend/allocator"
)

func main() {

	ipamConf, _, err := allocator.LoadIPAMConfig(args.StdinData, args.Args)
	if err != nil {
		return err
	}
}