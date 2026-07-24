import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import "txformat.js" as TxFormat

// Shared signed-amount input: a magnitude field whose validator forbids a sign, an
// Expense/Income toggle as the sole sign source, and a live signed-amount preview. Extracted
// from the transaction and recurring-rule forms so the 64-bit-overflow-safe cents handling
// lives in one place. The host reads `signedCents`/`amountValid` and reacts to `edited()`.
Item {
    id: control

    // Public, test-facing surface — hosts and specs drive the control through these.
    property alias amountText: amountField.text
    // Expense (true, default) vs Income. The magnitude field cannot carry a sign, so this
    // toggle alone decides whether cents are negative or positive.
    property bool expense: true
    // Hosts whose value has no user-chosen direction hide the toggle — a recurring transfer's
    // direction comes from its source/target pair, not from the author. Kept as a plain
    // property (not an alias to the row's `visible`) because an effective-visibility read is
    // unreliable offscreen, so this is what a spec asserts.
    property bool signSelectorVisible: true

    // parseFloat is NaN when blank or non-numeric; the regex validator already forbids a sign
    // or a comma decimal, so parseFloat reads the magnitude locale-cleanly.
    readonly property real magnitude: parseFloat(amountField.text)
    // Reject 0 and NaN: a $0 entry is meaningless, and the daemon does not validate amount.
    readonly property bool amountValid: !isNaN(magnitude) && magnitude > 0
    // Math.round avoids float drift (12.34 * 100 -> 1233.9999…).
    // Typed var, not int: cents cross into a 64-bit qlonglong and a QML int is 32-bit — an int
    // would overflow above ~$21.5M. A JS number marshals to qlonglong exactly (< 2^53).
    //
    // The unsigned magnitude is its own property rather than something a host derives by
    // negating signedCents: a host that needs a positive value (a transfer, whose amount is a
    // magnitude the daemon rejects unless positive) would otherwise send a negative one
    // whenever Income was the standing choice. Sign-free by construction.
    readonly property var unsignedCents: amountValid ? Math.round(magnitude * 100) : 0
    readonly property var signedCents: amountValid ? unsignedCents * (expense ? -1 : 1) : 0
    readonly property string previewText: amountValid ? TxFormat.formatAmount(signedCents) : ""

    // Emitted on any edit (magnitude or sign) so the host can clear a stale error message.
    signal edited()

    implicitWidth: layout.implicitWidth
    implicitHeight: layout.implicitHeight

    ColumnLayout {
        id: layout
        anchors.fill: parent
        spacing: 12

        RowLayout {
            Layout.fillWidth: true
            spacing: 12

            TextField {
                id: amountField
                Layout.fillWidth: true
                placeholderText: "0.00"
                inputMethodHints: Qt.ImhFormattedNumbersOnly
                // Locale-independent: always ".", forbids a leading "-" so the Expense/Income
                // toggle is the sole sign source, caps at 2 decimals. A default-locale
                // DoubleValidator would accept "19,99", which parseFloat truncates to 19.
                validator: RegularExpressionValidator { regularExpression: /^\d+(\.\d{0,2})?$/ }
                onTextChanged: control.edited()
            }

            // Live signed-amount preview: the sign is visible before submit.
            Label {
                text: control.previewText
                color: control.expense ? "#c0392b" : "#27ae60"
                visible: control.previewText.length > 0
            }
        }

        // The magnitude field cannot carry a sign, so this toggle is the only way the user
        // expresses direction — a control that can't express the wrong value.
        RowLayout {
            spacing: 12
            visible: control.signSelectorVisible
            ButtonGroup { id: signGroup }
            RadioButton {
                text: "Expense"
                ButtonGroup.group: signGroup
                checked: control.expense
                onClicked: { control.expense = true; control.edited() }
            }
            RadioButton {
                text: "Income"
                ButtonGroup.group: signGroup
                onClicked: { control.expense = false; control.edited() }
            }
        }
    }
}
