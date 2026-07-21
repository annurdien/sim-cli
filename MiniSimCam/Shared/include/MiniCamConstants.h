#pragma once
// MiniCamConstants.h
// Shared path conventions and runtime constants.

#include <stdint.h>

// ---- Shared-memory file path -----------------------------------------------
// Full path: MSC_SHM_DIR "/" MSC_SHM_PREFIX <UDID> MSC_SHM_SUFFIX
#define MSC_SHM_DIR    "/tmp"
#define MSC_SHM_PREFIX "minisimcam."
#define MSC_SHM_SUFFIX ".frames"

// Status JSON file written by the FrameHost.
#define MSC_STATUS_SUFFIX ".status"

// PID file for the FrameHost process.
#define MSC_PID_SUFFIX ".pid"

// ---- Environment variables (passed via SIMCTL_CHILD_*) ---------------------
// Full path to the shared-memory file injected by the CLI.
#define MSC_ENV_PATH    "MINISIMCAM_PATH"

// Optional: override frames-per-second for the injector's delivery timer.
#define MSC_ENV_FPS     "MINISIMCAM_FPS"

// ---- Delivery defaults ------------------------------------------------------
#define MSC_DEFAULT_WIDTH   1280u
#define MSC_DEFAULT_HEIGHT   720u
#define MSC_DEFAULT_FPS       30u

// Delivery timer tolerance: 10% of frame duration is acceptable jitter.
#define MSC_TIMER_LEEWAY_RATIO  0.1

// ---- Safety limits ----------------------------------------------------------
// Maximum frame dimension supported by this version.
#define MSC_MAX_DIMENSION   3840u

// Maximum time (ms) the consumer waits when a sequence is odd before giving up.
#define MSC_SEQ_RETRY_LIMIT_US 1000u

// ---- Producer health --------------------------------------------------------
// If no frame is published within this many nanoseconds, the injector treats
// the producer as stalled and may deliver a placeholder frame.
#define MSC_STALE_THRESHOLD_NS  (500 * 1000 * 1000ull)  // 500 ms
