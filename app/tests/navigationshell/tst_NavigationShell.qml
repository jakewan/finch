import QtQuick
import QtQuick.Layouts
import QtTest
// FinchShellTest is the test-only module (see app/CMakeLists.txt) holding the real
// shipping NavigationShell plus PlaceholderScreen — scoped to chart-free, client-free
// files so the harness drives the shell headless with lightweight injected screens.
import FinchShellTest

TestCase {
    id: testCase
    name: "NavigationShell"
    when: windowShown
    width: 400
    height: 300

    // The real shell composed with three placeholder destinations — the same shape
    // Main.qml uses, but with chart-free, client-free content.
    Component {
        id: shellFactory
        NavigationShell {
            anchors.fill: parent
            destinationTitles: ["Overview", "Accounts", "Transactions"]
            screens: [
                PlaceholderScreen { Layout.fillWidth: true; Layout.fillHeight: true; headline: "Overview" },
                PlaceholderScreen { Layout.fillWidth: true; Layout.fillHeight: true; headline: "Accounts" },
                PlaceholderScreen { Layout.fillWidth: true; Layout.fillHeight: true; headline: "Transactions" }
            ]
        }
    }

    property var shell: null

    function build() {
        shell = shellFactory.createObject(testCase)
        verify(shell !== null, "shell instantiated")
        waitForRendering(shell)
        return shell
    }

    function cleanup() {
        if (shell) { shell.destroy(); shell = null }
    }

    // The shell opens on the first destination.
    function test_loadsWithDefaultDestination() {
        var s = build()
        compare(s.currentIndex, 0)
        verify(s.currentDestination !== null, "has a current destination")
        compare(s.currentDestination.headline, "Overview")
    }

    // Every title gets a destination — proves host children actually sank into the stack
    // (a non-empty stack), guarding the default-property wiring.
    function test_destinationCountMatchesTitles() {
        var s = build()
        compare(s.count, s.destinationTitles.length)
        compare(s.count, 3)
    }

    // Writing currentIndex switches the active screen — the binding proven without
    // depending on offscreen click geometry.
    function test_navSelectionSwitchesContent() {
        var s = build()
        s.currentIndex = 2
        compare(s.currentDestination.headline, "Transactions")
    }

    // The rail renders exactly one button per destination, each carrying the objectName
    // and label for its index — the wiring a nav click targets. (The literal click→onClicked
    // step is left to manual verification: the offscreen test platform does not deliver
    // synthetic mouse events to Controls buttons, and this harness drives components through
    // their public API rather than simulated input, as the CreateAccountForm spec does.)
    function test_railRendersAButtonPerDestination() {
        var s = build()
        for (var i = 0; i < s.destinationTitles.length; i++) {
            var button = findChild(s, "navButton" + i)
            verify(button !== null, "nav button " + i + " exists")
            compare(button.text, s.destinationTitles[i])
        }
        compare(findChild(s, "navButton" + s.destinationTitles.length), null,
                "no extra nav buttons beyond the destination count")
    }

    // Only the active destination's nav entry is highlighted, before and after a switch.
    function test_activeHighlight() {
        var s = build()
        var b0 = findChild(s, "navButton0")
        var b1 = findChild(s, "navButton1")
        verify(b0.highlighted, "active entry highlighted at start")
        verify(!b1.highlighted, "inactive entry not highlighted at start")

        s.currentIndex = 1
        verify(!b0.highlighted, "previously-active entry no longer highlighted")
        verify(b1.highlighted, "newly-active entry highlighted")
    }

    // Navigating away and back returns the same destination instance (StackLayout keeps
    // siblings alive — guards against an accidental swap to a destroy/recreate container),
    // checked at index 0 and a non-zero index to catch an off-by-one in the indexing.
    function test_returningToDestinationPreservesIt() {
        var s = build()
        var first = s.currentDestination
        s.currentIndex = 2
        var third = s.currentDestination
        verify(third !== first, "different destination after switch")

        s.currentIndex = 0
        compare(s.currentDestination, first)
        s.currentIndex = 2
        compare(s.currentDestination, third)
    }
}
