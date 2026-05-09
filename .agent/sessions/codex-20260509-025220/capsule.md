# Session Capsule: codex-20260509-025220

Task: `T-TMP-030`
Status: `done`

## Summary

Acquisition API compose build-context defect fixed and verified.

## Completed Work

- Changed acquisition-api compose build to use the repo root as context with an explicit services/acquisition-api/Dockerfile path.
- Updated the acquisition API Dockerfile to copy common and service module inputs into paths matching the existing ../../common replacement.
- Built the acquisition API compose image successfully using temporary isolated Docker auth.
- Re-ran bounded compose smoke far enough to record downstream runtime blockers separately from image build readiness.
- Updated the full-system release matrix and TMP-021 value gate with current compose evidence.

## Unfinished Work

- Acquire acquisition-api runtime health. — next: Admin schema migration expects relation products before it exists in compose runtime.

## Next Tasks

