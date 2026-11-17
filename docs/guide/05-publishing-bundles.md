# Publishing bundles

A bundle is useful when many hosts run the same checks. This chapter shows where a bundle can be kept so that all of them read the same copy. Chapter 6 shows the matching command lines.

## How the format is recognised

`netcheck` must know whether a document is YAML or JSON:

| Source | How the format is decided |
| ----------------------- | ---------------------------------------------------------- |
| File | The extension: `.yaml` or `.yml` is YAML, `.json` is JSON |
| HTTP, HTTPS | The `Content-Type` header of the response |
| Redis, Consul | The content: a document that starts with `---` is YAML, one that starts with `{` or `[` is JSON |

A document whose format cannot be told is read as YAML (file and HTTP) or refused (Redis and Consul).

## A file

Keep the bundle in a file with the right extension, on a path every host can read: a shared mount, a configuration-management repository, or the host's own disk. This is the simplest source, and the only one that needs no network.

## An HTTP or HTTPS server

Serve the file and set the `Content-Type` header:

| Format | `Content-Type` |
| ------ | ---------------------------------- |
| JSON | `application/json` |
| YAML | `application/x-yaml` or `text/yaml` |

A server that sends another type, such as `text/plain` or `application/octet-stream`, makes `netcheck` read the document as YAML; a JSON document is also a valid YAML document in most cases, but do not rely on it. Static file servers usually need a line of configuration for `.yaml` files.

Protect the bundle if it should not be public: HTTP basic authentication (credentials in the URL), a Kerberos or NTLM protected site (the `+sso` schemes), or a network rule. A bundle only lists addresses, but a list of the addresses an application depends on is worth protecting.

## A Redis key

Store the bundle as the value of a string key:

```bash
$> redis-cli -n 3 -x SET netcheck/payments < payments.yaml
```

The value must start with `---` (YAML) or `{` (JSON). The key can be on any database; the location gives the database number when it is not 0.

## A Consul key

Store the bundle as the value of a key in the Consul key/value store:

```bash
$> consul kv put netcheck/payments @payments.yaml
```

## The metadata of a Consul service

A service registered in Consul can carry the bundle in its metadata, so that the bundle lives with the service it describes. In a service definition:

```json
{
  "service": {
    "name": "payments",
    "tags": ["netcheck"],
    "meta": {
      "netcheck-bundle": "---\nid: payments\nchecks:\n  - address: db.example.com:5432\n"
    }
  }
}
```

The location names the service, a tag and the metadata key (chapter 6). The service name and the tag together must match exactly one entry of the catalogue: with none, or with several, the run fails with an error that says how many were found.

## Keeping a bundle healthy

- Give every bundle an `id`: it is the heading of its results.
- Put what is specific to a host in the location, not in the bundle: one bundle per application, not per host.
- Keep timeouts realistic: a bundle used as a deployment gate with a 20 s default timeout and three retries can take a minute to fail.
- Review a bundle like code: it is a list of the network flows an application needs.
