# Diane.app Mac Crash Investigation

**Date:** 2026-02-20  
**Platform:** macOS (Mac.banglab - 100.123.170.53)  
**App Version:** 1.17.1  
**Issue:** App crashing repeatedly with SwiftUI constraint violations

---

## Symptoms

1. **Frequent Crashes** - App crashes multiple times per day (5+ crashes observed today)
2. **Excessive Logging** - Log files growing to massive sizes:
   - `dianemenu.log`: 5.8MB (49,034 lines)
   - `dianemenu.1.log`: 170MB (old log)
3. **Performance Issues** - DianeClient initialized 19,214 times (should be minimal)

---

## Root Cause Analysis

### 1. SwiftUI Constraint Crash

**Crash Location:**
```
-[NSWindow(NSDisplayCycle) _postWindowNeedsUpdateConstraints] + 1716
-[NSView _updateConstraintsForSubtreeIfNeededCollectingViewsWithInvalidBaselines:] + 596
SwiftUI13NSHostingViewC17updateConstraintsyyF + 132
```

**Exception Type:** `EXC_BREAKPOINT (SIGTRAP)`  
**Signal:** Trace/BPT trap: 5

**Analysis:**
- SwiftUI NSHostingView is failing during constraint updates
- This typically happens when:
  - Views are updating too frequently
  - Circular dependencies in view updates
  - State changes triggering layout recursion

**Latest Crash:** 2026-02-20 15:28:51  
**Crash Report:** `/Users/mcj/Library/Logs/DiagnosticReports/Diane-2026-02-20-152851.ips`

### 2. Excessive DianeClient Initialization

**Pattern Observed:**
```
[2026-02-20 15:27:00.562] [DEBUG] [DianeClient] DianeClient.swift:64 - DianeClient initialized with socket path
[2026-02-20 15:27:00.577] [DEBUG] [DianeClient] DianeClient.swift:64 - DianeClient initialized with socket path
[... repeats 6 times every 5 seconds ...]
```

**Statistics:**
- **19,214** DianeClient initializations in current session
- **6 initializations per 5-second interval**
- Creates **~5.8MB log file** in a few hours

**Source:** `DianeClient.swift:64`

**Analysis:**
- StatusMonitor refreshing every 5 seconds (normal)
- But DianeClient is being created 6 times per refresh instead of reusing instances
- This suggests:
  - No singleton pattern or instance caching
  - ViewModels recreating DianeClient on every update
  - Possible memory leaks

### 3. Network Timeout Errors

**Error Pattern:**
```
[ERROR] [DianeClient] curl failed with exit code 28:
[ERROR] [Agents] Failed to test agent 'opencode': Request to /agents/opencode/test failed (exit code: 28)
```

**Exit Code 28:** Operation timeout

**Affected Components:**
- Agent testing (opencode agent)
- MCP Registry loading

---

## Immediate Issues

| Issue | Severity | Impact |
|-------|----------|--------|
| SwiftUI constraint crashes | 🔴 **Critical** | App unusable, crashes multiple times daily |
| Excessive DianeClient creation | 🟡 **High** | Memory leaks, performance degradation, huge logs |
| Network timeouts | 🟢 **Medium** | Some features fail, but not critical |

---

## Recommended Fixes

### Priority 1: Fix SwiftUI Constraint Crashes

**Root Cause:** View update loop causing excessive constraint updates

**Fixes Required:**

1. **Audit `StatusMonitor.swift`** (line 219 - Remote status refresh)
   - Check if status updates trigger view rebuilds
   - Implement debouncing/throttling for status updates
   - Use `@Published` properties carefully to avoid cascading updates

2. **Audit ViewModel implementations:**
   - `AgentsViewModel.swift`
   - `MCPServersViewModel.swift`
   - `MCPRegistryViewModel.swift`
   - `ContextsViewModel.swift`
   - `ProvidersViewModel.swift`
   
   Look for:
   - Computed properties that trigger view updates
   - `objectWillChange.send()` calls in update loops
   - State changes during view rendering

