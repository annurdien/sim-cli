#pragma once
// IrisConstants.h
// Shared path conventions and runtime constants.

#include <stdint.h>

// Shared-memory file path
// Full path: IRIS_SHM_DIR "/" IRIS_SHM_PREFIX <UDID> IRIS_SHM_SUFFIX
#define IRIS_SHM_DIR    "/tmp"
#define IRIS_SHM_PREFIX "iris."
#define IRIS_SHM_SUFFIX ".frames"

// Status JSON file written by the FrameHost.
#define IRIS_STATUS_SUFFIX ".status"

// PID file for the FrameHost process.
#define IRIS_PID_SUFFIX ".pid"

// Environment variables (passed via SIMCTL_CHILD_*)
// Full path to the shared-memory file injected by the CLI.
#define IRIS_ENV_PATH    "IRIS_PATH"

// Optional: override frames-per-second for the injector's delivery timer.
#define IRIS_ENV_FPS     "IRIS_FPS"

// Delivery defaults
#define IRIS_DEFAULT_WIDTH   1280u
#define IRIS_DEFAULT_HEIGHT   720u
#define IRIS_DEFAULT_FPS       30u

// Delivery timer tolerance: 10% of frame duration is acceptable jitter.
#define IRIS_TIMER_LEEWAY_RATIO  0.1

// Safety limits
// Maximum frame dimension supported by this version.
#define IRIS_MAX_DIMENSION   3840u

// Maximum time (ms) the consumer waits when a sequence is odd before giving up.
#define IRIS_SEQ_RETRY_LIMIT_US 1000u

// Producer health
// If no frame is published within this many nanoseconds, the injector treats
// the producer as stalled and may deliver a placeholder frame.
#define IRIS_STALE_THRESHOLD_NS  (500 * 1000 * 1000ull)  // 500 ms
