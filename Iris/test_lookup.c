#include <stdio.h>
#include <stdlib.h>
#include <IOSurface/IOSurfaceRef.h>

int main(int argc, char** argv) {
    if (argc < 2) return 1;
    uint32_t id = atoi(argv[1]);
    IOSurfaceRef sfc = IOSurfaceLookup(id);
    if (sfc) {
        printf("Lookup success for %d\n", id);
        CFRelease(sfc);
    } else {
        printf("Lookup failed for %d\n", id);
    }
    return 0;
}
