# LLDP conversion

`netdiag discover lldp` converts LLDP discovery output into normal netdiag YAML. It
supports OpenConfig JSON and detailed show-command output from Cisco, Juniper,
and Arista:

```sh
netdiag discover lldp show-lldp-neighbors-detail.txt -o discovered.yaml
netdiag discover lldp juniper-lldp.txt --format juniper --local edge-01 -o discovered.yaml
netdiag discover lldp openconfig-lldp.json --format openconfig --local leaf-01 -o discovered.yaml
netdiag discover lldp captures/ -o discovered-network.yaml
netdiag render discovered.yaml -o discovered.svg
```

`netdiag lldp` remains available as a compatibility alias.

Use `-` to read from standard input. The default `--format auto` recognizes
JSON and common vendor markers. Provide `--format` when captured output omits
those markers. `--format juniper-xml` explicitly selects Junos XML.

Show commands frequently omit the local hostname. In that case, pass
`--local`. Cisco output containing a prompt such as
`leaf-01# show lldp neighbors detail` supplies it automatically.
Cisco NFVIS `show switch lldp neighbors` summary tables are also supported.
Cisco IOS XR `show lldp neighbors [interface]` summary tables and XR
location-prefixed prompts are supported.
Juniper Junos `show lldp neighbors` summary tables and `user@switch>` prompts
are supported. Sectioned `show lldp neighbors detail` output is also supported,
including capabilities and management addresses. Junos
`show lldp neighbors detail | display xml` is supported with or without the
surrounding CLI prompt text.

A directory input merges all immediate `.txt`, `.log`, `.out`, `.json`, `.xml`,
and extensionless capture files into one topology. Reciprocal observations of the
same node-and-port endpoints become one link. Each capture should include a
detectable local hostname; when it does not, the filename stem is used. For
example, promptless output in `edge-01.txt` is attributed to `edge-01`.

The converter uses the remote system name as the node identity, falling back to
the chassis ID or management address. It skips incomplete records lacking a
local port, remote port, or remote identity. Chassis ID, management address,
system description, and capabilities are preserved as node metadata.

## Architecture

The LLDP package separates format detection, vendor parsers, normalization,
topology conversion, and discovery reporting. Vendor adapters emit normalized
LLDP observations; only the converter creates netdiag documents. This keeps
future discovery protocols and vendor variants independent from rendering.
