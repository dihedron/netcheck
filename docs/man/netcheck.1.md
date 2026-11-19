---
title: NETCHECK
section: 1
header: User Commands
footer: netcheck
date: 2026-11-18
---

# NAME

netcheck - check network connectivity from this host

# SYNOPSIS

**netcheck** [*OPTIONS*] [*BUNDLE*...]

**netcheck version** [**-v**|**\--verbose**] [**-D**|**\--dotenv** *FILE*]

**netcheck completion** *SHELL*

**netcheck \--version**

# DESCRIPTION

**netcheck** tests whether this host can reach other endpoints. A *bundle* is a list of checks, written in JSON or YAML, with timeouts, retries and a concurrency. **netcheck** runs the bundles in the order given, one after the other; the checks of one bundle run concurrently. It tests TCP, UDP, ICMP, TLS, DTLS, SSH, HTTP, HTTPS, PostgreSQL (its TLS negotiation) and SPIFFE servers (a TLS server verified by its SVID), and a TLS check also verifies the server certificate and its expiry.

Every argument that is not an option is the location of a bundle. With no bundle, **netcheck** needs a template and runs it on mock data, to help write the template.

Options that take a value are written with an equals sign (**\--format=json**) or glued to the short option (**-fjson**); **-f json** does not take *json* as its value.

Ctrl-C ends the run: the checks that had not finished are reported as skipped.

# OPTIONS

**-f**, **\--format**=*FORMAT*
:   How to write the report: **json**, **yaml**, **text** (the default) or **template**.

**-t**, **\--template**=*FILE*
:   A Go template for the report; it implies **\--format=template**.

**\--print-diagnostics**
:   Print the report's diagnostics to the standard error.

**\--allow-net**=*NET*
:   Only connect to this network (CIDR or address); repeatable.

**\--deny-net**=*NET*
:   Never connect to this network (CIDR or address); repeatable, and it wins over **\--allow-net**.

**\--spiffe-bundle**=*FILE*
:   The trust bundle, a PEM file of the CAs of a SPIFFE trust domain, that verifies the servers of **spire** checks; read at each attempt. Environment: **NETCHECK_SPIFFE_BUNDLE**. Without it a **spire** check fails.

**\--fetch-timeout**=*DURATION*
:   The longest time to load one bundle; **0** means no limit. The default is **30s**.

**-h**, **\--help**
:   Show the help message.

**\--version**
:   Print the version and exit.

# COMMANDS

**version** [**-v**] [**-D** *FILE*]
:   Print one line with the name and the version; with **-v** the build details too. **-D** (**\--dotenv**) loads a *.env* file and lists the **NETCHECK_\*** variables, with the values of passwords, tokens and secrets masked.

**completion** *SHELL*
:   Print the completion script for **bash**, **zsh**, **fish** or **powershell**. The scripts are static: completing never runs **netcheck**.

# BUNDLE SOURCES

The scheme of a location picks its source; a location with none of these schemes is a file name.

*file*
:   A file, in JSON or YAML.

**http://**, **https://**
:   HTTP and HTTPS, the certificate verified. **https-://** does not verify the certificate; use it only for tests.

**http+sso://**, **https+sso://**
:   The same, authenticating with the local Kerberos or NTLM identity.

**redis://**, **rediss://**
:   A Redis key: *redis://[user:pass@]host[:port]?key=K[&db=N]*; **rediss://** is over TLS.

**consulkv://**, **consulkvs://**
:   A Consul KV key: *consulkv://[user:pass@]host[:port]?key=K[&dc=D]*; **consulkvs://** is over TLS.

**consulsr://**, **consulsrs://**
:   A Consul service's metadata: *consulsr://[user:pass@]host[:port]?service=S&tag=T&meta=M[&dc=D]*.

# EXIT STATUS

**0**
:   Every check of every bundle passed; also **\--help** and **version**.

**1**
:   The command line is invalid, a template or a bundle could not be loaded or is invalid, the output could not be written, or the run was interrupted.

**2**
:   At least one check failed.

# FILES

*./netcheck.conf*, *~/netcheck.conf*, */etc/netcheck.conf*
:   The defaults file, YAML: timeout, retries, wait, concurrency and the ping settings. The first one found is read; the settings a bundle or a check gives win.

# ENVIRONMENT

**NETCHECK_LOG_LEVEL**
:   **debug**, **info**, **warn**, **error** or **off**; unset means off.

**NETCHECK_LOG_STREAM**
:   **stderr** (the default), **stdout** or **file**, which writes *netcheck-PID.log*.

**NETCHECK_CPU_PROFILE**, **NETCHECK_MEM_PROFILE**
:   Write a CPU or memory profile to the file named.

**NETCHECK_SPIFFE_BUNDLE**
:   The trust bundle of **\--spiffe-bundle**.

**NETCHECK_DOTENV**
:   Load that *.env* file first; it may set the variables above.

# EXAMPLES

Run a bundle and read the report as JSON:

    netcheck --format=json bundle.yaml

Run a bundle kept on a server, refusing to connect outside 10.0.0.0/8:

    netcheck --allow-net=10.0.0.0/8 https://config.example.org/bundle.yaml

Use the status in a script:

    netcheck bundle.yaml || alert

Load the completion for bash in the current shell:

    source <(netcheck completion bash)

# SEE ALSO

The manual, attached to each release as a PDF; *https://github.com/dihedron/netcheck*.

# AUTHOR

Andrea Funtò.
