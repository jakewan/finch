import QtQuick
import QtQuick.Controls
import Finch 1.0

ApplicationWindow {
    visible: true
    width: 640
    height: 480
    title: "Finch"

    Column {
        anchors.centerIn: parent
        spacing: 16

        Label {
            text: "Finch — Personal Finance"
            font.pixelSize: 24
            anchors.horizontalCenter: parent.horizontalCenter
        }

        Label {
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
            anchors.horizontalCenter: parent.horizontalCenter
        }

        Button {
            text: finchClient.pingInProgress ? "Pinging…" : "Ping Daemon"
            enabled: !finchClient.pingInProgress
            anchors.horizontalCenter: parent.horizontalCenter
            onClicked: finchClient.ping()
        }
    }
}
