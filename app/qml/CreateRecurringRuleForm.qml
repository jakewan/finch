import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import "txformat.js" as TxFormat

// Self-contained recurring-rule create form. Like RecordTransactionForm it depends only on an
// injected `client` (the production FinchClient, a mock in tests) and an injected `accountId`
// fed from the Recurring Rules screen's selector, so it carries no second account picker.
// Deliberately importing no Finch C++ type keeps it instantiable by the Qt Quick Test harness.
Item {
    id: form

    // Injected by the host: RecurringRulesPanel passes finchClient; tests pass a mock.
    required property var client

    // Injected target account. Empty string means no account selected — the form stays invalid.
    property string accountId: ""

    // Public, test-facing surface — specs drive the form through these.
    property alias nameText: nameField.text
    property alias amountText: amountField.text
    property alias startDateText: startDateField.text
    property alias endDateText: endDateField.text
    property alias dayOfMonthText: dayOfMonthField.text
    property alias semiDay1Text: semiDay1Field.text
    property alias semiDay2Text: semiDay2Field.text
    property alias frequencyIndex: freqCombo.currentIndex
    property alias errorText: errorLabel.text
    // Expense (true, default) vs Income. The magnitude field cannot carry a sign, so this
    // toggle alone decides whether cents are negative or positive.
    property bool expense: true

    // Frequency enum values (proto Frequency): Weekly=1 … Yearly=5. The ComboBox has no
    // sentinel row (currentIndex starts -1 with a placeholder), so a row's index is one less
    // than its value — submit() sends currentValue, never currentIndex.
    readonly property bool frequencyChosen: freqCombo.currentIndex >= 0
    readonly property bool isMonthly: frequencyChosen && freqCombo.currentValue === 4
    readonly property bool isSemiMonthly: frequencyChosen && freqCombo.currentValue === 3

    // Amount: same treatment as RecordTransactionForm. The regex validator forbids a sign, so
    // the Expense/Income toggle is the sole sign source; parseFloat reads the magnitude cleanly.
    readonly property real magnitude: parseFloat(amountField.text)
    readonly property bool amountValid: !isNaN(magnitude) && magnitude > 0
    // Typed var, not int: the client's amount is a 64-bit qlonglong and a QML int is 32-bit —
    // an int would overflow above ~$21.5M. A JS number marshals to qlonglong exactly (< 2^53).
    readonly property var signedCents: amountValid ? Math.round(magnitude * 100) * (expense ? -1 : 1) : 0
    readonly property string previewText: amountValid ? TxFormat.formatAmount(signedCents) : ""

    // Day-of-month (Monthly) and the two semi-monthly days must each be 1-31 — mirroring core's
    // validation so a request the daemon would reject never leaves the form.
    readonly property int dayOfMonthValue: parseInt(dayOfMonthField.text)
    readonly property bool dayOfMonthValid: dayOfMonthValue >= 1 && dayOfMonthValue <= 31
    readonly property int semiDay1Value: parseInt(semiDay1Field.text)
    readonly property int semiDay2Value: parseInt(semiDay2Field.text)
    readonly property bool semiDaysValid:
        semiDay1Value >= 1 && semiDay1Value <= 31 && semiDay2Value >= 1 && semiDay2Value <= 31
    readonly property bool frequencyDetailsValid:
        isMonthly ? dayOfMonthValid : (isSemiMonthly ? semiDaysValid : true)

    // Dates are ISO "yyyy-MM-dd", so a lexical compare equals a chronological one. An end date
    // before the start date creates a rule that projects nothing (neither daemon nor core
    // validates ordering), so block it. Empty end date = open-ended.
    readonly property bool startDateValid: startDateField.text.trim().length > 0
    readonly property bool endDateOrderValid:
        endDateField.text.length === 0 || endDateField.text >= startDateField.text

    // canSubmit is the single validation predicate the button and the specs share.
    readonly property bool canSubmit:
        accountId !== ""
        && nameField.text.trim().length > 0
        && amountValid
        && frequencyChosen
        && startDateValid
        && endDateOrderValid
        && frequencyDetailsValid
        && !(client && client.createRecurringRuleInProgress)

    // Emitted once the daemon confirms creation; the host re-fetches the account's rules on it.
    signal created(string id)

    implicitWidth: 360
    implicitHeight: layout.implicitHeight

    function submit() {
        if (!canSubmit)
            return
        errorLabel.text = ""
        // Send day fields only for the frequency that uses them, so a stale value left from a
        // frequency switch is never persisted.
        var dom = isMonthly ? dayOfMonthValue : 0
        var semis = isSemiMonthly ? [semiDay1Value, semiDay2Value] : []
        client.createRecurringRule(accountId, nameField.text.trim(), signedCents,
                                   freqCombo.currentValue, startDateField.text,
                                   endDateField.text, dom, semis)
    }

    Connections {
        target: form.client
        function onRecurringRuleCreated(id) {
            nameField.text = ""
            amountField.text = ""
            dayOfMonthField.text = ""
            semiDay1Field.text = ""
            semiDay2Field.text = ""
            errorLabel.text = ""
            form.created(id)
        }
        function onRecurringRuleCreateFailed(message) {
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
            placeholderText: "e.g. Rent"
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
                // Locale-independent, forbids a leading "-" so the toggle is the sole sign
                // source, caps at 2 decimals — same validator as the transaction form.
                validator: RegularExpressionValidator { regularExpression: /^\d+(\.\d{0,2})?$/ }
                onTextChanged: errorLabel.text = ""
            }

            Label {
                text: form.previewText
                color: form.expense ? "#c0392b" : "#27ae60"
                visible: form.previewText.length > 0
            }
        }

        RowLayout {
            spacing: 12
            ButtonGroup { id: signGroup }
            RadioButton {
                text: "Expense"
                ButtonGroup.group: signGroup
                checked: form.expense
                onClicked: { form.expense = true; errorLabel.text = "" }
            }
            RadioButton {
                text: "Income"
                ButtonGroup.group: signGroup
                onClicked: { form.expense = false; errorLabel.text = "" }
            }
        }

        Label { text: "Frequency" }
        ComboBox {
            id: freqCombo
            Layout.fillWidth: true
            // No sentinel row: start unselected with a placeholder, so index != enum value.
            currentIndex: -1
            displayText: currentIndex < 0 ? "Select frequency…" : currentText
            textRole: "label"
            valueRole: "value"
            model: [
                { label: "Weekly",       value: 1 },
                { label: "Biweekly",     value: 2 },
                { label: "Semi-monthly", value: 3 },
                { label: "Monthly",      value: 4 },
                { label: "Yearly",       value: 5 }
            ]
            onCurrentIndexChanged: errorLabel.text = ""
        }

        // Monthly needs a day-of-month; only shown (and required) for Monthly.
        Label {
            text: "Day of month"
            visible: form.isMonthly
        }
        TextField {
            id: dayOfMonthField
            Layout.fillWidth: true
            visible: form.isMonthly
            placeholderText: "1–31"
            inputMethodHints: Qt.ImhDigitsOnly
            validator: IntValidator { bottom: 1; top: 31 }
            onTextChanged: errorLabel.text = ""
        }

        // Semi-monthly needs two days; only shown (and required) for Semi-monthly.
        Label {
            text: "Days of month (two)"
            visible: form.isSemiMonthly
        }
        RowLayout {
            Layout.fillWidth: true
            spacing: 12
            visible: form.isSemiMonthly
            TextField {
                id: semiDay1Field
                Layout.fillWidth: true
                placeholderText: "1–31"
                inputMethodHints: Qt.ImhDigitsOnly
                validator: IntValidator { bottom: 1; top: 31 }
                onTextChanged: errorLabel.text = ""
            }
            TextField {
                id: semiDay2Field
                Layout.fillWidth: true
                placeholderText: "1–31"
                inputMethodHints: Qt.ImhDigitsOnly
                validator: IntValidator { bottom: 1; top: 31 }
                onTextChanged: errorLabel.text = ""
            }
        }

        Label { text: "Start date" }
        TextField {
            id: startDateField
            Layout.fillWidth: true
            text: Qt.formatDate(new Date(), "yyyy-MM-dd")
            onTextChanged: errorLabel.text = ""
        }

        Label { text: "End date (optional)" }
        TextField {
            id: endDateField
            Layout.fillWidth: true
            placeholderText: "yyyy-MM-dd — leave blank for no end"
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
            text: (form.client && form.client.createRecurringRuleInProgress)
                  ? "Creating…" : "Create rule"
            enabled: form.canSubmit
            onClicked: form.submit()
        }
    }
}
