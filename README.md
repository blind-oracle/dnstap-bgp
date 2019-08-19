# dnstap-bgp

## Overview
This daemon was created to solve the problem of manipulating traffic based on domain names instead of IP addresses. It does this by intercepting DNS replies and exporting the resolved IPs using BGP protocol. The BGP peers can then use this information to influence the traffic flow.

## Workflow
* Client sends DNS request to a recursive DNS server which supports DNSTap (**unbound**, **bind** etc)
* DNS server logs the reply information using DNSTap to **dnstap-bgp**
* **dnstap-bgp** caches the IPs from the reply and announces them through BGP
* When the request for the already cached IP comes again - refresh it's TTL

## Features
* Load a list of domains to intercept: the prefix tree is used to match subdomains
* Export routes to any number of BGP peers
* Hot-reload of the domain list by a HUP signal
* Configurable timeout to purge entries from the cache
* Persist the cache on disk (in a Bolt database)
* Sync the obtained IPs with other instances of **dnstap-bgp** using simple HTTP requests
* Can switch itself to a pre-created network namespace before initializing network. This can be useful if you want to peer with a BGP server running on the same host (e.g. **bird** does not support peering with any of the local interfaces). This requires running as *root*.

## Synchronization
**dnstap-bgp** can optionally push the obtained IPs to any number of other **dnstap-bgp** instances. Also it periodically asks peers for a list of IPs they have to keep its own list in sync in case of network outages. The interaction is done using two simple HTTP queries and JSON.

## Limitations
* IDN (punycode) domain names are currenly not supported and silently skipped
* Sync is fetching the whole cache contents from peers, so if the lists are large it can be hard on memory and network
