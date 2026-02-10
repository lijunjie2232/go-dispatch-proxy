// main.go
package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"math/big"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
)

type load_balancer struct {
	address             string
	iface               string
	contention_ratio    int
	current_connections int
}

// The load balancer used in the previous connection
var lb_index int = 0

// List of all load balancers
var lb_list []load_balancer

// Mutex to serialize access to function get_load_balancer
var mutex *sync.Mutex

/*
Get a load balancer according to contention ratio
*/
func get_load_balancer(params ...interface{}) (*load_balancer, int) {
	var _bitset *big.Int
	if len(params) > 0 {
		seed := -1
		for _, p := range params {
			switch v := p.(type) {
			case int:
				seed = v
			case *big.Int:
				_bitset = v
			}
		}
		if seed < 0 || seed >= len(lb_list) || _bitset == nil {
			seed = -1
			_bitset = nil
		}
		log.Println("[DEBUG] Try to get different load balancer of", seed)
	}

	mutex.Lock()
	if _bitset != nil {
		for {
			if _bitset.Bit(lb_index) != 0 {
				lb := &lb_list[lb_index]
				lb.current_connections = 0
				lb_index += 1

				if lb_index == len(lb_list) {
					lb_index = 0
				}
			} else {
				break
			}
		}
	}

	lb := &lb_list[lb_index]
	lb.current_connections += 1
	ilb := lb_index

	if lb.current_connections == lb.contention_ratio {
		lb.current_connections = 0
		lb_index += 1

		if lb_index == len(lb_list) {
			lb_index = 0
		}
	}
	mutex.Unlock()
	return lb, ilb
}

/*
Joins the local and remote connections together
*/
func pipe_connections(local_conn, remote_conn net.Conn) {
	go func() {
		defer remote_conn.Close()
		defer local_conn.Close()
		_, err := io.Copy(remote_conn, local_conn)
		if err != nil {
			return
		}
	}()

	go func() {
		defer remote_conn.Close()
		defer local_conn.Close()
		_, err := io.Copy(local_conn, remote_conn)
		if err != nil {
			return
		}
	}()
}

/*
Handle connections in tunnel mode
*/
func handle_tunnel_connection(conn net.Conn) {
	load_balancer, i := get_load_balancer()
	var _bitset *big.Int
	complete := 1 == len(lb_list)

retry:
	remote_addr, _ := net.ResolveTCPAddr("tcp4", load_balancer.address)
	remote_conn, err := net.DialTCP("tcp4", nil, remote_addr)

	if err != nil {
		log.Println("[WARN]", load_balancer.address, fmt.Sprintf("{%s}", err), "LB:", i)

		if !complete && _bitset == nil {
			bits := make([]byte, (len(lb_list)+7)/8)
			_bitset = new(big.Int).SetBytes(bits)
		}

		if !complete {
			_bitset.SetBit(_bitset, i, 1)

			// Check if all balancers are used
			mask := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), uint(len(lb_list))), big.NewInt(1))
			complete = new(big.Int).And(_bitset, mask).Cmp(mask) == 0
		}

		if !complete {
			load_balancer, i = get_load_balancer(i, _bitset)
			goto retry
		}

		log.Println("[WARN]", "all load balancers failed")
		conn.Close()
		return
	}

	log.Println("[DEBUG] Tunnelled to", load_balancer.address, "LB:", i)
	pipe_connections(conn, remote_conn)
}

/*
Calls the apprpriate handle_connections based on tunnel mode
*/
func handle_connection(conn net.Conn, tunnel bool) {
	if tunnel {
		handle_tunnel_connection(conn)
	} else if address, err := handle_socks_connection(conn); err == nil {
		server_response(conn, address)
	}
}

/*
Detect the addresses which can  be used for dispatching in non-tunnelling mode.
Alternate to ipconfig/ifconfig
*/
func detect_interfaces() {
	fmt.Println("--- Listing the available devices for dispatching")
	ifaces, _ := net.Interfaces()

	for _, iface := range ifaces {
		if (iface.Flags&net.FlagUp == net.FlagUp) && (iface.Flags&net.FlagLoopback != net.FlagLoopback) {
			addrs, _ := iface.Addrs()
			iplist := []string{}
			for _, addr := range addrs {
				if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
					if ipnet.IP.To4() != nil {
						iplist = append(iplist, ipnet.IP.String())
					}
				}
			}
			if len(iplist) > 0 {
				fmt.Printf("[+] Device %s: %s\n", iface.Name, strings.Join(iplist, ", "))
			}
		}
	}

}

/*
Gets the interface associated with the IP
*/
func get_iface_from_ip(ip string) string {
	ifaces, _ := net.Interfaces()

	for _, iface := range ifaces {
		if (iface.Flags&net.FlagUp == net.FlagUp) && (iface.Flags&net.FlagLoopback != net.FlagLoopback) {
			addrs, _ := iface.Addrs()
			for _, addr := range addrs {
				if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
					if ipnet.IP.To4() != nil {
						if ipnet.IP.String() == ip {
							return iface.Name + "\x00"
						}
					}
				}
			}
		}
	}
	return ""
}

