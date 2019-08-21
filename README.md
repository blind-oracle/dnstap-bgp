[![Go Report Card](https://goreportcard.com/badge/github.com/blind-oracle/dnstap-bgp)](https://goreportcard.com/report/github.com/blind-oracle/dnstap-bgp)
[![Coverage Status](https://coveralls.io/repos/github/blind-oracle/dnstap-bgp/badge.svg?branch=master)](https://coveralls.io/github/blind-oracle/dnstap-bgp?branch=master)
[![Build Status](https://travis-ci.org/blind-oracle/dnstap-bgp.svg?branch=master)](https://travis-ci.org/blind-oracle/dnstap-bgp)

# dnstap-bgp

## Overview
This daemon was created to solve the problem of manipulating traffic based on domain names instead of IP addresses. It does this by intercepting DNS replies and exporting the resolved IPs using BGP protocol. The BGP peers can then use this information to influence the traffic flow.

## Workflow
* Client sends DNS request to a recursive DNS server which supports DNSTap (**unbound**, **bind** etc)
* DNS server logs the reply information through DNSTap to **dnstap-bgp**
* **dnstap-bgp** caches the IPs from the reply and announces them through BGP
* When the request for the already cached IP comes again - refresh it's TTL

## Features
* Load a list of domains to intercept: the prefix tree is used to match subdomains
* Hot-reload of the domain list by a HUP signal
* Full support for IPv6 (I hope): in DNS (AAAA RRs), in BGP and in syncer
* Support for CNAMEs - they are resolved and stored as separate ip -> domain entries
* Export routes to any number of BGP peers
* Configurable timeout to purge entries from the cache
* Persist the cache on disk (in a Bolt database)
* Sync the obtained IPs with other instances of **dnstap-bgp** using simple HTTP requests
* Can switch itself to a pre-created network namespace before initializing network. This can be useful if you want to peer with a BGP server running on the same host (e.g. **bird** does not support peering with any of the local interfaces). This requires running as *root*.

## Synchronization
**dnstap-bgp** can optionally push the obtained IPs to other **dnstap-bgp** instances. Also it periodically syncs its cache with peers to keep it up-to-date in case of network outages. The interaction is done using simple HTTP queries and JSON.

## Limitations
* IDN (punycode) domain names are currenly not supported and are silently skipped
* Sync is fetching the whole cache contents from peers, so if the lists are large (millions of entries) it can be hard on memory and network
* Performance was not measured very much, but it should be quite scalable - the only single-threaded part is reading from DNSTap socket, but it should be very lightweight
* The domain list and IP cache are stored in memory for performance reasons, so there should be enough RAM
* Logs only to stdout for now

## Configuration
See *deploy/dnstap-bgp.toml* for an example configuration and description of parameters.

## Examples
### unbound.conf
```
...

dnstap:
    dnstap-enable: yes
    dnstap-socket-path: "/tmp/dnstap.sock"
    dnstap-send-identity: no
    dnstap-send-version: no

    dnstap-log-client-response-messages: yes
```

**Important** In Ubuntu access to the DNSTap socket for Unbound is blocked by default by AppArmor rules. Either disable it for the Unbound binary or fix the rules.
