# examples

`sample-chatgpt-export.zip` is a synthetic ChatGPT data export (only
`conversations.json`, trimmed) for trying ChatPort without waiting for a real
export email:

```bash
chatport parse examples/sample-chatgpt-export.zip --stats
```

To use your own history: ChatGPT → Settings → Data controls → Export data,
then download the zip from the email and parse it the same way. The source
fixture lives in `sample-export/conversations.json` (`make example` rebuilds
the zip).
