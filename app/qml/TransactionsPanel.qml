import QtQuick
import QtQuick.Controls
import QtQuick.Layouts

// Transactions destination: a per-account master-detail view. An account selector drives a
// per-account fetch; the master list shows one row per transaction; selecting a row fills
// the detail pane. Like AccountsPanel it imports no Finch C++ type — the host injects the
// client and the connection gate — so it stays instantiable without a daemon.
//
// Master-detail interaction is binding-driven, not click-driven: the detail pane binds to
// the list's currentIndex, so a row "click" is just a currentIndex write. This is what lets
// the offscreen test harness (which delivers no synthetic mouse events) drive selection.
Item {
    id: panel

    // Injected by the host: Main.qml passes finchClient.
    required property var client

    // Whether the daemon is connected (host-gated). When false the panel shows a connect
    // hint instead of an account dropdown the user can't populate.
    property bool connected: true

    // The account list backing the selector; the transaction list backing the master view.
    readonly property var accounts: client ? client.accounts : []
    readonly property var transactions: client ? client.transactions : []

    // The currently selected transaction, or null when nothing valid is selected. Null-guards
    // the detail pane against transactions[-1] / an index past the end of a shorter new list.
    readonly property var selectedTransaction:
        (list.currentIndex >= 0 && list.currentIndex < transactions.length)
            ? transactions[list.currentIndex]
            : null

    // Whether the contextual state message is showing (no rows, not mid-fetch). A logical
    // property, not an alias to the Label's `visible`: QML's visible getter returns effective
    // visibility (ancestor- and window-dependent), which is unreliable under the offscreen
    // test harness, so specs assert this condition directly.
    readonly property bool stateMessageVisible:
        list.count === 0 && !(client && client.transactionsLoading)

    // Test/host surface: selection and detail field values are read back through these.
    property alias selectedAccountIndex: accountCombo.currentIndex
    property alias transactionList: list
    property alias stateMessageText: stateLabel.text
    property alias detailDateText: detailDate.text
    property alias detailNameText: detailName.text
    property alias detailAmountText: detailAmount.text
    property alias detailStatusText: detailStatus.text
    property alias detailDescriptionText: detailDescription.text
    property alias detailRecurringRuleText: detailRecurringRule.text

    // TransactionStatus enum (proto int 0-3) -> human label. No app-wide status map exists
    // yet; defined inline here, mirroring how CreateAccountForm hardcodes its AccountType map.
    function statusLabel(status) {
        switch (status) {
        case 1: return "Projected"
        case 2: return "Scheduled"
        case 3: return "Reconciled"
        default: return "Unspecified"
        }
    }

    // Signed int64 cents -> sign-correct currency. Math.abs keeps the "-" ahead of the "$"
    // ("-$12.34", not "$-12.34").
    // TODO: lift this (and statusLabel) into a shared helper once #38's record form needs it.
    function formatAmount(cents) {
        return (cents < 0 ? "-$" : "$") + Math.abs(cents / 100).toFixed(2)
    }

    function onAccountSelected() {
        // Drop any prior detail selection: the incoming list is a different account's.
        list.currentIndex = -1
        if (accountCombo.currentIndex >= 0 && client) {
            var account = accounts[accountCombo.currentIndex]
            if (account)
                client.listTransactions(account.id)
        }
    }

    // New data (even for the same account) invalidates the prior row index, which could now
    // point past the end of a shorter list. Clear it so the detail pane null-guards cleanly.
    Connections {
        target: panel.client
        function onTransactionsChanged() { list.currentIndex = -1 }
    }

    ColumnLayout {
        anchors.fill: parent
        anchors.margins: 16
        spacing: 16

        ComboBox {
            id: accountCombo
            Layout.fillWidth: true
            // Mirror AccountsPanel's null-guard so the panel is instantiable without a client.
            model: panel.client ? panel.client.accounts : []
            textRole: "name"
            // Start unselected: index 0 of client.accounts is a REAL account (unlike a
            // form-owned type list with a prependable sentinel), so a placeholder displayText
            // is the only way to offer an explicit "no account chosen" state and avoid an
            // auto-fetch on load.
            currentIndex: -1
            displayText: currentIndex < 0 ? "Select an account…" : currentText
            onCurrentIndexChanged: panel.onAccountSelected()
        }

        RowLayout {
            Layout.fillWidth: true
            Layout.fillHeight: true
            spacing: 16

            // Master: one row per transaction.
            ListView {
                id: list
                Layout.fillWidth: true
                Layout.fillHeight: true
                clip: true
                currentIndex: -1
                model: panel.transactions
                delegate: ItemDelegate {
                    id: row
                    required property int index
                    required property var modelData
                    width: ListView.view.width
                    // Size height from the row's own content. The RowLayout is anchored by
                    // width (to the fixed list width) but NOT stretched vertically, so its
                    // implicitHeight is independent of the delegate height — without that
                    // separation the layout feeds its size back into the delegate and Qt
                    // aborts with "recursive rearrange".
                    implicitHeight: rowContent.implicitHeight + 12
                    highlighted: ListView.isCurrentItem
                    // Production affordance only; the detail pane is driven by the
                    // currentIndex binding, not this handler (offscreen tests can't click).
                    onClicked: list.currentIndex = row.index
                    RowLayout {
                        id: rowContent
                        anchors.left: parent.left
                        anchors.right: parent.right
                        anchors.verticalCenter: parent.verticalCenter
                        anchors.leftMargin: 8
                        anchors.rightMargin: 8
                        spacing: 12
                        Label { text: row.modelData.date }
                        Label {
                            Layout.fillWidth: true
                            elide: Text.ElideRight
                            text: row.modelData.name
                        }
                        Label { text: panel.statusLabel(row.modelData.status) }
                        Label { text: panel.formatAmount(row.modelData.amount) }
                    }
                }
            }

            // Detail: the selected transaction's full record, or a placeholder.
            ColumnLayout {
                Layout.fillWidth: true
                Layout.fillHeight: true
                Layout.alignment: Qt.AlignTop
                spacing: 8

                Label {
                    visible: panel.selectedTransaction === null
                    text: "Select a transaction to see its details."
                    color: "gray"
                }

                GridLayout {
                    visible: panel.selectedTransaction !== null
                    columns: 2
                    columnSpacing: 12
                    rowSpacing: 4

                    Label { text: "Date"; font.bold: true }
                    Label { id: detailDate; text: panel.selectedTransaction ? panel.selectedTransaction.date : "" }

                    Label { text: "Name"; font.bold: true }
                    Label { id: detailName; text: panel.selectedTransaction ? panel.selectedTransaction.name : "" }

                    Label { text: "Amount"; font.bold: true }
                    Label {
                        id: detailAmount
                        text: panel.selectedTransaction ? panel.formatAmount(panel.selectedTransaction.amount) : ""
                    }

                    Label { text: "Status"; font.bold: true }
                    Label {
                        id: detailStatus
                        text: panel.selectedTransaction ? panel.statusLabel(panel.selectedTransaction.status) : ""
                    }

                    Label { text: "Description"; font.bold: true }
                    Label {
                        id: detailDescription
                        Layout.fillWidth: true
                        wrapMode: Text.WordWrap
                        text: panel.selectedTransaction ? panel.selectedTransaction.description : ""
                    }

                    Label { text: "Recurring rule"; font.bold: true }
                    Label {
                        id: detailRecurringRule
                        text: (panel.selectedTransaction && panel.selectedTransaction.recurringRuleId)
                              ? panel.selectedTransaction.recurringRuleId
                              : "—"
                    }
                }
            }
        }

        // Single contextual message that explains an empty master view: disconnected,
        // no account chosen, or an account with no transactions.
        Label {
            id: stateLabel
            Layout.fillWidth: true
            color: "gray"
            visible: panel.stateMessageVisible
            text: {
                if (!panel.connected)
                    return "Connect to the daemon to view transactions."
                if (accountCombo.currentIndex < 0)
                    return "Select an account to view its transactions."
                return "No transactions for this account."
            }
        }
    }
}
