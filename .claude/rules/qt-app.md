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

To keep the value-vs-index distinction testable, prefer the account-selector pattern — `currentIndex: -1` with a placeholder `displayText` and no sentinel row — so a row's index is one less than its enum value. A spec that then asserts the submitted enum value genuinely catches a `currentIndex`-for-`currentValue` bug.

Use the sentinel-row form when "none" must be **re-selectable**: a form that persists across navigation needs it, because with only a placeholder at `currentIndex: -1` a mis-click into a real row cannot be undone without submitting or restarting the app. Values that aren't a contiguous run from 1 are a secondary reason to reach for it. Either way, when the model is also a filtered view of a source list, see § *Sentinel-Prefixed Filtered Models* — the sentinel there introduces an offset coincidence distinct from the index-equals-value one above.

## Sentinel-Prefixed Filtered Models: Catching an Index-for-Value Read

A ComboBox whose model prepends a sentinel row to a source list with one element filtered out — the "none of these" shape, such as a transfer-destination picker that excludes the source account — submits an identity rather than an enum. The bug to catch is a row's `currentIndex` used to look up the *unfiltered* source list — `source[currentIndex]` — where `currentValue` was wanted: the index belongs to the filtered model, so the lookup yields a real but wrong identity.

**Assert that the sentinel row submits no identity.** That catches the bug whichever element the fixture filters out: at the sentinel the correct read yields the empty value while an unfiltered-source index read yields the first element's identity, so the two always differ. This is the cheap, fixture-independent guard — and a spec covering the "none selected" case already carries it if it asserts the submitted identity is empty.

If you also assert on a **real** row, the fixture decides whether that assertion can fail. Assuming that read, and the filtered-out element sitting at source index *s*, real row *i* holds source element `i-1` when `i-1 < s` and element `i` otherwise, while the buggy read returns element `i` throughout — so the two coincide wherever `i > s`:

- **Filter out the last element** and every real row diverges.
- Filter out the **first** and no real row diverges, so an assertion there cannot fail whichever real row it drives.
- Filter out a middle element and rows 1 through *s* inclusive diverge, none above. With three source elements and the middle one filtered out that means row 1 only, and an assertion on the last row proves nothing.

Prefer the last-element fixture over re-deriving this. Only where the scenario fixes the filtered-out element's position, tabulate row → identity for the correct read and for the read you suspect, then assert where they differ.

A wrong identity is silently acceptable only when it lands on some *other* account: this project's daemon and core both reject a transfer whose destination equals its source, so a buggy read returning the filtered-out element surfaces as `InvalidArgument` rather than a bad rule.

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

**Check which daemon build you are actually talking to before trusting a manual result.** The app and the daemon are separate binaries with separate release paths, so a freshly built app routinely talks to a much older daemon — most easily when the daemon runs as the installed systemd user service rather than from the build directory. Any app behavior that depends on a daemon-side or core-side change then fails for a reason that has nothing to do with the app change under test, and it fails looking exactly like an app bug.

Two things make this quieter than it sounds:

- `just install-service` ends in `systemctl --user enable --now`, which starts an inactive service but does **not** restart an active one. Combined with the recipe's deliberate cp+mv (which lets the install succeed while the old binary runs, since the running process keeps its file descriptor to the replaced inode), a service that was already up keeps serving the **previous** build after a successful-looking install. Restart it explicitly:

    ```bash
    just install-service
    systemctl --user restart finch.service
    ```

- The unit is named `finch.service`, not `finch-daemon` — so a status check on the latter reports "inactive" for a daemon that is running fine.

Compare the installed binary's timestamp against the commits the behavior under test depends on (`stat -c %y ~/.local/bin/finch-daemon`), and reinstall before concluding anything about the app.
