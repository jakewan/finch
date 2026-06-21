import QtQuick
import QtQuick.Controls

// Stand-in for a navigation destination whose real screen does not exist yet
// (Transactions, Recurring Rules). It carries no client and imports nothing app-specific,
// so it doubles as lightweight content for the NavigationShell spec.
Item {
    id: screen

    property string headline: ""

    Label {
        anchors.centerIn: parent
        text: screen.headline === "" ? "Coming soon" : screen.headline + " — coming soon"
        font.pointSize: 14
        color: "gray"
    }
}
