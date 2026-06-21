import QtQuick
import QtQuick.Controls
import QtQuick.Layouts

// Accounts destination: the account-creation form inline above a read-only list of
// existing accounts. Like CreateAccountForm it imports no Finch C++ type — the host
// injects the client and the connection gate — so it stays instantiable without a daemon.
Item {
    id: panel

    // Injected by the host: Main.qml passes finchClient.
    required property var client

    // Whether account creation is currently allowed (the host gates this on a live
    // daemon connection). The form never knew about connection state; the gate used to
    // live on the toolbar button that opened the now-removed modal dialog.
    property bool createEnabled: true

    ColumnLayout {
        anchors.fill: parent
        anchors.margins: 16
        spacing: 16

        CreateAccountForm {
            id: form
            Layout.fillWidth: true
            client: panel.client
            enabled: panel.createEnabled
        }

        Label {
            Layout.fillWidth: true
            visible: !panel.createEnabled
            text: "Connect to the daemon to add accounts."
            color: "gray"
        }

        Label {
            text: "Accounts"
            font.bold: true
        }

        ListView {
            id: accountList
            Layout.fillWidth: true
            Layout.fillHeight: true
            clip: true
            model: panel.client ? panel.client.accounts : []
            delegate: ItemDelegate {
                required property var modelData
                width: ListView.view.width
                text: modelData.name
            }
        }

        Label {
            Layout.fillWidth: true
            visible: accountList.count === 0
            text: "No accounts yet."
            color: "gray"
        }
    }
}
