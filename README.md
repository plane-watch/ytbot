# plane.watch Discord Youtube Bot

Posts new Youtube videos to Discord.

In the root of the repository, create a `.env` file containing the following:

| Environment Variable | CLI Flag Equiv. | Description                       |
|----------------------|-----------------|-----------------------------------|
| `YTBOT_GC_API_KEY`   | `--apikey`      | Google Cloud API Key              |
| `YTBOT_WEBHOOK`      | `--webhook`     | Discord Webhook for posting video |

## How to get channel IDs

1. Go to <https://developers.google.com/youtube/v3/docs/search/list>
2. Set `part` = `snippet`
3. Set `q` = the channel name
4. Execute
5. Check JSON output
