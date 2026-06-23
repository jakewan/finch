import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import "txformat.js" as TxFormat

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
    // into private control ids.
    property alias nameText: nameField.text
    property alias amountText: amountField.text
    property alias dateText: dateField.text
    property alias descriptionText: descriptionField.text
    property alias errorText: errorLabel.text
    // Expense (true, default) vs Income. The single sign source: the magnitude field cannot
    // express a sign, so this toggle alone decides whether cents are negative or positive.
    property bool expense: true

    // Derived state — one set of predicates shared by canSubmit, the preview, and the button,
    // so they can never disagree at a boundary.
    // parseFloat is NaN when the field is blank or non-numeric; the regex validator already
    // forbids a sign or a comma decimal, so parseFloat reads the magnitude locale-cleanly.
    readonly property real magnitude: parseFloat(amountField.text)
    // Reject 0 and NaN: a $0 manual transaction is meaningless, and the daemon does not
    // validate amount at all, so this form is the sole gate on amount quality.
    readonly property bool amountValid: !isNaN(magnitude) && magnitude > 0
    // Math.round avoids float drift (12.34 * 100 -> 1233.9999...); the toggle applies the sign.
    readonly property int signedCents: amountValid ? Math.round(magnitude * 100) * (expense ? -1 : 1) : 0
    readonly property string previewText: amountValid ? TxFormat.formatAmount(signedCents) : ""

    // canSubmit is the single validation predicate the button and the specs share.
    readonly property bool canSubmit:
        accountId !== ""
        && nameField.text.trim().length > 0
        && amountValid
        && !(client && client.recordTransactionInProgress)

    // Emitted once the daemon confirms the record; the host re-fetches the account on it.
    signal recorded(string id)

    implicitWidth: 360
    implicitHeight: layout.implicitHeight

    function submit() {
        if (!canSubmit)
            return
        errorLabel.text = ""
        // status 3 == TRANSACTION_STATUS_RECONCILED. #38 records actual events; the
        // Projected/Scheduled statuses belong to the recurring/projection world.
        client.recordTransaction(accountId, dateField.text, signedCents,
                                 nameField.text.trim(), descriptionField.text, 3)
    }

    Connections {
        target: form.client
        function onTransactionRecorded(id) {
            nameField.text = ""
            amountField.text = ""
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
        RowLayout {
            Layout.fillWidth: true
            spacing: 12

            TextField {
                id: amountField
                Layout.fillWidth: true
                placeholderText: "0.00"
                inputMethodHints: Qt.ImhFormattedNumbersOnly
                // Locale-independent: always "." (matching parseFloat), forbids a leading "-"
                // so the Expense/Income toggle is the sole sign source, caps at 2 decimals.
                // A default-locale DoubleValidator would accept "19,99", which parseFloat
                // truncates to 19 — the regex avoids that mismatch.
                validator: RegularExpressionValidator { regularExpression: /^\d+(\.\d{0,2})?$/ }
                onTextChanged: errorLabel.text = ""
            }

            // Live signed-amount preview: the sign is visible before submit, so the user sees
            // an Expense will subtract before committing it.
            Label {
                text: form.previewText
                color: form.expense ? "#c0392b" : "#27ae60"
                visible: form.previewText.length > 0
            }
        }

        // Expense/Income toggle. The magnitude field cannot carry a sign, so this is the only
        // way the user expresses direction — a control that can't express the wrong value.
        RowLayout {
            spacing: 12
            ButtonGroup { id: signGroup }
            RadioButton {
                id: expenseRadio
                text: "Expense"
                ButtonGroup.group: signGroup
                checked: form.expense
                onClicked: { form.expense = true; errorLabel.text = "" }
            }
            RadioButton {
                id: incomeRadio
                text: "Income"
                ButtonGroup.group: signGroup
                onClicked: { form.expense = false; errorLabel.text = "" }
            }
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
