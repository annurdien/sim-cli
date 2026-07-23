// CaptureHooks.h
#pragma once
#include "SharedFrameReader.hpp"
#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

void MSCInstallHooks(SharedFrameReader* reader, int32_t fps);
void MSCUninstallHooks(void);

#ifdef __cplusplus
}
#endif
