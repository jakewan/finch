import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import Qt.labs.settings
import Finch 1.0

ApplicationWindow {
    id: window
    visible: true
    width: 1024
    height: 768
    minimumWidth: 640
    minimumHeight: 480
    title: "Finch"

    Settings {
        property alias x: window.x
        property alias y: window.y
        property alias width: window.width
        property alias height: window.height
    }

    header: ToolBar {
        RowLayout {
            anchors.fill: parent
            anchors.leftMargin: 8
            anchors.rightMargin: 8

            Label {
                text: "Finch"
                font.pointSize: 14
            }

            Item { Layout.fillWidth: true }

            Button {
                text: "New Account"
                // Account creation needs a live daemon to write to.
                enabled: finchClient.connectionState === FinchClient.Connected
                onClicked: createAccountDialog.open()
            }

            Button {
                text: finchClient.pingInProgress ? "Pinging…" : "Ping Daemon"
                enabled: !finchClient.pingInProgress
                onClicked: finchClient.ping()
            }
        }
    }

    Connections {
        target: finchClient
        function onConnectionStateChanged() {
            if (finchClient.connectionState === FinchClient.Connected) {
                finchClient.listAccounts()
            }
        }
    }

    Item {
        anchors.fill: parent

        BalanceChart {
            anchors.fill: parent
            anchors.margins: 16
            visible: finchClient.connectionState === FinchClient.Connected
                     || !finchClient.timeSeriesEmpty
        }

        Label {
            anchors.centerIn: parent
            visible: finchClient.connectionState === FinchClient.Disconnected
                     && finchClient.timeSeriesEmpty
            text: "Not connected to daemon"
            font.pointSize: 12
            color: "gray"
        }

        BusyIndicator {
            anchors.centerIn: parent
            running: finchClient.pingInProgress
                     && finchClient.timeSeriesEmpty
        }
    }

    Dialog {
        id: createAccountDialog
        title: "New Account"
        anchors.centerIn: Overlay.overlay
        modal: true
        width: 420
        // The embedded form carries its own Create button; no dialog standard buttons.
        standardButtons: Dialog.NoButton
        // Don't let Escape/click-outside dismiss the dialog mid-create — a failure's
        // error is shown inside the form, so closing it would swallow that feedback.
        closePolicy: finchClient.createAccountInProgress
                     ? Popup.NoAutoClose
                     : (Popup.CloseOnEscape | Popup.CloseOnPressOutside)

        CreateAccountForm {
            id: createAccountForm
            width: parent.width
            client: finchClient
            onCreated: createAccountDialog.close()
        }

        // Start each opening from a clean slate.
        onOpened: {
            createAccountForm.nameText = ""
            createAccountForm.typeIndex = 0
            createAccountForm.errorText = ""
        }
    }

    footer: ToolBar {
        RowLayout {
            anchors.fill: parent
            anchors.leftMargin: 8
            anchors.rightMargin: 8

            Rectangle {
                width: 8
                height: 8
                radius: 4
                color: {
                    switch (finchClient.connectionState) {
                    case FinchClient.Connecting:
                        return "gray"
                    case FinchClient.Connected:
                        return "green"
                    default:
                        return "red"
                    }
                }
            }

            Label {
                font.pointSize: 10
                text: {
                    switch (finchClient.connectionState) {
                    case FinchClient.Connecting:
                        return "Connecting to daemon…"
                    case FinchClient.Connected:
                        return "Connected to daemon v" + finchClient.daemonVersion
                    default:
                        return "Not connected to daemon"
                    }
                }
            }
        }
    }
}
