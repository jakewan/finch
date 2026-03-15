# Qt App Development

## QApplication Required

The app MUST use `QApplication`, not `QGuiApplication`. QtCharts QML types depend on QWidgets internally (`QGraphicsView`, `QGraphicsScene`). Using `QGuiApplication` causes a segfault during `ChartView` initialization.

## Qt Version Parity with CI

CI installs Qt from Ubuntu `apt` packages, which lags behind rolling-release or developer installs. ALWAYS check "since" version annotations in Qt docs before using newer APIs.

When a Qt API has a deprecated form that works across all Qt6 versions and a replacement that requires a newer version, prefer the cross-compatible form for CI parity. A local deprecation warning is acceptable; a CI build failure is not.

## QML Property Names

Qt Quick Controls types mark many properties as FINAL. NEVER declare custom properties that shadow built-in names (e.g., `currentValue` on `ComboBox`). The QML engine will error with "Cannot override FINAL property" at runtime.

## Build and Test

Build: `just build-app` (or `cmake -S app -B app/build && cmake --build app/build`).

Manual testing requires a running daemon with seed data. The daemon must be built and started separately (`just build-daemon && daemon/finch-daemon`).
