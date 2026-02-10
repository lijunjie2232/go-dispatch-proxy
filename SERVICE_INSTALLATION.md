# Go Dispatch Proxy Service Installation Guide

This guide explains how to install and manage go-dispatch-proxy as a system service.

## Prerequisites

- Built go-dispatch-proxy binary (use `go build` to compile)
- Root/sudo access for installation
- systemd-based Linux system (Ubuntu 16.04+, CentOS 7+, Debian 8+, etc.)

## Quick Installation

1. **Build the binary** (if not already built):
   ```bash
   go build
   ```

2. **Run the installation script**:
   ```bash
   sudo ./install.sh
   ```

3. **Configure your load balancers**:
   ```bash
   sudo nano /etc/go-dispatch-proxy.yaml
   ```

4. **Restart the service**:
   ```bash
   sudo systemctl restart go-dispatch-proxy
   ```

## Service Management

### Start/Stop/Restart
```bash
sudo systemctl start go-dispatch-proxy
sudo systemctl stop go-dispatch-proxy
sudo systemctl restart go-dispatch-proxy
```

### Check Status
```bash
sudo systemctl status go-dispatch-proxy
```

### View Logs
```bash
sudo journalctl -u go-dispatch-proxy -f
```

### Enable/Disable Auto-start
```bash
sudo systemctl enable go-dispatch-proxy    # Start on boot
sudo systemctl disable go-dispatch-proxy   # Don't start on boot
```

## Configuration

The service uses `/etc/go-dispatch-proxy.yaml` for configuration. Here's an example:

```yaml
# Listen on localhost port 8080
listen_host: "127.0.0.1"
listen_port: 8080
tunnel_mode: false
quiet_mode: false
use_devices: false

load_balancers:
  # Load balance between two network interfaces
  - device: "eth0"
    cont_ratio: 1
  - device: "eth1"
    cont_ratio: 1
```

### Configuration Options

- `listen_host`: IP address to listen on (use `0.0.0.0` for all interfaces)
- `listen_port`: Port number for the SOCKS proxy
- `tunnel_mode`: Set to `true` for SOCKS5 tunnel load balancing
- `quiet_mode`: Set to `true` to suppress logs
- `use_devices`: Set to `true` to use device names instead of IPs
- `load_balancers`: List of network interfaces or tunnels to balance

## Uninstallation

To completely remove the service:

```bash
sudo ./uninstall.sh
```

This will:
- Stop and disable the service
- Remove the binary and configuration files
- Remove the systemd service definition
- Ask if you want to keep the configuration file for backup

## Troubleshooting

### Service Won't Start
1. Check configuration file syntax:
   ```bash
   sudo systemctl status go-dispatch-proxy
   ```

2. View detailed logs:
   ```bash
   sudo journalctl -u go-dispatch-proxy -f
   ```

3. Test configuration manually:
   ```bash
   sudo /usr/local/bin/go-dispatch-proxy -config /etc/go-dispatch-proxy.yaml
   ```

### Network Interface Issues
If you get permission errors binding to network interfaces:

1. The installer attempts to set the required capabilities automatically
2. If that fails, you can either:
   - Run as root (not recommended for production)
   - Use tunnel mode instead (doesn't require special permissions)
   - Manually set capabilities:
     ```bash
     sudo setcap cap_net_raw=eip /usr/local/bin/go-dispatch-proxy
     ```

### Configuration Validation
List available network interfaces:
```bash
/usr/local/bin/go-dispatch-proxy -list
```

## Security Considerations

- The service runs under the `proxy` user/group for security
- Configuration file is readable only by root and the proxy group
- Uses systemd security features (PrivateTmp, ProtectSystem, etc.)
- For public-facing deployments, consider:
  - Binding to localhost only
  - Using firewall rules
  - Adding authentication (SOCKS5 supports username/password)

## Advanced Usage

### Multiple Instances
To run multiple instances, create additional service files with different names and configuration files.

### Custom Installation Paths
Modify the installation script variables at the top if you need different paths.

### Monitoring
The service integrates with systemd-journald for logging. You can also set up monitoring with tools like:
- Prometheus (export metrics)
- Log aggregation systems
- Custom health checks