# Guide Agent

Real-time tour guide with vision and narration capabilities.

## Features

- Location-based guidance
- Image recognition for landmarks
- Cultural narrative generation
- Interactive Q&A

## Development

```bash
uv venv && source .venv/bin/activate
uv pip install -e ".[dev]"
uvicorn src.main:app --reload --port 8002
```
