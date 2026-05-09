# Session Capsule: codex-20260509-022242

Task: `T-TMP-028`
Status: `done`

## Summary

Compose secret and env hygiene completed.

## Completed Work

- Removed checked-in subscription service database credential material from docker-compose.yml.
- Defaulted local subscription DB host routing to the Docker database service and preserved literal SSL mode.
- Added root .env.example with safe placeholders for compose-required variables.
- Updated release and TMP-021 evidence so config render is verified while runtime start remains blocked until real env/provider values and the required Docker network are supplied.

## Unfinished Work


## Next Tasks

