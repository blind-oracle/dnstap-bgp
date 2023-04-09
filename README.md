[![Go Report Card](https://goreportcard.com/badge/github.com/blind-oracle/dnstap-bgp)](https://goreportcard.com/report/github.com/blind-oracle/dnstap-bgp)
[![Coverage Status](https://coveralls.io/repos/github/blind-oracle/dnstap-bgp/badge.svg?branch=master)](https://coveralls.io/github/blind-oracle/dnstap-bgp?branch=master)

# dnstap-bgp

## Overview
This daemon was created to solve the problem of manipulating traffic based on domain names instead of IP addresses. It does this by intercepting DNS replies and exporting the resolved IPs using BGP protocol. The BGP peers can then use this information to influence the traffic flow.

## Workflow
* Client sends DNS request to a recursive DNS server which supports DNSTap (**unbound**, **bind** etc)
* DNS server logs the reply information through DNSTap to **dnstap-bgp**
* **dnstap-bgp** caches the IPs from the reply and announces them through BGP
* When the request for the already cached IP comes again - refresh its TTL

## Features
* Load a list of domains to intercept: the prefix tree is used to match subdomains
* Hot-reload of the domain list by a HUP signal
* Support for IPv6 - in DNS (AAAA RRs), in BGP and in syncer
* Support for CNAMEs - they are resolved and stored as separate ip -> domain entries
* Export routes to any number of BGP peers
* Configurable timeout to purge entries from the cache
* Persist the cache on disk (in a Bolt database)
* Sync the obtained IPs with other instances of **dnstap-bgp**
* Can be switched to a dedicated namespace using `ip netns` - see `deploy/*` init scripts for systemd. Useful when running with BGP router on the same host - ususally it can't peer with its own IPs (at least `bird`)

## Synchronization
**dnstap-bgp** can optionally push the obtained IPs to other **dnstap-bgp** instances. It also periodically syncs its cache with peers to keep it up-to-date in case of network outages. The interaction is done using simple HTTP queries and JSON.

## Limitations
* IDN (punycode) domain names are currenly not supported and are silently skipped
* Sync is fetching the whole cache contents from peers, so if the lists are large (millions of entries) it can be hard on memory and network
* Performance was not measured very much, but it should be quite scalable - the only single-threaded part is reading from DNSTap socket, but it should be very lightweight
* The domain list and IP cache are stored in memory for performance reasons, so there should be enough RAM
* Logs only to stdout for now

## Installation
### From packages
Get *deb* or *rpm* packages from the releases page.

### From source
You'll need Go environment set up, then just run `make`

### Building packages
To build a package you'll need *fpm* tool installed, then just run `make rpm` or `make deb`

## Configuration
See *deploy/dnstap-bgp.conf* for an example configuration and description of parameters.

## DNS server examples
DNSTap protocol works in a client-server manner, where DNS server is the client and **dnstap-bgp** is a server.

### Unbound
Unbound seem to be able to work with DNSTap only through UNIX sockets.

```
dnstap:
    dnstap-enable: yes
    dnstap-socket-path: "/tmp/dnstap.sock"
    dnstap-log-client-response-messages: yes
```

**Important** In Ubuntu access to the DNSTap socket for Unbound is blocked by default by AppArmor rules. Either disable it for the Unbound binary or fix the rules.

### BIND
DNSTap is supported since 9.11, but usually is not built-in, at least in Ubuntu packages.
BIND also can connect only using UNIX socket.

```
dnstap {
    client response;
};

dnstap-output unix "/tmp/dnstap.sock"
```
