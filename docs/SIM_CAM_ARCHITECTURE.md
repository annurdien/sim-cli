# SIM-CLI Camera Architecture

This document explains the technical architecture of the `sim cam` feature. It details how `sim-cli` injects macOS cameras into iOS Simulator apps, a capability Apple does not natively support.

## The Problem
The iOS Simulator lacks hardware passthrough for the host Mac's physical cameras. When an app running in the simulator uses `AVFoundation` to request camera access, the simulator returns a mock black screen, provides a static placeholder, or crashes the app.

## The Solution
`sim-cli` solves this using a distributed, two-process architecture communicating over a lock-free shared memory header and hardware-accelerated `IOSurface` graphics memory.

```mermaid
flowchart TD
    subgraph Host["macOS Host"]
        CLI["sim-cli"]
        WebCam["WebCam (Hardware)"]
        
        subgraph FrameHost["FrameHost (macOS Process)"]
            CameraSource["CameraSource"]
            SharedFrameWriter["SharedFrameWriter"]
        end
        
        SHM[/"/tmp/iris.<udid>.frames (Shared Memory Header)"/]
        GPU[/"GPU Memory (IOSurface)"/]
    end

    subgraph Sim["iOS Simulator"]
        subgraph App["Target iOS App"]
            AVFoundation["AVFoundation"]
            
            subgraph Injector["IrisInject.dylib"]
                Swizzler["Swizzler"]
                SharedFrameReader["SharedFrameReader"]
                SampleBufferFactory["SampleBufferFactory"]
            end
        end
    end

    CLI -. "Spawns" .-> FrameHost
    CLI -. "Injects via launchctl" .-> App
    WebCam -- "Frames" --> CameraSource
    CameraSource -- "BGRA" --> SharedFrameWriter
    SharedFrameWriter -- "Writes Frame" --> GPU
    SharedFrameWriter -- "Seq-locked write ID" --> SHM
    SHM -- "Polling read ID" --> SharedFrameReader
    GPU -- "Zero-Copy Bind" --> SharedFrameReader
    SharedFrameReader -- "IOSurface" --> SampleBufferFactory
    SampleBufferFactory -- "CMSampleBuffer" --> Swizzler
    Swizzler -- "Spoofs feed" --> AVFoundation
```

---

## 1. Inter-Process Communication (IPC)

Passing uncompressed 1080p or 4K video at 60 frames per second between two separate processes requires moving up to 500 MB/s of data.

**The Design Decision:**
Standard IPC mechanisms like XPC, Mach ports, or local UNIX sockets require serializing the data, copying it into the operating system kernel space, and then copying it back out to the receiving process. This double-copy creates CPU bottlenecks and latency. To fix this, `sim-cli` uses **Memory-Mapped Files (`mmap`)** combined with **IOSurface**.

`IOSurface` is Apple’s native framework for sharing hardware-accelerated graphics memory across process boundaries. `FrameHost` writes frames to a global `IOSurface` pool, and only shares a 32-bit `ioSurfaceID` via a memory-mapped file. This enables **true zero-copy delivery** with virtually 0% CPU overhead for data transfer.

### Memory Layout
The shared memory file consists *only* of a 128-byte header (`IRISStreamHeader`). It does not contain the actual pixel data.

```c
typedef struct {
    uint32_t magic;                // "MSCC"
    uint32_t version;              // Version schema
    uint32_t width;                // Frame width
    uint32_t height;               // Frame height
    uint32_t bytesPerRow;          // Memory stride (aligned to 64 bytes)
    uint32_t pixelFormat;          // 'BGRA'
    uint32_t _pad1;                // Explicit padding
    uint32_t _pad2;                // Explicit padding
    uint64_t sequence;             // ATOMIC: Sequence lock for writers
    uint32_t ioSurfaceID;          // ATOMIC: ID of the active IOSurface
    uint32_t _pad0;                // Explicit padding
    uint64_t presentationTimeNs;   // Frame PTS (Nanoseconds)
    uint64_t framesProduced;       // ATOMIC: Total frames produced
    uint8_t  reserved[64];         // Future expansion padding
} IRISStreamHeader;                 // Exactly 128 bytes
```

### The Lock-Free Algorithm
Because the producer (`FrameHost`) and consumer (`IrisInject`) run in separate processes, they need a way to coordinate. If `FrameHost` updates the `ioSurfaceID` and timing metadata while the app is reading it, the video frame tears or presents with the wrong timestamp.

**The Design Decision:**
We cannot use standard POSIX mutexes (`pthread_mutex_t`). If the user stops the camera and `sim-cli` kills `FrameHost` via `SIGKILL` while it holds a shared mutex, the mutex is permanently locked. The iOS app would wait on that lock forever and freeze. 

Instead, `sim-cli` uses a **Sequence Lock** (SeqLock) implementation with C++20 standard atomic memory orders (`std::memory_order_acquire` / `std::memory_order_release`). Sequence locks eliminate deadlocks because the reader never blocks. It only retries.