3. **Add SwiftUI debugging:**
   ```swift
   let _ = Self._printChanges() // In View body
   ```

4. **Implement proper cancellation:**
   - Ensure all Combine subscriptions are properly cancelled
   - Use `.store(in: &cancellables)`
   - Cancel on deinit

### Priority 2: Fix DianeClient Initialization Loop

**Root Cause:** DianeClient created on every status check instead of being reused

**Fixes Required:**

1. **Implement Singleton Pattern in `DianeClient.swift`:**
   ```swift
   class DianeClient {
       static let shared = DianeClient()
       
       private init() {
           // Initialize socket connection once
       }
   }
   ```

2. **Update ViewModels to reuse DianeClient:**
   - Replace `let client = DianeClient()` with `DianeClient.shared`
   - Or inject DianeClient as environment object

3. **Reduce DEBUG logging:**
   - Change DianeClient initialization log from DEBUG to TRACE
   - Or remove it entirely
   - Only log on connection errors

### Priority 3: Fix Network Timeouts

**Root Cause:** Agent test endpoints timing out (exit code 28)

**Fixes Required:**

1. **Increase timeout for agent tests:**
   ```swift
   request.timeoutInterval = 30 // Increase from default
   ```

2. **Handle timeouts gracefully:**
   - Don't log ERROR for expected timeouts
   - Show "Testing..." or "Unavailable" in UI instead of errors

3. **Make agent testing optional/async:**
   - Don't block UI on agent tests
   - Test in background, update UI when complete

---

## Monitoring Recommendations

1. **Reduce Log Verbosity:**
   - Move DEBUG logs to TRACE level
   - Set default log level to INFO in production builds
   - Implement log rotation (max 10MB per file)

2. **Add Crash Analytics:**
   - Integrate Sentry or similar crash reporting
   - Send crash reports automatically
   - Track crash-free session rate

3. **Add Performance Metrics:**
   - Track DianeClient creation count
   - Monitor view update frequency
   - Alert if DianeClient created >100 times in 5 minutes

---

## File Locations for Investigation

### Swift Source Files:
- `/root/diane/Diane/Diane/Services/DianeClient.swift` (line 64)
- `/root/diane/Diane/Diane/Services/StatusMonitor.swift` (line 219)
- `/root/diane/Diane/Diane/ViewModels/AgentsViewModel.swift` (line 105)
- `/root/diane/Diane/Diane/ViewModels/MCPRegistryViewModel.swift` (line 142)

### Log Files (on Mac):
- `/Users/mcj/.diane/dianemenu.log` (5.8MB - current)
- `/Users/mcj/.diane/dianemenu.1.log` (170MB - rotated)
- `/Users/mcj/.diane/server.log` (556KB - backend logs)

### Crash Reports (on Mac):
- `/Users/mcj/Library/Logs/DiagnosticReports/Diane-*.ips`

---

## Temporary Workarounds

Until fixes are deployed:

1. **Reduce log file growth:**
   ```bash
   # On Mac.banglab
   echo > ~/.diane/dianemenu.log  # Clear log
   echo > ~/.diane/dianemenu.1.log
   ```

2. **Restart app more frequently:**
   - Restart Diane.app every few hours to prevent crashes
   - Use `launchd` or cron to auto-restart if crashed

3. **Monitor crash rate:**
   ```bash
   ls -lt ~/Library/Logs/DiagnosticReports/Diane-* | wc -l
   ```

---

## Next Steps

1. [ ] Fix DianeClient singleton pattern (easy win, reduces log spam)
2. [ ] Add `_printChanges()` debugging to identify problematic views
3. [ ] Review StatusMonitor update frequency and debounce if needed
4. [ ] Audit all ViewModel update patterns for circular dependencies
5. [ ] Implement proper Combine cancellation
6. [ ] Add integration tests for view update patterns
7. [ ] Set up crash analytics for production monitoring

---

**Status:** 🔴 **Urgent** - App is crashing frequently and logs are growing excessively
