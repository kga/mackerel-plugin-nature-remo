# mackerel-plugin-nature-remo

[Nature Remo](https://nature.global/) custom metrics plugin for [mackerel-agent](https://github.com/mackerelio/mackerel-agent).

It reads the newest sensor values of your devices from the Nature API and posts them to Mackerel.

| Metric | Unit |
| --- | --- |
| `NatureRemo.temperature.<device>` | float |
| `NatureRemo.humidity.<device>` | integer |
| `NatureRemo.illuminance.<device>` | float |

A metric appears only for sensors the device reports.

## Install

```sh
go install github.com/kga/mackerel-plugin-nature-remo@latest
```

## Usage

Issue an access token at <https://home.nature.global/> and pass it through the environment, because command-line arguments are visible to other processes.

```sh
NATURE_REMO_ACCESS_TOKEN=... mackerel-plugin-nature-remo
```

| Option | Default | Description |
| --- | --- | --- |
| `-access-token` | `$NATURE_REMO_ACCESS_TOKEN` | Nature API access token |
| `-metric-key-prefix` | `NatureRemo` | Metric key prefix: dot-separated `a-z`, `A-Z`, `0-9`, `-` and `_` |
| `-device-key` | `name` | What identifies a device in metric names: `name` or `id` |
| `-tempfile` | | Temp file name |

### Device names

Characters other than `a-z`, `A-Z`, `0-9`, `-` and `_` in a device name become `_`, one for each character. Names such as `寝室` and `居間` therefore both become `__`. When two devices collide, the plugin keeps the one with the smallest device ID and warns on stderr about the other. Use `-device-key=id` to avoid this; the metric names change, so existing graphs start over.

## mackerel-agent.conf

```toml
[plugin.metrics.NatureRemo]
command = ["/path/to/mackerel-plugin-nature-remo"]
env = { NATURE_REMO_ACCESS_TOKEN = "..." }
```

## Acknowledgements

This is an independent implementation inspired by [papix/mackerel-plugin-nature-remo](https://github.com/papix/mackerel-plugin-nature-remo). It keeps the metric names and options of that plugin, but shares no code with it.

## License

[MIT](LICENSE)
