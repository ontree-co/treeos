Docker's default network limit of ~31 networks comes from its default IP address allocation strategy:

  1. Default subnet pool: Docker uses the 172.17.0.0/16 to 172.31.0.0/16 range by default
  2. Subnet size: Each bridge network gets a /16 subnet (65,536 IP addresses)
  3. Available range: The 172.16.0.0/12 private IP range only has space for 16 /16 subnets
  4. Additional networks: Docker also creates networks from 192.168.0.0/16 range, adding ~15 more

  This gives approximately 31 total networks before exhaustion.

  You can increase this limit by configuring Docker's default-address-pools in /etc/docker/daemon.json:

  {
    "default-address-pools": [
      {"base":"10.0.0.0/8","size":24},
      {"base":"172.16.0.0/12","size":24},
      {"base":"192.168.0.0/16","size":24}
    ]
  }

  This would allow hundreds of networks by using smaller /24 subnets (256 IPs each) instead of /16 (65,536
   IPs each).

   