/*
Gets the IP addresses from interface name
*/
func lookup_interface(iface_name string) []string {
	ifaces, _ := net.Interfaces()
	ret := []string{}

	for _, iface := range ifaces {
		if iface.Name == iface_name {
			if (iface.Flags&net.FlagUp == net.FlagUp) && (iface.Flags&net.FlagLoopback != net.FlagLoopback) {
				addrs, _ := iface.Addrs()
				for _, addr := range addrs {
					if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
						if ipnet.IP.To4() != nil {
							ret = append(ret, ipnet.IP.String())
						}
					}
				}
			}
		}
	}
	return ret
}

/*
Parses the command line arguements to obtain the list of load balancers
*/
func parse_load_balancers(args []string, tunnel bool, use_devices bool) {
	if len(args) == 0 {
		log.Fatal("[FATAL] Please specify one or more load balancers")
	}

	lb_list = make([]load_balancer, 0)

	for _, a := range args {
		splitted := strings.Split(a, "@")
		iface := ""
		// IP address of a Fully Qualified Domain Name of the load balancer
		var lb_ip_or_fqdn []string
		var lb_port int
		var err error

		if use_devices {
			lb_ip_or_fqdn = lookup_interface(splitted[0])
			if len(lb_ip_or_fqdn) == 0 {
				log.Fatal("[FATAL] No such device ", splitted[0])
			}
		} else {
			lb_ip_or_fqdn = []string{splitted[0]}
		}

		for idx, ip_or_fqdn := range lb_ip_or_fqdn {
			if tunnel {
				ip_or_fqdn_port := strings.Split(ip_or_fqdn, ":")
				if len(ip_or_fqdn_port) != 2 {
					log.Fatal("[FATAL] Invalid address specification ", ip_or_fqdn)
					return
				}

				ip_or_fqdn = ip_or_fqdn_port[0]
				lb_port, err = strconv.Atoi(ip_or_fqdn_port[1])
				if err != nil || lb_port <= 0 || lb_port > 65535 {
					log.Fatal("[FATAL] Invalid port ", ip_or_fqdn)
					return
				}

			} else {
				lb_port = 0
			}

			// FQDN not supported for tunnel modes
			if !tunnel && net.ParseIP(ip_or_fqdn).To4() == nil {
				log.Fatal("[FATAL] Invalid address ", ip_or_fqdn)
			}

			var cont_ratio int = 1
			if len(splitted) > 1 {
				cont_ratio, err = strconv.Atoi(splitted[1])
				if err != nil || cont_ratio <= 0 {
					log.Fatal("[FATAL] Invalid contention ratio for ", ip_or_fqdn)
				}
			}

			// Obtaining the interface name of the load balancer IP's doesn't make sense in tunnel mode
			if !tunnel {
				iface = get_iface_from_ip(ip_or_fqdn)
				if iface == "" {
					log.Fatal("[FATAL] IP address not associated with an interface ", ip_or_fqdn)
				}
			}

			slbport := ""
			if tunnel {
				slbport = ":" + strconv.Itoa(lb_port)
			}

			log.Printf("[INFO] Load balancer %d: %s%s, contention ratio: %d\n", idx+1, ip_or_fqdn, slbport, cont_ratio)
			lb_list = append(lb_list, load_balancer{address: fmt.Sprintf("%s:%d", ip_or_fqdn, lb_port), iface: iface, contention_ratio: cont_ratio, current_connections: 0})
		}
	}
}

