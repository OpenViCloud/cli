# openvicloud — the OpenViCloud CLI

Manage the resources of your OpenViCloud tenant from a terminal: sign in on the web, create and control VMs, open an
SSH session to a VM that has no public address.

> No release yet. Until the first one, build it from source (below) or install it from the
> [CLI guide](https://site-staging.openvicloud.com/docs/guides/cli/).

## Build from source

Go 1.25 or newer:

```sh
go install github.com/OpenViCloud/cli/cmd/openvicloud@latest
```

## Sign in

```sh
openvicloud auth login     # prints a link and a code, finish on the web
openvicloud auth status
```

For CI jobs and scripts, create an API token on the portal (API tokens page) and set it as `OPENVICLOUD_TOKEN`;
no browser login is needed then.

## Commands

| Area | Commands |
|---|---|
| Account | `auth login` · `auth logout` · `auth status` · `tenant get` · `version` |
| Catalog | `zone list` · `template list [--zone]` · `size list` |
| VMs | `vm list` · `vm get` · `vm create` · `vm rename` · `vm start` · `vm stop` · `vm delete` · `vm ssh` |
| SSH keys | `ssh-key add` · `ssh-key list` · `ssh-key delete` |

`create`, `start`, `stop` and `delete` take `--wait` to return only once the request is done. Every command takes
`-o json`. Names or ids are accepted wherever a VM, network, template, size or SSH key is chosen.

The CLI targets the OpenViCloud staging API until production exists; `OPENVICLOUD_API_URL`, `OPENVICLOUD_AUTH_URL`
and `OPENVICLOUD_PORTAL_URL` point it elsewhere.

## Development

```sh
go vet ./...
go test ./...
go build -o openvicloud ./cmd/openvicloud
```

`api/client.gen.go` is generated from `api/openapi-v1.json`, a copy of the public API contract kept in
[OpenViCloud/titan](https://github.com/OpenViCloud/titan); `api/SOURCE` names the titan commit it was copied from. To
take a newer contract:

```sh
scripts/sync-spec.sh <titan commit, tag or branch>
```

CI fails when the committed client does not match the committed spec. The design rules of the CLI live in titan's
[docs/features/cli](https://github.com/OpenViCloud/titan/blob/main/docs/features/cli/design.md).

## License

[MPL-2.0](LICENSE)
