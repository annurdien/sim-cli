// TimeUtils.swift

import Foundation

/// Cached mach timebase info — initialised once, not on every call.
private let _timebaseInfo: mach_timebase_info_data_t = {
    var info = mach_timebase_info_data_t()
    mach_timebase_info(&info)
    return info
}()

/// Returns a monotonic nanosecond timestamp anchored to mach_absolute_time.
func currentMonotonicNs() -> UInt64 {
    let ticks = mach_absolute_time()
    return ticks * UInt64(_timebaseInfo.numer) / UInt64(_timebaseInfo.denom)
}

/// Converts a monotonic nanosecond timestamp age into milliseconds.
func ageInMs(fromMonotonicNs pastNs: UInt64) -> Double {
    let nowNs = currentMonotonicNs()
    if nowNs <= pastNs { return 0 }
    return Double(nowNs - pastNs) / 1_000_000.0
}
