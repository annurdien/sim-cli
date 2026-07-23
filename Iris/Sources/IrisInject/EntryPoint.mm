// EntryPoint.mm
// dylib constructor — called automatically by dyld when the library is loaded.
// Checks the IRIS_PATH environment variable, opens shared memory,
// and installs AVFoundation hooks.

#import <Foundation/Foundation.h>
#import "CaptureHooks.h"
#import "SharedFrameReader.hpp"
#include "IrisConstants.h"
#include <cstdlib>
#include <string>

// Process-global reader (heap-allocated; intentional lifetime until process exit).
static SharedFrameReader* gGlobalReader = nullptr;

static int32_t parseFPS(void) {
    const char* envFPS = getenv(IRIS_ENV_FPS);
    if (!envFPS) return IRIS_DEFAULT_FPS;
    int v = atoi(envFPS);
    return (v > 0 && v <= 120) ? (int32_t)v : IRIS_DEFAULT_FPS;
}

__attribute__((constructor))
static void IrisInjectInit(void) {
    @autoreleasepool {
        const char* pathEnv = getenv(IRIS_ENV_PATH);
        if (!pathEnv || pathEnv[0] == '\0') {
            NSLog(@"[IrisInject] %s not set — injector inactive.", IRIS_ENV_PATH);
            return;
        }

        std::string shmPath(pathEnv);
        NSLog(@"[IrisInject] loading — shm=%s", shmPath.c_str());

        gGlobalReader = new SharedFrameReader(shmPath);

        if (!gGlobalReader->open()) {
            NSLog(@"[IrisInject] ⚠️  Cannot open shared memory at %s. "
                  "Start 'sim cam start' before launching the app.",
                  shmPath.c_str());
            // Hooks still installed; they will check isOpen() before delivering.
        }

        int32_t fps = parseFPS();
        MSCInstallHooks(gGlobalReader, fps);

        NSLog(@"[IrisInject] ✅ injector ready (fps=%d, shm=%s)", fps, shmPath.c_str());
    }
}

__attribute__((destructor))
static void IrisInjectFini(void) {
    MSCUninstallHooks();
    if (gGlobalReader) {
        gGlobalReader->close();
        delete gGlobalReader;
        gGlobalReader = nullptr;
    }
    NSLog(@"[IrisInject] unloaded.");
}
