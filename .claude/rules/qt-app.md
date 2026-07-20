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

## ComboBox Enum Submission and Test Coverage

A form ComboBox that submits a proto enum value must read `currentValue` (via `valueRole`), never `currentIndex`. The two coincide — and hide the bug — when a sentinel row sits at index 0 over a contiguous `1..N` enum: the row index then equals the enum value at every row, so a spec asserting the submitted value passes even if the code wrongly submits `currentIndex`.

To keep the value-vs-index distinction testable, prefer the account-selector pattern — `currentIndex: -1` with a placeholder `displayText` and no sentinel row — so a row's index is one less than its enum value. A spec that then asserts the submitted enum value genuinely catches a `currentIndex`-for-`currentValue` bug. Reserve the sentinel-row form for models whose values aren't a contiguous run from 1.

## QML Numeric Property Width

A QML `int` is 32-bit. A value in cents — which the daemon and proto carry as 64-bit `int64` — overflows a QML `int` above ~21.5 million cents (~$214,748.36) and silently records a truncated amount. Type any property holding cents, or any value that crosses into a C++ `qlonglong` / proto `int64` sink, as `var` (a JS number marshals to `qlonglong` exactly for integers up to 2^53), never `int`. The narrowing is silent — no compile error, no runtime warning — so the thing that catches it is a boundary test with an amount past `INT32_MAX`, not code reading.

## Qt Quick Layouts

A `Layout` nested directly inside another `Layout` (e.g. a `ColumnLayout` rail inside a `RowLayout`) defaults `Layout.fillWidth`/`Layout.fillHeight` to **true** — unlike plain Items and controls, which default false. A fixed-size child therefore expands and starves its siblings. To pin a sidebar/rail at a fixed width, set `Layout.fillWidth: false` explicitly plus `Layout.preferredWidth`/`minimumWidth`/`maximumWidth`. Headless tests that assert child existence rather than geometry will not catch this.

NEVER bind a child's `Layout.preferredWidth`/`preferredHeight` to its parent layout's own size (e.g. `Layout.preferredWidth: parent.width * 0.55` inside the `RowLayout` being sized). The parent sizes from the child, so the binding feeds back and Qt aborts the pass with "Detected recursive rearrange. Aborting after two iterations" — a runtime warning headless tests spam but still pass through, so only a runtime log surfaces it. Split the space with `Layout.fillWidth` on the children instead. Same feedback for a `ListView` delegate: give the `ItemDelegate` a fixed `width` (`ListView.view.width`) and size its height from content, rather than nesting a `fillWidth` layout as the delegate's `contentItem`.

## Binding Freshness in Change Handlers

Inside an `onXChanged` handler, a *derived* property that depends on `x` may still hold its pre-change value — the engine does not guarantee the dependent binding is re-evaluated before the explicit handler runs. Read the source property that changed (`x`) directly in the handler; reserve the derived property for ordinary bindings and other handlers, which run after re-evaluation. Seen as: a `ComboBox.onCurrentIndexChanged` handler read a `currentAccountId` derived from `currentIndex`, got the *previous* account, and the first selection out of an unselected state silently did nothing.

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
- **Mock injection.** The shipping component declares `required property var client`; the app injects the real C++ `FinchClient`, tests inject `MockFinchClient`, which records calls and exposes `succeed()`/`fail()` to drive outcomes. When the mock stands in for client-*internal* behavior (re-entrancy guards, request reconciliation), mirror the real client's observable state transitions so specs see realistic states (e.g. the list cleared during an in-flight fetch), not an idealized double.
- **Headless + deterministic style.** Tests run with `QT_QPA_PLATFORM=offscreen` (no display) and a pinned `QT_QUICK_CONTROLS_STYLE=Basic`, set per-test in CMake, so results are reproducible across machines and CI needn't carry other Controls styles' modules.
- **Offscreen input.** The `offscreen` platform does not deliver synthetic mouse events to Qt Quick Controls — `mouseClick()` on a `Button` fires nothing and its `clicked` signal never emits. Drive components through their public API (property writes, `Q_INVOKABLE`/QML functions) or emit the signal directly (`button.clicked()`) to exercise an `onClicked` handler. The existing specs already follow this.
- **Offscreen assertions.** A control's `visible`/`enabled` getter returns *effective* state — false when an ancestor is hidden/disabled or the item is not on a shown window — so under `offscreen` it reads false even when the local binding is true. NEVER assert on a component's `visible`/`enabled` (directly or via a property alias to one); the assertion is unreliable in the harness. Instead expose the underlying condition as a logical `readonly property bool` and assert that, binding the visual `visible`/`enabled` to the same property (e.g. `TransactionsPanel`'s `stateMessageVisible`).

Manual testing requires a running daemon with seed data. The daemon must be built and started separately (`just build-daemon && daemon/finch-daemon`).
