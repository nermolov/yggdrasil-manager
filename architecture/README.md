# Yggdrasil connection manager

Basic functionality

- VPN via firewall, filter out connections from unknown devices
- Read known device list from config file
- Automated direct peering setup w/ NAT traversal

Additional functionality

- Read device list from management server (which reads from a config file)
- Allocate additional addresses to processes
- Allocate additional addresses to micro VMs (app runner)
- Move management server into app runner
- Add editing + sync to management server
- Logging of peer history
- Settings to toggle multicast/network peer auto acceptance

Mobile basic support

- System wide VPN

Mobile stretch

- Create API for other apps on the phone to use connection
- Allow other apps to use connection even with system VPN disabled
  - Probably possible on android
  - Probably impossible on iOS, would need an embedding solution instead
- Embed into + replace an app's networking with yggdrasil

## Places to draw from

- https://github.com/syncthing/syncthing has a NAT traversal implementation, and is written in go
