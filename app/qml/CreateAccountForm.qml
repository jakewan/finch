import QtQuick
import QtQuick.Controls
import QtQuick.Layouts

// Self-contained account-creation form. It depends only on an injected `client`
// exposing createAccount(name, type), a createAccountInProgress property, and
// accountCreated/accountCreateFailed signals — the production FinchClient in the app,
// a mock in tests. Deliberately importing no Finch C++ type keeps it instantiable by
// the Qt Quick Test harness with a mock client and no daemon.
Item {
    id: form

    // Injected by the host: Main.qml passes finchClient; tests pass a mock.
    required property var client

    // Public, test-facing surface — specs drive the form through these rather than
    // reaching into private control ids.
    property alias nameText: nameField.text
    property alias typeIndex: typeCombo.currentIndex
    readonly property int selectedType: typeCombo.currentValue
    property alias errorText: errorLabel.text

    // canSubmit is the single validation predicate the button and the specs share.
    // selectedType > 0 rejects the UNSPECIFIED sentinel (index 0).
    readonly property bool canSubmit:
        nameField.text.trim().length > 0
        && selectedType > 0
        && !(client && client.createAccountInProgress)

    // Emitted once the daemon confirms creation; the host closes the dialog on it.
    signal created(string id)

    implicitWidth: 360
    implicitHeight: layout.implicitHeight

    function submit() {
        if (!canSubmit)
            return
        errorLabel.text = ""
        client.createAccount(nameField.text.trim(), selectedType)
    }

    Connections {
        target: form.client
        function onAccountCreated(id) {
            nameField.text = ""
            typeCombo.currentIndex = 0
            errorLabel.text = ""
            form.created(id)
        }
        function onAccountCreateFailed(message) {
            errorLabel.text = message
        }
    }

    ColumnLayout {
        id: layout
        anchors.fill: parent
        spacing: 12

        Label { text: "Account name" }
        TextField {
            id: nameField
            Layout.fillWidth: true
            placeholderText: "e.g. Everyday Checking"
            // Drop a stale error the moment the user edits the name.
            onTextChanged: errorLabel.text = ""
        }

        Label { text: "Account type" }
        ComboBox {
            id: typeCombo
            Layout.fillWidth: true
            textRole: "label"
            valueRole: "value"
            // Index 0 is an unselected sentinel (value 0 == UNSPECIFIED) so the form
            // starts invalid until the user picks a real type. Values mirror the proto
            // AccountType enum (Checking=1 … Brokerage=7).
            model: [
                { label: "Select type…",  value: 0 },
                { label: "Checking",       value: 1 },
                { label: "Savings",        value: 2 },
                { label: "Credit Card",    value: 3 },
                { label: "Auto Loan",      value: 4 },
                { label: "Personal Loan",  value: 5 },
                { label: "Line of Credit", value: 6 },
                { label: "Brokerage",      value: 7 }
            ]
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
            text: (form.client && form.client.createAccountInProgress)
                  ? "Creating…" : "Create account"
            enabled: form.canSubmit
            onClicked: form.submit()
        }
    }
}
