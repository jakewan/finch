import QtQuick
import QtQuick.Controls
import QtQuick.Layouts

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
    property alias amountText: amountInput.amountText
    property alias startDateText: startDateField.text
    property alias endDateText: endDateField.text
    property alias dayOfMonthText: dayOfMonthField.text
    property alias semiDay1Text: semiDay1Field.text
    property alias semiDay2Text: semiDay2Field.text
    property alias frequencyIndex: freqCombo.currentIndex
    property alias transferTargetIndex: destCombo.currentIndex
    property alias errorText: errorLabel.text
    // Amount, its sign, and the cents derivations live in the shared SignedAmountField.
    property alias expense: amountInput.expense
    property alias previewText: amountInput.previewText
    readonly property var signedCents: amountInput.signedCents
    readonly property var unsignedCents: amountInput.unsignedCents
    readonly property bool signSelectorVisible: !isTransfer

    // The destination list: a real "Not a transfer" row at index 0, then every account except
    // the source. The sentinel is a selectable row rather than a placeholder so leaving
    // transfer mode is possible — this form persists across navigation, so a placeholder-only
    // control would make a mis-click unescapable without submitting or restarting the app.
    //
    // The source is absent from the list rather than validated against, so choosing an account
    // as its own transfer target is unrepresentable. Empty while no source is chosen: the form
    // is disabled then anyway, and offering a destination before a source has no meaning.
    readonly property var transferTargets: {
        var rows = [{ id: "", name: "Not a transfer" }]
        if (accountId === "" || !client)
            return rows
        var all = client.accounts || []
        for (var i = 0; i < all.length; i++) {
            if (all[i].id !== accountId)
                rows.push({ id: all[i].id, name: all[i].name })
        }
        return rows
    }

    // Transfer mode is derived from the selected row's *value*, never its index: the model is
    // filtered, so an index into it does not index `client.accounts`. Reading currentValue also
    // means a model swap cannot leave a stale id behind, so correctness here does not depend on
    // a reset winning a race with the model's own re-evaluation.
    readonly property string transferTargetId: destCombo.currentValue ? destCombo.currentValue : ""
    readonly property bool isTransfer: transferTargetId !== ""

    // Holds the author's Expense/Income choice while transfer mode overrides it, so leaving
    // transfer mode restores it instead of silently discarding it.
    property bool priorExpense: true

    onIsTransferChanged: {
        if (isTransfer) {
            priorExpense = amountInput.expense
            // A transfer debits its source, so the preview must read as a debit. Leaving
            // `expense` untouched while hiding its control would show a green credit above a
            // rule that takes money out — the wrong direction, stated at authoring time, with
            // the explaining control removed.
            amountInput.expense = true
        } else {
            amountInput.expense = priorExpense
        }
        errorLabel.text = ""
    }

    // A destination held against the previous source's list is meaningless. Index 0 is always
    // the sentinel, so this lands on "not a transfer" whichever order it and the model
    // re-evaluation run in.
    onAccountIdChanged: destCombo.currentIndex = 0

    // Frequency enum values (proto Frequency): Weekly=1 … Yearly=5. The ComboBox has no
    // sentinel row (currentIndex starts -1 with a placeholder), so a row's index is one less
    // than its value — submit() sends currentValue, never currentIndex.
    readonly property bool frequencyChosen: freqCombo.currentIndex >= 0
    readonly property bool isMonthly: frequencyChosen && freqCombo.currentValue === 4
    readonly property bool isSemiMonthly: frequencyChosen && freqCombo.currentValue === 3

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
        && amountInput.amountValid
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
        // A params object, not positional args: named fields make a transpose impossible, and
        // the key names here are the contract the C++ side unpacks.
        client.createRecurringRule({
            accountId: accountId,
            name: nameField.text.trim(),
            // A transfer's amount is a positive magnitude — direction comes from the
            // source/target pair, and the daemon rejects a non-positive one.
            amount: isTransfer ? amountInput.unsignedCents : amountInput.signedCents,
            frequency: freqCombo.currentValue,
            startDate: startDateField.text,
            endDate: endDateField.text,
            dayOfMonth: dom,
            semiMonthlyDays: semis,
            isTransfer: isTransfer,
            transferTargetAccountId: transferTargetId
        })
    }

    Connections {
        target: form.client
        function onRecurringRuleCreated(id) {
            nameField.text = ""
            amountInput.amountText = ""
            dayOfMonthField.text = ""
            semiDay1Field.text = ""
            semiDay2Field.text = ""
            // Back to "not a transfer", so a follow-up rule does not inherit transfer mode.
            destCombo.currentIndex = 0
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

        // Seated above Amount because it decides what kind of rule this is, and that decision
        // governs whether Amount carries an Expense/Income choice at all. Choosing the kind
        // first keeps the sign toggle from vanishing under the author's cursor. Hidden when the
        // account list offers no candidate destination (a single-account database).
        Label {
            text: "Transfer to"
            visible: form.transferTargets.length > 1
        }
        ComboBox {
            id: destCombo
            Layout.fillWidth: true
            visible: form.transferTargets.length > 1
            model: form.transferTargets
            textRole: "name"
            // valueRole, so the form submits the row's account id. The model is filtered, so an
            // index into it is not an index into client.accounts.
            valueRole: "id"
            currentIndex: 0
            onCurrentIndexChanged: errorLabel.text = ""
        }

        Label { text: "Amount" }
        SignedAmountField {
            id: amountInput
            Layout.fillWidth: true
            // A transfer's direction is implied by its source/target pair, so there is no
            // author-chosen sign to offer.
            signSelectorVisible: !form.isTransfer
            onEdited: errorLabel.text = ""
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
