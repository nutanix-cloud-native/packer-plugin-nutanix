package main

import (
	"fmt"
	"os"

	"github.com/hashicorp/packer-plugin-sdk/plugin"
	"github.com/nutanix-cloud-native/packer-plugin-nutanix/builder/nutanix"
	"github.com/nutanix-cloud-native/packer-plugin-nutanix/version"
)

func main() {
	pps := plugin.NewSet()
	pps.RegisterBuilder(plugin.DEFAULT_NAME, new(nutanix.Builder))
	pps.SetVersion(version.PluginVersion)
	err := pps.Run()
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}
