#!/usr/bin/env bash

set -euo pipefail

MODEL="${OLLAMA_MODEL:-qwen2.5:7b}"

if ! command -v ollama >/dev/null 2>&1; then
  echo "ollama is not installed or not on PATH" >&2
  exit 1
fi

echo "Unloading model from the running Ollama daemon: ${MODEL}"

ollama stop "${MODEL}"

echo "Model unloaded: ${MODEL}"
