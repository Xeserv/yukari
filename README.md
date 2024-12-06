# Yukari

Yukari is a pull-through cache for Ollama registries. The [Ollama](https://ollama.com/) registry is somewhat a Docker registry, but also somewhat not. It's just compatible enough with the Docker registry that you can use one as storage for Ollama models, but incompatible with pull-through caching. This project offers a simple pull-through cache that you can deploy to your networks to speed up pulling models.

As a side effect, this also makes your models resistant to "left-pad" style attacks where the models you rely on are no longer available. This stores models in [Tigris](https://tigrisdata.com), but theoretically can be extended to support any S3 compatible object storage system (S3, Ceph, etc).

## Deploying

Deploy this to your Kubernetes cluster by following TODO directions.

## Use the cache

This is quite easy, **just prepend `your.yukari.instance/library/` to the image you want to run/pull**

This `ollama pull <image>:<tag>` becomes

```bash
ollama pull your.yukari.instance/library/<image>:<tag>
```

## Architecture

This proxy will forward all uncached requests to the upstream Ollama registry. When it sees you fetching a manifest, it'll scrape that manifest for the component layers and start caching them in Tigris. All subsequent fetches will be from Tigris instead of the Ollama registry.

Every half an hour, Yukari will check if any manifests it has cached are more than 240 hours (10 days) old. If it finds any, it schedules reprocessing of those manifests. Any new model versions will automatically be put into Tigris, making things faster.

## Configuration options (via environment variables)

| Environment Variable | Description                                                   | Default                                 |
| -------------------- | ------------------------------------------------------------- | --------------------------------------- |
| `BIND`               | The TCP host:port to bind on when serving HTTP.               | `:9200` (port 9200 on all addresses)    |
| `INVALIDATOR_PERIOD` | How often the cache invalidator logic runs.                   | `30m` (30 minutes)                      |
| `MANIFEST_LIFETIME`  | How long a manifest can live before it is considered invalid. | `240h` (240 hours, or 10 days)          |
| `SLOG_LEVEL`         | The log level for [slog](https://pkg.go.dev/log/slog).        | `ERROR`                                 |
| `TIGRIS_BUCKET`      | The Tigris bucket to cache model information in.              | `yukari` (you will need to change this) |
| `UPSTREAM_REGISTRY`  | The upstream Ollama registry you are mirroring.               | `https://registry.ollama.ai/`           |

## Contributing

Feel free to create issues and PRs. The project is tiny as of now, so no dedicated guidelines.

Disclaimer: This is a side project. Don't expect any fast responses on anything.

## Related Information

Yukari is a fork of [simonfrey/ollama-registry-pull-through-proxy](https://github.com/simonfrey/ollama-registry-pull-through-proxy), but there has been an almost complete rewrite during the process of making it use Tigris as a storage backend.

- It is a fix to https://github.com/ollama/ollama/issues/914#issuecomment-1953482174
- To make its behavior work better, we would need this PR merged: https://github.com/ollama/ollama/pull/5241

## License

MIT
