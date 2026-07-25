import QtQuick
import QtQuick.Controls
import QtQuick.Layouts

// Self-contained transaction-recording form. Like CreateAccountForm it depends only on an
// injected `client` (the production FinchClient, a mock in tests) and an injected `accountId`
// — the target account is fed from the Transactions screen's existing selector, so the form
// carries no second account picker. Deliberately importing no Finch C++ type keeps it
// instantiable by the Qt Quick Test harness with a mock client and no daemon.
Item {
    id: form

    // Injected by the host: Main.qml/TransactionsPanel passes finchClient; tests pass a mock.
    required property var client

    // Injected target account. Empty string means no account selected — the form stays
    // invalid, since a transaction must record against an account.
    property string accountId: ""

    // Public, test-facing surface — specs drive the form through these rather than reaching
    // into private control ids. The amount, its sign, and the signed-cents/preview derivation
    // live in the shared SignedAmountField (amountInput); the form forwards its surface.
    property alias nameText: nameField.text
    property alias amountText: amountInput.amountText
    property alias dateText: dateField.text
    property alias descriptionText: descriptionField.text
    property alias errorText: errorLabel.text
    property alias expense: amountInput.expense
    readonly property var signedCents: amountInput.signedCents
    readonly property string previewText: amountInput.previewText

    // canSubmit is the single validation predicate the button and the specs share.
    readonly property bool canSubmit:
        accountId !== ""
        && nameField.text.trim().length > 0
        && amountInput.amountValid
        && !(client && client.recordTransactionInProgress)

    // Emitted once the daemon confirms the record; the host re-fetches the account on it.
    signal recorded(string id)

    implicitWidth: 360
    implicitHeight: layout.implicitHeight

    function submit() {
        if (!canSubmit)
            return
        errorLabel.text = ""
        // A params object, not positional args: named fields make a transpose impossible, and
        // the key names here are the contract the C++ side unpacks.
        client.recordTransaction({
            accountId: accountId,
            date: dateField.text,
            amount: signedCents,
            name: nameField.text.trim(),
            description: descriptionField.text,
            // status 3 == TRANSACTION_STATUS_RECONCILED. #38 records actual events; the
            // Projected/Scheduled statuses belong to the recurring/projection world.
            status: 3
        })
    }

    Connections {
        target: form.client
        function onTransactionRecorded(id) {
            nameField.text = ""
            amountInput.amountText = ""
            descriptionField.text = ""
            errorLabel.text = ""
            form.recorded(id)
        }
        function onTransactionRecordFailed(message) {
            errorLabel.text = message
        }
    }

    ColumnLayout {
        id: layout
        anchors.fill: parent
        spacing: 12

        Label { text: "Name" }
        TextField {
            id: nameField
            Layout.fillWidth: true
            placeholderText: "e.g. Groceries"
            onTextChanged: errorLabel.text = ""
        }

        Label { text: "Amount" }
        SignedAmountField {
            id: amountInput
            Layout.fillWidth: true
            onEdited: errorLabel.text = ""
        }

        Label { text: "Date" }
        TextField {
            id: dateField
            Layout.fillWidth: true
            // Local-calendar "today" by design: the field is user-editable and the daemon
            // takes the date string at face value (no zone conversion), so a local default
            // matches the user's intent.
            text: Qt.formatDate(new Date(), "yyyy-MM-dd")
            // Drop a stale daemon error the moment the user edits any input — consistent with
            // the name/amount fields, so fixing the rejected field clears its message.
            onTextChanged: errorLabel.text = ""
        }

        Label { text: "Description" }
        TextField {
            id: descriptionField
            Layout.fillWidth: true
            placeholderText: "Optional"
            onTextChanged: errorLabel.text = ""
        }

        Label {
            id: errorLabel
            Layout.fillWidth: true
            color: "#c0392b"
            wrapMode: Text.WordWrap
            visible: text.length > 0
        }

        Button {
            id: submitButton
            text: (form.client && form.client.recordTransactionInProgress)
                  ? "Recording…" : "Record transaction"
            enabled: form.canSubmit
            onClicked: form.submit()
        }
    }
}
