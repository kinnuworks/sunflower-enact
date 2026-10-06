# SDK 1.5.0 AI assistant loops on a default Ollama install

**What happens.** With Ollama 0.40 and `qwen2.5:3b` from the SDK's own catalogue, asking for a
RuntimePolicy made the assistant call `generate_runtime_policy` 23 times. It wrote a policy
for region `us-west-2`, namespace `default`, green ratio 0.9 and an `aiCompliance` block, none
of which were in the request, overwriting the file on each call.

**Why.** The SDK sends about 4,300 tokens of system prompt and tool definitions. Ollama's
default context is 4,096, and it truncates the prompt: the server log shows
`truncating input prompt limit=2050 prompt=4308` on every call. The user's message is cut
off, so the model fills the tool arguments from nothing. A larger model hits the same limit.

**What worked.** The same model with a larger context, created once:

```bash
curl localhost:11434/api/create -d '{"model":"qwen2.5:3b-16k","from":"qwen2.5:3b","parameters":{"num_ctx":16384}}'
```

The step then took one tool call.

**Suggestions**

- Send `options.num_ctx` with each request, sized to the prompt.
- Stop after a few identical tool calls, and show why a call failed.
- Ask before overwriting an existing policy file.
- After generating, list any requested fields that are not in the result. In our run the
  policy passed validation but had dropped `location.mode: Hard` and `memory.min`.
