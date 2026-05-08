# Task.md - Batch Processor Alignment with Async Batch API

- Task: Align batch-processor with async Batch API in `subscription_handler.go` (202 + jobId + polling)
- Owner: AI assistant
- Status: Done
- Started: 2025-08-08
- Completed: 2025-08-08

## Subtasks
- [x] Review handler async flow (enqueue + poll endpoints)
- [x] Update client to handle 202 Accepted and parse `jobId`
- [x] Implement polling of `/api/v1/subscription-external/batch?jobId=...` until terminal state
- [x] Normalize polled status to existing `BatchOptinResponse` shape for logging/saving
- [x] Add config/flag for poll interval (`poll_interval`, `-poll`)
- [x] Build and verify compilation

## Notes
- New types added locally: `batchJobEnqueueResponse`, `batchJobStatus`, with states to mirror server.
- Config defaults include `poll_interval: "2s"`, overridable via flag `-poll`.
- Result saving remains unchanged; now reflects final job status.

## Next Steps
- [x] Add max polling timeout to avoid infinite loops if server never reaches a terminal state.
- [x] Update `README.md` with new flag and async behavior.
- [x] Expose Prometheus metrics and add dashboard JSON
- [x] Implement configurable pause/resume windows with timezone support
- [x] Add metrics scrape job to Prometheus config

## New Task: Entry Channel Rotation

- Task: Implement entry channel rotation between multiple channels (USSD, WEB, SMS)
- Owner: AI assistant
- Status: Done
- Started: 2025-08-08
- Completed: 2025-08-08

### Subtasks
- [x] Update ProcessorConfig to support multiple entry channels
- [x] Add rotation logic with thread-safe channel cycling
- [x] Implement GetNextEntryChannel() method for channel rotation
- [x] Add new -channels command line flag for multiple channels
- [x] Update configuration file to support entry_channels array
- [x] Maintain backward compatibility with single entry_channel
- [x] Update logging to show current channel for each batch
- [x] Update README.md with new feature documentation
- [x] Test configuration loading and channel rotation logic

### Implementation Details
- Added `EntryChannels []string` field to ProcessorConfig
- Added `currentChannelIndex` and `channelMutex` for thread-safe rotation
- Implemented `GetNextEntryChannel()` method that cycles through channels
- Added `-channels` flag for command line configuration
- Updated config.json with example of multiple channels
- Maintained backward compatibility with existing `entry_channel` field
- Each batch request now uses the next channel in rotation sequence
