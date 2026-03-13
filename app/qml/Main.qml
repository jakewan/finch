import QtQuick
import QtQuick.Controls

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
            text: finchClient.daemonVersion
                ? "Connected to daemon v" + finchClient.daemonVersion
                : "Not connected to daemon"
            anchors.horizontalCenter: parent.horizontalCenter
            color: finchClient.daemonVersion ? "green" : "red"
        }

        Button {
            text: "Ping Daemon"
            anchors.horizontalCenter: parent.horizontalCenter
            onClicked: finchClient.ping()
        }
    }
}
