SHELL := /bin/bash
.DEFAULT_GOAL := help

.PHONY: help tools doctor
help:
	@echo 'Day 1: tools (install missing prerequisites), doctor (client checks; no cluster required)'
tools:
	/bin/bash scripts/install-tools.sh
doctor:
	/bin/bash scripts/doctor.sh
