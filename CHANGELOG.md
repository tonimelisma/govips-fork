# Changelog

## Unreleased

### Added

-   **Fusion Pipeline Proof-of-Concept:** Added a new proof-of-concept to demonstrate the "fusion" of multiple libvips operations (`resize` -> `sharpen`) into a single execution graph. This is a foundational step for building a more advanced, high-level processing pipeline in the future. The implementation lives in the following new files:
    -   `vips/fusion_poc.go`
    -   `vips/fusion_poc_test.go`
    -   `vips/gvalue_helpers.h`

### Development & Debugging Summary

This increment of work focused on validating a core libvips feature. The process involved significant debugging and refactoring to align with the project's existing coding patterns for `cgo`.

-   **Issues Encountered & Solved:**
    1.  **`cgo` in Test Files:** The initial implementation placed `cgo` code in a `_test.go` file, which is not supported by the build system in this configuration. This was resolved by refactoring the code into a `fusion_poc.go` file for the `cgo` logic and a corresponding `fusion_poc_test.go` for the Go test.
    2.  **Incorrect Method Signatures:** The test code initially used incorrect arguments for the `Sharpen`, `Subtract`, and `Stats` methods. This was fixed by inspecting the codebase to find the correct signatures and replacing the call to `Stats()` with the more appropriate `Average()` method.
    3.  **Build System Oddities:** The Go build system proved sensitive to how C files were included. An initial implementation using separate `.h` and `.c` files for `GValue` helpers failed to compile. The issue was resolved by refactoring the C code to use `static inline` functions within the header file (`gvalue_helpers.h`), precisely matching the pattern used by the existing `spike_poc.go`.

-   **Technical Debt & Risks:**
    -   The reliance on specific `cgo` patterns (such as including C implementation in `.h` files) makes the build process somewhat brittle and potentially confusing for new contributors. A more robust and standardized `cgo` build strategy may be needed in the future.

-   **Final Blocker (Unknown):**
    -   The proof-of-concept is **unverified** due to a persistent, phantom build error. The compiler reports `gpointer` type mismatch errors (`cannot use ... as _Ctype_gpointer`) despite the code on disk being verifiably correct and matching the project's working patterns. This suggests a deep-seated issue with the build toolchain or its cache that could not be resolved by cleaning the cache or re-applying code changes. The implemented code has been committed but should be considered non-functional until this external build issue is resolved. 