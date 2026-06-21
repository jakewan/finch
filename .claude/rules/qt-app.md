---
paths: "app/**/*"
---

# Qt App Development

## QApplication Required

The app MUST use `QApplication`, not `QGuiApplication`. QtCharts QML types depend on QWidgets internally (`QGraphicsView`, `QGraphicsScene`). Using `QGuiApplication` causes a segfault during `ChartView` initialization.

## Qt Version Parity with CI

CI pins an exact Qt version via `jurplel/install-qt-action` (see `.github/workflows/ci-qt.yml`), independent of the runner's distro packages. The pinned version is the **API floor**: code freely to APIs available at that floor, and when raising it, bump the pin deliberately.

Before using a Qt API, check its "since" version annotation against the **pinned floor** (not against Ubuntu `apt`, which is irrelevant now that CI no longer sources Qt from it). A newer-than-floor API means raising the pin first. The local dev install is typically newer than the floor, so it will not catch a floor violation. CI on the pinned version catches *C++-level* violations at compile time, but a too-new *QML* import is not caught by the default build (`qmlcachegen` warns and falls back rather than failing) — verify new QML imports at runtime.

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

Automated tests: `just test-app` configures, builds, and runs the Qt Quick Test suite via `ctest`. Conventions (defined in `app/CMakeLists.txt`):

- **Test-only QML module.** Each form/view is tested through a dedicated module (e.g. `FinchFormTest`) scoped to just the real shipping component plus a mock client (`MockFinchClient.qml`). Scoping the module to those self-contained files — rather than the whole `qml/` directory — keeps the test's runtime dependencies to what the component itself imports, so it does not drag in `Main.qml`/`BalanceChart.qml` and their transitive modules (QtCharts, QtQml.WorkerScript). The component under test is the exact file the app ships, not a stub.
- **Mock injection.** The shipping component declares `required property var client`; the app injects the real C++ `FinchClient`, tests inject `MockFinchClient`, which records calls and exposes `succeed()`/`fail()` to drive outcomes.
- **Headless + deterministic style.** Tests run with `QT_QPA_PLATFORM=offscreen` (no display) and a pinned `QT_QUICK_CONTROLS_STYLE=Basic`, set per-test in CMake, so results are reproducible across machines and CI needn't carry other Controls styles' modules.

Manual testing requires a running daemon with seed data. The daemon must be built and started separately (`just build-daemon && daemon/finch-daemon`).
