# Antigravity CLI Workspace Context: ADS-B Performance Monitor

## 1. Project Overview & Architecture
This workspace houses a high-performance ADS-B tracking and telemetry dashboard utility written in Go. The system functions as a localized processing engine interacting directly with a local FlightAware tracking stack (`dump1090-fa`).

* **Language & Infrastructure:** Go (Golang) targeting single-board microcomputer architectures (Raspberry Pi Zero series).
* **Performance Goals:** Zero-allocation json unmarshaling, efficient microsecond terminal updates, low overhead, and highly optimized calculation tracking loops.
* **Key Source File:** `stats.go` (Handles structural data parsing, telemetry mathematics, and kernel diagnostic tracking via `/proc`).

---

## 2. Real-World Physical Setup Constraints
When conducting automated code reviews, refactoring, or generating logic plans, the agent **MUST** prioritize and account for the following environmental and physical parameters:

* **Location Profile:** High-density, heavy-interference urban environment.
* **Antenna Profile:** Window deployment with directional line-of-sight constraints. Building infrastructure may obstruct certain headings; low-altitude tracking may rely on multipath reflections off nearby structures.
* **Cable Dynamics:** Low-loss RF extension cable with SMA connectors.
* **Signal Conditioning:** Inline SAW filter for 1090 MHz band isolation.
* **Software Radio Configuration:** Operating with FlightAware Adaptive Dynamic Range enabled (`adaptive-dynamic-range yes`), allowing the system to scale gain dynamically under clean airspace conditions.

---

## 3. Explicit Telemetry Math & Logic Rules
Apply these strict logical corrections during code analysis and refactoring tasks to eliminate diagnostic miscalculations:

### A. Unique Aircraft Tracking Logic
* **Do NOT** use `Local.Accepted[0]` or any index of the `Accepted` array to determine live aircraft counts. `Local.Accepted` represents message bit-length categorization arrays inside `dump1090`, not physical targets.
* **DO** use `Tracks.All` within the active time window (e.g., `s.Last1Min.Tracks.All`) to query the real-time collection of unique aircraft paths currently heard by the tuner.

### B. Preamble & Error Rate Calculations
* **Do NOT** calculate error rates by compounding data fields like `Local.Modes + Local.Bad` in the denominator.
* `Local.Modes` already represents the raw aggregate pool of hardware preamble triggers guessed by the radio slicer (inclusive of message failures).
* **Correct Mathematical Derivation:** $$\text{Error Rate \%} = \left( \frac{\text{Local.Bad}}{\text{Local.Modes}} \right) \times 100$$

### C. Live Real-Time Integration
* Be aware that `stats.json` inside dump1090-fa is throttle-written on a **60-second delay disk buffer**.
* For sub-second or immediate real-time rendering of aircraft coordinates and active paths without minute-long latency, reference the rapid local RAM stream at `/run/dump1090-fa/aircraft.json` which hits 1Hz updates.

---

## 4. AGY Execution Directives
* Maintain the lightweight, highly scannable formatting layout implemented on the frontend telemetry visualization.
* When proposing changes to system calls, always verify low-resource processing overhead to prevent triggering the `vcgencmd get_throttled` under-voltage or frequency-capping limits on low-power Pi hardware boards.