#!/usr/bin/env bash
cd "$(dirname "$0")"
exec kimi -p "$(cat prompt-kimi.txt)"
