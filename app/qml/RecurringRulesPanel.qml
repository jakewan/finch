import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import "txformat.js" as TxFormat

// Recurring Rules destination: a per-account master-detail view, mirroring TransactionsPanel.
// An account selector drives a per-account fetch; an inline create form sits above the list;
// the master list shows one row per rule; selecting a row fills the detail pane. Imports no
// Finch C++ type — the host injects the client and the connection gate — so it stays
// instantiable without a daemon. Selection is binding-driven (a currentIndex write), so the
// offscreen test harness can drive it.
Item {
    id: panel

    // Injected by the host: Main.qml passes finchClient.
    required property var client

    // Whether the daemon is connected (host-gated). When false the account list is empty and
    // the selector is disabled; the state message shows a connect hint.
    property bool connected: true

    readonly property var accounts: client ? client.accounts : []
    readonly property var rules: client ? client.recurringRules : []

    // The id of the currently selected account, or "" when none is chosen. Single source for
    // both the create form's injected accountId and the post-create re-fetch.
    readonly property string currentAccountId:
        (client && accountCombo.currentIndex >= 0 && accounts[accountCombo.currentIndex])
            ? accounts[accountCombo.currentIndex].id
            : ""

    // The currently selected rule, or null. Null-guards the detail pane against rules[-1] or an
    // index past the end of a shorter new list.
    readonly property var selectedRule:
        (list.currentIndex >= 0 && list.currentIndex < rules.length)
            ? rules[list.currentIndex]
            : null

    // Whether the contextual state message is showing (no rows, not mid-fetch). A logical
    // property, not an alias to the Label's visible (which is unreliable offscreen).
    readonly property bool stateMessageVisible:
        list.count === 0 && !(client && client.recurringRulesLoading)

    // Test/host surface.
    property alias selectedAccountIndex: accountCombo.currentIndex
    property alias ruleList: list
    property alias stateMessageText: stateLabel.text
    property alias detailNameText: detailName.text
    property alias detailAmountText: detailAmount.text
    property alias detailFrequencyText: detailFrequency.text
    property alias detailStartDateText: detailStartDate.text
    property alias detailEndDateText: detailEndDate.text
    property alias detailScheduleText: detailSchedule.text
    property alias detailStatusText: detailStatus.text
    property alias createForm: createRuleForm

    // Human-readable schedule for a rule's frequency-conditional day fields.
    function scheduleText(rule) {
        if (!rule)
            return ""
        if (rule.frequency === 4)
            return "Day " + rule.dayOfMonth
        if (rule.frequency === 3 && rule.semiMonthlyDays && rule.semiMonthlyDays.length === 2)
            return "Days " + rule.semiMonthlyDays[0] + " & " + rule.semiMonthlyDays[1]
        return "—"
    }

    function onAccountSelected() {
        // Drop any prior detail selection: the incoming list is a different account's.
        list.currentIndex = -1
        // Read the freshly-changed index directly rather than the derived currentAccountId:
        // inside this handler that binding can still hold its pre-change value.
        if (accountCombo.currentIndex >= 0 && client) {
            var account = accounts[accountCombo.currentIndex]
            if (account)
                client.listRecurringRules(account.id)
        }
    }

    // New data (even for the same account) invalidates the prior row index. Clear it so the
    // detail pane null-guards cleanly.
    Connections {
        target: panel.client
        function onRecurringRulesChanged() { list.currentIndex = -1 }
    }

    ColumnLayout {
        anchors.fill: parent
        anchors.margins: 16
        spacing: 16

        ComboBox {
            id: accountCombo
            Layout.fillWidth: true
            enabled: panel.connected
            model: panel.accounts
            textRole: "name"
            // Start unselected: index 0 is a real account, so a placeholder displayText is the
            // only way to offer "no account chosen" and avoid an auto-fetch on load.
            currentIndex: -1
            displayText: currentIndex < 0 ? "Select an account…" : currentText
            onCurrentIndexChanged: panel.onAccountSelected()
        }

        // Create-rule form, seated above the master-detail content and fed by the selector above
        // it (no second account picker). Disabled until connected and an account is chosen.
        CreateRecurringRuleForm {
            id: createRuleForm
            Layout.fillWidth: true
            client: panel.client
            accountId: panel.currentAccountId
            enabled: panel.connected && panel.currentAccountId !== ""
            // The daemon persists before recurringRuleCreated fires, so re-fetching the account
            // here brings the new row in; the onRecurringRulesChanged Connection clears selection.
            onCreated: {
                if (panel.currentAccountId !== "")
                    panel.client.listRecurringRules(panel.currentAccountId)
            }
        }

        RowLayout {
            Layout.fillWidth: true
            Layout.fillHeight: true
            spacing: 16

            // Master: one row per rule.
            ListView {
                id: list
                Layout.fillWidth: true
                Layout.fillHeight: true
                clip: true
                currentIndex: -1
                model: panel.rules
                delegate: ItemDelegate {
                    id: row
                    required property int index
                    required property var modelData
                    width: ListView.view.width
                    implicitHeight: rowContent.implicitHeight + 12
                    highlighted: ListView.isCurrentItem
                    onClicked: list.currentIndex = row.index
                    RowLayout {
                        id: rowContent
                        anchors.left: parent.left
                        anchors.right: parent.right
                        anchors.verticalCenter: parent.verticalCenter
                        anchors.leftMargin: 8
                        anchors.rightMargin: 8
                        spacing: 12
                        Label {
                            Layout.fillWidth: true
                            elide: Text.ElideRight
                            text: row.modelData.paused
                                  ? row.modelData.name + " (paused)"
                                  : row.modelData.name
                        }
                        Label { text: TxFormat.frequencyLabel(row.modelData.frequency) }
                        Label { text: TxFormat.formatAmount(row.modelData.amount) }
                    }
                }
            }

            // Detail: the selected rule's full record, or a placeholder.
            ColumnLayout {
                Layout.fillWidth: true
                Layout.fillHeight: true
                Layout.alignment: Qt.AlignTop
                spacing: 8

                Label {
                    visible: panel.selectedRule === null
                    text: "Select a rule to see its details."
                    color: "gray"
                }

                GridLayout {
                    visible: panel.selectedRule !== null
                    columns: 2
                    columnSpacing: 12
                    rowSpacing: 4

                    Label { text: "Name"; font.bold: true }
                    Label { id: detailName; text: panel.selectedRule ? panel.selectedRule.name : "" }

                    Label { text: "Amount"; font.bold: true }
                    Label {
                        id: detailAmount
                        text: panel.selectedRule ? TxFormat.formatAmount(panel.selectedRule.amount) : ""
                    }

                    Label { text: "Frequency"; font.bold: true }
                    Label {
                        id: detailFrequency
                        text: panel.selectedRule ? TxFormat.frequencyLabel(panel.selectedRule.frequency) : ""
                    }

                    Label { text: "Schedule"; font.bold: true }
                    Label { id: detailSchedule; text: panel.scheduleText(panel.selectedRule) }

                    Label { text: "Start date"; font.bold: true }
                    Label { id: detailStartDate; text: panel.selectedRule ? panel.selectedRule.startDate : "" }

                    Label { text: "End date"; font.bold: true }
                    Label {
                        id: detailEndDate
                        text: (panel.selectedRule && panel.selectedRule.endDate)
                              ? panel.selectedRule.endDate
                              : "—"
                    }

                    Label { text: "Status"; font.bold: true }
                    Label {
                        id: detailStatus
                        text: !panel.selectedRule ? ""
                              : (panel.selectedRule.paused ? "Paused" : "Active")
                    }
                }
            }
        }

        // Single contextual message explaining an empty master view.
        Label {
            id: stateLabel
            Layout.fillWidth: true
            color: "gray"
            visible: panel.stateMessageVisible
            text: {
                if (!panel.connected)
                    return "Connect to the daemon to view recurring rules."
                if (accountCombo.currentIndex < 0)
                    return "Select an account to view its recurring rules."
                return "No recurring rules for this account."
            }
        }
    }
}