```mermaid
sequenceDiagram
    participant P as FrameHost<br>(Producer)
    participant SHM as Shared Memory
    participant C as IrisInject<br>(Consumer)

    %% Producer Write
    Note over P, C: Producer Write
    P->>SHM: load sequence
    P->>SHM: sequence = sequence + 1 (Odd: Locked)
    P->>SHM: write new ioSurfaceID
    P->>SHM: write presentationTimeNs
    P->>SHM: sequence = sequence + 1 (Even: Unlocked)

    %% Consumer Read
    Note over P, C: Consumer Read
    loop until valid frame
        C->>SHM: seqA = load sequence
        alt seqA is Odd
            C->>C: __builtin_arm_yield() (retry)
        else seqA is Even
            C->>SHM: read ioSurfaceID and PTS
            C->>SHM: seqB = load sequence
            alt seqA == seqB
                C->>C: Valid frame! Break loop.
            else Torn Read
                C->>C: Frame modified during read. Retry.
            end
        end
    end
```

1. **Producer writes:** `FrameHost` writes a frame to a new `IOSurface` in its pool. It atomically increments the `sequence` integer to an **odd** number, signaling a write is in progress. It updates `ioSurfaceID` and timestamp metadata, and increments `sequence` to an **even** number.
2. **Consumer reads:** `IrisInject` polls the `sequence` integer. If it is even, it begins reading the IDs and metadata. After copying the 128-byte header, it checks `sequence` again. If `sequence` changed during the read, a torn read occurred. The consumer discards the read and retries instantly. It then invokes `IOSurfaceLookup` using the valid ID.

---

## 2. Initialization and Injection

To force the iOS app to read from our custom camera feed instead of the simulator's hardware abstraction, the system uses dynamic library injection.

**The Design Decision:**
We could patch the source code of the iOS app, but that requires modifying user code and re-compiling, which slows down development. By injecting a dynamic library (`.dylib`) at runtime, the developer does not need to change a single line of their application code.

```mermaid
sequenceDiagram
    actor User
    participant CLI as sim-cli
    participant LaunchCtl as launchctl
    participant Host as FrameHost
    participant App as iOS App
    participant Inject as IrisInject
    participant AV as AVCaptureSession

    User->>CLI: sim cam start
    CLI->>Host: spawn(udid)
    activate Host
    Host->>Host: allocate /tmp/iris.<udid>.frames
    Host->>Host: start AVFoundation capture
    CLI->>LaunchCtl: setenv DYLD_INSERT_LIBRARIES
    CLI->>LaunchCtl: setenv IRIS_PATH

    User->>App: launches app
    activate App
    App->>Inject: dyld loads injected library
    activate Inject
    Inject->>Inject: __attribute__((constructor)) init()
    Inject->>AV: method_exchangeImplementations(startRunning, msc_startRunning)
    deactivate Inject

    App->>AV: [session startRunning]
    AV->>Inject: msc_startRunning() (intercepted)
    activate Inject
    Inject->>Inject: setup GCD background queue
    Inject->>Inject: mmap(/tmp/iris.<udid>.frames)
    deactivate Inject
```

### Global Injection Mechanics
When you start a camera via `sim-cli cam start`, the CLI executes `xcrun simctl spawn <udid> launchctl setenv`. This modifies the global environment of the booted iOS Simulator. 
- `DYLD_INSERT_LIBRARIES=/path/to/IrisInject.dylib`: Forces the Apple dynamic linker (`dyld`) to load our library into every app launched on the simulator before the app's `main()` function executes.
- `IRIS_PATH`: Passes the path of the shared memory file so the dylib knows where to read frames.

Because this is set globally via `launchctl`, *any* app launched on the simulator automatically receives the injection.

### Objective-C Method Swizzling
Objective-C allows developers to change the mapping between a method name (selector) and its underlying C function (implementation) at runtime. This technique is known as Method Swizzling.

Inside `IrisInject.dylib`, a C constructor function (`__attribute__((constructor))`) runs immediately upon load. It uses the Objective-C runtime function `method_exchangeImplementations` to swap the memory addresses of Apple's internal `AVCaptureSession` methods with our custom implementations.

When the iOS app calls `[session startRunning]`, execution jumps to our code. The code bypasses Apple's hardware initialization, creates a Grand Central Dispatch (GCD) background thread, and begins reading frames from the shared memory.

---

## 3. Frame Delivery

When the iOS app's GCD queue successfully reads a valid `ioSurfaceID` from shared memory, it looks up the surface and packages it for `AVFoundation`.

```mermaid
flowchart LR
    SHM[("Shared Memory<br>(128B Header)")]
    Factory["SampleBufferFactory<br>(Obj-C++)"]
    CVPixelBuffer["CoreVideo<br>CVPixelBuffer"]
    CMTime["CMTime<br>(Hardware Clock)"]
    CMSampleBuffer["CMSampleBuffer"]
    Delegate["AVCaptureVideoDataOutput<br>SampleBufferDelegate"]

    SHM -- "Reads IOSurfaceID" --> Factory
    Factory -- "IOSurfaceLookup" --> CVPixelBuffer
    Factory -- "Attaches" --> CMTime
    CVPixelBuffer --> CMSampleBuffer
    CMTime --> CMSampleBuffer
    CMSampleBuffer -- "Pushes to App" --> Delegate
```

