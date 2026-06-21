import QtQuick
import QtQuick.Controls
import QtQuick.Layouts

// Content-agnostic navigation container: a persistent left nav rail driving a
// StackLayout content area. It imports no Finch C++ type and no QtCharts, so the
// Qt Quick Test harness can drive it headless with lightweight injected screens —
// the host (Main.qml) composes the real chart/client-bound destinations into it.
//
// Destinations are supplied via the `screens` list and reparented into the StackLayout
// on completion. They are NOT the default property: the rail + content RowLayout is the
// shell's own visual child, and a default property aliased to the stack would capture
// that chrome too and parent it into its own descendant.
Item {
    id: shell

    // Host-supplied destination screens, in declaration order. Reparented into the
    // StackLayout below once the component is complete.
    property list<Item> screens

    // Parallel list of nav-rail labels, one per screen, in declaration order.
    property var destinationTitles: []

    // The active destination. Writing currentIndex switches the visible screen.
    property int currentIndex: 0

    // Number of composed destinations. Exposed so specs (and the mismatch guard) can
    // reason about the stack without reaching into the private contentStack id.
    readonly property int count: contentStack.children.length
    readonly property Item currentDestination:
        currentIndex >= 0 && currentIndex < count
        ? contentStack.children[currentIndex]
        : null

    function _warnOnTitleMismatch() {
        if (destinationTitles.length !== count)
            console.warn("NavigationShell: destinationTitles ("
                         + destinationTitles.length + ") does not match destination count ("
                         + count + ")")
    }

    Component.onCompleted: {
        for (var i = 0; i < screens.length; i++)
            screens[i].parent = contentStack
        _warnOnTitleMismatch()
    }
    onDestinationTitlesChanged: _warnOnTitleMismatch()

    RowLayout {
        anchors.fill: parent
        spacing: 0

        // Nav rail: one button per destination title; the active one is highlighted.
        ColumnLayout {
            Layout.fillHeight: true
            Layout.preferredWidth: 160
            spacing: 4

            Repeater {
                model: shell.destinationTitles
                delegate: Button {
                    required property int index
                    required property string modelData
                    objectName: "navButton" + index
                    text: modelData
                    Layout.fillWidth: true
                    highlighted: index === shell.currentIndex
                    onClicked: shell.currentIndex = index
                }
            }

            Item { Layout.fillHeight: true } // push buttons to the top
        }

        StackLayout {
            id: contentStack
            Layout.fillWidth: true
            Layout.fillHeight: true
            currentIndex: shell.currentIndex
        }
    }
}