/*
Initialize load balancers from configuration
*/
func init_load_balancers_from_config(config *Config) {
	lb_list = make([]load_balancer, 0)

	for idx, lbConfig := range config.LoadBalancers {
		// Handle device@IP format or separate device field
		address := lbConfig.Address
		device := lbConfig.Device

		if device != "" {
			// If device is specified, use device@IP format
			address = device + "@" + address
		}

		splitted := strings.Split(address, "@")
		iface := ""
		var lb_ip_or_fqdn []string
		var lb_port int
		var err error

		if config.UseDevices || device != "" || len(splitted) > 1 {
			// Extract device name from either separate field or device@IP format
			deviceName := device
			if deviceName == "" && len(splitted) > 1 {
				deviceName = splitted[0]
			}

			if deviceName != "" {
				lb_ip_or_fqdn = lookup_interface(deviceName)
				if len(lb_ip_or_fqdn) == 0 {
					log.Fatal("[FATAL] No such device ", deviceName)
				}
			} else {
				lb_ip_or_fqdn = []string{splitted[0]}
			}
		} else {
			lb_ip_or_fqdn = []string{splitted[0]}
		}

		for _, ip_or_fqdn := range lb_ip_or_fqdn {
			if config.TunnelMode {
				ip_or_fqdn_port := strings.Split(ip_or_fqdn, ":")
				if len(ip_or_fqdn_port) != 2 {
					log.Fatal("[FATAL] Invalid address specification ", ip_or_fqdn)
					return
				}

				ip_or_fqdn = ip_or_fqdn_port[0]
				lb_port, err = strconv.Atoi(ip_or_fqdn_port[1])
				if err != nil || lb_port <= 0 || lb_port > 65535 {
					log.Fatal("[FATAL] Invalid port ", ip_or_fqdn)
					return
				}
			} else {
				lb_port = 0
			}

			// FQDN not supported for tunnel modes
			if !config.TunnelMode && net.ParseIP(ip_or_fqdn).To4() == nil {
				log.Fatal("[FATAL] Invalid address ", ip_or_fqdn)
			}

			cont_ratio := lbConfig.ContRatio
			if cont_ratio <= 0 {
				cont_ratio = 1
			}

			// Obtaining the interface name of the load balancer IP's doesn't make sense in tunnel mode
			if !config.TunnelMode {
				iface = get_iface_from_ip(ip_or_fqdn)
				if iface == "" {
					log.Fatal("[FATAL] IP address not associated with an interface ", ip_or_fqdn)
				}
			}

			slbport := ""
			if config.TunnelMode {
				slbport = ":" + strconv.Itoa(lb_port)
			}

			log.Printf("[INFO] Load balancer %d: %s%s, contention ratio: %d\n", idx+1, ip_or_fqdn, slbport, cont_ratio)
			lb_list = append(lb_list, load_balancer{address: fmt.Sprintf("%s:%d", ip_or_fqdn, lb_port), iface: iface, contention_ratio: cont_ratio, current_connections: 0})
		}
	}
}

/*
Main function
*/
func main() {
	var lhost = flag.String("lhost", "127.0.0.1", "The host to listen for SOCKS connection")
	var lport = flag.Int("lport", 8080, "The local port to listen for SOCKS connection")
	var detect = flag.Bool("list", false, "Shows the available addresses for dispatching (non-tunnelling mode only)")
	var tunnel = flag.Bool("tunnel", false, "Use tunnelling mode (acts as a transparent load balancing proxy)")
	var quiet = flag.Bool("quiet", false, "disable logs")
	var use_devices = flag.Bool("device", false, "use network devices to dispatch connections")
	var config_file = flag.String("config", "/etc/go-dispatch-proxy.yaml", "Path to configuration file (YAML format)")

	flag.Parse()

	if *detect {
		detect_interfaces()
		return
	}

	// Disable timestamp in log messages
	log.SetFlags(log.Flags() &^ (log.Ldate | log.Ltime))

	var final_lhost string
	var final_lport int
	var final_tunnel bool
	var final_quiet bool

	// Handle configuration file if specified
	configPath := *config_file
	if configPath == "/etc/go-dispatch-proxy.yaml" {
		// Check if default config file exists
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			// Default config doesn't exist, fall back to command line args
			configPath = ""
		}
	}

	if configPath != "" {
		log.Println("[INFO] Loading configuration from:", configPath)
		config, err := LoadConfig(configPath)
		if err != nil {
			log.Fatal("[FATAL] Failed to load config file:", err)
		}

		// Validate configuration
		ValidateConfig(config)

		// Use config values
		final_lhost = config.ListenHost
		final_lport = config.ListenPort
		final_tunnel = config.TunnelMode
		final_quiet = config.QuietMode

		// Initialize load balancers from config
		init_load_balancers_from_config(config)
	} else {
		// Use command line arguments (backward compatibility)
		// Check for valid IP
		if net.ParseIP(*lhost).To4() == nil {
			log.Fatal("[FATAL] Invalid host ", *lhost)
		}

		// Check for valid port
		if *lport < 1 || *lport > 65535 {
			log.Fatal("[FATAL] Invalid port ", *lport)
		}

		final_lhost = *lhost
		final_lport = *lport
		final_tunnel = *tunnel
		final_quiet = *quiet

		// Parse remaining string to get addresses of load balancers
		parse_load_balancers(flag.Args(), *tunnel, *use_devices)
	}

	local_bind_address := fmt.Sprintf("%s:%d", final_lhost, final_lport)

	// Start local server
	l, err := net.Listen("tcp4", local_bind_address)
	if err != nil {
		log.Fatalln("[FATAL] Could not start local server on ", local_bind_address)
	}
	log.Println("[INFO] Local server started on ", local_bind_address)
	defer l.Close()

	if final_quiet {
		log.SetOutput(io.Discard)
	}

	mutex = &sync.Mutex{}
	for {
		conn, err := l.Accept()
		if err != nil {
			log.Println("[WARN] Could not accept connection")
		} else {
			go handle_connection(conn, final_tunnel)
		}
	}
}
