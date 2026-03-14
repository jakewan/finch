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
                text: finchClient.pingInProgress ? "Pinging…" : "Ping Daemon"
                enabled: !finchClient.pingInProgress
                onClicked: finchClient.ping()
            }
        }
    }

    Item {
        anchors.fill: parent

        Label {
            anchors.centerIn: parent
            text: "Chart view coming soon"
            font.pointSize: 12
            color: "gray"
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
