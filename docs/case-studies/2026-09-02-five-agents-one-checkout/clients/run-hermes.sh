#!/usr/bin/env bash
cd "$(dirname "$0")"
exec dogfood chat -Q --yolo -q "$(cat prompt-hermes.txt)"
