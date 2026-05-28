# Qt App Development

## QApplication Required

The app MUST use `QApplication`, not `QGuiApplication`. QtCharts QML types depend on QWidgets internally (`QGraphicsView`, `QGraphicsScene`). Using `QGuiApplication` causes a segfault during `ChartView` initialization.

## Qt Version Parity with CI

CI installs Qt from Ubuntu `apt` packages, which lags behind rolling-release or developer installs. ALWAYS check "since" version annotations in Qt docs before using newer APIs.

When a Qt API has a deprecated form that works across all Qt6 versions and a replacement that requires a newer version, prefer the cross-compatible form for CI parity. A local deprecation warning is acceptable; a CI build failure is not.

## QML Property Names

Qt Quick Controls types mark many properties as FINAL. NEVER declare custom properties that shadow built-in names (e.g., `currentValue` on `ComboBox`). The QML engine will error with "Cannot override FINAL property" at runtime.

## Qt Charts Gotchas

**Theme on dark desktops:** ChartView's default theme renders transparent/invisible on dark desktop themes. ALWAYS set an explicit theme (e.g., `theme: ChartView.ChartThemeLight`).

**Anchor series for axis initialization:** Axes declared as ChartView children without a static series may not initialize properly for dynamically created series. Include a hidden anchor `LineSeries` that references the axes (`visible: false`) to ensure they register with the chart.

**Dynamic series lifecycle:** `ChartView.removeSeries()` detaches a series but does not destroy it. ALWAYS call `series.destroy()` after removal to avoid memory leaks.

**`attachAxis` not available in QML:** `QAbstractSeries::attachAxis` is not exposed on `DeclarativeLineSeries`. Pass axes as arguments to `ChartView.createSeries(type, name, axisX, axisY)` instead.

## Desktop Integration (Wayland)

On Wayland the compositor does not let a client set its own taskbar/window icon — it derives the icon from the window's `app_id`, maps that to a `.desktop` file, and reads that file's `Icon=` field. Qt takes the `app_id` from `QGuiApplication::setDesktopFileName()`; when unset it defaults to the executable name.

So the taskbar/window icon resolves only when three names line up:

- the advertised `app_id` — `setDesktopFileName("finch")` in `main.cpp`;
- the installed desktop entry filename — `finch.desktop`;
- that entry's `Icon=` value — `Icon=finch`, resolved against a themed icon (e.g. `hicolor/scalable/apps/finch.svg`).

The binary is `finch-app`, so the default `app_id` (`finch-app`) would not match `finch.desktop` — set it explicitly. `setWindowIcon(QIcon::fromTheme("finch"))` covers the in-window and X11 icon.

After installing or changing a desktop entry or icon, a running Plasma session may keep a stale in-memory cache; a logout/login is the reliable refresh. Avoid restarting plasmashell as a shortcut — on a Wayland session it can destabilize the whole session.

## Build and Test

Build: `just build-app` (or `cmake -S app -B app/build && cmake --build app/build`).

Manual testing requires a running daemon with seed data. The daemon must be built and started separately (`just build-daemon && daemon/finch-daemon`).
