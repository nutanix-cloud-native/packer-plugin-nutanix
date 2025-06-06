//go:generate packer-sdc struct-markdown
//go:generate packer-sdc mapstructure-to-hcl2 -type WaitIpConfig

package nutanix

import (
	"context"
	"fmt"
	"log"
	"net"
	"strings"
	"time"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
)

type WaitIpConfig struct {
	// Amount of time to wait for VM's IP, similar to 'ssh_timeout'.
	// Defaults to `30m` (30 minutes). Refer to the Golang
	// [ParseDuration](https://golang.org/pkg/time/#ParseDuration)
	// documentation for full details.
	WaitTimeout time.Duration `mapstructure:"ip_wait_timeout"`
	// Amount of time to wait for VM's IP to settle down, sometimes VM may
	// report incorrect IP initially, then it is recommended to set that
	// parameter to apx. 2 minutes. Examples `45s` and `10m`.
	// Defaults to `5s` (5 seconds). Refer to the Golang
	// [ParseDuration](https://golang.org/pkg/time/#ParseDuration)
	// documentation for full details.
	SettleTimeout time.Duration `mapstructure:"ip_settle_timeout"`
	// Set this to a CIDR address to cause the service to wait for an address that is contained in
	// this network range. Defaults to `0.0.0.0/0` for any IPv4 address. Examples include:
	//
	// * empty string ("") - remove all filters
	// * `0:0:0:0:0:0:0:0/0` - allow only ipv6 addresses
	// * `192.168.1.0/24` - only allow ipv4 addresses from 192.168.1.1 to 192.168.1.254
	WaitAddress *string `mapstructure:"ip_wait_address"`
	ipnet       *net.IPNet
}

type stepWaitForIp struct {
	Config *WaitIpConfig
}

func (c *WaitIpConfig) Prepare() []error {
	var errs []error

	if c.SettleTimeout == 0 {
		c.SettleTimeout = 5 * time.Second
	}
	if c.WaitTimeout == 0 {
		c.WaitTimeout = 30 * time.Minute
	}
	if c.WaitAddress == nil {
		addr := "0.0.0.0/0"
		c.WaitAddress = &addr
	}

	if *c.WaitAddress != "" {
		var err error
		_, c.ipnet, err = net.ParseCIDR(*c.WaitAddress)
		if err != nil {
			errs = append(errs, fmt.Errorf("unable to parse \"ip_wait_address\": %w", err))
		}
	}

	return errs
}

func (c *WaitIpConfig) GetIPNet() *net.IPNet {
	return c.ipnet
}

func (s *stepWaitForIp) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get("ui").(packersdk.Ui)
	vm := state.Get("vm_uuid").(string)
	d := state.Get("driver").(*NutanixDriver)

	var ip string
	var err error

	sub, cancel := context.WithCancel(ctx)
	waitDone := make(chan bool, 1)
	defer func() {
		cancel()
	}()

	ui.Say("Waiting for IP...")

	go func() {
		ip, err = doGetIp(d, vm, sub, s.Config)
		waitDone <- true
	}()

	log.Printf("Waiting for IP, up to total timeout: %s, settle timeout: %s", s.Config.WaitTimeout, s.Config.SettleTimeout)
	timeout := time.After(s.Config.WaitTimeout)
	for {
		select {
		case <-timeout:
			cancel()
			<-waitDone
			if ip != "" {
				state.Put("ip", ip)
				log.Printf("[WARN] API timeout waiting for IP but one IP was found. Using IP: %s", ip)
				return multistep.ActionContinue
			}
			err := fmt.Errorf("timeout waiting for IP address")
			state.Put("error", err)
			ui.Errorf("%s", err)
			return multistep.ActionHalt
		case <-ctx.Done():
			cancel()
			log.Println("[WARN] Interrupt detected, quitting waiting for IP.")
			return multistep.ActionHalt
		case <-waitDone:
			if err != nil {
				state.Put("error", err)
				return multistep.ActionHalt
			}
			state.Put("ip", ip)
			ui.Sayf("IP address: %v", ip)
			return multistep.ActionContinue
		case <-time.After(1 * time.Second):
			if _, ok := state.GetOk(multistep.StateCancelled); ok {
				return multistep.ActionHalt
			}
		}
	}
}

func doGetIp(d *NutanixDriver, uuid string, ctx context.Context, c *WaitIpConfig) (string, error) {
	var prevIp = ""
	var stopTime time.Time
	var interval time.Duration
	if c.SettleTimeout.Seconds() >= 120 {
		interval = 30 * time.Second
	} else if c.SettleTimeout.Seconds() >= 60 {
		interval = 15 * time.Second
	} else if c.SettleTimeout.Seconds() >= 10 {
		interval = 5 * time.Second
	} else {
		interval = 1 * time.Second
	}
loop:
	ip, err := d.WaitForIP(ctx, uuid, c.ipnet)
	if err != nil {
		return "", err
	}

	// Check for ctx cancellation to avoid printing any IP logs at the timeout
	select {
	case <-ctx.Done():
		return ip, fmt.Errorf("cancelled waiting for IP address")
	default:
	}

	if prevIp == "" || prevIp != ip {
		if prevIp == "" {
			log.Printf("VM IP acquired: %s", ip)
		} else {
			log.Printf("VM IP changed from %s to %s", prevIp, ip)
		}
		prevIp = ip
		stopTime = time.Now().Add(c.SettleTimeout)
		goto loop
	} else {
		log.Printf("VM IP is still the same: %s", prevIp)
		if time.Now().After(stopTime) {
			if strings.Contains(ip, ":") {
				// To use a literal IPv6 address in a URL the literal address should be enclosed in
				// "[" and "]" characters. Refer to https://www.ietf.org/rfc/rfc2732.
				// Example: ssh example@[2010:836B:4179::836B:4179]
				ip = "[" + ip + "]"
			}
			log.Printf("VM IP seems stable enough: %s", ip)
			return ip, nil
		}
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("IP wait cancelled")
		case <-time.After(interval):
			goto loop
		}
	}

}

func (s *stepWaitForIp) Cleanup(state multistep.StateBag) {
	// No cleanup required
}
