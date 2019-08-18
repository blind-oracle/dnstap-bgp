# dnstap-bgp

## Overview
This program was created to solve the problem of manipulating traffic based on domain names instead of IP addresses. It does this by intercepting DNS replies and exporting the IPs from replies using BGP protocol. The BGP peers can then use this information to influence the traffic flow.

## Workflow
* Client sends DNS request to a recursive DNS server which supports DNSTap (**unbound**, **bind** etc)
* DNS server logs the reply information using DNSTap to **dnstap-bgp**
* **dnstap-bgp** exports the IP adresses of the domain through BGP to a number of peers

## Features
* Export routes to any number of BGP peers
* Load a list of domains to intercept: the prefix tree is used to match subdomains
* Hot-reload of domain list by a HUP signal
* Configurable timeout to purge entries from the cache
* Persist the cache on disk (in a Bolt database)
* Sync the obtained IPs with other instances of **dnstap-bgp** using simple HTTP requests
