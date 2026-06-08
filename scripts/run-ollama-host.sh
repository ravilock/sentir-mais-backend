#!/usr/bin/env bash

set -euo pipefail

MODEL="${OLLAMA_MODEL:-qwen2.5:7b}"
PULL_MODEL="${OLLAMA_PULL_MODEL:-true}"

if ! command -v ollama >/dev/null 2>&1; then
  echo "ollama is not installed or not on PATH" >&2
  exit 1
fi

if [[ "${PULL_MODEL}" == "true" ]]; then
  echo "Pulling Ollama model: ${MODEL}"
  ollama pull "${MODEL}"
fi

echo "Preloading model on the running Ollama daemon: ${MODEL}"
ollama run "${MODEL}" ""

echo "Model loaded and kept alive: ${MODEL}"