1. `SampleBufferFactory` reads the `ioSurfaceID` and looks up the `IOSurfaceRef` via the kernel.
2. It wraps the `IOSurface` into a CoreVideo `CVPixelBuffer`.
3. It attaches hardware timing data (`CMTime`) to match the simulator's internal clock.
4. It packages the `CVPixelBuffer` into a `CMSampleBuffer`.
5. It pushes the `CMSampleBuffer` to the `AVCaptureVideoDataOutputSampleBufferDelegate` of the app.

The host app receives the delegate callbacks exactly as it would from a hardware camera.

---

## 4. Camera Switching Logic
`sim-cli` supports hot-swapping cameras without crashing or restarting the iOS app.

```mermaid
sequenceDiagram
    actor User
    participant CLI as sim-cli
    participant OldHost as FrameHost (Old)
    participant NewHost as FrameHost (New)
    participant SHM as Shared Memory (.frames)
    participant App as iOS App

    User->>CLI: sim cam start <new_camera>
    CLI->>OldHost: SIGKILL
    Note over OldHost, SHM: OldHost dies immediately.<br>May leave sequence lock as ODD.
    
    CLI->>NewHost: spawn(new_camera)
    activate NewHost
    NewHost->>SHM: fstat() to check file size
    alt Resolution Matches
        NewHost->>SHM: mmap() existing file
        NewHost->>SHM: Check sequence lock
        alt Sequence is Odd
            NewHost->>SHM: Force sequence to Even (Repair)
        end
    else Resolution Changed
        NewHost->>SHM: Delete old file
        NewHost->>SHM: Create new file & mmap()
    end
    
    NewHost->>SHM: Resume writing frames
    Note over SHM, App: If reused, App sees new frames instantly.<br>If deleted, App is frozen on old inode.
    deactivate NewHost
```

When the active camera is changed:
1. `sim-cli` sends a termination signal (SIGKILL) to the running `FrameHost` daemon.
2. `sim-cli` spins up a new `FrameHost` daemon for the new camera.
3. The shared memory `.frames` file on disk is kept intact unless the requested resolution changes.
4. The new `FrameHost` uses `fstat` to verify the file size. If it matches, it memory maps the existing file instead of recreating it.
5. If the previous `FrameHost` was killed mid-write (leaving `sequence` as an odd number), the new `FrameHost` detects this and forces the atomic `sequence` back to an even number to repair the lock state.
6. **If the resolution matched (file reused):** The iOS app, which polls the memory map continuously, sees the `sequence` advance and resumes rendering new frames instantly without a restart.
7. **If the resolution changed (file recreated):** The old file is deleted from disk, but the iOS app still holds the old inode in memory (`mmap`). The app's camera feed will freeze because the old sequence lock never advances. The user must restart the iOS app to map the new file.

---

## 5. Future Architectural Improvements

The current architecture is highly functional, but advanced improvements could elevate performance and provide a completely seamless "magic" experience.

```mermaid
flowchart TD
    subgraph Host["macOS Host"]
        subgraph FrameHost["FrameHost (Next-Gen)"]
            CameraSource["CameraSource"]
            ControlReader["Control Message Reader"]
        end
        
        SHM[/"Shared Memory (Header & Control)"/]
    end

    subgraph Sim["iOS Simulator"]
        subgraph App["Target iOS App"]
            subgraph Injector["IrisInject.dylib"]
                VNODE["VNODE File Monitor"]
                ControlWriter["Control Message Writer"]
            end
            AVF["AVFoundation"]
        end
    end

    AVF -- "UI Events (Focus, Flip)" --> ControlWriter
    ControlWriter -- "Control Messages" --> SHM
    SHM -- "Reads Messages" --> ControlReader
    ControlReader -. "Adjusts Lens/Config" .-> CameraSource
    
    VNODE -. "Listens for deletion" .-> SHM
```

### 1. Seamless Hot-Swapping (The VNODE File Monitor)
To fix the limitation where resolution changes freeze the camera feed, the iOS app's `SharedFrameReader` could implement a Grand Central Dispatch (GCD) `DISPATCH_SOURCE_TYPE_VNODE` monitor. This allows the iOS app to listen for `NOTE_DELETE` or `NOTE_RENAME` events on the shared memory file. If `sim-cli` deletes the file to change resolutions, the app detects it instantly, closes the old memory map, and `mmap`s the new file on the fly—making camera swapping perfectly seamless.

### 2. Bidirectional Control Channel
Currently, data flows one way (macOS → iOS). By adding a lock-free **Control Ring-Buffer** in the shared memory header, the iOS app could send camera control events *back* to macOS. If the user taps the "Flip Camera" button or taps to focus inside the iOS Simulator, `IrisInject` could write those events to the control buffer. `FrameHost` would read them and automatically switch from the Mac's FaceTime camera to a plugged-in DSLR or adjust the hardware focus, making the injection truly interactive.
